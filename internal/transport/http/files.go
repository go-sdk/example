package httptransport

import (
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-sdk/core/errx"
	"github.com/go-sdk/core/logx"
	"github.com/go-sdk/core/seq"
	"github.com/go-sdk/server/standard"
	"gorm.io/gorm"

	commonv1 "github.com/go-sdk/example/gen/common/v1"
	appauth "github.com/go-sdk/example/internal/auth"
	"github.com/go-sdk/example/internal/model"
)

type FileHandler struct {
	db         *gorm.DB
	authorizer *appauth.Authorizer
	root       string
	maxBytes   int64
}

type fileResponse struct {
	ID           string `json:"id"`
	OriginalName string `json:"original_name"`
	MIMEType     string `json:"mime_type"`
	Size         int64  `json:"size"`
	URL          string `json:"url"`
}

func NewFileHandler(db *gorm.DB, authorizer *appauth.Authorizer, root string, maxBytes int64) *FileHandler {
	return &FileHandler{db: db, authorizer: authorizer, root: root, maxBytes: maxBytes}
}

func (h *FileHandler) Register(server *standard.Server) error {
	if err := os.MkdirAll(h.root, 0o750); err != nil {
		return errx.Wrap(err, "create storage directory")
	}
	routes := []struct {
		method, path string
		handler      standard.HandlerFunc
	}{
		{http.MethodPost, "/api/v1/files", h.Upload},
		{http.MethodGet, "/api/v1/files/{id}", h.Download},
		{http.MethodPut, "/api/v1/users/{id}/avatar", h.ReplaceAvatar},
	}
	for _, route := range routes {
		if err := server.HandlePath(route.method, route.path, route.handler); err != nil {
			return errx.Wrapf(err, "register %s %s", route.method, route.path)
		}
	}
	return nil
}

func (h *FileHandler) Upload(c *standard.Context) error {
	if err := h.authorizer.Require(c, "files.write"); err != nil {
		return err
	}
	value, path, err := h.receive(c)
	if err != nil {
		return err
	}
	if err = h.db.WithContext(c).Create(&value).Error; err != nil {
		_ = os.Remove(path)
		return err
	}
	return c.JSON(http.StatusCreated, map[string]any{"data": fileResponseOf(value)})
}

func (h *FileHandler) ReplaceAvatar(c *standard.Context) error {
	if err := h.authorizer.Require(c, "users.write"); err != nil {
		return err
	}
	var user model.User
	if err := h.db.WithContext(c).First(&user, "id = ?", c.Param("id")).Error; err != nil {
		return err
	}
	value, path, err := h.receive(c)
	if err != nil {
		return err
	}
	var oldFile model.File
	err = h.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&value).Error; err != nil {
			return err
		}
		if user.AvatarFileID != nil {
			if err := tx.First(&oldFile, "id = ?", *user.AvatarFileID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		return tx.Model(&user).Updates(map[string]any{
			"avatar_file_id": value.Id,
			"updated_by":     appauth.Subject(c),
		}).Error
	})
	if err != nil {
		_ = os.Remove(path)
		return err
	}
	h.cleanupReplacedAvatar(c, oldFile)
	return c.JSON(http.StatusOK, map[string]any{"data": fileResponseOf(value)})
}

func (h *FileHandler) Download(c *standard.Context) error {
	if err := h.authorizer.Require(c, "files.read"); err != nil {
		return err
	}
	var value model.File
	if err := h.db.WithContext(c).First(&value, "id = ?", c.Param("id")).Error; err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(h.root, value.StoredName))
	if errors.Is(err, os.ErrNotExist) {
		return standard.ErrNotFound.
			WithErrorCode(commonv1.ErrorCode_ERROR_CODE_FILE_NOT_FOUND).
			WithData(map[string]any{"Name": value.OriginalName})
	}
	if err != nil {
		return errx.Wrap(err, "read stored file")
	}
	c.SetHeader("Content-Length", strconv.Itoa(len(data)))
	c.SetHeader("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": value.OriginalName}))
	return c.Blob(http.StatusOK, value.MIMEType, data)
}

func (h *FileHandler) receive(c *standard.Context) (model.File, string, error) {
	var value model.File
	filename, data, err := c.ReadFormFile("file", h.maxBytes)
	if err != nil {
		return value, "", err
	}
	originalName := filepath.Base(strings.ReplaceAll(filename, `\`, "/"))
	if originalName == "" || originalName == "." || originalName == ".." {
		return value, "", standard.ErrInvalidParam.
			WithErrorCode(commonv1.ErrorCode_ERROR_CODE_INVALID_FILE_NAME)
	}
	storedName := seq.UUID() + strings.ToLower(filepath.Ext(originalName))
	path := filepath.Join(h.root, storedName)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return value, "", errx.Wrap(err, "create stored file")
	}
	_, writeErr := file.Write(data)
	closeErr := file.Close()
	if err = errx.Join(writeErr, closeErr); err != nil {
		_ = os.Remove(path)
		return value, "", errx.Wrap(err, "store uploaded file")
	}
	actor := appauth.Subject(c)
	value = model.File{
		Id:           seq.NextID(),
		CreatedBy:    actor,
		UpdatedBy:    actor,
		OwnerID:      actor,
		OriginalName: originalName,
		StoredName:   storedName,
		MIMEType:     http.DetectContentType(data),
		Size:         int64(len(data)),
	}
	return value, path, nil
}

func (h *FileHandler) cleanupReplacedAvatar(c *standard.Context, oldFile model.File) {
	if oldFile.Id == "" {
		return
	}
	if err := h.db.WithContext(c).Delete(&oldFile).Error; err != nil {
		logx.Ctx(c).Warn().Err(err).Str("file_id", oldFile.Id).Msg("delete replaced avatar record")
		return
	}
	if err := os.Remove(filepath.Join(h.root, oldFile.StoredName)); err != nil && !errors.Is(err, os.ErrNotExist) {
		logx.Ctx(c).Warn().Err(err).Str("file_id", oldFile.Id).Msg("delete replaced avatar file")
	}
}

func fileResponseOf(value model.File) fileResponse {
	return fileResponse{
		ID: value.Id, OriginalName: value.OriginalName, MIMEType: value.MIMEType,
		Size: value.Size, URL: "/api/v1/files/" + value.Id,
	}
}

var _ standard.HandlerFunc = (*FileHandler)(nil).Upload
var _ standard.HandlerFunc = (*FileHandler)(nil).Download
var _ standard.HandlerFunc = (*FileHandler)(nil).ReplaceAvatar
