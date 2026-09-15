package model

import "github.com/go-sdk/database/dbx"

type UserRole struct {
	UserId string `gorm:"type:varchar(32);primaryKey"`
	RoleId string `gorm:"type:varchar(32);primaryKey;index:idx_user_roles_role_id"`
}

func EnsureUserRole(tx *dbx.DB, userId, roleId string) error {
	return tx.Clauses(dbx.OnConflict{DoNothing: true}).Create(&UserRole{UserId: userId, RoleId: roleId}).Error
}
