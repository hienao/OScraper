package handler

import (
	"strconv"
	"strings"

	"oscraper/internal/logging"
	"oscraper/internal/model"
	"oscraper/internal/service"
	"oscraper/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type LogHandler struct {
	manager    *logging.Manager
	businessDB *gorm.DB
	service    *service.LogService
}

type logPage struct {
	Items interface{} `json:"items"`
	Total int64       `json:"total"`
	Page  int         `json:"page"`
	Size  int         `json:"size"`
}

func NewLogHandler(manager *logging.Manager, businessDB *gorm.DB, logService *service.LogService) *LogHandler {
	return &LogHandler{manager: manager, businessDB: businessDB, service: logService}
}

type logSettingsRequest struct {
	RetentionDays int `json:"retention_days" binding:"required,min=1,max=30"`
}

func (h *LogHandler) Settings(c *gin.Context) {
	settings, err := h.service.Settings()
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *LogHandler) SaveSettings(c *gin.Context) {
	var request logSettingsRequest
	if !bindJSON(c, &request) {
		return
	}
	settings, err := h.service.SaveSettings(c.Request.Context(), currentUserID(c), request.RetentionDays)
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *LogHandler) Clear(c *gin.Context) {
	stats, err := h.service.Clear(c.Request.Context(), currentUserID(c), strings.TrimSpace(c.Param("type")))
	if err != nil {
		respondError(c, err)
		return
	}
	response.Success(c, stats)
}

func (h *LogHandler) API(c *gin.Context) {
	page, size := pagination(c)
	query := h.manager.DB.Model(&model.APIRequestLog{})
	if method := strings.TrimSpace(c.Query("method")); method != "" {
		query = query.Where("method = ?", strings.ToUpper(method))
	}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		query = query.Where("status_code = ?", status)
	}
	if q := logSearch(c); q != "" {
		like := "%" + q + "%"
		query = query.Where("route LIKE ? OR request_id LIKE ? OR error_code LIKE ? OR error_message LIKE ?", like, like, like, like)
	}
	var ok bool
	if query, ok = applyLogIDs(c, query); !ok {
		return
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		response.Internal(c, "logs.query_failed", "Failed to query API logs")
		return
	}
	var items []model.APIRequestLog
	if err := query.Order("occurred_at DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		response.Internal(c, "logs.query_failed", "Failed to query API logs")
		return
	}
	response.Success(c, logPage{Items: items, Total: total, Page: page, Size: size})
}

func (h *LogHandler) Application(c *gin.Context) {
	page, size := pagination(c)
	query := h.manager.DB.Model(&model.ApplicationLog{})
	if level := strings.TrimSpace(c.Query("level")); level != "" {
		query = query.Where("level = ?", strings.ToUpper(level))
	}
	if source := strings.TrimSpace(c.Query("source")); source != "" {
		query = query.Where("source = ?", source)
	}
	if q := logSearch(c); q != "" {
		like := "%" + q + "%"
		query = query.Where("message LIKE ? OR fields LIKE ? OR request_id LIKE ?", like, like, like)
	}
	var ok bool
	if query, ok = applyLogIDs(c, query); !ok {
		return
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		response.Internal(c, "logs.query_failed", "Failed to query application logs")
		return
	}
	var items []model.ApplicationLog
	if err := query.Order("occurred_at DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		response.Internal(c, "logs.query_failed", "Failed to query application logs")
		return
	}
	response.Success(c, logPage{Items: items, Total: total, Page: page, Size: size})
}

func (h *LogHandler) Audit(c *gin.Context) {
	page, size := pagination(c)
	query := h.businessDB.Model(&model.AdminAuditLog{})
	if action := strings.TrimSpace(c.Query("action")); action != "" {
		query = query.Where("action = ?", action)
	}
	if q := logSearch(c); q != "" {
		like := "%" + q + "%"
		query = query.Where("action LIKE ? OR target LIKE ? OR detail LIKE ?", like, like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		response.Internal(c, "logs.query_failed", "Failed to query audit logs")
		return
	}
	var items []model.AdminAuditLog
	if err := query.Order("occurred_at DESC").Offset((page - 1) * size).Limit(size).Find(&items).Error; err != nil {
		response.Internal(c, "logs.query_failed", "Failed to query audit logs")
		return
	}
	response.Success(c, logPage{Items: items, Total: total, Page: page, Size: size})
}

func logSearch(c *gin.Context) string {
	value := strings.TrimSpace(c.Query("q"))
	if len(value) > 200 {
		return value[:200]
	}
	return value
}

func applyLogIDs(c *gin.Context, query *gorm.DB) (*gorm.DB, bool) {
	for _, filter := range []struct {
		query string
		field string
		code  string
	}{{"job_id", "job_id", "job.invalid_id"}, {"target_id", "target_id", "target.invalid_id"}} {
		raw := strings.TrimSpace(c.Query(filter.query))
		if raw == "" {
			continue
		}
		value, err := strconv.ParseUint(raw, 10, 64)
		if err != nil || value == 0 {
			response.BadRequest(c, filter.code, "Log filter ID is invalid")
			return query, false
		}
		query = query.Where(filter.field+" = ?", value)
	}
	return query, true
}

func pagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "50"))
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 50
	}
	if size > 200 {
		size = 200
	}
	return page, size
}
