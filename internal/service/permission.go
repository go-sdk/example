package service

import (
	"context"
	"strings"

	"github.com/go-sdk/core/seq"
	"github.com/go-sdk/server/standard"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"

	appv1 "github.com/go-sdk/example/gen/app/v1"
	commonv1 "github.com/go-sdk/example/gen/common/v1"
	appauth "github.com/go-sdk/example/internal/auth"
	"github.com/go-sdk/example/internal/model"
)

type Permission struct {
	appv1.UnimplementedPermissionServiceServer
	db *gorm.DB
}

func NewPermission(db *gorm.DB) *Permission { return &Permission{db: db} }

func (s *Permission) Create(ctx context.Context, req *appv1.CreatePermissionReq) (*appv1.Permission, error) {
	actor := appauth.Subject(ctx)
	value := model.Permission{
		Id: seq.NextID(), CreatedBy: actor, UpdatedBy: actor,
		Code: req.GetCode(), Name: req.GetName(),
	}
	if err := s.db.WithContext(ctx).Create(&value).Error; err != nil {
		return nil, err
	}
	return permissionToProto(value), nil
}

func (s *Permission) Get(ctx context.Context, req *appv1.GetPermissionReq) (*appv1.Permission, error) {
	var value model.Permission
	if err := s.db.WithContext(ctx).First(&value, "id = ?", req.GetId()).Error; err != nil {
		return nil, err
	}
	return permissionToProto(value), nil
}

func (s *Permission) List(ctx context.Context, req *appv1.ListPermissionReq) (*appv1.ListPermissionResp, error) {
	page, pageSize := int32(0), int32(0)
	if req.GetPaging() != nil {
		page, pageSize = req.GetPaging().GetPage(), req.GetPaging().GetPageSize()
	}
	page, pageSize, offset := paging(page, pageSize)
	query := s.db.WithContext(ctx).Model(&model.Permission{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var values []model.Permission
	if err := query.Order("created_at DESC").Offset(offset).Limit(int(pageSize)).Find(&values).Error; err != nil {
		return nil, err
	}
	records := make([]*appv1.Permission, 0, len(values))
	for _, value := range values {
		records = append(records, permissionToProto(value))
	}
	return &appv1.ListPermissionResp{Records: records, Paging: &commonv1.Paging{Page: page, PageSize: pageSize, Total: total}}, nil
}

func (s *Permission) Update(ctx context.Context, req *appv1.UpdatePermissionReq) (*appv1.Permission, error) {
	result := s.db.WithContext(ctx).Model(&model.Permission{}).Where("id = ?", req.GetId()).
		Updates(map[string]any{"name": req.GetName(), "updated_by": appauth.Subject(ctx)})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return s.Get(ctx, &appv1.GetPermissionReq{Id: req.GetId()})
}

func (s *Permission) Delete(ctx context.Context, req *appv1.DeletePermissionReq) (*commonv1.Empty, error) {
	var value model.Permission
	if err := s.db.WithContext(ctx).First(&value, "id = ?", req.GetId()).Error; err != nil {
		return nil, err
	}
	if strings.HasPrefix(value.Id, "permission_") {
		return nil, standard.NewError(codes.FailedPrecondition, "built-in permission cannot be deleted")
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
