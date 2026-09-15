package model

import (
	"context"

	"github.com/go-sdk/app"
	"github.com/go-sdk/database/dbx"
)

type File struct {
	dbx.Metadata
	OwnerId      string `gorm:"type:varchar(32);not null;index"`
	OriginalName string `gorm:"type:varchar(255);not null"`
	StoredName   string `gorm:"type:varchar(255);not null;uniqueIndex"`
	MIMEType     string `gorm:"type:varchar(255);not null"`
	Size         int64  `gorm:"not null"`
}

func CreateFile(ctx context.Context, value *File) error {
	return app.DB().WithContext(ctx).Create(value).Error
}

func GetFile(ctx context.Context, id string) (File, error) {
	var value File
	err := app.DB().WithContext(ctx).First(&value, "id = ?", id).Error
	return value, err
}

func DeleteFile(ctx context.Context, value *File) error {
	return app.DB().WithContext(ctx).Delete(value).Error
}

func ReplaceAvatar(ctx context.Context, user *User, value *File, actor string) (File, error) {
	var oldFile File
	err := app.DB().WithContext(ctx).Transaction(func(tx *dbx.DB) error {
		if err := tx.Create(value).Error; err != nil {
			return err
		}
		if user.AvatarFileId != nil {
			if err := tx.First(&oldFile, "id = ?", *user.AvatarFileId).Error; err != nil && !dbx.IsRecordNotFound(err) {
				return err
			}
		}
		return tx.Model(user).Updates(map[string]any{
			"avatar_file_id": value.Id,
			"updated_by":     actor,
		}).Error
	})
	return oldFile, err
}
