package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/go-sdk/core/errx"
	"github.com/go-sdk/core/logx"
	"github.com/go-sdk/core/seq"
	servercommon "github.com/go-sdk/server/common"
	"github.com/go-sdk/server/standard"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	appv1 "github.com/go-sdk/example/gen/app/v1"
	commonv1 "github.com/go-sdk/example/gen/common/v1"
	appauth "github.com/go-sdk/example/internal/auth"
	"github.com/go-sdk/example/internal/model"
)

type User struct {
	appv1.UnimplementedUserServiceServer
	db          *gorm.DB
	storageRoot string
}

func NewUser(db *gorm.DB, storageRoot string) *User {
	return &User{db: db, storageRoot: storageRoot}
}

func (s *User) Create(ctx context.Context, req *appv1.CreateUserReq) (*appv1.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.GetPassword()), bcrypt.DefaultCost)
	if err != nil {
		return nil, errx.Wrap(err, "hash password")
	}
	actor := appauth.Subject(ctx)
	value := model.User{
		Id: seq.NextID(), CreatedBy: actor, UpdatedBy: actor,
		Username: req.GetUsername(), Email: req.GetEmail(), PasswordHash: string(hash), Enabled: req.GetEnabled(),
	}
	if err := s.db.WithContext(ctx).Create(&value).Error; err != nil {
		return nil, err
	}
	return userToProto(value), nil
}

func (s *User) Get(ctx context.Context, req *appv1.GetUserReq) (*appv1.User, error) {
	value, err := s.get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return userToProto(value), nil
}

func (s *User) List(ctx context.Context, req *appv1.ListUserReq) (*appv1.ListUserResp, error) {
	paging := req.GetPaging()
	offset, limit := paging.GetOffsetLimit()
	query := s.db.WithContext(ctx).Model(&model.User{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var values []model.User
	if err := query.Preload("Roles").Order("created_at DESC").Offset(offset).Limit(limit).Find(&values).Error; err != nil {
		return nil, err
	}
	records := make([]*appv1.User, 0, len(values))
	for _, value := range values {
		records = append(records, userToProto(value))
	}
	return &appv1.ListUserResp{Records: records, Paging: paging.WithTotal(total)}, nil
}

func (s *User) Update(ctx context.Context, req *appv1.UpdateUserReq) (*appv1.User, error) {
	updates := map[string]any{"email": req.GetEmail(), "enabled": req.GetEnabled(), "updated_by": appauth.Subject(ctx)}
	result := s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", req.GetId()).Updates(updates)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	value, err := s.get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return userToProto(value), nil
}

func (s *User) Delete(ctx context.Context, req *appv1.DeleteUserReq) (*servercommon.Empty, error) {
	if req.GetId() == appauth.Subject(ctx) {
		return nil, standard.ErrFailedPrecondition.
			WithErrorCode(commonv1.ErrorCode_ERROR_CODE_CURRENT_USER_CANNOT_BE_DELETED)
	}
	var avatar model.File
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.First(&user, "id = ?", req.GetId()).Error; err != nil {
			return err
		}
		if user.AvatarFileID != nil {
			if err := tx.First(&avatar, "id = ?", *user.AvatarFileID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
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
	if err != nil {
		return nil, err
	}
	if avatar.Id != "" {
		path := filepath.Join(s.storageRoot, avatar.StoredName)
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			logx.Ctx(ctx).Warn().Err(err).Str("file_id", avatar.Id).Msg("delete user avatar file")
		}
	}
	return &servercommon.Empty{}, nil
}

func (s *User) SetRoles(ctx context.Context, req *appv1.SetUserRolesReq) (*servercommon.Empty, error) {
	value, err := s.get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	ids := unique(req.GetRoleIds())
	var roles []model.Role
	if len(ids) > 0 {
		if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&roles).Error; err != nil {
			return nil, err
		}
		if len(roles) != len(ids) {
			return nil, standard.ErrInvalidParam.
				WithErrorCode(commonv1.ErrorCode_ERROR_CODE_INVALID_ROLE_IDS)
		}
	}
	if err := s.db.WithContext(ctx).Model(&value).Association("Roles").Replace(roles); err != nil {
		return nil, err
	}
	return &servercommon.Empty{}, nil
}

func (s *User) get(ctx context.Context, id string) (model.User, error) {
	var value model.User
	err := s.db.WithContext(ctx).Preload("Roles").First(&value, "id = ?", id).Error
	return value, err
}

func unique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; !ok {
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	return result
}
