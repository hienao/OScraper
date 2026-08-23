package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Code      int         `json:"code"`
	ErrorCode string      `json:"error_code,omitempty"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{Code: 0, Message: "success", Data: data})
}

func Created(c *gin.Context, data interface{}) {
	c.JSON(http.StatusCreated, Response{Code: 0, Message: "success", Data: data})
}

func Accepted(c *gin.Context, data interface{}) {
	c.JSON(http.StatusAccepted, Response{Code: 0, Message: "accepted", Data: data})
}

func Error(c *gin.Context, status int, code, message string) {
	c.Set("error_code", code)
	c.Set("error_message", message)
	c.JSON(status, Response{Code: -1, ErrorCode: code, Message: message})
}

func BadRequest(c *gin.Context, code, message string) { Error(c, http.StatusBadRequest, code, message) }
func Unauthorized(c *gin.Context, code, message string) {
	Error(c, http.StatusUnauthorized, code, message)
}
func Forbidden(c *gin.Context, code, message string) { Error(c, http.StatusForbidden, code, message) }
func NotFound(c *gin.Context, code, message string)  { Error(c, http.StatusNotFound, code, message) }
func Conflict(c *gin.Context, code, message string)  { Error(c, http.StatusConflict, code, message) }
func Internal(c *gin.Context, code, message string) {
	Error(c, http.StatusInternalServerError, code, message)
}
