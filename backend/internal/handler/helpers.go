package handler

import (
	"errors"

	"openlistscraper/internal/service"
	"openlistscraper/pkg/response"

	"github.com/gin-gonic/gin"
)

func respondError(c *gin.Context, err error) {
	var serviceError *service.Error
	if errors.As(err, &serviceError) {
		response.Error(c, serviceError.Status, serviceError.Code, serviceError.Message)
		return
	}
	response.Internal(c, "internal.error", "An unexpected error occurred")
}

func bindJSON(c *gin.Context, destination interface{}) bool {
	if err := c.ShouldBindJSON(destination); err != nil {
		response.BadRequest(c, "request.invalid", "Request data is invalid")
		return false
	}
	return true
}

func currentUserID(c *gin.Context) uint {
	value, _ := c.Get("user_id")
	userID, _ := value.(uint)
	return userID
}
