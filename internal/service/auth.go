package service

import (
	"context"
	"errors"
	"time"

	"github.com/go-sdk/server/standard"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"

	appv1 "github.com/go-sdk/example/gen/app/v1"
	appauth "github.com/go-sdk/example/internal/auth"
	"github.com/go-sdk/example/internal/model"
)

type Auth struct {
	appv1.UnimplementedAuthServiceServer
	db        *gorm.DB
	secret    []byte
	expiresIn time.Duration
}

func NewAuth(db *gorm.DB, secret []byte, expiresIn time.Duration) *Auth {
	return &Auth{db: db, secret: secret, expiresIn: expiresIn}
}

func (s *Auth) Login(ctx context.Context, req *appv1.LoginReq) (*appv1.LoginResp, error) {
	var user model.User
	if err := s.db.WithContext(ctx).Where("username = ?", req.GetUsername()).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, invalidCredentials()
		}
		return nil, err
	}
	if !user.Enabled || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.GetPassword())) != nil {
		return nil, invalidCredentials()
	}
	token, expiresAt, err := appauth.Sign(user, s.secret, s.expiresIn)
	if err != nil {
		return nil, err
	}
	return &appv1.LoginResp{AccessToken: token, ExpiresAt: expiresAt.Unix()}, nil
}

func invalidCredentials() error {
	return standard.NewError(codes.Unauthenticated, "invalid username or password").
		WithDomainReason("INVALID_CREDENTIALS", "auth.invalid_credentials")
}
