package model

import (
	"context"

	"github.com/go-sdk/app"
	"github.com/go-sdk/database/dbx"
)

type User struct {
	dbx.Metadata
	Username     string  `gorm:"type:varchar(64);not null;uniqueIndex"`
	Email        string  `gorm:"type:varchar(255);not null;uniqueIndex"`
	PasswordHash string  `gorm:"type:varchar(255);not null"`
	Enabled      bool    `gorm:"not null"`
	AvatarFileID *string `gorm:"type:varchar(32);index"`
	Roles        []Role  `gorm:"many2many:user_roles"`
}

func CreateUser(ctx context.Context, value *User) error {
	return app.DB().WithContext(ctx).Create(value).Error
}

func GetUser(ctx context.Context, id string) (User, error) {
	var value User
	err := app.DB().WithContext(ctx).Preload("Roles").First(&value, "id = ?", id).Error
	return value, err
}

func GetUserByUsername(ctx context.Context, username string) (User, error) {
	var value User
	err := app.DB().WithContext(ctx).Where("username = ?", username).First(&value).Error
	return value, err
}

func ListUsers(ctx context.Context, offset, limit int) ([]User, int64, error) {
	query := app.DB().WithContext(ctx).Model(&User{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []User
	if err := query.Preload("Roles").Order("created_at DESC").Offset(offset).Limit(limit).Find(&values).Error; err != nil {
		return nil, 0, err
	}
	return values, total, nil
}

func UpdateUser(ctx context.Context, id string, updates map[string]any) error {
	result := app.DB().WithContext(ctx).Model(&User{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return dbx.ErrRecordNotFound
	}
	return nil
}

func DeleteUser(ctx context.Context, id string) (File, error) {
	var avatar File
	err := app.DB().WithContext(ctx).Transaction(func(tx *dbx.DB) error {
		var user User
		if err := tx.First(&user, "id = ?", id).Error; err != nil {
			return err
		}
		if user.AvatarFileID != nil {
			if err := tx.First(&avatar, "id = ?", *user.AvatarFileID).Error; err != nil && !dbx.IsRecordNotFound(err) {
				return err
			}
		}
		if err := tx.Delete(&user).Error; err != nil {
			return err
		}
		if avatar.Id != "" {
			return tx.Delete(&avatar).Error
		}
		return nil
	})
	return avatar, err
}

func SetUserRoles(ctx context.Context, id string, roleIDs []string) (bool, error) {
	user, err := GetUser(ctx, id)
	if err != nil {
		return false, err
	}
	ids := uniqueIDs(roleIDs)
	roles, err := FindRoles(ctx, ids)
	if err != nil {
		return false, err
	}
	if len(roles) != len(ids) {
		return false, nil
	}
	if err = app.DB().WithContext(ctx).Model(&user).Association("Roles").Replace(roles); err != nil {
		return false, err
	}
	return true, nil
}

// EnsureUser 按用户名复用用户，仅在记录不存在时调用 create 构造新用户。
func EnsureUser(tx *dbx.DB, username string, create func() (User, error)) (User, error) {
	var value User
	err := tx.Where("username = ?", username).First(&value).Error
	if err == nil {
		return value, nil
	}
	if !dbx.IsRecordNotFound(err) {
		return User{}, err
	}
	item, err := create()
	if err != nil {
		return User{}, err
	}
	result := tx.Clauses(dbx.OnConflict{DoNothing: true}).Create(&item)
	if result.Error != nil {
		return User{}, result.Error
	}
	if result.RowsAffected > 0 {
		return item, nil
	}
	err = tx.Where("username = ?", username).First(&value).Error
	return value, err
}
