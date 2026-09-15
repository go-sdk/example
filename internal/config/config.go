package config

import (
	"strings"
	"time"

	coreconfig "github.com/go-sdk/core/config"
	"github.com/go-sdk/core/errx"
	"github.com/go-sdk/core/osx"
)

type Config struct {
	Auth      Auth      `json:"auth"`
	Storage   Storage   `json:"storage"`
	Bootstrap Bootstrap `json:"bootstrap"`
}

type Auth struct {
	JWTSecret string        `json:"jwt_secret"`
	ExpiresIn time.Duration `json:"expires_in"`
}

type Storage struct {
	Root           string `json:"root"`
	MaxUploadBytes int64  `json:"max_upload_bytes"`
}

type Bootstrap struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
}

var global = load()

func load() Config {
	value := Config{
		Auth: Auth{
			ExpiresIn: 24 * time.Hour,
		},
		Storage: Storage{
			Root:           "./data",
			MaxUploadBytes: 10 << 20,
		},
		Bootstrap: Bootstrap{
			Username: "admin",
			Email:    "admin@example.invalid",
		},
	}
	if err := coreconfig.DecodeTo(&value); err != nil {
		osx.Panicf("decode business config: %v", err)
	}
	return value
}

// G 返回应用业务配置的只读快照。
func G() Config {
	return global
}

// Validate 校验 app 未覆盖的业务配置。
func Validate() error {
	value := G()
	if len(value.Auth.JWTSecret) < 32 {
		return errx.New("jwt secret must contain at least 32 characters")
	}
	if value.Auth.ExpiresIn <= 0 {
		return errx.New("jwt expiry must be greater than zero")
	}
	if strings.TrimSpace(value.Storage.Root) == "" || value.Storage.MaxUploadBytes <= 0 {
		return errx.New("valid storage configuration is required")
	}
	if value.Bootstrap.Username == "" || len(value.Bootstrap.Password) < 8 || value.Bootstrap.Email == "" {
		return errx.New("valid bootstrap administrator configuration is required")
	}
	return nil
}
