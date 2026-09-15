package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-sdk/app/testapp"
	_ "github.com/go-sdk/database/dbx/sqlite"
	"github.com/go-sdk/server/standard"
	"github.com/go-sdk/server/standard/testserver"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	appauth "github.com/go-sdk/example/internal/auth"
	appconfig "github.com/go-sdk/example/internal/config"
	"github.com/go-sdk/example/internal/model"
	appv1 "github.com/go-sdk/example/pb/app/v1"
)

const testJWTSecret = "0123456789abcdef0123456789abcdef"

func TestAuthLogin(t *testing.T) {
	newTestDB(t)
	hash, err := bcrypt.GenerateFromPassword([]byte("password"), bcrypt.MinCost)
	if err != nil {
		t.Fatal(err)
	}
	user := model.User{
		Id:           "user-1",
		Username:     "tester",
		Email:        "tester@example.invalid",
		PasswordHash: string(hash),
		Enabled:      true,
	}
	if err = model.CreateUser(context.Background(), &user); err != nil {
		t.Fatal(err)
	}
	server := testserver.New(t,
		standard.WithJWTSecret([]byte(testJWTSecret)),
		standard.WithUnaryInterceptors(appauth.UnaryInterceptor()),
		standard.WithGRPCRegister(func(registrar grpc.ServiceRegistrar) {
			appv1.RegisterAuthServiceServer(registrar, NewAuth(testConfig()))
		}),
	)
	client := appv1.NewAuthServiceClient(server.Conn())
	response, err := client.Login(context.Background(), &appv1.LoginReq{
		Username: "tester",
		Password: "password",
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.GetAccessToken() == "" || response.GetExpiresAt() <= time.Now().Unix() {
		t.Fatalf("unexpected login response: %#v", response)
	}
}

func TestUserCreateRequiresAuthentication(t *testing.T) {
	newTestDB(t)
	server := testserver.New(t,
		standard.WithJWTSecret([]byte(testJWTSecret)),
		standard.WithUnaryInterceptors(appauth.UnaryInterceptor()),
		standard.WithGRPCRegister(func(registrar grpc.ServiceRegistrar) {
			appv1.RegisterUserServiceServer(registrar, NewUser(testConfig()))
		}),
	)
	client := appv1.NewUserServiceClient(server.Conn())
	_, err := client.Create(context.Background(), &appv1.CreateUserReq{
		Username: "tester",
		Email:    "tester@example.invalid",
		Password: "password",
		Enabled:  true,
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPermissionToProto(t *testing.T) {
	value := model.Permission{
		Id:        "permission-1",
		CreatedBy: "user-1",
		UpdatedBy: "user-2",
		Code:      "users.read",
		Name:      "Read users",
	}
	got := permissionToProto(value)
	if got.GetId() != value.Id || got.GetCode() != value.Code || got.GetMetadata().GetCreatedBy() != value.CreatedBy {
		t.Fatalf("unexpected proto value: %#v", got)
	}
}

func newTestDB(t *testing.T) {
	t.Helper()
	testapp.NewDB(
		t,
		"sqlite",
		filepath.Join(t.TempDir(), "test.db"),
		&model.Permission{},
		&model.Role{},
		&model.User{},
		&model.File{},
		&model.RolePermission{},
		&model.UserRole{},
	)
}

func testConfig() appconfig.Config {
	return appconfig.Config{
		Auth: appconfig.Auth{
			JWTSecret: testJWTSecret,
			ExpiresIn: time.Hour,
		},
	}
}
