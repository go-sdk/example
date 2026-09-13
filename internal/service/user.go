package service

import (
	"context"

	"github.com/go-sdk/core/errx"
	"github.com/go-sdk/core/seq"
	"github.com/go-sdk/server/standard"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"

	appv1 "github.com/go-sdk/example/gen/app/v1"
	commonv1 "github.com/go-sdk/example/gen/common/v1"
	appauth "github.com/go-sdk/example/internal/auth"
	"github.com/go-sdk/example/internal/model"
)

type User struct {
	appv1.UnimplementedUserServiceServer
	db *gorm.DB
}

func NewUser(db *gorm.DB) *User { return &User{db: db} }

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
	var pageReq *commonv1.PagingReq
	if req != nil {
		pageReq = req.GetPaging()
	}
	var page, pageSize int32
	if pageReq != nil {
		page, pageSize = pageReq.GetPage(), pageReq.GetPageSize()
	}
	page, pageSize, offset := paging(page, pageSize)
	query := s.db.WithContext(ctx).Model(&model.User{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var values []model.User
	if err := query.Preload("Roles").Order("created_at DESC").Offset(offset).Limit(int(pageSize)).Find(&values).Error; err != nil {
		return nil, err
	}
	records := make([]*appv1.User, 0, len(values))
	for _, value := range values {
		records = append(records, userToProto(value))
	}
	return &appv1.ListUserResp{Records: records, Paging: &commonv1.Paging{Page: page, PageSize: pageSize, Total: total}}, nil
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

func (s *User) Delete(ctx context.Context, req *appv1.DeleteUserReq) (*commonv1.Empty, error) {
	if req.GetId() == appauth.Subject(ctx) {
		return nil, standard.NewError(codes.FailedPrecondition, "current user cannot be deleted")
	}
	result := s.db.WithContext(ctx).Where("id = ?", req.GetId()).Delete(&model.User{})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &commonv1.Empty{}, nil
}

func (s *User) SetRoles(ctx context.Context, req *appv1.SetUserRolesReq) (*commonv1.Empty, error) {
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
			return nil, standard.NewError(codes.InvalidArgument, "one or more roles do not exist").
				WithDomainReason("INVALID_ROLE_IDS", "user.invalid_role_ids")
		}
	}
	if err := s.db.WithContext(ctx).Model(&value).Association("Roles").Replace(roles); err != nil {
		return nil, err
	}
	return &commonv1.Empty{}, nil
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
