package openlist

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	pathpkg "path"
	"sort"
	"strings"
	"time"
)

const maxResponseBytes = 8 << 20

type Client struct {
	httpClient *http.Client
}

type Identity struct {
	Username string `json:"username"`
	BasePath string `json:"base_path"`
}

type DirectoryEntry struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	IsDir    bool   `json:"is_dir"`
	Size     int64  `json:"size"`
	Modified string `json:"modified,omitempty"`
	Sign     string `json:"-"`
}

type apiResponse struct {
	Code    int      `json:"code"`
	Message string   `json:"message"`
	Data    identity `json:"data"`
}

type identity struct {
	Username string `json:"username"`
	BasePath string `json:"base_path"`
	Disabled bool   `json:"disabled"`
}

type directoryResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Content []struct {
			Name     string `json:"name"`
			Size     int64  `json:"size"`
			IsDir    bool   `json:"is_dir"`
			Modified string `json:"modified"`
			Sign     string `json:"sign"`
		} `json:"content"`
	} `json:"data"`
}

type mutationResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type APIError struct {
	Code    string
	Message string
	Cause   error
}

func (e *APIError) Error() string { return e.Message }
func (e *APIError) Unwrap() error { return e.Cause }

func NewClient(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	client := &Client{}
	client.httpClient = &http.Client{
		Timeout: timeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			return ValidateEndpoint(request.Context(), request.URL)
		},
	}
	return client
}

func NormalizeBaseURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" {
		return "", &APIError{Code: "openlist.invalid_url", Message: "OpenList URL is invalid", Cause: err}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", &APIError{Code: "openlist.invalid_scheme", Message: "OpenList URL must use HTTP or HTTPS"}
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", &APIError{Code: "openlist.invalid_url", Message: "OpenList URL must not contain credentials, query, or fragment"}
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	parsed.RawPath = ""
	return strings.TrimRight(parsed.String(), "/"), nil
}

func ValidateEndpoint(ctx context.Context, endpoint *url.URL) error {
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return &APIError{Code: "openlist.invalid_scheme", Message: "OpenList URL must use HTTP or HTTPS"}
	}
	host := endpoint.Hostname()
	if host == "" {
		return &APIError{Code: "openlist.invalid_url", Message: "OpenList URL has no host"}
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return &APIError{Code: "openlist.dns_failed", Message: "OpenList host could not be resolved", Cause: err}
	}
	for _, address := range addresses {
		if blockedIP(address.IP) {
			return &APIError{Code: "openlist.blocked_address", Message: "OpenList address is not allowed"}
		}
	}
	return nil
}

func blockedIP(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalMulticast() || ip.IsLinkLocalUnicast() {
		return true
	}
	metadata := net.ParseIP("169.254.169.254")
	return metadata != nil && ip.Equal(metadata)
}

func (c *Client) TestConnection(ctx context.Context, rawBaseURL, token string) (*Identity, error) {
	baseURL, err := NormalizeBaseURL(rawBaseURL)
	if err != nil {
		return nil, err
	}
	endpoint, err := url.Parse(baseURL + "/api/me")
	if err != nil {
		return nil, &APIError{Code: "openlist.invalid_url", Message: "OpenList URL is invalid", Cause: err}
	}
	if err := ValidateEndpoint(ctx, endpoint); err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, &APIError{Code: "openlist.request_failed", Message: "Failed to create OpenList request", Cause: err}
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", strings.TrimSpace(token))
	request.Header.Set("User-Agent", "OpenlistScraper/1.0")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, &APIError{Code: "openlist.connection_failed", Message: "Could not connect to OpenList", Cause: err}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return nil, &APIError{Code: "openlist.invalid_response", Message: "Could not read OpenList response", Cause: err}
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, &APIError{Code: "openlist.authentication_failed", Message: "OpenList token is invalid or lacks permission"}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &APIError{Code: "openlist.http_error", Message: fmt.Sprintf("OpenList returned HTTP %d", response.StatusCode)}
	}
	var payload apiResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, &APIError{Code: "openlist.invalid_response", Message: "OpenList returned invalid JSON", Cause: err}
	}
	if payload.Code != 200 {
		message := strings.TrimSpace(payload.Message)
		if message == "" {
			message = "OpenList rejected the request"
		}
		return nil, &APIError{Code: "openlist.api_error", Message: message}
	}
	if payload.Data.Disabled {
		return nil, &APIError{Code: "openlist.account_disabled", Message: "OpenList account is disabled"}
	}
	basePath := strings.TrimSpace(payload.Data.BasePath)
	if basePath == "" {
		basePath = "/"
	}
	return &Identity{Username: payload.Data.Username, BasePath: basePath}, nil
}

