package migration

import (
	"context"

	"github.com/go-sdk/app"
	"github.com/go-sdk/core/errx"
	"github.com/go-sdk/core/seq"
	"github.com/go-sdk/database/dbx"
	"golang.org/x/crypto/bcrypt"

	appconfig "github.com/go-sdk/example/internal/config"
	"github.com/go-sdk/example/internal/model"
)

var builtinPermissions = []model.Permission{
	{
		Id:   "permission_users_read",
		Code: "users.read",
		Name: "查看用户",
	},
	{
		Id:   "permission_users_write",
		Code: "users.write",
		Name: "管理用户",
	},
	{
		Id:   "permission_roles_read",
		Code: "roles.read",
		Name: "查看角色",
	},
	{
		Id:   "permission_roles_write",
		Code: "roles.write",
		Name: "管理角色",
	},
	{
		Id:   "permission_permissions_read",
		Code: "permissions.read",
		Name: "查看权限",
	},
	{
		Id:   "permission_permissions_write",
		Code: "permissions.write",
		Name: "管理权限",
	},
	{
		Id:   "permission_files_read",
		Code: "files.read",
		Name: "读取文件",
	},
	{
		Id:   "permission_files_write",
		Code: "files.write",
		Name: "上传文件",
	},
}

func init() {
	app.RegisterBootstrap(bootstrap)
}

func bootstrap(ctx context.Context, tx *dbx.DB) error {
	return tx.WithContext(ctx).Transaction(func(tx *dbx.DB) error {
		permissions := make([]model.Permission, 0, len(builtinPermissions))
		for _, item := range builtinPermissions {
			permission, err := model.EnsurePermission(tx, item)
			if err != nil {
				return errx.Wrap(err, "seed permission")
			}
			permissions = append(permissions, permission)
		}

		role, err := model.EnsureRole(tx, model.Role{
			Id:   seq.NextID(),
			Code: "admin",
			Name: "管理员",
		})
		if err != nil {
			return errx.Wrap(err, "seed administrator role")
		}
		if err = model.EnsureRolePermissions(tx, role, permissions); err != nil {
			return errx.Wrap(err, "assign administrator permissions")
		}

		config := appconfig.G()
		username := config.Bootstrap.Username
		user, err := model.EnsureUser(tx, username, func() (model.User, error) {
			hash, hashErr := bcrypt.GenerateFromPassword([]byte(config.Bootstrap.Password), bcrypt.DefaultCost)
			if hashErr != nil {
				return model.User{}, errx.Wrap(hashErr, "hash bootstrap administrator password")
			}
			return model.User{
				Id:           seq.NextID(),
				Username:     username,
				Email:        config.Bootstrap.Email,
				PasswordHash: string(hash),
				Enabled:      true,
			}, nil
		})
		if err != nil {
			return errx.Wrap(err, "create bootstrap administrator")
		}
		if err = model.EnsureUserRole(tx, user.Id, role.Id); err != nil {
			return errx.Wrap(err, "bind administrator role")
		}
		return nil
	})
}
