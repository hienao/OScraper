package service

import (
	"testing"
	"time"

	"openlistscraper/internal/provider/tmdb"
)

func TestTVArtifactsOnlyIncludeAvailableImages(t *testing.T) {
	detail := &tmdb.Detail{ID: 1399, MediaType: "tv", Title: "Show", Year: 2011, PosterURL: "https://image.example/poster.jpg"}
	artifacts := buildMetadataArtifacts(detail, "/tv/Show (2011) {tmdbid-1399}", "Show (2011) {tmdbid-1399}", time.Now())
	if len(artifacts) != 2 || artifacts[0].Path != "/tv/Show (2011) {tmdbid-1399}/tvshow.nfo" || artifacts[1].Kind != "poster" {
		t.Fatalf("unexpected artifacts: %#v", artifacts)
	}
}
