package handler

import (
	"strconv"

	"oscraper/internal/logging"
	"oscraper/internal/service"
	"oscraper/pkg/response"

	"github.com/gin-gonic/gin"
)

type TargetHandler struct{ service *service.TargetService }

func NewTargetHandler(targetService *service.TargetService) *TargetHandler {
	return &TargetHandler{service: targetService}
}

func (h *TargetHandler) List(c *gin.Context) {
	targets, err := h.service.List()
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, targets)
}

func (h *TargetHandler) Get(c *gin.Context) {
	id, ok := targetID(c)
	if !ok {
		return
	}
	target, err := h.service.Get(id)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, target)
}

func (h *TargetHandler) Create(c *gin.Context) {
	var request targetRequest
	if !bindJSON(c, &request) {
		return
	}
	target, err := h.service.Create(c.Request.Context(), currentUserID(c), request.command())
	if err != nil {
		respondError(c, err)
		return
	}
	c.Set("target_id", target.ID)
	logging.Info("catalog", "scrape target created", logging.Fields{"request_id": c.GetString("request_id"), "user_id": currentUserID(c), "target_id": target.ID, "connection_id": target.ConnectionID})
	response.Created(c, target)
}

func (h *TargetHandler) Update(c *gin.Context) {
	id, ok := targetID(c)
	if !ok {
		return
	}
	var request targetRequest
	if !bindJSON(c, &request) {
		return
	}
	target, err := h.service.Update(c.Request.Context(), id, currentUserID(c), request.command())
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, target)
}

func (h *TargetHandler) Delete(c *gin.Context) {
	id, ok := targetID(c)
	if !ok {
		return
	}
	if err := h.service.Delete(id, currentUserID(c)); err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, nil)
}

func (h *TargetHandler) Browse(c *gin.Context) {
	id, ok := targetID(c)
	if !ok {
		return
	}
	level, err := h.service.Browse(c.Request.Context(), id, c.Query("path"), c.Query("refresh") == "true")
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, level)
}

func (h *TargetHandler) BrowseConnection(c *gin.Context) {
	id, ok := connectionID(c)
	if !ok {
		return
	}
	level, err := h.service.BrowseConnection(c.Request.Context(), id, c.Query("path"), c.Query("refresh") == "true")
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, level)
}

func (h *TargetHandler) LocalStatus(c *gin.Context) {
	response.Success(c, h.service.LocalStatus())
}

func (h *TargetHandler) BrowseLocal(c *gin.Context) {
	level, err := h.service.BrowseLocal(c.Request.Context(), c.Query("path"))
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, level)
}

func targetID(c *gin.Context) (uint, bool) {
	value, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || value == 0 {
		response.BadRequest(c, "target.invalid_id", "Target ID is invalid")
		return 0, false
	}
	id := uint(value)
	c.Set("target_id", id)
	return id, true
}
