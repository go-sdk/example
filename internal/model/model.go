package model

import "github.com/go-sdk/database/dbx"

type User struct {
	dbx.Metadata
	Username     string  `gorm:"type:varchar(64);not null;uniqueIndex"`
	Email        string  `gorm:"type:varchar(255);not null;uniqueIndex"`
	PasswordHash string  `gorm:"type:varchar(255);not null"`
	Enabled      bool    `gorm:"not null"`
	AvatarFileID *string `gorm:"type:varchar(32);index"`
	Roles        []Role  `gorm:"many2many:user_roles"`
}

type Role struct {
	dbx.Metadata
	Code        string       `gorm:"type:varchar(64);not null;uniqueIndex"`
	Name        string       `gorm:"type:varchar(128);not null"`
	Permissions []Permission `gorm:"many2many:role_permissions"`
}

type Permission struct {
	dbx.Metadata
	Code string `gorm:"type:varchar(128);not null;uniqueIndex"`
	Name string `gorm:"type:varchar(128);not null"`
}

type UserRole struct {
	UserID string `gorm:"type:varchar(32);primaryKey"`
	RoleID string `gorm:"type:varchar(32);primaryKey;index:idx_user_roles_role_id"`
}

type RolePermission struct {
	RoleID       string `gorm:"type:varchar(32);primaryKey"`
	PermissionID string `gorm:"type:varchar(32);primaryKey;index:idx_role_permissions_permission_id"`
}

type File struct {
	dbx.Metadata
	OwnerID      string `gorm:"type:varchar(32);not null;index"`
	OriginalName string `gorm:"type:varchar(255);not null"`
	StoredName   string `gorm:"type:varchar(255);not null;uniqueIndex"`
	MIMEType     string `gorm:"type:varchar(255);not null"`
	Size         int64  `gorm:"not null"`
}
