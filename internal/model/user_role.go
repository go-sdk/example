package model

import "github.com/go-sdk/database/dbx"

type UserRole struct {
	UserID string `gorm:"type:varchar(32);primaryKey"`
	RoleID string `gorm:"type:varchar(32);primaryKey;index:idx_user_roles_role_id"`
}

func EnsureUserRole(tx *dbx.DB, userID, roleID string) error {
	return tx.Clauses(dbx.OnConflict{DoNothing: true}).Create(&UserRole{UserID: userID, RoleID: roleID}).Error
}

func uniqueIDs(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
