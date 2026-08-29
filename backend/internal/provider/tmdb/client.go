package tmdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const maxResponseBytes = 4 << 20

type Config struct {
	APIKey       string
	BaseURL      string
	ImageBaseURL string
	Language     string
	Region       string
	PosterSize   string
	BackdropSize string
	ProxyHost    string
	ProxyPort    int
	Timeout      time.Duration
}

type SearchResult struct {
	ID            int     `json:"id"`
	MediaType     string  `json:"media_type"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Year          int     `json:"year,omitempty"`
	Overview      string  `json:"overview"`
	PosterURL     string  `json:"poster_url,omitempty"`
	BackdropURL   string  `json:"backdrop_url,omitempty"`
	VoteAverage   float64 `json:"vote_average"`
	VoteCount     int     `json:"vote_count"`
	Popularity    float64 `json:"popularity"`
}

type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Detail struct {
	ID               int      `json:"id"`
	MediaType        string   `json:"media_type"`
	Title            string   `json:"title"`
	OriginalTitle    string   `json:"original_title"`
	Year             int      `json:"year,omitempty"`
	ReleaseDate      string   `json:"release_date,omitempty"`
	Overview         string   `json:"overview"`
	PosterURL        string   `json:"poster_url,omitempty"`
	BackdropURL      string   `json:"backdrop_url,omitempty"`
	VoteAverage      float64  `json:"vote_average"`
	VoteCount        int      `json:"vote_count"`
	Genres           []Genre  `json:"genres"`
	Runtime          int      `json:"runtime,omitempty"`
	NumberOfSeasons  int      `json:"number_of_seasons,omitempty"`
	NumberOfEpisodes int      `json:"number_of_episodes,omitempty"`
	OriginalLanguage string   `json:"original_language,omitempty"`
	Tagline          string   `json:"tagline,omitempty"`
	Status           string   `json:"status,omitempty"`
	IMDBID           string   `json:"imdb_id,omitempty"`
	Country          string   `json:"country,omitempty"`
	Studios          []string `json:"studios"`
}

type Episode struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	Overview      string  `json:"overview"`
	AirDate       string  `json:"air_date,omitempty"`
	SeasonNumber  int     `json:"season_number"`
	EpisodeNumber int     `json:"episode_number"`
	Runtime       int     `json:"runtime,omitempty"`
	VoteAverage   float64 `json:"vote_average"`
	VoteCount     int     `json:"vote_count"`
	StillURL      string  `json:"still_url,omitempty"`
}

type Error struct {
	Code    string
	Message string
	Cause   error
}

func (e *Error) Error() string { return e.Message }
func (e *Error) Unwrap() error { return e.Cause }

type Client struct{}

func NewClient() *Client { return &Client{} }

func (c *Client) Test(ctx context.Context, config Config) error {
	_, err := c.get(ctx, config, "/configuration", nil)
	return err
}

func (c *Client) Search(ctx context.Context, config Config, mediaType, query string, year int) ([]SearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, &Error{Code: "tmdb.invalid_query", Message: "TMDB search title is required"}
	}
	endpoint := "/search/movie"
	if mediaType == "tv" || mediaType == "anime" {
		endpoint = "/search/tv"
		mediaType = "tv"
	} else {
		mediaType = "movie"
	}
	parameters := url.Values{"query": []string{query}}
	if year > 0 {
		if mediaType == "movie" {
			parameters.Set("year", strconv.Itoa(year))
		} else {
			parameters.Set("first_air_date_year", strconv.Itoa(year))
		}
	}
	body, err := c.get(ctx, config, endpoint, parameters)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Results []rawResult `json:"results"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, &Error{Code: "tmdb.invalid_response", Message: "TMDB returned invalid search results", Cause: err}
	}
	results := make([]SearchResult, 0, len(payload.Results))
	for _, item := range payload.Results {
		title := firstNonBlank(item.Title, item.Name, item.OriginalTitle, item.OriginalName)
		if item.ID <= 0 || title == "" {
			continue
		}
		date := firstNonBlank(item.ReleaseDate, item.FirstAirDate)
		results = append(results, SearchResult{
			ID: item.ID, MediaType: mediaType, Title: title,
			OriginalTitle: firstNonBlank(item.OriginalTitle, item.OriginalName), Year: yearFromDate(date),
			Overview: item.Overview, PosterURL: imageURL(config, item.PosterPath, config.PosterSize),
			BackdropURL: imageURL(config, item.BackdropPath, config.BackdropSize),
			VoteAverage: item.VoteAverage, VoteCount: item.VoteCount, Popularity: item.Popularity,
		})
	}
	return results, nil
}

