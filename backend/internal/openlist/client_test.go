package openlist

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

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
