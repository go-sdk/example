package main

import (
	"context"
	"os"

	"github.com/go-sdk/core/cmdx"
	"github.com/go-sdk/core/errx"
	"github.com/go-sdk/core/lifex"
	"github.com/go-sdk/core/logx"
	"github.com/go-sdk/core/osx"
	"github.com/go-sdk/database/dbx"
	_ "github.com/go-sdk/database/dbx/postgres"
	"github.com/go-sdk/server/standard"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"gorm.io/gorm"

	appv1 "github.com/go-sdk/example/gen/app/v1"
	appauth "github.com/go-sdk/example/internal/auth"
	appconfig "github.com/go-sdk/example/internal/config"
	"github.com/go-sdk/example/internal/migration"
	"github.com/go-sdk/example/internal/service"
	httptransport "github.com/go-sdk/example/internal/transport/http"
)

func main() {
	root := cmdx.NewRoot("app")
	root.AddCommand(&cmdx.Command{
		Use:   "serve",
		Short: "启动 API 服务",
		RunE: cmdx.WrapRunE(func(_ *cmdx.Command, _ []string) error {
			return serve()
		}),
	})
	if err := root.Execute(); err != nil {
		logx.Error().Err(err).Msg("application exited")
		os.Exit(1)
	}
}

func serve() error {
	cfg, err := appconfig.Load()
	if err != nil {
		return err
	}
	if err := appauth.ValidatePolicies(); err != nil {
		return err
	}
	logx.Init("")
	logx.SetGlobalKV("app", "go-sdk-example")
	logx.SetGlobalKV("version", osx.GetVersion().Version)

	db, err := dbx.Open("postgres", cfg.Database.DSN)
	if err != nil {
		return err
	}
	migrator, err := migration.New(db)
	if err != nil {
		return err
	}
	lifex.OnInit(func() error { return migrator.Up(context.Background()) })
	lifex.OnInit(func() error { return migration.Bootstrap(context.Background(), db, cfg) })

	authorizer := appauth.NewAuthorizer(db)
	server, err := standard.New(
		standard.WithName("go-sdk-example"),
		standard.WithAddress(cfg.Server.Address),
		standard.WithJWTSecret([]byte(cfg.Auth.JWTSecret)),
		standard.WithUnaryInterceptors(authorizer.UnaryInterceptor()),
		standard.WithErrorConverters(databaseErrorConverter()),
		standard.WithGRPCRegister(func(registrar grpc.ServiceRegistrar) {
			appv1.RegisterAuthServiceServer(registrar, service.NewAuth(db, []byte(cfg.Auth.JWTSecret), cfg.Auth.ExpiresIn))
			appv1.RegisterUserServiceServer(registrar, service.NewUser(db))
			appv1.RegisterRoleServiceServer(registrar, service.NewRole(db))
			appv1.RegisterPermissionServiceServer(registrar, service.NewPermission(db))
		}),
		standard.WithGatewayRegister(
			appv1.RegisterAuthServiceHandlerFromEndpoint,
			appv1.RegisterUserServiceHandlerFromEndpoint,
			appv1.RegisterRoleServiceHandlerFromEndpoint,
			appv1.RegisterPermissionServiceHandlerFromEndpoint,
		),
	)
	if err != nil {
		return err
	}
	files := httptransport.NewFileHandler(db, authorizer, cfg.Storage.Root, cfg.Storage.MaxUploadBytes)
	if err := files.Register(server); err != nil {
		return err
	}
	if err := lifex.Init(); err != nil {
		lifex.Shutdown(err)
	}
	return lifex.Wait()
}

func databaseErrorConverter() standard.ErrorConverter {
	return standard.ErrorConvertFunc(func(err error) (standard.RespError, bool) {
		switch {
		case errx.Is(err, gorm.ErrRecordNotFound):
			return standard.NewError(codes.NotFound, "record not found").
				WithDomainReason("RECORD_NOT_FOUND", "record.not_found"), true
		case errx.Is(err, gorm.ErrDuplicatedKey):
			return standard.NewError(codes.AlreadyExists, "record already exists").
				WithDomainReason("RECORD_ALREADY_EXISTS", "record.already_exists"), true
		case errx.Is(err, gorm.ErrForeignKeyViolated):
			return standard.NewError(codes.FailedPrecondition, "record is in use").
				WithDomainReason("RECORD_IN_USE", "record.in_use"), true
		default:
			return standard.RespError{}, false
		}
	})
}
