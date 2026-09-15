package model

import (
	"context"

	"github.com/go-sdk/app"
	"github.com/go-sdk/database/dbx"
)

type Permission struct {
	dbx.Metadata
	Code string `gorm:"type:varchar(128);not null;uniqueIndex"`
	Name string `gorm:"type:varchar(128);not null"`
}

func CreatePermission(ctx context.Context, value *Permission) error {
	return app.DB().WithContext(ctx).Create(value).Error
}

func GetPermission(ctx context.Context, id string) (Permission, error) {
	var value Permission
	err := app.DB().WithContext(ctx).First(&value, "id = ?", id).Error
	return value, err
}

func ListPermissions(ctx context.Context, offset, limit int) ([]Permission, int64, error) {
	query := app.DB().WithContext(ctx).Model(&Permission{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var values []Permission
	if err := query.Order("created_at DESC").Offset(offset).Limit(limit).Find(&values).Error; err != nil {
		return nil, 0, err
	}
	return values, total, nil
}

func UpdatePermission(ctx context.Context, id string, updates map[string]any) error {
	result := app.DB().WithContext(ctx).Model(&Permission{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return dbx.ErrRecordNotFound
	}
	return nil
}

func DeletePermission(ctx context.Context, value *Permission) error {
	result := app.DB().WithContext(ctx).Delete(value)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return dbx.ErrRecordNotFound
	}
	return nil
}

func FindPermissions(ctx context.Context, ids []string) ([]Permission, error) {
	if len(ids) == 0 {
		return []Permission{}, nil
	}
	var values []Permission
	err := app.DB().WithContext(ctx).Where("id IN ?", ids).Find(&values).Error
	return values, err
}

func HasPermission(ctx context.Context, userId, permission string) (bool, error) {
	var count int64
	err := app.DB().WithContext(ctx).Table("permissions AS p").
		Joins("JOIN role_permissions AS rp ON rp.permission_id = p.id").
		Joins("JOIN roles AS r ON r.id = rp.role_id").
		Joins("JOIN user_roles AS ur ON ur.role_id = rp.role_id").
		Joins("JOIN users AS u ON u.id = ur.user_id").
		Where("u.id = ? AND u.enabled = ? AND u.deleted_at = 0 AND r.deleted_at = 0 AND p.deleted_at = 0 AND p.code = ?", userId, true, permission).
		Count(&count).Error
	return count > 0, err
}

// EnsurePermission 按编码复用或创建权限，已存在时保留原有记录。
func EnsurePermission(tx *dbx.DB, item Permission) (Permission, error) {
	var value Permission
	err := tx.Where("code = ?", item.Code).First(&value).Error
	if err == nil {
		return value, nil
	}
	if !dbx.IsRecordNotFound(err) {
		return Permission{}, err
	}
	result := tx.Clauses(dbx.OnConflict{DoNothing: true}).Create(&item)
	if result.Error != nil {
		return Permission{}, result.Error
	}
	if result.RowsAffected > 0 {
		return item, nil
	}
	err = tx.Where("code = ?", item.Code).First(&value).Error
	return value, err
}
