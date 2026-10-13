package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

func healthHandler(db Pinger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := db.Ping(c.Request.Context()); err != nil {
			slog.ErrorContext(c.Request.Context(), "health check: database ping failed", "err", err)
			writeError(c, http.StatusServiceUnavailable, CodeUnavailable, "数据库不可用")
			return
		}
		writeData(c, gin.H{"status": "ok", "db": "ok"})
	}
}
