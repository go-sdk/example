package model

import (
	"context"

	"github.com/go-sdk/app"
	"github.com/go-sdk/database/dbx"
)

type Role struct {
	dbx.Metadata
	Code        string       `gorm:"type:varchar(64);not null;uniqueIndex"`
	Name        string       `gorm:"type:varchar(128);not null"`
	Permissions []Permission `gorm:"many2many:role_permissions"`
}

func CreateRole(ctx context.Context, value *Role) error {
	return app.DB().WithContext(ctx).Create(value).Error
}

func GetRole(ctx context.Context, id string) (Role, error) {
	var value Role
	err := app.DB().WithContext(ctx).Preload("Permissions").First(&value, "id = ?", id).Error
	return value, err
}

func ListRoles(ctx context.Context, offset, limit int) ([]Role, int64, error) {
	query := app.DB().WithContext(ctx).Model(&Role{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []Role
	if err := query.Preload("Permissions").Order("created_at DESC").Offset(offset).Limit(limit).Find(&values).Error; err != nil {
		return nil, 0, err
	}
	return values, total, nil
}

func UpdateRole(ctx context.Context, id string, updates map[string]any) error {
	result := app.DB().WithContext(ctx).Model(&Role{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return dbx.ErrRecordNotFound
	}
	return nil
}

func DeleteRole(ctx context.Context, value *Role) error {
	result := app.DB().WithContext(ctx).Delete(value)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return dbx.ErrRecordNotFound
	}
	return nil
}

func SetRolePermissions(ctx context.Context, id string, permissionIDs []string) (bool, error) {
	role, err := GetRole(ctx, id)
	if err != nil {
		return false, err
	}
	ids := uniqueIDs(permissionIDs)
	permissions, err := FindPermissions(ctx, ids)
	if err != nil {
		return false, err
	}
	if len(permissions) != len(ids) {
		return false, nil
	}
	if err = app.DB().WithContext(ctx).Model(&role).Association("Permissions").Replace(permissions); err != nil {
		return false, err
	}
	return true, nil
}

func FindRoles(ctx context.Context, ids []string) ([]Role, error) {
	if len(ids) == 0 {
		return []Role{}, nil
	}
	var values []Role
	err := app.DB().WithContext(ctx).Where("id IN ?", ids).Find(&values).Error
	return values, err
}

// EnsureRole 按编码复用或创建角色，已存在时保留原有记录。
func EnsureRole(tx *dbx.DB, item Role) (Role, error) {
	var value Role
	err := tx.Where("code = ?", item.Code).First(&value).Error
	if err == nil {
		return value, nil
	}
	if !dbx.IsRecordNotFound(err) {
		return Role{}, err
	}
	result := tx.Clauses(dbx.OnConflict{DoNothing: true}).Create(&item)
	if result.Error != nil {
		return Role{}, result.Error
	}
	if result.RowsAffected > 0 {
		return item, nil
	}
	err = tx.Where("code = ?", item.Code).First(&value).Error
	return value, err
}
