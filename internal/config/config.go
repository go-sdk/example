package config

import (
	"strings"
	"time"

	"github.com/go-sdk/core/config"
	"github.com/go-sdk/core/errx"
)

// Validate 校验 app 未覆盖的业务配置。
func Validate() error {
	if len(JWTSecret()) < 32 {
		return errx.New("jwt secret must contain at least 32 characters")
	}
	if JWTExpiresIn() <= 0 {
		return errx.New("jwt expiry must be greater than zero")
	}
	if strings.TrimSpace(StorageRoot()) == "" || MaxUploadBytes() <= 0 {
		return errx.New("valid storage configuration is required")
	}
	if BootstrapUsername() == "" || len(BootstrapPassword()) < 8 || BootstrapEmail() == "" {
		return errx.New("valid bootstrap administrator configuration is required")
	}
	return nil
}

func JWTSecret() string {
	return config.MustGet("auth.jwt_secret", "")
}

func JWTExpiresIn() time.Duration {
	return config.MustGet("auth.expires_in", 24*time.Hour)
}

func StorageRoot() string {
	return config.MustGet("storage.root", "./data")
}

func MaxUploadBytes() int64 {
	return config.MustGet[int64]("storage.max_upload_bytes", 10<<20)
}

func BootstrapUsername() string {
	return config.MustGet("bootstrap.username", "admin")
}

func BootstrapPassword() string {
	return config.MustGet("bootstrap.password", "")
}

func BootstrapEmail() string {
	return config.MustGet("bootstrap.email", "admin@example.invalid")
}
