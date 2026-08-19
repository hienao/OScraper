package handler

import (
	"oscraper/internal/logging"
	"oscraper/internal/service"
	"oscraper/pkg/response"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct{ service *service.AuthService }

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{service: authService}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var request service.LoginRequest
	if !bindJSON(c, &request) {
		return
	}
	result, err := h.service.Login(request)
	if err != nil {
		logging.Warn("auth", "login failed", logging.Fields{"request_id": c.GetString("request_id"), "username": request.Username})
		respondError(c, err)
		return
	}
	logging.Info("auth", "login succeeded", logging.Fields{"request_id": c.GetString("request_id"), "username": request.Username})
	response.Success(c, result)
}

func (h *AuthHandler) SetupAdmin(c *gin.Context) {
	var request service.SetupAdminRequest
	if !bindJSON(c, &request) {
		return
	}
	result, err := h.service.SetupAdmin(currentUserID(c), request)
	if err != nil {
		respondError(c, err)
		return
	}
	logging.Info("auth", "administrator setup completed", logging.Fields{"request_id": c.GetString("request_id"), "user_id": currentUserID(c)})
	response.Success(c, result)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	if err := h.service.Logout(currentUserID(c)); err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *AuthHandler) Profile(c *gin.Context) {
	profile, err := h.service.Profile(currentUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, profile)
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var request service.ChangePasswordRequest
	if !bindJSON(c, &request) {
		return
	}
	if err := h.service.ChangePassword(currentUserID(c), request); err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, nil)
}
