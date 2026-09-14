package service

import (
	"context"

	"github.com/go-sdk/core/seq"
	servercommon "github.com/go-sdk/server/common"
	"github.com/go-sdk/server/standard"

	appv1 "github.com/go-sdk/example/gen/app/v1"
	commonv1 "github.com/go-sdk/example/gen/common/v1"
	appauth "github.com/go-sdk/example/internal/auth"
	"github.com/go-sdk/example/internal/model"
)

type Role struct {
	appv1.UnimplementedRoleServiceServer
}

func (s *Role) Create(ctx context.Context, req *appv1.CreateRoleReq) (*appv1.Role, error) {
	actor := appauth.Subject(ctx)
	value := model.Role{
		Id:        seq.NextID(),
		CreatedBy: actor,
		UpdatedBy: actor,
		Code:      req.GetCode(),
		Name:      req.GetName(),
	}
	if err := model.CreateRole(ctx, &value); err != nil {
		return nil, err
	}
	return roleToProto(value), nil
}

func (s *Role) Get(ctx context.Context, req *appv1.GetRoleReq) (*appv1.Role, error) {
	value, err := model.GetRole(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return roleToProto(value), nil
}

func (s *Role) List(ctx context.Context, req *appv1.ListRoleReq) (*appv1.ListRoleResp, error) {
	paging := req.GetPaging()
	offset, limit := paging.GetOffsetLimit()
	values, total, err := model.ListRoles(ctx, offset, limit)
	if err != nil {
		return nil, err
	}
	records := make([]*appv1.Role, 0, len(values))
	for _, value := range values {
		records = append(records, roleToProto(value))
	}
	return &appv1.ListRoleResp{Records: records, Paging: paging.WithTotal(total)}, nil
}

func (s *Role) Update(ctx context.Context, req *appv1.UpdateRoleReq) (*appv1.Role, error) {
	if err := model.UpdateRole(ctx, req.GetId(), map[string]any{
		"name":       req.GetName(),
		"updated_by": appauth.Subject(ctx),
	}); err != nil {
		return nil, err
	}
	return s.Get(ctx, &appv1.GetRoleReq{Id: req.GetId()})
}

func (s *Role) Delete(ctx context.Context, req *appv1.DeleteRoleReq) (*servercommon.Empty, error) {
	value, err := model.GetRole(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if value.Code == "admin" {
		return nil, standard.ErrFailedPrecondition.
			WithErrorCode(commonv1.ErrorCode_ERROR_CODE_ADMIN_ROLE_CANNOT_BE_DELETED)
	}
	if err = model.DeleteRole(ctx, &value); err != nil {
		return nil, err
	}
	return &servercommon.Empty{}, nil
}

func (s *Role) SetPermissions(ctx context.Context, req *appv1.SetRolePermissionsReq) (*servercommon.Empty, error) {
	valid, err := model.SetRolePermissions(ctx, req.GetId(), req.GetPermissionIds())
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, standard.ErrInvalidParam.
			WithErrorCode(commonv1.ErrorCode_ERROR_CODE_INVALID_PERMISSION_IDS)
	}
	return &servercommon.Empty{}, nil
}
