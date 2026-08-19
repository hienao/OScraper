package handler

import (
	"strconv"

	"oscraper/internal/service"
	"oscraper/pkg/response"

	"github.com/gin-gonic/gin"
)

type CatalogHandler struct{ service *service.CatalogService }

func NewCatalogHandler(catalogService *service.CatalogService) *CatalogHandler {
	return &CatalogHandler{service: catalogService}
}

func (h *CatalogHandler) Scan(c *gin.Context) {
	id, ok := targetID(c)
	if !ok {
		return
	}
	result, err := h.service.Scan(c.Request.Context(), id, currentUserID(c), c.Query("refresh") == "true")
	if err != nil {
		respondError(c, err)
		return
	}
	response.Created(c, result)
}

func (h *CatalogHandler) GetScan(c *gin.Context) {
	targetIDValue, ok := targetID(c)
	if !ok {
		return
	}
	scanID, ok := positiveID(c, "scanId", "scan.invalid_id")
	if !ok {
		return
	}
	result, err := h.service.GetScan(targetIDValue, scanID)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *CatalogHandler) Candidates(c *gin.Context) {
	targetIDValue, ok := targetID(c)
	if !ok {
		return
	}
	var scanID uint
	if raw := c.Query("scan_id"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || parsed == 0 {
			response.BadRequest(c, "scan.invalid_id", "Scan ID is invalid")
			return
		}
		scanID = uint(parsed)
	}
	result, err := h.service.Candidates(targetIDValue, scanID)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, result)
}

func positiveID(c *gin.Context, parameter, errorCode string) (uint, bool) {
	value, err := strconv.ParseUint(c.Param(parameter), 10, 64)
	if err != nil || value == 0 {
		response.BadRequest(c, errorCode, "ID is invalid")
		return 0, false
	}
	return uint(value), true
}
