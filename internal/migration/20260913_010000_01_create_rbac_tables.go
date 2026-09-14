package migration

import (
	"gorm.io/gorm"

	"github.com/go-sdk/example/internal/model"
)

func init() {
	migrations.Add(
		func(tx *gorm.DB) error {
			return tx.AutoMigrate(
				&model.User{}, &model.Role{}, &model.Permission{},
				&model.UserRole{}, &model.RolePermission{}, &model.File{},
			)
		},
		func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(
				&model.RolePermission{}, &model.UserRole{}, &model.File{},
				&model.Permission{}, &model.Role{}, &model.User{},
			)
		},
	)
}
