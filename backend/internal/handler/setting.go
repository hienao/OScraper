package handler

import (
	"oscraper/internal/service"
	"oscraper/pkg/response"

	"github.com/gin-gonic/gin"
)

type SettingHandler struct{ service *service.SettingService }

func NewSettingHandler(settingService *service.SettingService) *SettingHandler {
	return &SettingHandler{service: settingService}
}

func (h *SettingHandler) GetScraping(c *gin.Context) {
	settings, err := h.service.GetScraping()
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *SettingHandler) SaveScraping(c *gin.Context) {
	var request scrapingSettingsRequest
	if !bindJSON(c, &request) {
		return
	}
	settings, err := h.service.SaveScraping(currentUserID(c), request.command())
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *SettingHandler) TestTMDB(c *gin.Context) {
	if err := h.service.TestTMDB(c.Request.Context()); err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}

func (h *SettingHandler) TestAI(c *gin.Context) {
	if err := h.service.TestAI(c.Request.Context()); err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, gin.H{"ok": true})
}