func (c *Client) Detail(ctx context.Context, config Config, mediaType string, id int) (*Detail, error) {
	if id <= 0 {
		return nil, &Error{Code: "tmdb.invalid_id", Message: "TMDB ID must be positive"}
	}
	endpointType := "movie"
	if mediaType == "tv" || mediaType == "anime" {
		endpointType = "tv"
	}
	body, err := c.get(ctx, config, "/"+endpointType+"/"+strconv.Itoa(id), nil)
	if err != nil {
		return nil, err
	}
	var item rawDetail
	if err := json.Unmarshal(body, &item); err != nil {
		return nil, &Error{Code: "tmdb.invalid_response", Message: "TMDB returned invalid media details", Cause: err}
	}
	title := firstNonBlank(item.Title, item.Name, item.OriginalTitle, item.OriginalName)
	if item.ID <= 0 || title == "" {
		return nil, &Error{Code: "tmdb.invalid_response", Message: "TMDB media details are incomplete"}
	}
	date := firstNonBlank(item.ReleaseDate, item.FirstAirDate)
	return &Detail{
		ID: item.ID, MediaType: endpointType, Title: title, OriginalTitle: firstNonBlank(item.OriginalTitle, item.OriginalName),
		Year: yearFromDate(date), ReleaseDate: date, Overview: item.Overview,
		PosterURL: imageURL(config, item.PosterPath, config.PosterSize), BackdropURL: imageURL(config, item.BackdropPath, config.BackdropSize),
		VoteAverage: item.VoteAverage, VoteCount: item.VoteCount, Genres: item.Genres, Runtime: detailRuntime(item, endpointType),
		NumberOfSeasons: item.NumberOfSeasons, NumberOfEpisodes: item.NumberOfEpisodes, OriginalLanguage: item.OriginalLanguage,
		Tagline: item.Tagline, Status: item.Status, IMDBID: item.IMDBID, Country: detailCountry(item, endpointType), Studios: detailStudios(item, endpointType),
	}, nil
}

