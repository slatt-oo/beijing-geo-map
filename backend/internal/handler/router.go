package handler

import (
	"net/http"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
)

const (
	requestTimeoutDuration = 5 * time.Second
	gzipMinLength          = 1024
)

type Deps struct {
	DB Pinger
}

func NewRouter(deps Deps) *gin.Engine {
	r := gin.New()
	_ = r.SetTrustedProxies(nil)
	// recovery 必须位于 gzip 内层：panic 后写出的 500 响应要经过仍然有效的 gzip writer。
	r.Use(
		requestLogger(),
		gzip.Gzip(gzip.DefaultCompression, gzip.WithMinLength(gzipMinLength)),
		recovery(),
		requestTimeout(requestTimeoutDuration),
	)
	r.NoRoute(func(c *gin.Context) {
		writeError(c, http.StatusNotFound, CodeNotFound, "接口不存在")
	})

	api := r.Group("/api/v1")
	api.GET("/health", healthHandler(deps.DB))
	return r
}
