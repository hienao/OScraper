package logging

import "testing"

func TestSanitizeFieldsRedactsNestedSecrets(t *testing.T) {
	fields := sanitizeFields(Fields{
		"connection_id": uint(3),
		"token":         "top-secret",
		"nested":        map[string]interface{}{"api_key": "hidden", "path": "/media"},
	})
	if fields["token"] != "[REDACTED]" {
		t.Fatal("token was not redacted")
	}
	nested := fields["nested"].(Fields)
	if nested["api_key"] != "[REDACTED]" || nested["path"] != "/media" {
		t.Fatalf("unexpected nested fields: %#v", nested)
	}
}
