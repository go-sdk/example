package httpapi

import (
	"net/http"

	"github.com/go-sdk/app"

	appconfig "github.com/go-sdk/example/internal/config"
)

func init() {
	handler := New(appconfig.G())
	app.RegisterBootstrap(handler.prepareStorage)
	app.RegisterRoute(http.MethodPost, "/api/v1/files", handler.uploadFile)
	app.RegisterRoute(http.MethodGet, "/api/v1/files/{id}", handler.downloadFile)
	app.RegisterRoute(http.MethodPut, "/api/v1/users/{id}/avatar", handler.replaceAvatar)
}
