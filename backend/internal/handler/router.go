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
	// handler 会直接把 *gin.Context 传给 pgx；开启后 c.Deadline()/c.Done() 才能看到 requestTimeout 设置的超时。
	r.ContextWithFallback = true
	_ = r.SetTrustedProxies(nil)
	// recovery 必须位于 gzip 内层：否则 panic 穿过 gzip 时，gzip 的 defer 会先写出 200 状态行，recovery 随后写的 500 不再生效。
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
