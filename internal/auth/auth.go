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
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"gorm.io/gorm"

	"github.com/go-sdk/example/internal/model"
)

type Authorizer struct{ db *gorm.DB }

func NewAuthorizer(db *gorm.DB) *Authorizer { return &Authorizer{db: db} }

// ValidatePolicies 保证每个应用 RPC 都显式声明匿名访问或所需权限。
func ValidatePolicies() error {
	methodCount := 0
	var validationErr error
	protoregistry.GlobalFiles.RangeFiles(func(file protoreflect.FileDescriptor) bool {
		if file.Package() != "app.v1" {
			return true
		}
		services := file.Services()
		for serviceIndex := 0; serviceIndex < services.Len(); serviceIndex++ {
			methods := services.Get(serviceIndex).Methods()
			for methodIndex := 0; methodIndex < methods.Len(); methodIndex++ {
				methodCount++
				method := methods.Get(methodIndex)
				value, err := optionFromDescriptor(method)
				if err != nil {
					validationErr = err
					return false
				}
				if value == nil || (!value.GetSkipAuth() && len(value.GetPermissions()) == 0) {
					validationErr = errx.Newf("auth policy is required for %s", method.FullName())
					return false
				}
			}
		}
		return true
	})
	if validationErr != nil {
		return validationErr
	}
	if methodCount == 0 {
		return errx.New("no application rpc policies found")
	}
	return nil
}

func (a *Authorizer) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		methodOption, err := resolveMethodOption(info.FullMethod)
		if err != nil {
			return nil, standard.NewError(codes.Internal, "resolve method permissions")
		}
		if strings.HasPrefix(info.FullMethod, "/app.v1.") && methodOption == nil {
			return nil, standard.NewError(codes.Internal, "method permission policy is required")
		}
		for _, permission := range methodOption.GetPermissions() {
			if err := a.Require(ctx, permission); err != nil {
				return nil, err
			}
		}
		return handler(ctx, req)
	}
}

func (a *Authorizer) Require(ctx context.Context, permission string) error {
	userID := Subject(ctx)
	if strings.TrimSpace(userID) == "" {
		return standard.NewError(codes.Unauthenticated, "authentication required")
	}
	var count int64
	err := a.db.WithContext(ctx).Table("permissions AS p").
		Joins("JOIN role_permissions AS rp ON rp.permission_id = p.id").
		Joins("JOIN roles AS r ON r.id = rp.role_id").
		Joins("JOIN user_roles AS ur ON ur.role_id = rp.role_id").
		Joins("JOIN users AS u ON u.id = ur.user_id").
		Where("u.id = ? AND u.enabled = ? AND u.deleted_at = 0 AND r.deleted_at = 0 AND p.deleted_at = 0 AND p.code = ?", userID, true, permission).
		Count(&count).Error
	if err != nil {
		return errx.Wrap(err, "check permission")
	}
	if count == 0 {
		return standard.NewError(codes.PermissionDenied, "permission denied").
			WithDomainReason("PERMISSION_DENIED", "auth.permission_denied")
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
		"sub": user.Id, "username": user.Username,
		"iat": now.Unix(), "exp": expiresAt.Unix(),
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
	methodOptions, ok := descriptor.Options().(*descriptorpb.MethodOptions)
	if !ok || !proto.HasExtension(methodOptions, options.E_Method) {
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
