package handler

import (
	"strconv"

	"oscraper/internal/logging"
	"oscraper/internal/service"
	"oscraper/pkg/response"

	"github.com/gin-gonic/gin"
)

type ConnectionHandler struct{ service *service.ConnectionService }

func NewConnectionHandler(connectionService *service.ConnectionService) *ConnectionHandler {
	return &ConnectionHandler{service: connectionService}
}

func (h *ConnectionHandler) List(c *gin.Context) {
	connections, err := h.service.List()
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, connections)
}

func (h *ConnectionHandler) Get(c *gin.Context) {
	id, ok := connectionID(c)
	if !ok {
		return
	}
	connection, err := h.service.Get(id)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, connection)
}

func (h *ConnectionHandler) Test(c *gin.Context) {
	var request service.TestConnectionRequest
	if !bindJSON(c, &request) {
		return
	}
	result, err := h.service.Test(c.Request.Context(), request)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ConnectionHandler) Create(c *gin.Context) {
	var request service.ConnectionRequest
	if !bindJSON(c, &request) {
		return
	}
	connection, err := h.service.Create(c.Request.Context(), currentUserID(c), request)
	if err != nil {
		respondError(c, err)
		return
	}
	c.Set("connection_id", connection.ID)
	logging.Info("openlist", "connection created", logging.Fields{"request_id": c.GetString("request_id"), "user_id": currentUserID(c), "connection_id": connection.ID})
	response.Created(c, connection)
}

func (h *ConnectionHandler) Update(c *gin.Context) {
	id, ok := connectionID(c)
	if !ok {
		return
	}
	var request service.ConnectionUpdateRequest
	if !bindJSON(c, &request) {
		return
	}
	connection, err := h.service.Update(id, currentUserID(c), request)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, connection)
}

func (h *ConnectionHandler) TestSaved(c *gin.Context) {
	id, ok := connectionID(c)
	if !ok {
		return
	}
	result, err := h.service.TestSaved(c.Request.Context(), id)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, result)
}

func (h *ConnectionHandler) RotateToken(c *gin.Context) {
	id, ok := connectionID(c)
	if !ok {
		return
	}
	var request service.TokenRequest
	if !bindJSON(c, &request) {
		return
	}
	connection, err := h.service.RotateToken(c.Request.Context(), id, currentUserID(c), request)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, connection)
}

func (h *ConnectionHandler) Delete(c *gin.Context) {
	id, ok := connectionID(c)
	if !ok {
		return
	}
	if err := h.service.Delete(id, currentUserID(c)); err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, nil)
}

func connectionID(c *gin.Context) (uint, bool) {
	value, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || value == 0 {
		response.BadRequest(c, "connection.invalid_id", "Connection ID is invalid")
		return 0, false
	}
	id := uint(value)
	c.Set("connection_id", id)
	return id, true
}
