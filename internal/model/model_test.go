package model

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/go-sdk/app/testapp"
	_ "github.com/go-sdk/database/dbx/sqlite"
)

func TestUserRoles(t *testing.T) {
	newTestDB(t)
	ctx := context.Background()
	role := Role{
		Id:   "role-1",
		Code: "reader",
		Name: "Reader",
	}
	if err := CreateRole(ctx, &role); err != nil {
		t.Fatal(err)
	}
	user := User{
		Id:           "user-1",
		Username:     "tester",
		Email:        "tester@example.invalid",
		PasswordHash: "hash",
		Enabled:      true,
	}
	if err := CreateUser(ctx, &user); err != nil {
		t.Fatal(err)
	}
	valid, err := SetUserRoles(ctx, user.Id, []string{role.Id, role.Id})
	if err != nil {
		t.Fatal(err)
	}
	if !valid {
		t.Fatal("expected role IDs to be valid")
	}
	got, err := GetUser(ctx, user.Id)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Roles) != 1 || got.Roles[0].Id != role.Id {
		t.Fatalf("unexpected roles: %#v", got.Roles)
	}
}

func TestSetUserRolesRejectsUnknownRole(t *testing.T) {
	newTestDB(t)
	ctx := context.Background()
	user := User{
		Id:           "user-1",
		Username:     "tester",
		Email:        "tester@example.invalid",
		PasswordHash: "hash",
		Enabled:      true,
	}
	if err := CreateUser(ctx, &user); err != nil {
		t.Fatal(err)
	}
	valid, err := SetUserRoles(ctx, user.Id, []string{"missing"})
	if err != nil {
		t.Fatal(err)
	}
	if valid {
		t.Fatal("expected unknown role ID to be rejected")
	}
}

func newTestDB(t *testing.T) {
	t.Helper()
	testapp.NewDB(
		t,
		"sqlite",
		filepath.Join(t.TempDir(), "test.db"),
		&Permission{},
		&Role{},
		&User{},
		&File{},
		&RolePermission{},
		&UserRole{},
	)
}
