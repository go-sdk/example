package auth

import (
	"context"
	"strings"
	"time"

	"github.com/go-sdk/core/errx"
	"github.com/go-sdk/server/options"
	"github.com/go-sdk/server/standard"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/cast"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	commonv1 "github.com/go-sdk/example/gen/common/v1"
	"github.com/go-sdk/example/internal/model"
)

func UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		methodOption, err := resolveMethodOption(info.FullMethod)
		if err != nil {
			return nil, standard.ErrInternal.WithMessage("resolve method permissions")
		}
		if strings.HasPrefix(info.FullMethod, "/app.v1.") &&
			(methodOption == nil || (!methodOption.GetSkipAuth() && len(methodOption.GetPermissions()) == 0)) {
			return nil, standard.ErrUnauthenticated
		}
		for _, permission := range methodOption.GetPermissions() {
			if err := Require(ctx, permission); err != nil {
				return nil, err
			}
		}
		return handler(ctx, req)
	}
}

func Require(ctx context.Context, permission string) error {
	userID := Subject(ctx)
	if strings.TrimSpace(userID) == "" {
		return standard.ErrUnauthenticated
	}
	allowed, err := model.HasPermission(ctx, userID, permission)
	if err != nil {
		return errx.Wrap(err, "check permission")
	}
	if !allowed {
		return standard.ErrPermissionDenied.
			WithErrorCode(commonv1.ErrorCode_ERROR_CODE_PERMISSION_DENIED)
	}
	return nil
}

func Subject(ctx context.Context) string {
	return cast.ToString(standard.FromContext(ctx).JWT()["sub"])
}

func Sign(user model.User, secret []byte, expiresIn time.Duration) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(expiresIn)
	claims := jwt.MapClaims{
		"sub":      user.Id,
		"username": user.Username,
		"iat":      now.Unix(),
		"exp":      expiresAt.Unix(),
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(secret)
	return token, expiresAt, err
}

func resolveMethodOption(fullMethod string) (*options.MethodOptions, error) {
	descriptor, err := protoregistryFindMethod(fullMethod)
	if err != nil {
		return nil, err
	}
	return optionFromDescriptor(descriptor)
}

func optionFromDescriptor(descriptor protoreflect.MethodDescriptor) (*options.MethodOptions, error) {
	methodOptions := descriptor.Options()
	if methodOptions == nil || !proto.HasExtension(methodOptions, options.E_Method) {
		return nil, nil
	}
	value, _ := proto.GetExtension(methodOptions, options.E_Method).(*options.MethodOptions)
	if value == nil {
		return nil, errx.New("invalid method options")
	}
	return value, nil
}

func protoregistryFindMethod(fullMethod string) (protoreflect.MethodDescriptor, error) {
	name := strings.TrimPrefix(fullMethod, "/")
	index := strings.LastIndexByte(name, '/')
	if index < 0 {
		return nil, errx.New("invalid grpc method")
	}
	descriptor, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(name[:index]))
	if err != nil {
		return nil, err
	}
	service, ok := descriptor.(protoreflect.ServiceDescriptor)
	if !ok {
		return nil, errx.New("grpc service descriptor not found")
	}
	method := service.Methods().ByName(protoreflect.Name(name[index+1:]))
	if method == nil {
		return nil, errx.New("grpc method descriptor not found")
	}
	return method, nil
}
