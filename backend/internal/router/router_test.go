package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"oscraper/config"
	"oscraper/internal/app"
	"oscraper/internal/logging"
	"oscraper/pkg/cryptoutil"
	"oscraper/pkg/database"
)

func TestAuthenticatedConnectionTargetAndScanFlow(t *testing.T) {
	openList := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/me":
			_, _ = writer.Write([]byte(`{"code":200,"data":{"username":"alice","base_path":"/media"}}`))
		case "/api/fs/list":
			var input struct {
				Path string `json:"path"`
			}
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
				t.Fatal(err)
			}
			switch input.Path {
			case "/media":
				_, _ = writer.Write([]byte(`{"code":200,"data":{"content":[{"name":"Movies","is_dir":true},{"name":"Harry Potter /DTS","is_dir":true}]}}`))
			case "/media/Movies":
				_, _ = writer.Write([]byte(`{"code":200,"data":{"content":[{"name":"Arrival (2016)","is_dir":true},{"name":"Lost Horizon (1937)","is_dir":true}]}}`))
			case "/media/Movies/Arrival (2016)":
				_, _ = writer.Write([]byte(`{"code":200,"data":{"content":[{"name":"Arrival.mkv","is_dir":false,"size":100,"modified":"2026-01-01"}]}}`))
			case "/media/Movies/Lost Horizon (1937)":
				_, _ = writer.Write([]byte(`{"code":200,"data":{"content":[{"name":"Lost Horizon.mkv","is_dir":false,"size":100,"modified":"2026-01-01"}]}}`))
			default:
				_, _ = writer.Write([]byte(`{"code":500,"message":"unexpected path"}`))
			}
		case "/3/configuration":
			if request.URL.Query().Get("api_key") != "tmdb-key" {
				writer.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = writer.Write([]byte(`{"images":{"secure_base_url":"https://image.tmdb.org/t/p/"}}`))
		case "/3/search/movie":
			switch request.URL.Query().Get("query") {
			case "Lost Horizon":
				_, _ = writer.Write([]byte(`{"results":[{"id":100,"title":"Lost Horizon","original_title":"Lost Horizon","release_date":"1937-03-02","vote_average":6.1},{"id":101,"title":"Lost Horizon","original_title":"Lost Horizon","release_date":"1973-03-11","vote_average":5.4}]}`))
			default:
				_, _ = writer.Write([]byte(`{"results":[{"id":329865,"title":"Arrival","original_title":"Arrival","release_date":"2016-11-10","vote_average":7.6}]}`))
			}
		case "/3/movie/329865":
			_, _ = writer.Write([]byte(`{"id":329865,"title":"Arrival","original_title":"Arrival","release_date":"2016-11-10","overview":"A linguist meets visitors.","poster_path":"/arrival.jpg","vote_average":7.6,"genres":[{"id":18,"name":"Drama"}]}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer openList.Close()

	temporary := t.TempDir()
	cfg := &config.Config{
		AppEnv: "test", GinMode: "test", ServerPort: "0",
		SQLitePath: filepath.Join(temporary, "business.db"), JWTSecret: "test-jwt-secret-012345678901234567890",
		AccessTokenHours: 1, CredentialEncryptionKey: "0123456789abcdef0123456789abcdef",
		APILogPath: filepath.Join(temporary, "logs.db"), APILogQueueSize: 100, APILogBatchSize: 10,
		HTTPTimeoutSeconds: 2,
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	logManager, err := logging.NewManager(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer logManager.Close()
	cipher, err := cryptoutil.New(cfg.CredentialEncryptionKey)
	if err != nil {
		t.Fatal(err)
	}
	application, err := app.New(cfg, db, logManager, cipher)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = application.Shutdown(context.Background()) }()
	server := httptest.NewServer(application.Engine)
	defer server.Close()
	health := requestJSON(t, server.URL, http.MethodGet, "/api/health/ready", "", nil, http.StatusOK)
	components := responseData(t, health)["components"].(map[string]any)
	if components["database"].(map[string]any)["ok"] != true || components["scans"] == nil || components["maintenance"] == nil {
		t.Fatalf("health response is missing runtime components: %s", health)
	}

	bootstrap := requestJSON(t, server.URL, http.MethodPost, "/api/auth/login", "", map[string]any{"username": "admin", "password": "admin"}, http.StatusOK)
	bootstrapToken := responseToken(t, bootstrap)
	setup := requestJSON(t, server.URL, http.MethodPost, "/api/auth/setup-admin", bootstrapToken, map[string]any{"username": "owner", "password": "secure-password"}, http.StatusOK)
	token := responseToken(t, setup)
	logSettings := requestJSON(t, server.URL, http.MethodGet, "/api/admin/logs/settings", token, nil, http.StatusOK)
	if days := responseData(t, logSettings)["retention_days"].(float64); days != 7 {
		t.Fatalf("unexpected default log retention: %s", logSettings)
	}
	logSettings = requestJSON(t, server.URL, http.MethodPut, "/api/admin/logs/settings", token, map[string]any{"retention_days": 3}, http.StatusOK)
	if days := responseData(t, logSettings)["retention_days"].(float64); days != 3 {
		t.Fatalf("log retention was not saved: %s", logSettings)
	}
	requestJSON(t, server.URL, http.MethodDelete, "/api/admin/logs/api", token, nil, http.StatusOK)
	jobSettings := requestJSON(t, server.URL, http.MethodGet, "/api/scrape-jobs/settings", token, nil, http.StatusOK)
	if days := responseData(t, jobSettings)["retention_days"].(float64); days != 7 {
		t.Fatalf("unexpected default job record retention: %s", jobSettings)
	}
	jobSettings = requestJSON(t, server.URL, http.MethodPut, "/api/scrape-jobs/settings", token, map[string]any{"retention_days": 5}, http.StatusOK)
	if days := responseData(t, jobSettings)["retention_days"].(float64); days != 5 {
		t.Fatalf("job record retention was not saved: %s", jobSettings)
	}
	jobs := requestJSON(t, server.URL, http.MethodGet, "/api/scrape-jobs", token, nil, http.StatusOK)
	if total := responseData(t, jobs)["total"].(float64); total != 0 {
		t.Fatalf("new database returned scrape jobs: %s", jobs)
	}

	connection := requestJSON(t, server.URL, http.MethodPost, "/api/openlist-connections", token, map[string]any{
		"name": "Home", "base_url": openList.URL, "token": "openlist-token", "qps_limit": 5, "qpm_limit": 120,
	}, http.StatusCreated)
	connectionID := responseID(t, connection)
	connectionTree := requestJSON(t, server.URL, http.MethodGet, fmt.Sprintf("/api/openlist-connections/%d/tree", connectionID), token, nil, http.StatusOK)
	connectionTreeData := responseData(t, connectionTree)
	if connectionTreeData["root_path"] != "/media" || connectionTreeData["path"] != "/media" || len(connectionTreeData["entries"].([]any)) != 1 {
		t.Fatalf("unexpected connection tree response: %s", connectionTree)
	}
	warnings := connectionTreeData["warnings"].([]any)
	if len(warnings) != 1 {
		t.Fatalf("unexpected connection tree warnings: %s", connectionTree)
	}
	warning := warnings[0].(map[string]any)
	if warning["code"] != "openlist.unsafe_entry_skipped" || warning["reason"] != "path_separator" || warning["invalid_character"] != "/" || warning["name"] != "Harry Potter /DTS" {
		t.Fatalf("unexpected connection tree warning: %#v", warning)
	}
	target := requestJSON(t, server.URL, http.MethodPost, "/api/scrape-targets", token, map[string]any{
		"connection_id": connectionID, "name": "Movies", "root_path": "/media/Movies", "library_type": "movie", "rename_enabled": true, "enabled": true,
	}, http.StatusCreated)
	targetID := responseID(t, target)
	scan := requestJSON(t, server.URL, http.MethodPost, fmt.Sprintf("/api/scrape-targets/%d/scans?refresh=true", targetID), token, nil, http.StatusAccepted)
	data := responseData(t, scan)
	scanID := uint(data["id"].(float64))
	deadline := time.Now().Add(5 * time.Second)
	for data["status"] == "pending" || data["status"] == "running" {
		if time.Now().After(deadline) {
			t.Fatalf("scan did not finish: %s", scan)
		}
		time.Sleep(20 * time.Millisecond)
		scan = requestJSON(t, server.URL, http.MethodGet, fmt.Sprintf("/api/scrape-targets/%d/scans/%d", targetID, scanID), token, nil, http.StatusOK)
		data = responseData(t, scan)
	}
	if data["status"] != "succeeded" || int(data["candidate_count"].(float64)) != 2 || int(data["video_count"].(float64)) != 2 {
		t.Fatalf("unexpected scan response: %s", scan)
	}
	candidates := data["candidates"].([]any)
	first := candidates[0].(map[string]any)
	if first["parsed_title"] != "Arrival" || int(first["year"].(float64)) != 2016 || first["fingerprint"] == "" {
		t.Fatalf("unexpected candidate: %#v", first)
	}
	candidateID := uint(first["id"].(float64))
	settingsBody := requestJSON(t, server.URL, http.MethodPut, "/api/settings/scraping", token, map[string]any{
		"api_key": "tmdb-key", "base_url": openList.URL, "image_base_url": openList.URL,
		"language": "en-US", "region": "US", "poster_size": "w500", "backdrop_size": "w1280", "timeout_seconds": 5,
	}, http.StatusOK)
	if bytes.Contains(settingsBody, []byte("tmdb-key")) {
		t.Fatalf("TMDB API key leaked in settings response: %s", settingsBody)
	}
	requestJSON(t, server.URL, http.MethodPost, "/api/settings/scraping/test-tmdb", token, nil, http.StatusOK)
	searchBody := requestJSON(t, server.URL, http.MethodPost, fmt.Sprintf("/api/scrape-targets/%d/previews/search", targetID), token, map[string]any{
		"candidate_id": candidateID, "title": "Arrival", "year": 2016,
	}, http.StatusOK)
	if !bytes.Contains(searchBody, []byte(`"id":329865`)) {
		t.Fatalf("unexpected TMDB search response: %s", searchBody)
	}
	previewBody := requestJSON(t, server.URL, http.MethodPost, fmt.Sprintf("/api/scrape-targets/%d/previews", targetID), token, map[string]any{
		"candidate_id": candidateID, "tmdb_id": 329865,
	}, http.StatusCreated)
	preview := responseData(t, previewBody)
	match := preview["match"].(map[string]any)
	plan := preview["plan"].(map[string]any)
	if int(match["id"].(float64)) != 329865 || plan["read_only"] != true || plan["ready"] != true || preview["fingerprint"] != first["fingerprint"] {
		t.Fatalf("unexpected preview: %s", previewBody)
	}
	if len(plan["proposed_directory_renames"].([]any)) != 1 || len(plan["proposed_file_renames"].([]any)) != 1 || len(plan["conflicts"].([]any)) != 0 {
		t.Fatalf("live directory was not expanded into a complete rename plan: %s", previewBody)
	}
	artifacts := plan["artifacts"].([]any)
	if len(artifacts) != 2 || !strings.Contains(artifacts[0].(map[string]any)["content"].(string), "<movie>") {
		t.Fatalf("metadata artifacts were not included in the immutable preview: %s", previewBody)
	}
	batchBody := requestJSON(t, server.URL, http.MethodPost, fmt.Sprintf("/api/scrape-targets/%d/batches", targetID), token, map[string]any{}, http.StatusAccepted)
	batch := responseData(t, batchBody)
	batchID := uint(batch["id"].(float64))
	if int(batch["total_count"].(float64)) != 2 {
		t.Fatalf("unexpected batch size: %s", batchBody)
	}
	deadline = time.Now().Add(10 * time.Second)
	for batch["status"] == "pending" || batch["status"] == "running" {
		if time.Now().After(deadline) {
			t.Fatalf("batch did not finish: %s", batchBody)
		}
		time.Sleep(20 * time.Millisecond)
		batchBody = requestJSON(t, server.URL, http.MethodGet, fmt.Sprintf("/api/scrape-targets/%d/batches/%d", targetID, batchID), token, nil, http.StatusOK)
		batch = responseData(t, batchBody)
	}
	if batch["status"] != "succeeded" || int(batch["submitted_count"].(float64)) != 1 || int(batch["skipped_count"].(float64)) != 1 {
		t.Fatalf("unexpected batch response: %s", batchBody)
	}
	submitted, ambiguous := false, false
	for _, raw := range batch["items"].([]any) {
		item := raw.(map[string]any)
		switch item["status"] {
		case "submitted":
			if item["path"] != "/media/Movies/Arrival (2016)" || item["job_id"] == nil || int(item["tmdb_id"].(float64)) != 329865 {
				t.Fatalf("unexpected submitted batch item: %#v", item)
			}
			submitted = true
		case "skipped":
			if item["skip_reason"] != "multiple_matches" {
				t.Fatalf("unexpected skipped batch item: %#v", item)
			}
			ambiguous = true
		}
	}
	if !submitted || !ambiguous {
		t.Fatalf("batch items are missing expected outcomes: %s", batchBody)
	}
	jobs = requestJSON(t, server.URL, http.MethodGet, "/api/scrape-jobs", token, nil, http.StatusOK)
	if total := responseData(t, jobs)["total"].(float64); total != 1 {
		t.Fatalf("batch did not submit exactly one scrape job: %s", jobs)
	}
}

func requestJSON(t *testing.T, baseURL, method, route, token string, input any, expectedStatus int) []byte {
	t.Helper()
	var body bytes.Buffer
	if input != nil {
		if err := json.NewEncoder(&body).Encode(input); err != nil {
			t.Fatal(err)
		}
	}
	request, err := http.NewRequest(method, baseURL+route, &body)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var output bytes.Buffer
	_, _ = output.ReadFrom(response.Body)
	if response.StatusCode != expectedStatus {
		t.Fatalf("%s %s returned %d, want %d: %s", method, route, response.StatusCode, expectedStatus, output.String())
	}
	return output.Bytes()
}

func responseData(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatal(err)
	}
	return envelope.Data
}

func responseToken(t *testing.T, body []byte) string {
	t.Helper()
	token, ok := responseData(t, body)["token"].(string)
	if !ok || token == "" {
		t.Fatalf("missing token: %s", body)
	}
	return token
}

func responseID(t *testing.T, body []byte) uint {
	t.Helper()
	value, ok := responseData(t, body)["id"].(float64)
	if !ok || value <= 0 {
		t.Fatalf("missing id: %s", body)
	}
	return uint(value)
}
