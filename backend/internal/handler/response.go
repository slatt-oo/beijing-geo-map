package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/slatt-oo/beijing-geo-map/backend/internal/service"
)

const (
	CodeInvalidParam = "INVALID_PARAM"
	CodeNotFound     = "NOT_FOUND"
	CodeInternal     = "INTERNAL"
	CodeUnavailable  = "UNAVAILABLE"
)

const internalErrorMessage = "服务器内部错误"

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeData(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"data": data})
}

func writeError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": errorBody{Code: code, Message: message}})
}

func writeInternal(c *gin.Context) {
	writeError(c, http.StatusInternalServerError, CodeInternal, internalErrorMessage)
}

// statusClientClosedRequest 沿用 nginx 的约定，表示客户端在响应之前断开了连接（前端移动地图后会取消上一次查询）。
// 这个状态码只出现在访问日志里，客户端收不到。
const statusClientClosedRequest = 499

const geoJSONContentType = "application/geo+json"

// writeGeoJSON 不用 c.Data：c.Data 会先设置 Content-Length，gin-contrib/gzip 看到小于 1 KB 的 Content-Length 时
// 直接写出未压缩的内容，却不删除已经加上的 Content-Encoding: gzip，浏览器解码就会失败。
func writeGeoJSON(c *gin.Context, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "encode geojson response", "err", err)
		writeInternal(c)
		return
	}
	c.Header("Content-Type", geoJSONContentType)
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write(body)
}

func writeParamError(c *gin.Context, err error) {
	writeError(c, http.StatusBadRequest, CodeInvalidParam, err.Error())
}

// writeServiceError 把 service 返回的错误映射为 HTTP 响应。notFoundMessage 为空时，ErrNotFound 也按内部错误处理。
func writeServiceError(c *gin.Context, err error, notFoundMessage string) {
	var unknown service.UnknownCategoryError
	switch {
	case errors.As(err, &unknown):
		writeError(c, http.StatusBadRequest, CodeInvalidParam, "未知的分类编码："+unknown.Code)
	case notFoundMessage != "" && errors.Is(err, service.ErrNotFound):
		writeError(c, http.StatusNotFound, CodeNotFound, notFoundMessage)
	// 只看 Canceled：请求超时（DeadlineExceeded）是服务端的问题，要按内部错误记录。
	case errors.Is(err, context.Canceled) && errors.Is(c.Request.Context().Err(), context.Canceled):
		c.AbortWithStatus(statusClientClosedRequest)
	default:
		slog.ErrorContext(c.Request.Context(), "request failed", "path", c.Request.URL.Path, "err", err)
		writeInternal(c)
	}
}
