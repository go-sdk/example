package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-sdk/app"
	"github.com/go-sdk/app/testapp"
	_ "github.com/go-sdk/database/dbx/sqlite"
	"github.com/go-sdk/server/standard"
	"github.com/go-sdk/server/standard/testserver"

	appauth "github.com/go-sdk/example/internal/auth"
	appconfig "github.com/go-sdk/example/internal/config"
	"github.com/go-sdk/example/internal/model"
)

const testJWTSecret = "0123456789abcdef0123456789abcdef"

func TestUploadFile(t *testing.T) {
	user := newHTTPTestDB(t, true)
	root := t.TempDir()
	handler := New(appconfig.Config{
		Storage: appconfig.Storage{
			Root:           root,
			MaxUploadBytes: 1024,
		},
	})
	server := testserver.NewHTTP(
		t,
		http.MethodPost,
		"/api/v1/files",
		handler.uploadFile,
		standard.WithJWTSecret([]byte(testJWTSecret)),
	)
	request := uploadRequest(t, server.URL("/api/v1/files"), user, "report.txt", []byte("content"))
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status: %d", response.StatusCode)
	}
	var body struct {
		Data fileResponse `json:"data"`
	}
	if err = json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	stored, err := model.GetFile(context.Background(), body.Data.Id)
	if err != nil {
		t.Fatal(err)
	}
	if stored.OwnerId != user.Id || stored.OriginalName != "report.txt" {
		t.Fatalf("unexpected file metadata: %#v", stored)
	}
	data, err := os.ReadFile(filepath.Join(root, stored.StoredName))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "content" {
		t.Fatalf("unexpected stored content: %q", data)
	}
}

func TestUploadFileRequiresPermission(t *testing.T) {
	user := newHTTPTestDB(t, false)
	handler := New(appconfig.Config{
		Storage: appconfig.Storage{
			Root:           t.TempDir(),
			MaxUploadBytes: 1024,
		},
	})
	server := testserver.NewHTTP(
		t,
		http.MethodPost,
		"/api/v1/files",
		handler.uploadFile,
		standard.WithJWTSecret([]byte(testJWTSecret)),
	)
	request := uploadRequest(t, server.URL("/api/v1/files"), user, "report.txt", []byte("content"))
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusForbidden {
		t.Fatalf("unexpected status: %d", response.StatusCode)
	}
	var count int64
	if err = app.DB().Model(&model.File{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("permission denial created %d files", count)
	}
}

func uploadRequest(t *testing.T, url string, user model.User, name string, data []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	token, _, err := appauth.Sign(user, []byte(testJWTSecret), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	return request
}

func newHTTPTestDB(t *testing.T, grantPermission bool) model.User {
	t.Helper()
	db := testapp.NewDB(
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
	user := model.User{
		Id:           "user-1",
		Username:     "tester",
		Email:        "tester@example.invalid",
		PasswordHash: "hash",
		Enabled:      true,
	}
	if err := model.CreateUser(context.Background(), &user); err != nil {
		t.Fatal(err)
	}
	if !grantPermission {
		return user
	}
	permission := model.Permission{
		Id:   "permission-files-write",
		Code: "files.write",
		Name: "Write files",
	}
	role := model.Role{
		Id:   "role-writer",
		Code: "writer",
		Name: "Writer",
	}
	if err := db.Create(&permission).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&role).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.RolePermission{
		RoleId:       role.Id,
		PermissionId: permission.Id,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.UserRole{
		UserId: user.Id,
		RoleId: role.Id,
	}).Error; err != nil {
		t.Fatal(err)
	}
	return user
}
