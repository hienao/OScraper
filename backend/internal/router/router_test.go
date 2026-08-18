package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"openlistscraper/config"
	"openlistscraper/internal/logging"
	"openlistscraper/pkg/cryptoutil"
	"openlistscraper/pkg/database"
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
			case "/media/Movies":
				_, _ = writer.Write([]byte(`{"code":200,"data":{"content":[{"name":"Arrival (2016)","is_dir":true}]}}`))
			case "/media/Movies/Arrival (2016)":
				_, _ = writer.Write([]byte(`{"code":200,"data":{"content":[{"name":"Arrival.mkv","is_dir":false,"size":100,"modified":"2026-01-01"}]}}`))
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
			_, _ = writer.Write([]byte(`{"results":[{"id":329865,"title":"Arrival","original_title":"Arrival","release_date":"2016-11-10","vote_average":7.6}]}`))
		case "/3/movie/329865":
			_, _ = writer.Write([]byte(`{"id":329865,"title":"Arrival","original_title":"Arrival","release_date":"2016-11-10","overview":"A linguist meets visitors.","poster_path":"/arrival.jpg","vote_average":7.6,"genres":[{"id":18,"name":"Drama"}]}`))
		default:
			writer.WriteHeader(http.StatusNotFound)
		}
	}))
	defer openList.Close()

	temporary := t.TempDir()
	cfg := &config.Config{
		AppEnv: "test", GinMode: "test", ServerPort: "0", DBDriver: "sqlite",
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
	server := httptest.NewServer(Setup(cfg, db, logManager, cipher))
	defer server.Close()

	bootstrap := requestJSON(t, server.URL, http.MethodPost, "/api/auth/login", "", map[string]any{"username": "admin", "password": "admin"}, http.StatusOK)
	bootstrapToken := responseToken(t, bootstrap)
	setup := requestJSON(t, server.URL, http.MethodPost, "/api/auth/setup-admin", bootstrapToken, map[string]any{"username": "owner", "password": "secure-password"}, http.StatusOK)
	token := responseToken(t, setup)

	connection := requestJSON(t, server.URL, http.MethodPost, "/api/openlist-connections", token, map[string]any{
		"name": "Home", "base_url": openList.URL, "token": "openlist-token", "qps_limit": 5, "qpm_limit": 120,
	}, http.StatusCreated)
	connectionID := responseID(t, connection)
	target := requestJSON(t, server.URL, http.MethodPost, "/api/scrape-targets", token, map[string]any{
		"connection_id": connectionID, "name": "Movies", "root_path": "/media/Movies", "library_type": "movie", "rename_enabled": false, "enabled": true,
	}, http.StatusCreated)
	targetID := responseID(t, target)
	scan := requestJSON(t, server.URL, http.MethodPost, fmt.Sprintf("/api/scrape-targets/%d/scans?refresh=true", targetID), token, nil, http.StatusCreated)
	data := responseData(t, scan)
	if data["status"] != "succeeded" || int(data["candidate_count"].(float64)) != 1 || int(data["video_count"].(float64)) != 1 {
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
