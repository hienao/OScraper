package logging

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"oscraper/internal/model"
)

type Fields map[string]interface{}

var defaultManager atomic.Pointer[Manager]

func SetDefaultManager(manager *Manager)          { defaultManager.Store(manager) }
func Debug(source, message string, fields Fields) { write("DEBUG", source, message, fields) }
func Info(source, message string, fields Fields)  { write("INFO", source, message, fields) }
func Warn(source, message string, fields Fields)  { write("WARN", source, message, fields) }
func Error(source, message string, fields Fields) { write("ERROR", source, message, fields) }

func write(level, source, message string, fields Fields) {
	safe := sanitizeFields(fields)
	encoded, err := json.Marshal(safe)
	if err != nil {
		encoded = []byte(`{"logging_error":"failed to encode fields"}`)
	}
	log.Printf("[%s] [%s] %s %s", level, source, message, encoded)
	manager := defaultManager.Load()
	if manager == nil {
		return
	}
	manager.SubmitApplication(model.ApplicationLog{
		OccurredAt: time.Now(), Level: level, Source: truncate(source, 100), Message: truncate(message, 4000),
		Fields: string(encoded), RequestID: fieldString(safe, "request_id"), UserID: fieldUint(safe, "user_id"),
		ConnectionID: fieldUint(safe, "connection_id"), TargetID: fieldUint(safe, "target_id"), JobID: fieldUint(safe, "job_id"),
	})
}

func sanitizeFields(fields Fields) Fields {
	result := make(Fields, len(fields))
	for key, value := range fields {
		if sensitiveKey(key) {
			result[key] = "[REDACTED]"
			continue
		}
		switch typed := value.(type) {
		case Fields:
			result[key] = sanitizeFields(typed)
		case map[string]interface{}:
			result[key] = sanitizeFields(Fields(typed))
		case error:
			result[key] = typed.Error()
		default:
			result[key] = typed
		}
	}
	return result
}

func sensitiveKey(key string) bool {
	lower := strings.ToLower(key)
	for _, part := range []string{"authorization", "cookie", "token", "secret", "password", "signature", "api-key", "api_key", "apikey"} {
		if strings.Contains(lower, part) {
			return true
		}
	}
	return false
}

func fieldString(fields Fields, key string) string {
	value, exists := fields[key]
	if !exists || value == nil {
		return ""
	}
	return truncate(fmt.Sprint(value), 255)
}
func fieldUint(fields Fields, key string) uint {
	switch value := fields[key].(type) {
	case uint:
		return value
	case int:
		if value > 0 {
			return uint(value)
		}
	case float64:
		if value > 0 {
			return uint(value)
		}
	}
	return 0
}

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
