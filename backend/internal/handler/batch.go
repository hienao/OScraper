package handler

import (
	"oscraper/internal/service"
	"oscraper/pkg/response"

	"github.com/gin-gonic/gin"
)

type BatchHandler struct{ service *service.BatchScrapeService }

func NewBatchHandler(batchService *service.BatchScrapeService) *BatchHandler {
	return &BatchHandler{service: batchService}
}

func (h *BatchHandler) Create(c *gin.Context) {
	targetIDValue, ok := targetID(c)
	if !ok {
		return
	}
	var request createBatchRequest
	if c.Request.ContentLength > 0 && !bindJSON(c, &request) {
		return
	}
	batch, err := h.service.StartBatch(targetIDValue, currentUserID(c), request.command())
	if err != nil {
		respondError(c, err)
		return
	}
	response.Accepted(c, batch)
}

func (h *BatchHandler) Get(c *gin.Context) {
	targetIDValue, ok := targetID(c)
	if !ok {
		return
	}
	batchID, ok := positiveID(c, "batchId", "batch.invalid_id")
	if !ok {
		return
	}
	batch, err := h.service.GetBatch(targetIDValue, batchID)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, batch)
}

func (h *BatchHandler) Cancel(c *gin.Context) {
	targetIDValue, ok := targetID(c)
	if !ok {
		return
	}
	batchID, ok := positiveID(c, "batchId", "batch.invalid_id")
	if !ok {
		return
	}
	batch, err := h.service.Cancel(targetIDValue, batchID, currentUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, batch)
}
