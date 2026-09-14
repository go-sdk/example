package migration

import (
	"github.com/go-sdk/app"
	"github.com/go-sdk/database/dbx"

	"github.com/go-sdk/example/internal/model"
)

func init() {
	app.RegisterMigration(
		func(tx *dbx.DB) error {
			return tx.AutoMigrate(
				&model.User{}, &model.Role{}, &model.Permission{},
				&model.UserRole{}, &model.RolePermission{}, &model.File{},
			)
		},
		func(tx *dbx.DB) error {
			return tx.Migrator().DropTable(
				&model.RolePermission{}, &model.UserRole{}, &model.File{},
				&model.Permission{}, &model.Role{}, &model.User{},
			)
		},
	)
}
