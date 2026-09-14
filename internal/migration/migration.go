package migration

import (
	"context"
	"errors"

	"github.com/go-sdk/core/errx"
	"github.com/go-sdk/core/seq"
	"github.com/go-sdk/database/dbx"
	"github.com/go-sdk/database/dbx/migrate"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/go-sdk/example/internal/config"
	"github.com/go-sdk/example/internal/model"
)

var migrations migrate.Migrations

var builtinPermissions = []model.Permission{
	{Metadata: dbx.Metadata{Id: "permission_users_read"}, Code: "users.read", Name: "查看用户"},
	{Metadata: dbx.Metadata{Id: "permission_users_write"}, Code: "users.write", Name: "管理用户"},
	{Metadata: dbx.Metadata{Id: "permission_roles_read"}, Code: "roles.read", Name: "查看角色"},
	{Metadata: dbx.Metadata{Id: "permission_roles_write"}, Code: "roles.write", Name: "管理角色"},
	{Metadata: dbx.Metadata{Id: "permission_permissions_read"}, Code: "permissions.read", Name: "查看权限"},
	{Metadata: dbx.Metadata{Id: "permission_permissions_write"}, Code: "permissions.write", Name: "管理权限"},
	{Metadata: dbx.Metadata{Id: "permission_files_read"}, Code: "files.read", Name: "读取文件"},
	{Metadata: dbx.Metadata{Id: "permission_files_write"}, Code: "files.write", Name: "上传文件"},
}

func New(db *gorm.DB) (*migrate.Migrator, error) {
	return migrate.New(db, migrations)
}

func Bootstrap(ctx context.Context, db *gorm.DB, cfg config.Config) error {
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		permissions := make([]model.Permission, 0, len(builtinPermissions))
		for _, item := range builtinPermissions {
			permission, err := ensurePermission(tx, item)
			if err != nil {
				return errx.Wrap(err, "seed permission")
			}
			permissions = append(permissions, permission)
		}

		role, err := ensureAdminRole(tx)
		if err != nil {
			return errx.Wrap(err, "seed administrator role")
		}
		if err := ensureRolePermissions(tx, role, permissions); err != nil {
			return errx.Wrap(err, "assign administrator permissions")
		}

		user, _, err := ensureAdminUser(tx, cfg)
		if err != nil {
			return errx.Wrap(err, "create bootstrap administrator")
		}
		link := model.UserRole{UserID: user.Id, RoleID: role.Id}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error; err != nil {
			return errx.Wrap(err, "bind administrator role")
		}
		return nil
	})
}

// ensurePermission 按 code 复用或创建内置权限；主键仅在创建时生成，多副本并发启动时仅一个副本写入成功。
func ensurePermission(tx *gorm.DB, item model.Permission) (model.Permission, error) {
	var permission model.Permission
	err := tx.Where("code = ?", item.Code).First(&permission).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Permission{}, err
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&item)
		if result.Error != nil {
			return model.Permission{}, result.Error
		}
		if result.RowsAffected > 0 {
			return item, nil
		}
		// 并发副本已创建同 code 记录，读取其结果。
		if err := tx.Where("code = ?", item.Code).First(&permission).Error; err != nil {
			return model.Permission{}, err
		}
	}
	return permission, nil
}

// ensureAdminRole 按 code 复用或创建管理员角色，已存在时保留原有记录。
func ensureAdminRole(tx *gorm.DB) (model.Role, error) {
	var role model.Role
	err := tx.Where("code = ?", "admin").First(&role).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return model.Role{}, err
		}
		role = model.Role{Metadata: dbx.Metadata{Id: seq.NextID()}, Code: "admin", Name: "管理员"}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&role)
		if result.Error != nil {
			return model.Role{}, result.Error
		}
		if result.RowsAffected == 0 {
			// 并发副本已创建 admin 角色，读取其结果。
			if err := tx.Where("code = ?", "admin").First(&role).Error; err != nil {
				return model.Role{}, err
			}
		}
	}
	return role, nil
}

// ensureRolePermissions 只补齐内置权限，保留管理员角色后来绑定的其他权限。
func ensureRolePermissions(tx *gorm.DB, role model.Role, permissions []model.Permission) error {
	if len(permissions) == 0 {
		return nil
	}
	relations := make([]model.RolePermission, 0, len(permissions))
	for _, permission := range permissions {
		relations = append(relations, model.RolePermission{RoleID: role.Id, PermissionID: permission.Id})
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&relations).Error
}

// ensureAdminUser 查询或创建首次管理员；已存在时返回 false 且不覆盖密码和其他资料。
func ensureAdminUser(tx *gorm.DB, cfg config.Config) (model.User, bool, error) {
	var user model.User
	err := tx.Where("username = ?", cfg.Bootstrap.Username).First(&user).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return model.User{}, false, err
		}
		hash, hashErr := bcrypt.GenerateFromPassword([]byte(cfg.Bootstrap.Password), bcrypt.DefaultCost)
		if hashErr != nil {
			return model.User{}, false, errx.Wrap(hashErr, "hash bootstrap administrator password")
		}
		user = model.User{
			Id: seq.NextID(), Username: cfg.Bootstrap.Username,
			Email: cfg.Bootstrap.Email, PasswordHash: string(hash), Enabled: true,
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&user)
		if result.Error != nil {
			return model.User{}, false, result.Error
		}
		if result.RowsAffected == 0 {
			// 并发副本已创建同名管理员，读取其结果且不覆盖密码。
			if err := tx.Where("username = ?", cfg.Bootstrap.Username).First(&user).Error; err != nil {
				return model.User{}, false, err
			}
			return user, false, nil
		}
		return user, true, nil
	}
	return user, false, nil
}
