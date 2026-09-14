package service

import (
	"context"

	"github.com/go-sdk/database/dbx"
	"github.com/go-sdk/server/standard"
	"golang.org/x/crypto/bcrypt"

	appv1 "github.com/go-sdk/example/gen/app/v1"
	commonv1 "github.com/go-sdk/example/gen/common/v1"
	appauth "github.com/go-sdk/example/internal/auth"
	appconfig "github.com/go-sdk/example/internal/config"
	"github.com/go-sdk/example/internal/model"
)

type Auth struct {
	appv1.UnimplementedAuthServiceServer
}

func (s *Auth) Login(ctx context.Context, req *appv1.LoginReq) (*appv1.LoginResp, error) {
	user, err := model.GetUserByUsername(ctx, req.GetUsername())
	if err != nil {
		if dbx.IsRecordNotFound(err) {
			return nil, invalidCredentials()
		}
		return nil, err
	}
	if !user.Enabled || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.GetPassword())) != nil {
		return nil, invalidCredentials()
	}
	token, expiresAt, err := appauth.Sign(user, []byte(appconfig.JWTSecret()), appconfig.JWTExpiresIn())
	if err != nil {
		return nil, err
	}
	return &appv1.LoginResp{AccessToken: token, ExpiresAt: expiresAt.Unix()}, nil
}

func invalidCredentials() error {
	return standard.ErrUnauthenticated.
		WithErrorCode(commonv1.ErrorCode_ERROR_CODE_INVALID_CREDENTIALS)
}
