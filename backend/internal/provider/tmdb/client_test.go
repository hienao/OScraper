package tmdb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSearchMovieUsesYearAndNormalizesResult(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/3/search/movie" || request.URL.Query().Get("api_key") != "secret" || request.URL.Query().Get("year") != "2016" {
			t.Fatalf("unexpected request: %s?%s", request.URL.Path, request.URL.RawQuery)
		}
		_, _ = writer.Write([]byte(`{"results":[{"id":329865,"title":"Arrival","original_title":"Arrival","release_date":"2016-11-10","poster_path":"/poster.jpg","vote_average":7.6}]}`))
	}))
	defer server.Close()
	results, err := NewClient().Search(context.Background(), Config{APIKey: "secret", BaseURL: server.URL, ImageBaseURL: server.URL, Language: "zh-CN", PosterSize: "w500", Timeout: time.Second}, "movie", "Arrival", 2016)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ID != 329865 || results[0].Year != 2016 || results[0].PosterURL != server.URL+"/t/p/w500/poster.jpg" {
		t.Fatalf("unexpected results: %#v", results)
	}
}

func TestAnimeUsesTVEndpointAndDetail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/3/search/tv":
			if request.URL.Query().Get("first_air_date_year") != "2023" {
				t.Fatalf("missing TV year query: %s", request.URL.RawQuery)
			}
			_, _ = writer.Write([]byte(`{"results":[]}`))
		case "/3/tv/209867":
			_, _ = writer.Write([]byte(`{"id":209867,"name":"Frieren","original_name":"葬送のフリーレン","first_air_date":"2023-09-29","number_of_seasons":2,"number_of_episodes":32,"genres":[{"id":16,"name":"Animation"}]}`))
		default:
			t.Fatalf("unexpected endpoint: %s", request.URL.Path)
		}
	}))
	defer server.Close()
	config := Config{APIKey: "secret", BaseURL: server.URL, Timeout: time.Second}
	if _, err := NewClient().Search(context.Background(), config, "anime", "Frieren", 2023); err != nil {
		t.Fatal(err)
	}
	detail, err := NewClient().Detail(context.Background(), config, "anime", 209867)
	if err != nil {
		t.Fatal(err)
	}
	if detail.MediaType != "tv" || detail.Year != 2023 || detail.NumberOfEpisodes != 32 {
		t.Fatalf("unexpected detail: %#v", detail)
	}
}

func TestAuthenticationErrorHasStableCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) { writer.WriteHeader(http.StatusUnauthorized) }))
	defer server.Close()
	err := NewClient().Test(context.Background(), Config{APIKey: "bad", BaseURL: server.URL, Timeout: time.Second})
	tmdbError, ok := err.(*Error)
	if !ok || tmdbError.Code != "tmdb.authentication_failed" {
		t.Fatalf("unexpected error: %#v", err)
	}
}

func TestImageBaseAcceptsExistingTMDBPath(t *testing.T) {
	url := imageURL(Config{ImageBaseURL: "https://image.tmdb.org/t/p", PosterSize: "w500"}, "/poster.jpg", "w500")
	if url != "https://image.tmdb.org/t/p/w500/poster.jpg" {
		t.Fatalf("unexpected image URL: %s", url)
	}
}
