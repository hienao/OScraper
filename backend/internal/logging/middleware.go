package logging

import (
	"strings"
	"time"

	"oscraper/internal/model"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if requestID == "" || len(requestID) > 100 || strings.ContainsAny(requestID, "\r\n") {
			requestID = "req-" + uuid.NewString()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func AccessLogMiddleware(manager *Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()
		if isLogManagementPath(c.Request.URL.Path) || strings.HasPrefix(c.Request.URL.Path, "/swagger") {
			return
		}
		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		manager.Submit(model.APIRequestLog{
			RequestID: contextString(c, "request_id"), OccurredAt: started, Method: c.Request.Method,
			Route: truncate(route, 255), StatusCode: c.Writer.Status(), LatencyMs: time.Since(started).Milliseconds(),
			RequestBytes: maxInt64(c.Request.ContentLength, 0), ResponseBytes: maxInt64(int64(c.Writer.Size()), 0),
			ClientIP: c.ClientIP(), UserAgent: truncate(c.GetHeader("User-Agent"), 500), UserID: contextUint(c, "user_id"),
			Username: contextString(c, "username"), ConnectionID: contextUint(c, "connection_id"),
			TargetID: contextUint(c, "target_id"), JobID: contextUint(c, "job_id"),
			ErrorCode: contextString(c, "error_code"), ErrorMessage: truncate(contextString(c, "error_message"), 1000),
		})
	}
}

func isLogManagementPath(value string) bool {
	return strings.HasPrefix(value, "/api/admin/logs") || value == "/api/admin/application-logs" || value == "/api/admin/audit-logs"
}

func contextString(c *gin.Context, key string) string {
	value, _ := c.Get(key)
	text, _ := value.(string)
	return text
}

func contextUint(c *gin.Context, key string) uint {
	value, _ := c.Get(key)
	number, _ := value.(uint)
	return number
}

func maxInt64(value, minimum int64) int64 {
	if value < minimum {
		return minimum
	}
	return value
}
