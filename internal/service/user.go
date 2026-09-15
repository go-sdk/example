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

	appauth "github.com/go-sdk/example/internal/auth"
	appconfig "github.com/go-sdk/example/internal/config"
	"github.com/go-sdk/example/internal/model"
	appv1 "github.com/go-sdk/example/pb/app/v1"
	commonv1 "github.com/go-sdk/example/pb/common/v1"
)

type User struct {
	appv1.UnimplementedUserServiceServer
	config appconfig.Config
}

func NewUser(config appconfig.Config) *User {
	return &User{
		config: config,
	}
}

func (s *User) Create(ctx context.Context, req *appv1.CreateUserReq) (*appv1.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.GetPassword()), bcrypt.DefaultCost)
	if err != nil {
		return nil, errx.Wrap(err, "hash password")
	}
	actor := appauth.Subject(ctx)
	value := model.User{
		Id:           seq.NextID(),
		CreatedBy:    actor,
		UpdatedBy:    actor,
		Username:     req.GetUsername(),
		Email:        req.GetEmail(),
		PasswordHash: string(hash),
		Enabled:      req.GetEnabled(),
	}
	if err = model.CreateUser(ctx, &value); err != nil {
		return nil, err
	}
	return userToProto(value), nil
}

func (s *User) Get(ctx context.Context, req *appv1.GetUserReq) (*appv1.User, error) {
	value, err := model.GetUser(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return userToProto(value), nil
}

func (s *User) List(ctx context.Context, req *appv1.ListUserReq) (*appv1.ListUserResp, error) {
	paging := req.GetPaging()
	offset, limit := paging.GetOffsetLimit()
	values, total, err := model.ListUsers(ctx, offset, limit)
	if err != nil {
		return nil, err
	}
	return &appv1.ListUserResp{
		Records: convertSlice(values, userToProto),
		Paging:  paging.WithTotal(total),
	}, nil
}

func (s *User) Update(ctx context.Context, req *appv1.UpdateUserReq) (*appv1.User, error) {
	if err := model.UpdateUser(ctx, req.GetId(), map[string]any{
		"email":      req.GetEmail(),
		"enabled":    req.GetEnabled(),
		"updated_by": appauth.Subject(ctx),
	}); err != nil {
		return nil, err
	}
	return s.Get(ctx, &appv1.GetUserReq{
		Id: req.GetId(),
	})
}

func (s *User) Delete(ctx context.Context, req *appv1.DeleteUserReq) (*servercommon.Empty, error) {
	if req.GetId() == appauth.Subject(ctx) {
		return nil, standard.ErrFailedPrecondition.WithErrorCode(commonv1.ErrorCode_ERROR_CODE_CURRENT_USER_CANNOT_BE_DELETED)
	}
	avatar, err := model.DeleteUser(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if avatar.Id != "" {
		path := filepath.Join(s.config.Storage.Root, avatar.StoredName)
		if err = os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			logx.Ctx(ctx).Warn().Err(err).Str("file_id", avatar.Id).Msg("delete user avatar file")
		}
	}
	return &servercommon.Empty{}, nil
}

func (s *User) SetRoles(ctx context.Context, req *appv1.SetUserRolesReq) (*servercommon.Empty, error) {
	valid, err := model.SetUserRoles(ctx, req.GetId(), req.GetRoleIds())
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, standard.ErrInvalidParam.WithErrorCode(commonv1.ErrorCode_ERROR_CODE_INVALID_ROLE_IDS)
	}
	return &servercommon.Empty{}, nil
}
