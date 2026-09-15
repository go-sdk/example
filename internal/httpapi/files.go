package httpapi

import (
	"context"
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
	"github.com/go-sdk/database/dbx"
	"github.com/go-sdk/server/standard"

	appauth "github.com/go-sdk/example/internal/auth"
	appconfig "github.com/go-sdk/example/internal/config"
	"github.com/go-sdk/example/internal/model"
	commonv1 "github.com/go-sdk/example/pb/common/v1"
)

type Handler struct {
	config appconfig.Config
}

type fileResponse struct {
	Id           string `json:"id"`
	OriginalName string `json:"original_name"`
	MIMEType     string `json:"mime_type"`
	Size         int64  `json:"size"`
	URL          string `json:"url"`
}

func New(config appconfig.Config) *Handler {
	return &Handler{
		config: config,
	}
}

func (h *Handler) prepareStorage(_ context.Context, _ *dbx.DB) error {
	if err := os.MkdirAll(h.config.Storage.Root, 0o750); err != nil {
		return errx.Wrap(err, "create storage directory")
	}
	return nil
}

func (h *Handler) uploadFile(c *standard.Context) error {
	if err := appauth.Require(c, "files.write"); err != nil {
		return err
	}
	value, path, err := h.receiveFile(c)
	if err != nil {
		return err
	}
	if err = model.CreateFile(c, &value); err != nil {
		_ = os.Remove(path)
		return err
	}
	return c.JSON(http.StatusCreated, map[string]any{
		"data": fileResponseOf(value),
	})
}

func (h *Handler) replaceAvatar(c *standard.Context) error {
	if err := appauth.Require(c, "users.write"); err != nil {
		return err
	}
	user, err := model.GetUser(c, c.Param("id"))
	if err != nil {
		return err
	}
	value, path, err := h.receiveFile(c)
	if err != nil {
		return err
	}
	oldFile, err := model.ReplaceAvatar(c, &user, &value, appauth.Subject(c))
	if err != nil {
		_ = os.Remove(path)
		return err
	}
	h.cleanupReplacedAvatar(c, oldFile)
	return c.JSON(http.StatusOK, map[string]any{
		"data": fileResponseOf(value),
	})
}

func (h *Handler) downloadFile(c *standard.Context) error {
	if err := appauth.Require(c, "files.read"); err != nil {
		return err
	}
	value, err := model.GetFile(c, c.Param("id"))
	if err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(h.config.Storage.Root, value.StoredName))
	if errors.Is(err, os.ErrNotExist) {
		return standard.ErrNotFound.WithErrorCode(commonv1.ErrorCode_ERROR_CODE_FILE_NOT_FOUND).WithData(map[string]any{
			"Name": value.OriginalName,
		})
	}
	if err != nil {
		return errx.Wrap(err, "read stored file")
	}
	c.SetHeader("Content-Length", strconv.Itoa(len(data)))
	c.SetHeader("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{
		"filename": value.OriginalName,
	}))
	return c.Blob(http.StatusOK, value.MIMEType, data)
}

func (h *Handler) receiveFile(c *standard.Context) (model.File, string, error) {
	var value model.File
	filename, data, err := c.ReadFormFile("file", h.config.Storage.MaxUploadBytes)
	if err != nil {
		return value, "", err
	}
	originalName := filepath.Base(strings.ReplaceAll(filename, `\`, "/"))
	if originalName == "" || originalName == "." || originalName == ".." {
		return value, "", standard.ErrInvalidParam.WithErrorCode(commonv1.ErrorCode_ERROR_CODE_INVALID_FILE_NAME)
	}
	storedName := seq.UUID() + strings.ToLower(filepath.Ext(originalName))
	path := filepath.Join(h.config.Storage.Root, storedName)
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
		OwnerId:      actor,
		OriginalName: originalName,
		StoredName:   storedName,
		MIMEType:     http.DetectContentType(data),
		Size:         int64(len(data)),
	}
	return value, path, nil
}

func (h *Handler) cleanupReplacedAvatar(c *standard.Context, oldFile model.File) {
	if oldFile.Id == "" {
		return
	}
	if err := model.DeleteFile(c, &oldFile); err != nil {
		logx.Ctx(c).Warn().Err(err).Str("file_id", oldFile.Id).Msg("delete replaced avatar record")
		return
	}
	if err := os.Remove(filepath.Join(h.config.Storage.Root, oldFile.StoredName)); err != nil && !errors.Is(err, os.ErrNotExist) {
		logx.Ctx(c).Warn().Err(err).Str("file_id", oldFile.Id).Msg("delete replaced avatar file")
	}
}

func fileResponseOf(value model.File) fileResponse {
	return fileResponse{
		Id:           value.Id,
		OriginalName: value.OriginalName,
		MIMEType:     value.MIMEType,
		Size:         value.Size,
		URL:          "/api/v1/files/" + value.Id,
	}
}
