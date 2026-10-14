package handler

import (
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

const geoJSONContentType = "application/geo+json"

func writeGeoJSON(c *gin.Context, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "encode geojson response", "err", err)
		writeInternal(c)
		return
	}
	c.Data(http.StatusOK, geoJSONContentType, body)
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
	default:
		slog.ErrorContext(c.Request.Context(), "request failed", "path", c.Request.URL.Path, "err", err)
		writeInternal(c)
	}
}
