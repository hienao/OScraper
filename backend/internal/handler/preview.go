package handler

import (
	"openlistscraper/internal/service"
	"openlistscraper/pkg/response"

	"github.com/gin-gonic/gin"
)

type PreviewHandler struct{ service *service.PreviewService }

func NewPreviewHandler(previewService *service.PreviewService) *PreviewHandler {
	return &PreviewHandler{service: previewService}
}

func (h *PreviewHandler) Search(c *gin.Context) {
	targetIDValue, ok := targetID(c)
	if !ok {
		return
	}
	var request service.PreviewSearchRequest
	if !bindJSON(c, &request) {
		return
	}
	results, err := h.service.Search(c.Request.Context(), targetIDValue, request)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, results)
}

func (h *PreviewHandler) Create(c *gin.Context) {
	targetIDValue, ok := targetID(c)
	if !ok {
		return
	}
	var request service.CreatePreviewRequest
	if !bindJSON(c, &request) {
		return
	}
	preview, err := h.service.Create(c.Request.Context(), targetIDValue, currentUserID(c), request)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Created(c, preview)
}

func (h *PreviewHandler) Get(c *gin.Context) {
	targetIDValue, ok := targetID(c)
	if !ok {
		return
	}
	previewID, ok := positiveID(c, "previewId", "preview.invalid_id")
	if !ok {
		return
	}
	preview, err := h.service.Get(targetIDValue, previewID)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, preview)
}
