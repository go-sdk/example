package model

import "github.com/go-sdk/database/dbx"

type RolePermission struct {
	RoleID       string `gorm:"type:varchar(32);primaryKey"`
	PermissionID string `gorm:"type:varchar(32);primaryKey;index:idx_role_permissions_permission_id"`
}

func EnsureRolePermissions(tx *dbx.DB, role Role, permissions []Permission) error {
	if len(permissions) == 0 {
		return nil
	}
	relations := make([]RolePermission, 0, len(permissions))
	for _, permission := range permissions {
		relations = append(relations, RolePermission{RoleID: role.Id, PermissionID: permission.Id})
	}
	return tx.Clauses(dbx.OnConflict{DoNothing: true}).Create(&relations).Error
}
