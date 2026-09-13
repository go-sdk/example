package service

import (
	"google.golang.org/protobuf/types/known/timestamppb"

	appv1 "github.com/go-sdk/example/gen/app/v1"
	commonv1 "github.com/go-sdk/example/gen/common/v1"
	"github.com/go-sdk/example/internal/model"
)

func userToProto(value model.User) *appv1.User {
	roles := make([]string, 0, len(value.Roles))
	for _, role := range value.Roles {
		roles = append(roles, role.Id)
	}
	avatarID := ""
	if value.AvatarFileID != nil {
		avatarID = *value.AvatarFileID
	}
	return &appv1.User{
		Id: value.Id, Username: value.Username, Email: value.Email, Enabled: value.Enabled,
		AvatarFileId: avatarID, RoleIds: roles,
		Metadata: &commonv1.Metadata{
			CreatedBy: value.CreatedBy, CreatedAt: timestamppb.New(value.CreatedAt),
			UpdatedBy: value.UpdatedBy, UpdatedAt: timestamppb.New(value.UpdatedAt),
		},
	}
}

func roleToProto(value model.Role) *appv1.Role {
	permissions := make([]string, 0, len(value.Permissions))
	for _, permission := range value.Permissions {
		permissions = append(permissions, permission.Id)
	}
	return &appv1.Role{
		Id: value.Id, Code: value.Code, Name: value.Name, PermissionIds: permissions,
		Metadata: &commonv1.Metadata{
			CreatedBy: value.CreatedBy, CreatedAt: timestamppb.New(value.CreatedAt),
			UpdatedBy: value.UpdatedBy, UpdatedAt: timestamppb.New(value.UpdatedAt),
		},
	}
}

func permissionToProto(value model.Permission) *appv1.Permission {
	return &appv1.Permission{
		Id: value.Id, Code: value.Code, Name: value.Name,
		Metadata: &commonv1.Metadata{
			CreatedBy: value.CreatedBy, CreatedAt: timestamppb.New(value.CreatedAt),
			UpdatedBy: value.UpdatedBy, UpdatedAt: timestamppb.New(value.UpdatedAt),
		},
	}
}
