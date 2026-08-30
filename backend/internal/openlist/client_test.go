package openlist

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestMutationEndpointsUseNonOverwritingMediaOperationsAndMetadataOverwrite(t *testing.T) {
	requests := make([]*http.Request, 0)
	bodies := make([][]byte, 0)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		requests = append(requests, request.Clone(context.Background()))
		bodies = append(bodies, body)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"code":200,"message":"success"}`))
	}))
	defer server.Close()
	client := NewClient(time.Second)
	if err := client.CreateDirectory(context.Background(), server.URL, "token", "/movies/New"); err != nil {
		t.Fatal(err)
	}
	if err := client.RenameEntry(context.Background(), server.URL, "token", "/movies/Old.mkv", "New.mkv"); err != nil {
		t.Fatal(err)
	}
	if err := client.MoveEntries(context.Background(), server.URL, "token", "/movies", "/movies/New", []string{"New.mkv"}); err != nil {
		t.Fatal(err)
	}
	if err := client.Upload(context.Background(), server.URL, "token", "/电影/海报 poster.jpg", "image/jpeg", 4, strings.NewReader("data")); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 4 || requests[0].URL.Path != "/api/fs/mkdir" || requests[1].URL.Path != "/api/fs/rename" || requests[2].URL.Path != "/api/fs/move" || requests[3].URL.Path != "/api/fs/put" {
		t.Fatalf("unexpected mutation requests: %#v", requests)
	}
	var rename map[string]any
	if err := json.Unmarshal(bodies[1], &rename); err != nil || rename["overwrite"] != false {
		t.Fatalf("rename did not disable overwrite: %s", bodies[1])
	}
	if requests[3].Header.Get("File-Path") != "%2F%E7%94%B5%E5%BD%B1%2F%E6%B5%B7%E6%8A%A5%20poster.jpg" || requests[3].Header.Get("Overwrite") != "true" || string(bodies[3]) != "data" {
		t.Fatalf("unexpected upload: headers=%v body=%q", requests[3].Header, bodies[3])
	}
}

func TestConnectionReadsIdentityAndAuthorization(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/me" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "token-value" {
			t.Fatalf("unexpected authorization header")
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"code":200,"message":"success","data":{"username":"alice","base_path":"/media","disabled":false}}`))
	}))
	defer server.Close()

	client := NewClient(time.Second)
	identity, err := client.TestConnection(context.Background(), server.URL, "token-value")
	if err != nil {
		t.Fatal(err)
	}
	if identity.Username != "alice" || identity.BasePath != "/media" {
		t.Fatalf("unexpected identity: %#v", identity)
	}
}

func TestNormalizeBaseURLRejectsCredentials(t *testing.T) {
	if _, err := NormalizeBaseURL("http://user:pass@example.com"); err == nil {
		t.Fatal("expected credentials to be rejected")
	}
}

func TestListDirectoryUsesOfficialEndpointAndSortsDirectoriesFirst(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.URL.Path != "/api/fs/list" {
			t.Fatalf("unexpected request: %s %s", request.Method, request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"code":200,"message":"success","data":{"content":[{"name":"movie.mkv","size":12,"is_dir":false,"modified":"now","sign":"secret-sign"},{"name":"Movies","size":0,"is_dir":true,"modified":"now","sign":""}]}}`))
	}))
	defer server.Close()
	entries, err := NewClient(time.Second).ListDirectory(context.Background(), server.URL, "token", "/media", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || !entries[0].IsDir || entries[0].Path != "/media/Movies" || entries[1].Sign != "secret-sign" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
}

func TestRemotePathBoundaryDoesNotUseStringPrefixOnly(t *testing.T) {
	if !IsWithinPath("/media", "/media/movies") {
		t.Fatal("expected child path")
	}
	if IsWithinPath("/media", "/media-old/movies") {
		t.Fatal("prefix-only path escaped root")
	}
}

func TestNormalizeRemotePathRejectsDotSegments(t *testing.T) {
	for _, value := range []string{"/media/../other", "/media/./movies"} {
		if _, err := NormalizeRemotePath(value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}

func TestListDirectoryRejectsUnsafeEntryNames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"code":200,"data":{"content":[{"name":"../escape","is_dir":true}]}}`))
	}))
	defer server.Close()
	_, err := NewClient(time.Second).ListDirectory(context.Background(), server.URL, "token", "/media", false)
	if err == nil {
		t.Fatal("expected unsafe entry name to be rejected")
	}
	if !strings.Contains(err.Error(), strconv.Quote("../escape")) || !strings.Contains(err.Error(), "path separator") {
		t.Fatalf("unsafe entry error did not identify the offending name: %v", err)
	}
}

func TestListDirectoryAllowsBackslashAsLiteralNameCharacter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"code":200,"data":{"content":[{"name":"Movie\\Part","is_dir":true}]}}`))
	}))
	defer server.Close()
	entries, err := NewClient(time.Second).ListDirectory(context.Background(), server.URL, "token", "/media", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != `Movie\Part` || entries[0].Path != `/media/Movie\Part` {
		t.Fatalf("unexpected literal backslash entry: %#v", entries)
	}
	if normalized, err := NormalizeRemotePath(entries[0].Path); err != nil || normalized != entries[0].Path {
		t.Fatalf("literal backslash path was not reusable: %q %v", normalized, err)
	}
}

func TestBlockedIPAllowsPrivateAndLoopback(t *testing.T) {
	for _, value := range []string{"127.0.0.1", "10.0.0.1", "192.168.1.2"} {
		if blockedIP(netParseIP(t, value)) {
			t.Fatalf("expected %s to be allowed", value)
		}
	}
	if !blockedIP(netParseIP(t, "169.254.169.254")) {
		t.Fatal("expected metadata address to be blocked")
	}
}

func netParseIP(t *testing.T, value string) []byte {
	t.Helper()
	parsed := net.ParseIP(value)
	if parsed == nil {
		t.Fatalf("invalid test IP: %s", value)
	}
	return parsed
}
