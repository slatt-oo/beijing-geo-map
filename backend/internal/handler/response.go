package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
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