func (c *Client) Season(ctx context.Context, config Config, tvID, season int) ([]Episode, error) {
	if tvID <= 0 || season < 0 {
		return nil, &Error{Code: "tmdb.invalid_id", Message: "TMDB TV and season IDs are invalid"}
	}
	body, err := c.get(ctx, config, "/tv/"+strconv.Itoa(tvID)+"/season/"+strconv.Itoa(season), nil)
	if err != nil {
		return nil, err
	}
	var payload struct {
		Episodes []struct {
			ID            int     `json:"id"`
			Name          string  `json:"name"`
			Overview      string  `json:"overview"`
			AirDate       string  `json:"air_date"`
			SeasonNumber  int     `json:"season_number"`
			EpisodeNumber int     `json:"episode_number"`
			Runtime       int     `json:"runtime"`
			VoteAverage   float64 `json:"vote_average"`
			VoteCount     int     `json:"vote_count"`
			StillPath     string  `json:"still_path"`
		} `json:"episodes"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, &Error{Code: "tmdb.invalid_response", Message: "TMDB returned invalid season details", Cause: err}
	}
	result := make([]Episode, 0, len(payload.Episodes))
	for _, episode := range payload.Episodes {
		if episode.ID <= 0 || episode.EpisodeNumber <= 0 {
			continue
		}
		result = append(result, Episode{
			ID: episode.ID, Name: episode.Name, Overview: episode.Overview, AirDate: episode.AirDate,
			SeasonNumber: episode.SeasonNumber, EpisodeNumber: episode.EpisodeNumber, Runtime: episode.Runtime,
			VoteAverage: episode.VoteAverage, VoteCount: episode.VoteCount,
			StillURL: imageURL(config, episode.StillPath, config.BackdropSize),
		})
	}
	return result, nil
}

func (c *Client) get(ctx context.Context, config Config, endpoint string, parameters url.Values) ([]byte, error) {
	if strings.TrimSpace(config.APIKey) == "" {
		return nil, &Error{Code: "tmdb.not_configured", Message: "TMDB API key is not configured"}
	}
	baseURL, err := normalizeBaseURL(config.BaseURL, "https://api.themoviedb.org")
	if err != nil {
		return nil, err
	}
	parsed, err := url.Parse(baseURL + "/3" + endpoint)
	if err != nil {
		return nil, &Error{Code: "tmdb.invalid_url", Message: "TMDB URL is invalid", Cause: err}
	}
	query := parsed.Query()
	for key, values := range parameters {
		for _, value := range values {
			query.Add(key, value)
		}
	}
	query.Set("api_key", strings.TrimSpace(config.APIKey))
	query.Set("language", firstNonBlank(config.Language, "zh-CN"))
	if strings.TrimSpace(config.Region) != "" {
		query.Set("region", strings.TrimSpace(config.Region))
	}
	parsed.RawQuery = query.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, &Error{Code: "tmdb.request_failed", Message: "Failed to create TMDB request", Cause: err}
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", "OScraper/1.0")
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	client, err := HTTPClient(config, timeout)
	if err != nil {
		return nil, err
	}
	response, err := client.Do(request)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, &Error{Code: "tmdb.timeout", Message: "TMDB request timed out", Cause: err}
		}
		return nil, &Error{Code: "tmdb.connection_failed", Message: "Could not connect to TMDB", Cause: err}
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes))
	if err != nil {
		return nil, &Error{Code: "tmdb.invalid_response", Message: "Could not read TMDB response", Cause: err}
	}
	switch response.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, &Error{Code: "tmdb.authentication_failed", Message: "TMDB API key is invalid"}
	case http.StatusNotFound:
		return nil, &Error{Code: "tmdb.not_found", Message: "TMDB media item was not found"}
	case http.StatusTooManyRequests:
		return nil, &Error{Code: "tmdb.rate_limited", Message: "TMDB rate limit was exceeded"}
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, &Error{Code: "tmdb.http_error", Message: fmt.Sprintf("TMDB returned HTTP %d", response.StatusCode)}
	}
	return body, nil
}

func HTTPClient(config Config, timeout time.Duration) (*http.Client, error) {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, &Error{Code: "tmdb.proxy_failed", Message: "Could not configure the TMDB HTTP client"}
	}
	configured := transport.Clone()
	configured.Proxy = nil
	if strings.TrimSpace(config.ProxyHost) != "" || config.ProxyPort != 0 {
		if strings.TrimSpace(config.ProxyHost) == "" || config.ProxyPort < 1 || config.ProxyPort > 65535 {
			return nil, &Error{Code: "tmdb.invalid_proxy", Message: "TMDB proxy host and port are invalid"}
		}
		proxyURL := &url.URL{Scheme: "http", Host: net.JoinHostPort(strings.TrimSpace(config.ProxyHost), strconv.Itoa(config.ProxyPort))}
		configured.Proxy = http.ProxyURL(proxyURL)
	}
	return &http.Client{Timeout: timeout, Transport: configured}, nil
}

func normalizeBaseURL(raw, fallback string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = fallback
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", &Error{Code: "tmdb.invalid_url", Message: "TMDB base URL is invalid", Cause: err}
	}
	parsed.Path = strings.TrimSuffix(strings.TrimRight(parsed.Path, "/"), "/3")
	return strings.TrimRight(parsed.String(), "/"), nil
}

func imageURL(config Config, imagePath, size string) string {
	if strings.TrimSpace(imagePath) == "" {
		return ""
	}
	baseURL, err := normalizeBaseURL(config.ImageBaseURL, "https://image.tmdb.org")
	if err != nil {
		return ""
	}
	if strings.TrimSpace(size) == "" {
		size = "original"
	}
	baseURL = strings.TrimSuffix(baseURL, "/t/p")
	return baseURL + "/t/p/" + strings.Trim(size, "/") + "/" + strings.TrimLeft(imagePath, "/")
}

func yearFromDate(value string) int {
	if len(value) < 4 {
		return 0
	}
	year, _ := strconv.Atoi(value[:4])
	return year
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

type rawResult struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	Name          string  `json:"name"`
	OriginalTitle string  `json:"original_title"`
	OriginalName  string  `json:"original_name"`
	Overview      string  `json:"overview"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	ReleaseDate   string  `json:"release_date"`
	FirstAirDate  string  `json:"first_air_date"`
	VoteAverage   float64 `json:"vote_average"`
	VoteCount     int     `json:"vote_count"`
	Popularity    float64 `json:"popularity"`
}

type rawDetail struct {
	rawResult
	Genres              []Genre       `json:"genres"`
	Runtime             int           `json:"runtime"`
	EpisodeRunTime      []int         `json:"episode_run_time"`
	NumberOfSeasons     int           `json:"number_of_seasons"`
	NumberOfEpisodes    int           `json:"number_of_episodes"`
	OriginalLanguage    string        `json:"original_language"`
	Tagline             string        `json:"tagline"`
	Status              string        `json:"status"`
	IMDBID              string        `json:"imdb_id"`
	ProductionCompanies []namedEntity `json:"production_companies"`
	ProductionCountries []country     `json:"production_countries"`
	Networks            []namedEntity `json:"networks"`
	OriginCountry       []string      `json:"origin_country"`
}

type namedEntity struct {
	Name string `json:"name"`
}

type country struct {
	Code string `json:"iso_3166_1"`
	Name string `json:"name"`
}

func detailStudios(item rawDetail, mediaType string) []string {
	entities := item.ProductionCompanies
	if mediaType == "tv" && len(item.Networks) > 0 {
		entities = item.Networks
	}
	studios := make([]string, 0, len(entities))
	for _, entity := range entities {
		if name := strings.TrimSpace(entity.Name); name != "" {
			studios = append(studios, name)
		}
	}
	return studios
}

func detailCountry(item rawDetail, mediaType string) string {
	if mediaType == "tv" && len(item.OriginCountry) > 0 {
		return strings.TrimSpace(item.OriginCountry[0])
	}
	if len(item.ProductionCountries) > 0 {
		if name := strings.TrimSpace(item.ProductionCountries[0].Name); name != "" {
			return name
		}
		return strings.TrimSpace(item.ProductionCountries[0].Code)
	}
	return ""
}

func detailRuntime(item rawDetail, mediaType string) int {
	if item.Runtime > 0 || mediaType != "tv" || len(item.EpisodeRunTime) == 0 {
		return item.Runtime
	}
	total := 0
	count := 0
	for _, runtime := range item.EpisodeRunTime {
		if runtime > 0 {
			total += runtime
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return total / count
}
