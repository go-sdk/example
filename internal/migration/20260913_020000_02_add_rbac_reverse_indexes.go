package migration

import (
	"gorm.io/gorm"

	"github.com/go-sdk/example/internal/model"
)

// rbacReverseIndexes 为权限检查使用的反向关联查询提供索引。
var rbacReverseIndexes = []struct {
	value any
	name  string
}{
	{&model.UserRole{}, "idx_user_roles_role_id"},
	{&model.RolePermission{}, "idx_role_permissions_permission_id"},
}

func init() {
	migrations.Add(
		func(tx *gorm.DB) error {
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
		func(tx *gorm.DB) error {
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
	)
}
