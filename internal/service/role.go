package service

import (
	"context"

	"github.com/go-sdk/core/seq"
	"github.com/go-sdk/database/dbx"
	"github.com/go-sdk/server/standard"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"

	appv1 "github.com/go-sdk/example/gen/app/v1"
	commonv1 "github.com/go-sdk/example/gen/common/v1"
	appauth "github.com/go-sdk/example/internal/auth"
	"github.com/go-sdk/example/internal/model"
)

type Role struct {
	appv1.UnimplementedRoleServiceServer
	db *gorm.DB
}

func NewRole(db *gorm.DB) *Role { return &Role{db: db} }

func (s *Role) Create(ctx context.Context, req *appv1.CreateRoleReq) (*appv1.Role, error) {
	actor := appauth.Subject(ctx)
	value := model.Role{
		Metadata: dbx.Metadata{Id: seq.NextID(), CreatedBy: actor, UpdatedBy: actor},
		Code:     req.GetCode(), Name: req.GetName(),
	}
	if err := s.db.WithContext(ctx).Create(&value).Error; err != nil {
		return nil, err
	}
	return roleToProto(value), nil
}

func (s *Role) Get(ctx context.Context, req *appv1.GetRoleReq) (*appv1.Role, error) {
	value, err := s.get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return roleToProto(value), nil
}

func (s *Role) List(ctx context.Context, req *appv1.ListRoleReq) (*appv1.ListRoleResp, error) {
	page, pageSize := int32(0), int32(0)
	if req.GetPaging() != nil {
		page, pageSize = req.GetPaging().GetPage(), req.GetPaging().GetPageSize()
	}
	page, pageSize, offset := paging(page, pageSize)
	query := s.db.WithContext(ctx).Model(&model.Role{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var values []model.Role
	if err := query.Preload("Permissions").Order("created_at DESC").Offset(offset).Limit(int(pageSize)).Find(&values).Error; err != nil {
		return nil, err
	}
	records := make([]*appv1.Role, 0, len(values))
	for _, value := range values {
		records = append(records, roleToProto(value))
	}
	return &appv1.ListRoleResp{Records: records, Paging: &commonv1.Paging{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *Role) Update(ctx context.Context, req *appv1.UpdateRoleReq) (*appv1.Role, error) {
	result := s.db.WithContext(ctx).Model(&model.Role{}).Where("id = ?", req.GetId()).
		Updates(map[string]any{"name": req.GetName(), "updated_by": appauth.Subject(ctx)})
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
	return roleToProto(value), nil
}

func (s *Role) Delete(ctx context.Context, req *appv1.DeleteRoleReq) (*commonv1.Empty, error) {
	var value model.Role
	if err := s.db.WithContext(ctx).First(&value, "id = ?", req.GetId()).Error; err != nil {
		return nil, err
	}
	if value.Code == "admin" {
		return nil, standard.NewError(codes.FailedPrecondition, "administrator role cannot be deleted")
	}
	result := s.db.WithContext(ctx).Delete(&value)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &commonv1.Empty{}, nil
}

func (s *Role) SetPermissions(ctx context.Context, req *appv1.SetRolePermissionsReq) (*commonv1.Empty, error) {
	value, err := s.get(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	ids := unique(req.GetPermissionIds())
	var permissions []model.Permission
	if len(ids) > 0 {
		if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&permissions).Error; err != nil {
			return nil, err
		}
		if len(permissions) != len(ids) {
			return nil, standard.NewError(codes.InvalidArgument, "one or more permissions do not exist").
				WithDomainReason("INVALID_PERMISSION_IDS", "role.invalid_permission_ids")
		}
	}
	if err := s.db.WithContext(ctx).Model(&value).Association("Permissions").Replace(permissions); err != nil {
		return nil, err
	}
	return &commonv1.Empty{}, nil
}

func (s *Role) get(ctx context.Context, id string) (model.Role, error) {
	var value model.Role
	err := s.db.WithContext(ctx).Preload("Permissions").First(&value, "id = ?", id).Error
	return value, err
}