func NormalizeRemotePath(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || !strings.HasPrefix(value, "/") || strings.Contains(value, "\\") || strings.ContainsRune(value, 0) {
		return "", &APIError{Code: "target.invalid_path", Message: "OpenList path must be an absolute path"}
	}
	for _, character := range value {
		if character < 32 || character == 127 {
			return "", &APIError{Code: "target.invalid_path", Message: "OpenList path contains control characters"}
		}
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "." || segment == ".." {
			return "", &APIError{Code: "target.invalid_path", Message: "OpenList path must not contain dot segments"}
		}
	}
	cleaned := pathpkg.Clean(value)
	if cleaned == "." {
		cleaned = "/"
	}
	return cleaned, nil
}

func IsWithinPath(root, candidate string) bool {
	normalizedRoot, rootErr := NormalizeRemotePath(root)
	normalizedCandidate, candidateErr := NormalizeRemotePath(candidate)
	if rootErr != nil || candidateErr != nil {
		return false
	}
	return normalizedRoot == "/" || normalizedCandidate == normalizedRoot || strings.HasPrefix(normalizedCandidate, normalizedRoot+"/")
}

func (c *Client) ListDirectory(ctx context.Context, rawBaseURL, token, remotePath string, refresh bool) ([]DirectoryEntry, error) {
	baseURL, err := NormalizeBaseURL(rawBaseURL)
	if err != nil {
		return nil, err
	}
	normalizedPath, err := NormalizeRemotePath(remotePath)
	if err != nil {
		return nil, err
	}
	endpoint, err := url.Parse(baseURL + "/api/fs/list")
	if err != nil {
		return nil, &APIError{Code: "openlist.invalid_url", Message: "OpenList URL is invalid", Cause: err}
	}
	if err := ValidateEndpoint(ctx, endpoint); err != nil {
		return nil, err
	}
	body, err := json.Marshal(map[string]interface{}{
		"path": normalizedPath, "password": "", "page": 1, "per_page": 0, "refresh": refresh,
	})
	if err != nil {
		return nil, &APIError{Code: "openlist.request_failed", Message: "Failed to encode OpenList request", Cause: err}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), strings.NewReader(string(body)))
	if err != nil {
		return nil, &APIError{Code: "openlist.request_failed", Message: "Failed to create OpenList request", Cause: err}
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", strings.TrimSpace(token))
	request.Header.Set("User-Agent", "OpenlistScraper/1.0")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, &APIError{Code: "openlist.connection_failed", Message: "Could not read OpenList directory", Cause: err}
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return nil, &APIError{Code: "openlist.invalid_response", Message: "Could not read OpenList response", Cause: err}
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return nil, &APIError{Code: "openlist.authentication_failed", Message: "OpenList token is invalid or lacks permission"}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &APIError{Code: "openlist.http_error", Message: fmt.Sprintf("OpenList returned HTTP %d", response.StatusCode)}
	}
	var payload directoryResponse
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return nil, &APIError{Code: "openlist.invalid_response", Message: "OpenList returned invalid JSON", Cause: err}
	}
	if payload.Code != 200 {
		return nil, &APIError{Code: "openlist.api_error", Message: firstNonBlank(payload.Message, "OpenList rejected the directory request")}
	}
	entries := make([]DirectoryEntry, 0, len(payload.Data.Content))
	for _, item := range payload.Data.Content {
		if !validEntryName(item.Name) {
			return nil, &APIError{Code: "openlist.invalid_response", Message: "OpenList returned an unsafe directory entry name"}
		}
		entries = append(entries, DirectoryEntry{
			Name: item.Name, Path: joinRemotePath(normalizedPath, item.Name), IsDir: item.IsDir,
			Size: item.Size, Modified: item.Modified, Sign: item.Sign,
		})
	}
	sort.Slice(entries, func(left, right int) bool {
		if entries[left].IsDir != entries[right].IsDir {
			return entries[left].IsDir
		}
		return strings.ToLower(entries[left].Name) < strings.ToLower(entries[right].Name)
	})
	return entries, nil
}

func validEntryName(name string) bool {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\") {
		return false
	}
	for _, character := range name {
		if character == 0 || character < 32 || character == 127 {
			return false
		}
	}
	return true
}

func joinRemotePath(parent, name string) string {
	if parent == "/" {
		return "/" + strings.TrimLeft(name, "/")
	}
	return strings.TrimRight(parent, "/") + "/" + strings.TrimLeft(name, "/")
}

