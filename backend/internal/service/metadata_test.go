package service

import (
	"context"
	"testing"
	"time"

	"openlistscraper/internal/provider/tmdb"
)

type seasonCatalogStub struct{ episodes []tmdb.Episode }

func (s seasonCatalogStub) Season(context.Context, tmdb.Config, int, int) ([]tmdb.Episode, error) {
	return s.episodes, nil
}

func TestTVArtifactsOnlyIncludeAvailableImages(t *testing.T) {
	detail := &tmdb.Detail{ID: 1399, MediaType: "tv", Title: "Show", Year: 2011, PosterURL: "https://image.example/poster.jpg"}
	artifacts := buildMetadataArtifacts(detail, "/tv/Show (2011) {tmdbid-1399}", "Show (2011) {tmdbid-1399}", time.Now())
	if len(artifacts) != 2 || artifacts[0].Path != "/tv/Show (2011) {tmdbid-1399}/tvshow.nfo" || artifacts[1].Kind != "poster" {
		t.Fatalf("unexpected artifacts: %#v", artifacts)
	}
}

func TestEpisodeArtifactsUseFinalVideoBaseName(t *testing.T) {
	plan := PreviewPlan{Ready: true, Artifacts: []PreviewArtifact{}, GeneratedFiles: []string{}, EpisodeFiles: []EpisodeFilePlan{{SourcePath: "/tv/Show/raw.mkv", TargetPath: "/tv/Show/Season 01/Show - S01E01.mkv", Season: 1, Episode: 1}}}
	detail := &tmdb.Detail{ID: 1, MediaType: "tv", Title: "Show"}
	err := expandEpisodeArtifacts(context.Background(), seasonCatalogStub{episodes: []tmdb.Episode{{ID: 11, Name: "Pilot", SeasonNumber: 1, EpisodeNumber: 1, StillURL: "https://image.example/e1.jpg"}}}, tmdb.Config{}, detail, &plan)
	if err != nil || len(plan.Artifacts) != 2 || plan.Artifacts[0].Path != "/tv/Show/Season 01/Show - S01E01.nfo" || plan.Artifacts[1].Kind != "episode_thumb" || !plan.Ready {
		t.Fatalf("unexpected episode artifacts: %#v %v", plan, err)
	}
}
