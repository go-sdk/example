package config

import (
	"os"
	"strings"
	"time"

	coreconfig "github.com/go-sdk/core/config"
	"github.com/go-sdk/core/errx"
)

type Config struct {
	Server struct {
		Address string `json:"address"`
	} `json:"server"`
	Database struct {
		DSN string `json:"dsn"`
	} `json:"database"`
	Auth struct {
		JWTSecret string        `json:"jwt_secret"`
		ExpiresIn time.Duration `json:"expires_in"`
	} `json:"auth"`
	Storage struct {
		Root           string `json:"root"`
		MaxUploadBytes int64  `json:"max_upload_bytes"`
	} `json:"storage"`
	Bootstrap struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Email    string `json:"email"`
	} `json:"bootstrap"`
}

func Load() (Config, error) {
	var value Config
	path := "config.yaml"
	if configuredPath, ok := os.LookupEnv("CONFIG_PATH"); ok {
		path = configuredPath
	}
	cfg := coreconfig.New(coreconfig.WithFile(path))
	if err := cfg.Load(); err != nil {
		return value, errx.Wrap(err, "load config")
	}
	if err := cfg.DecodeTo(&value); err != nil {
		return value, errx.Wrap(err, "decode config")
	}
	if strings.TrimSpace(value.Database.DSN) == "" {
		return value, errx.New("database dsn is required")
	}
	if len(value.Auth.JWTSecret) < 32 {
		return value, errx.New("jwt secret must contain at least 32 characters")
	}
	if value.Auth.ExpiresIn <= 0 {
		return value, errx.New("jwt expiry must be greater than zero")
	}
	if strings.TrimSpace(value.Storage.Root) == "" || value.Storage.MaxUploadBytes <= 0 {
		return value, errx.New("valid storage configuration is required")
	}
	if value.Bootstrap.Username == "" || len(value.Bootstrap.Password) < 8 || value.Bootstrap.Email == "" {
		return value, errx.New("valid bootstrap administrator configuration is required")
	}
	return value, nil
}
