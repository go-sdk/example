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

// rbacReverseIndexes 权限检查按 role_id、permission_id 反向连接关联表，需为两个关联表补充反向索引。
var rbacReverseIndexes = []struct {
	value any
	name  string
}{
	{&model.UserRole{}, "idx_user_roles_role_id"},
	{&model.RolePermission{}, "idx_role_permissions_permission_id"},
}

func New(db *gorm.DB) (*migrate.Migrator, error) {
	return migrate.New(db, migrate.Migrations{
		{
			ID: "20260913_010000_01_create_rbac_tables",
			Up: func(tx *gorm.DB) error {
				return tx.AutoMigrate(
					&model.User{}, &model.Role{}, &model.Permission{},
					&model.UserRole{}, &model.RolePermission{}, &model.File{},
				)
			},
			Down: func(tx *gorm.DB) error {
				return tx.Migrator().DropTable(
					&model.RolePermission{}, &model.UserRole{}, &model.File{},
					&model.Permission{}, &model.Role{}, &model.User{},
				)
			},
		},
		{
			ID: "20260913_020000_02_add_rbac_reverse_indexes",
			Up: func(tx *gorm.DB) error {
				migrator := tx.Migrator()
				for _, index := range rbacReverseIndexes {
					if migrator.HasIndex(index.value, index.name) {
						continue
					}
					if err := migrator.CreateIndex(index.value, index.name); err != nil {
						return err
					}
				}
				return nil
			},
			Down: func(tx *gorm.DB) error {
				migrator := tx.Migrator()
				for _, index := range rbacReverseIndexes {
					if !migrator.HasIndex(index.value, index.name) {
						continue
					}
					if err := migrator.DropIndex(index.value, index.name); err != nil {
						return err
					}
				}
				return nil
			},
		},
	})
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
		if err := syncRolePermissions(tx, role, permissions); err != nil {
			return errx.Wrap(err, "assign administrator permissions")
		}

		user, created, err := ensureAdminUser(tx, cfg)
		if err != nil {
			return errx.Wrap(err, "create bootstrap administrator")
		}
		if !created {
			return nil
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

// syncRolePermissions 以显式关联表写入同步角色权限；并发副本重复插入时忽略冲突，避免 Association.Replace 报错。
func syncRolePermissions(tx *gorm.DB, role model.Role, permissions []model.Permission) error {
	if len(permissions) == 0 {
		return nil
	}
	permissionIDs := make([]string, 0, len(permissions))
	relations := make([]model.RolePermission, 0, len(permissions))
	for _, permission := range permissions {
		permissionIDs = append(permissionIDs, permission.Id)
		relations = append(relations, model.RolePermission{RoleID: role.Id, PermissionID: permission.Id})
	}
	if err := tx.Where("role_id = ? AND permission_id NOT IN ?", role.Id, permissionIDs).
		Delete(&model.RolePermission{}).Error; err != nil {
		return err
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
