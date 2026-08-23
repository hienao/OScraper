package handler

import (
	"strconv"
	"strings"

	"oscraper/internal/service"
	"oscraper/pkg/response"

	"github.com/gin-gonic/gin"
)

type JobHandler struct{ service *service.JobService }

func NewJobHandler(jobService *service.JobService) *JobHandler {
	return &JobHandler{service: jobService}
}

func (h *JobHandler) Submit(c *gin.Context) {
	targetIDValue, ok := targetID(c)
	if !ok {
		return
	}
	var request submitJobRequest
	if !bindJSON(c, &request) {
		return
	}
	job, err := h.service.Submit(targetIDValue, currentUserID(c), request.command(), c.GetHeader("Idempotency-Key"))
	if err != nil {
		respondError(c, err)
		return
	}
	response.Created(c, job)
}

func (h *JobHandler) List(c *gin.Context) {
	page, size := pagination(c)
	var targetIDValue uint
	if raw := strings.TrimSpace(c.Query("target_id")); raw != "" {
		value, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || value == 0 {
			response.BadRequest(c, "target.invalid_id", "Target ID is invalid")
			return
		}
		targetIDValue = uint(value)
	}
	jobs, err := h.service.List(c.Query("status"), targetIDValue, page, size)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, jobs)
}

func (h *JobHandler) Get(c *gin.Context) {
	id, ok := positiveID(c, "id", "job.invalid_id")
	if !ok {
		return
	}
	job, err := h.service.Get(id)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, job)
}

func (h *JobHandler) Operations(c *gin.Context) {
	id, ok := positiveID(c, "id", "job.invalid_id")
	if !ok {
		return
	}
	operations, err := h.service.Operations(id)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, operations)
}

func (h *JobHandler) Retry(c *gin.Context) {
	id, ok := positiveID(c, "id", "job.invalid_id")
	if !ok {
		return
	}
	job, err := h.service.Retry(id, currentUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, job)
}

func (h *JobHandler) Cancel(c *gin.Context) {
	id, ok := positiveID(c, "id", "job.invalid_id")
	if !ok {
		return
	}
	job, err := h.service.Cancel(id, currentUserID(c))
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, job)
}
