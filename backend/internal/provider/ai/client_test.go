package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRecognizeUsesStructuredOutputAndValidatesResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/chat/completions" || request.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("unexpected AI request: %s", request.URL.Path)
		}
		var payload struct {
			ResponseFormat map[string]any `json:"response_format"`
			Messages       []struct {
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload.ResponseFormat["type"] != "json_schema" || len(payload.Messages) != 2 {
			t.Fatalf("structured output was not requested: %#v", payload)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"success\":true,\"titleCandidates\":[\"The Show\"],\"title\":\"The Show\",\"year\":\"2024\",\"season\":1,\"episode\":2,\"type\":\"tv\",\"reason\":null}"}}]}`))
	}))
	defer server.Close()

	result, err := NewClient().Recognize(context.Background(), Config{
		Enabled: true, APIKey: "secret", BaseURL: server.URL + "/v1", Model: "test-model", Timeout: time.Second,
	}, "The.Show.S01E02.mkv", "The Show/Season 1/The.Show.S01E02.mkv", "tv")
	if err != nil {
		t.Fatal(err)
	}
	if result.Title != "The Show" || result.Year == nil || *result.Year != 2024 || result.Season == nil || *result.Season != 1 || result.Episode == nil || *result.Episode != 2 {
		t.Fatalf("unexpected AI recognition result: %#v", result)
	}
}

func TestRecognizeFallsBackWhenJSONSchemaIsUnsupported(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls++
		var payload struct {
			ResponseFormat map[string]any `json:"response_format"`
		}
		_ = json.NewDecoder(request.Body).Decode(&payload)
		if payload.ResponseFormat["type"] == "json_schema" {
			writer.WriteHeader(http.StatusBadRequest)
			_, _ = writer.Write([]byte(`{"error":{"message":"response_format json_schema is not supported"}}`))
			return
		}
		_, _ = writer.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"content":"{\"success\":false,\"titleCandidates\":[],\"title\":null,\"year\":null,\"season\":null,\"episode\":null,\"type\":\"unknown\",\"reason\":\"insufficient filename data\"}"}}]}`))
	}))
	defer server.Close()

	result, err := NewClient().Recognize(context.Background(), Config{Enabled: true, APIKey: "secret", BaseURL: server.URL, Model: "test", Timeout: time.Second}, "x.mkv", "x.mkv", "movie")
	if err != nil {
		t.Fatal(err)
	}
	if result.Success || calls != 2 {
		t.Fatalf("unexpected fallback result: %#v after %d calls", result, calls)
	}
}
