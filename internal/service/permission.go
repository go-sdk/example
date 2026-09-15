package service

import (
	"context"
	"strings"

	"github.com/go-sdk/core/seq"
	servercommon "github.com/go-sdk/server/common"
	"github.com/go-sdk/server/standard"

	appauth "github.com/go-sdk/example/internal/auth"
	"github.com/go-sdk/example/internal/model"
	appv1 "github.com/go-sdk/example/pb/app/v1"
	commonv1 "github.com/go-sdk/example/pb/common/v1"
)

type Permission struct {
	appv1.UnimplementedPermissionServiceServer
}

func NewPermission() *Permission {
	return &Permission{}
}

func (s *Permission) Create(ctx context.Context, req *appv1.CreatePermissionReq) (*appv1.Permission, error) {
	actor := appauth.Subject(ctx)
	value := model.Permission{
		Id:        seq.NextID(),
		CreatedBy: actor,
		UpdatedBy: actor,
		Code:      req.GetCode(),
		Name:      req.GetName(),
	}
	if err := model.CreatePermission(ctx, &value); err != nil {
		return nil, err
	}
	return permissionToProto(value), nil
}

func (s *Permission) Get(ctx context.Context, req *appv1.GetPermissionReq) (*appv1.Permission, error) {
	value, err := model.GetPermission(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return permissionToProto(value), nil
}

func (s *Permission) List(ctx context.Context, req *appv1.ListPermissionReq) (*appv1.ListPermissionResp, error) {
	paging := req.GetPaging()
	offset, limit := paging.GetOffsetLimit()
	values, total, err := model.ListPermissions(ctx, offset, limit)
	if err != nil {
		return nil, err
	}
	return &appv1.ListPermissionResp{
		Records: convertSlice(values, permissionToProto),
		Paging:  paging.WithTotal(total),
	}, nil
}

func (s *Permission) Update(ctx context.Context, req *appv1.UpdatePermissionReq) (*appv1.Permission, error) {
	if err := model.UpdatePermission(ctx, req.GetId(), map[string]any{
		"name":       req.GetName(),
		"updated_by": appauth.Subject(ctx),
	}); err != nil {
		return nil, err
	}
	return s.Get(ctx, &appv1.GetPermissionReq{
		Id: req.GetId(),
	})
}

func (s *Permission) Delete(ctx context.Context, req *appv1.DeletePermissionReq) (*servercommon.Empty, error) {
	value, err := model.GetPermission(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(value.Id, "permission_") {
		return nil, standard.ErrFailedPrecondition.WithErrorCode(commonv1.ErrorCode_ERROR_CODE_BUILTIN_PERMISSION_CANNOT_BE_DELETED)
	}
	if err = model.DeletePermission(ctx, &value); err != nil {
		return nil, err
	}
	return &servercommon.Empty{}, nil
}
