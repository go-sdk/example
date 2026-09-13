package httptransport

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-sdk/core/errx"
	"github.com/go-sdk/core/logx"
	"github.com/go-sdk/core/seq"
	"github.com/go-sdk/server/standard"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"gorm.io/gorm"

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
		handler      runtime.HandlerFunc
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

func (h *FileHandler) Upload(w http.ResponseWriter, r *http.Request, _ map[string]string) {
	if err := h.authorizer.Require(r.Context(), "files.write"); err != nil {
		writeError(w, err)
		return
	}
	value, path, err := h.receive(w, r)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := h.db.WithContext(r.Context()).Create(&value).Error; err != nil {
		_ = os.Remove(path)
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, fileResponseOf(value))
}

func (h *FileHandler) ReplaceAvatar(w http.ResponseWriter, r *http.Request, params map[string]string) {
	if err := h.authorizer.Require(r.Context(), "users.write"); err != nil {
		writeError(w, err)
		return
	}
	userID := params["id"]
	var user model.User
	if err := h.db.WithContext(r.Context()).First(&user, "id = ?", userID).Error; err != nil {
		writeError(w, err)
		return
	}
	value, path, err := h.receive(w, r)
	if err != nil {
		writeError(w, err)
		return
	}
	var oldFile model.File
	err = h.db.WithContext(r.Context()).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&value).Error; err != nil {
			return err
		}
		if user.AvatarFileID != nil {
			if err := tx.First(&oldFile, "id = ?", *user.AvatarFileID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		return tx.Model(&user).Updates(map[string]any{
			"avatar_file_id": value.Id, "updated_by": appauth.Subject(r.Context()),
		}).Error
	})
	if err != nil {
		_ = os.Remove(path)
		writeError(w, err)
		return
	}
	if oldFile.Id != "" {
		if err := h.db.WithContext(r.Context()).Delete(&oldFile).Error; err != nil {
			logx.Ctx(r.Context()).Warn().Err(err).Str("file_id", oldFile.Id).Msg("delete replaced avatar record")
		} else if err := os.Remove(filepath.Join(h.root, oldFile.StoredName)); err != nil && !errors.Is(err, os.ErrNotExist) {
			logx.Ctx(r.Context()).Warn().Err(err).Str("file_id", oldFile.Id).Msg("delete replaced avatar file")
		}
	}
	writeJSON(w, http.StatusOK, fileResponseOf(value))
}

func (h *FileHandler) Download(w http.ResponseWriter, r *http.Request, params map[string]string) {
	if err := h.authorizer.Require(r.Context(), "files.read"); err != nil {
		writeError(w, err)
		return
	}
	var value model.File
	if err := h.db.WithContext(r.Context()).First(&value, "id = ?", params["id"]).Error; err != nil {
		writeError(w, err)
		return
	}
	file, err := os.Open(filepath.Join(h.root, value.StoredName))
	if err != nil {
		writeError(w, err)
		return
	}
	defer func() { _ = file.Close() }()
	w.Header().Set("Content-Type", value.MIMEType)
	w.Header().Set("Content-Length", fmt.Sprintf("%d", value.Size))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": value.OriginalName}))
	if _, err := io.Copy(w, file); err != nil {
		return
	}
}

func (h *FileHandler) receive(w http.ResponseWriter, r *http.Request) (model.File, string, error) {
	var value model.File
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBytes)
	reader, err := r.MultipartReader()
	if err != nil {
		return value, "", errx.Wrap(err, "read multipart request")
	}
	var part *multipart.Part
	for {
		part, err = reader.NextPart()
		if errors.Is(err, io.EOF) {
			return value, "", errx.New("multipart field file is required")
		}
		if err != nil {
			return value, "", errx.Wrap(err, "read uploaded file")
		}
		if part.FormName() == "file" && part.FileName() != "" {
			break
		}
		if err := part.Close(); err != nil {
			return value, "", errx.Wrap(err, "close multipart field")
		}
	}
	defer func() { _ = part.Close() }()
	originalName := filepath.Base(part.FileName())
	extension := strings.ToLower(filepath.Ext(originalName))
	storedName := seq.UUID() + extension
	path := filepath.Join(h.root, storedName)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return value, "", errx.Wrap(err, "create stored file")
	}
	size, copyErr := io.Copy(file, part)
	closeErr := file.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(path)
		return value, "", errx.Wrap(errors.Join(copyErr, closeErr), "store uploaded file")
	}
	value = model.File{
		Id: seq.NextID(), CreatedBy: appauth.Subject(r.Context()), UpdatedBy: appauth.Subject(r.Context()),
		OwnerID: appauth.Subject(r.Context()), OriginalName: originalName, StoredName: storedName,
		MIMEType: part.Header.Get("Content-Type"), Size: size,
	}
	if value.MIMEType == "" {
		value.MIMEType = "application/octet-stream"
	}
	return value, path, nil
}

func fileResponseOf(value model.File) fileResponse {
	return fileResponse{
		ID: value.Id, OriginalName: value.OriginalName, MIMEType: value.MIMEType,
		Size: value.Size, URL: "/api/v1/files/" + value.Id,
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
}

func writeError(w http.ResponseWriter, err error) {
	httpStatus := http.StatusInternalServerError
	var maxBytesError *http.MaxBytesError
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		httpStatus = http.StatusNotFound
	case errors.As(err, &maxBytesError):
		httpStatus = http.StatusRequestEntityTooLarge
	case strings.Contains(err.Error(), "multipart") || strings.Contains(err.Error(), "uploaded file"):
		httpStatus = http.StatusBadRequest
	case grpcstatus.Code(err) == codes.Unauthenticated:
		httpStatus = http.StatusUnauthorized
	case grpcstatus.Code(err) == codes.PermissionDenied:
		httpStatus = http.StatusForbidden
	}
	http.Error(w, http.StatusText(httpStatus), httpStatus)
}

var _ runtime.HandlerFunc = (*FileHandler)(nil).Upload
