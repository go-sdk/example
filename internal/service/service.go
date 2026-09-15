package service

import (
	"github.com/go-sdk/app"
	"github.com/go-sdk/database/dbx"
	"github.com/go-sdk/server/standard"
	"google.golang.org/grpc"

	appauth "github.com/go-sdk/example/internal/auth"
	appconfig "github.com/go-sdk/example/internal/config"
	appi18n "github.com/go-sdk/example/internal/i18n"
	appv1 "github.com/go-sdk/example/pb/app/v1"
	commonv1 "github.com/go-sdk/example/pb/common/v1"
)

func init() {
	app.RegisterServerOptions(
		standard.WithErrorConverters(databaseErrorConverter()),
		standard.WithI18nFS(appi18n.Files),
	)
	app.RegisterServerOptionFactories(func() (standard.Option, error) {
		if err := appconfig.Validate(); err != nil {
			return nil, err
		}
		return standard.WithUnaryInterceptors(appauth.UnaryInterceptor()), nil
	})
	app.RegisterGRPC(func(registrar grpc.ServiceRegistrar) {
		config := appconfig.G()
		appv1.RegisterAuthServiceServer(registrar, NewAuth(config))
		appv1.RegisterUserServiceServer(registrar, NewUser(config))
		appv1.RegisterRoleServiceServer(registrar, NewRole())
		appv1.RegisterPermissionServiceServer(registrar, NewPermission())
	})
	app.RegisterGateway(
		appv1.RegisterAuthServiceHandlerFromEndpoint,
		appv1.RegisterUserServiceHandlerFromEndpoint,
		appv1.RegisterRoleServiceHandlerFromEndpoint,
		appv1.RegisterPermissionServiceHandlerFromEndpoint,
	)
}

func databaseErrorConverter() standard.ErrorConverter {
	return standard.ErrorConvertFunc(func(err error) (standard.RespError, bool) {
		switch {
		case dbx.IsRecordNotFound(err):
			return standard.ErrNotFound.WithErrorCode(commonv1.ErrorCode_ERROR_CODE_RECORD_NOT_FOUND), true
		case dbx.IsDuplicatedKey(err):
			return standard.ErrAlreadyExists.WithErrorCode(commonv1.ErrorCode_ERROR_CODE_RECORD_ALREADY_EXISTS), true
		case dbx.IsForeignKeyViolated(err):
			return standard.ErrFailedPrecondition.WithErrorCode(commonv1.ErrorCode_ERROR_CODE_RECORD_IN_USE), true
		default:
			return standard.RespError{}, false
		}
	})
}