func firstNonBlank(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func (c *Client) CreateDirectory(ctx context.Context, rawBaseURL, token, remotePath string) error {
	normalized, err := NormalizeRemotePath(remotePath)
	if err != nil {
		return err
	}
	return c.mutate(ctx, rawBaseURL, token, "/api/fs/mkdir", map[string]any{"path": normalized})
}

func (c *Client) RenameEntry(ctx context.Context, rawBaseURL, token, sourcePath, newName string) error {
	normalized, err := NormalizeRemotePath(sourcePath)
	if err != nil {
		return err
	}
	if !validEntryName(newName) {
		return &APIError{Code: "openlist.invalid_name", Message: "OpenList target name is invalid"}
	}
	return c.mutate(ctx, rawBaseURL, token, "/api/fs/rename", map[string]any{"path": normalized, "name": newName, "overwrite": false})
}

func (c *Client) MoveEntries(ctx context.Context, rawBaseURL, token, sourceDirectory, targetDirectory string, names []string) error {
	source, err := NormalizeRemotePath(sourceDirectory)
	if err != nil {
		return err
	}
	target, err := NormalizeRemotePath(targetDirectory)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return &APIError{Code: "openlist.invalid_move", Message: "OpenList move requires at least one entry"}
	}
	for _, name := range names {
		if !validEntryName(name) {
			return &APIError{Code: "openlist.invalid_name", Message: "OpenList move entry name is invalid"}
		}
	}
	return c.mutate(ctx, rawBaseURL, token, "/api/fs/move", map[string]any{"src_dir": source, "dst_dir": target, "names": names})
}

func (c *Client) Upload(ctx context.Context, rawBaseURL, token, remotePath, contentType string, size int64, content io.Reader) error {
	normalized, err := NormalizeRemotePath(remotePath)
	if err != nil {
		return err
	}
	endpoint, err := mutationEndpoint(ctx, rawBaseURL, "/api/fs/put")
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint.String(), content)
	if err != nil {
		return &APIError{Code: "openlist.request_failed", Message: "Failed to create OpenList upload request", Cause: err}
	}
	setMutationHeaders(request, token)
	request.Header.Set("Content-Type", firstNonBlank(contentType, "application/octet-stream"))
	request.Header.Set("File-Path", strings.ReplaceAll(url.QueryEscape(normalized), "+", "%20"))
	request.Header.Set("As-Task", "false")
	request.Header.Set("Overwrite", "true")
	request.ContentLength = size
	return c.doMutation(request)
}

func (c *Client) mutate(ctx context.Context, rawBaseURL, token, route string, input any) error {
	endpoint, err := mutationEndpoint(ctx, rawBaseURL, route)
	if err != nil {
		return err
	}
	body, err := json.Marshal(input)
	if err != nil {
		return &APIError{Code: "openlist.request_failed", Message: "Failed to encode OpenList mutation", Cause: err}
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), strings.NewReader(string(body)))
	if err != nil {
		return &APIError{Code: "openlist.request_failed", Message: "Failed to create OpenList mutation request", Cause: err}
	}
	setMutationHeaders(request, token)
	request.Header.Set("Content-Type", "application/json")
	return c.doMutation(request)
}

func mutationEndpoint(ctx context.Context, rawBaseURL, route string) (*url.URL, error) {
	baseURL, err := NormalizeBaseURL(rawBaseURL)
	if err != nil {
		return nil, err
	}
	endpoint, err := url.Parse(baseURL + route)
	if err != nil {
		return nil, &APIError{Code: "openlist.invalid_url", Message: "OpenList URL is invalid", Cause: err}
	}
	if err := ValidateEndpoint(ctx, endpoint); err != nil {
		return nil, err
	}
	return endpoint, nil
}

func setMutationHeaders(request *http.Request, token string) {
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", strings.TrimSpace(token))
	request.Header.Set("User-Agent", "OpenlistScraper/1.0")
}

func (c *Client) doMutation(request *http.Request) error {
	response, err := c.httpClient.Do(request)
	if err != nil {
		return &APIError{Code: "openlist.connection_failed", Message: "OpenList mutation request failed", Cause: err}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return &APIError{Code: "openlist.invalid_response", Message: "Could not read OpenList mutation response", Cause: err}
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return &APIError{Code: "openlist.authentication_failed", Message: "OpenList token is invalid or lacks permission"}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &APIError{Code: "openlist.http_error", Message: fmt.Sprintf("OpenList returned HTTP %d", response.StatusCode)}
	}
	var payload mutationResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return &APIError{Code: "openlist.invalid_response", Message: "OpenList returned invalid mutation JSON", Cause: err}
	}
	if payload.Code != 200 {
		return &APIError{Code: "openlist.api_error", Message: firstNonBlank(payload.Message, "OpenList rejected the mutation")}
	}
	return nil
}
