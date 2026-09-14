package config

import (
	"net"
	"net/url"
	"os"
	"strconv"
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
		DSN      string `json:"dsn"`
		Host     string `json:"host"`
		Port     int    `json:"port"`
		Name     string `json:"name"`
		User     string `json:"user"`
		Password string `json:"password"`
		SSLMode  string `json:"ssl_mode"`
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
	if err := value.resolveDatabaseDSN(); err != nil {
		return value, err
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
	coreconfig.SetDefault(cfg)
	return value, nil
}

// resolveDatabaseDSN 优先保留显式 DSN，否则对拆分字段编码后构造 PostgreSQL URL。
func (c *Config) resolveDatabaseDSN() error {
	if strings.TrimSpace(c.Database.DSN) != "" {
		return nil
	}
	if strings.TrimSpace(c.Database.Host) == "" || c.Database.Port <= 0 || c.Database.Port > 65535 ||
		strings.TrimSpace(c.Database.Name) == "" || strings.TrimSpace(c.Database.User) == "" || c.Database.Password == "" {
		return errx.New("database dsn or valid connection fields are required")
	}
	sslMode := strings.TrimSpace(c.Database.SSLMode)
	if sslMode == "" {
		sslMode = "disable"
	}
	value := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.Database.User, c.Database.Password),
		Host:   net.JoinHostPort(c.Database.Host, strconv.Itoa(c.Database.Port)),
		Path:   c.Database.Name,
	}
	query := url.Values{}
	query.Set("sslmode", sslMode)
	value.RawQuery = query.Encode()
	c.Database.DSN = value.String()
	return nil
}
