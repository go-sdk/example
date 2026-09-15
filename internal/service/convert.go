package service

import (
	"github.com/go-sdk/database/dbx"
	servercommon "github.com/go-sdk/server/common"

	"github.com/go-sdk/example/internal/model"
	appv1 "github.com/go-sdk/example/pb/app/v1"
)

func userToProto(value model.User) *appv1.User {
	avatarId := ""
	if value.AvatarFileId != nil {
		avatarId = *value.AvatarFileId
	}
	return &appv1.User{
		Id:           value.Id,
		Username:     value.Username,
		Email:        value.Email,
		Enabled:      value.Enabled,
		AvatarFileId: avatarId,
		RoleIds:      convertSlice(value.Roles, func(role model.Role) string { return role.Id }),
		Metadata:     metadataToProto(value.Metadata),
	}
}

func roleToProto(value model.Role) *appv1.Role {
	return &appv1.Role{
		Id:            value.Id,
		Code:          value.Code,
		Name:          value.Name,
		PermissionIds: convertSlice(value.Permissions, func(permission model.Permission) string { return permission.Id }),
		Metadata:      metadataToProto(value.Metadata),
	}
}

func permissionToProto(value model.Permission) *appv1.Permission {
	return &appv1.Permission{
		Id:       value.Id,
		Code:     value.Code,
		Name:     value.Name,
		Metadata: metadataToProto(value.Metadata),
	}
}

func metadataToProto(value dbx.Metadata) *servercommon.Metadata {
	return &servercommon.Metadata{
		CreatedBy: value.CreatedBy,
		CreatedAt: servercommon.NewTimestamp(value.CreatedAt),
		UpdatedBy: value.UpdatedBy,
		UpdatedAt: servercommon.NewTimestamp(value.UpdatedAt),
	}
}

func convertSlice[S, T any](values []S, convert func(S) T) []T {
	result := make([]T, 0, len(values))
	for _, value := range values {
		result = append(result, convert(value))
	}
	return result
}
