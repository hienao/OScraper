package service

import (
	"context"
	"testing"
	"time"

	"oscraper/internal/provider/tmdb"
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

func TestMissingEpisodesAreSkippedInsteadOfBlocking(t *testing.T) {
	plan := PreviewPlan{
		Ready: true, Artifacts: []PreviewArtifact{}, GeneratedFiles: []string{}, Warnings: []string{},
		EpisodeFiles: []EpisodeFilePlan{
			{SourcePath: "/tv/Show/raw1.mkv", TargetPath: "/tv/Show/Season 01/Show - S01E01.mkv", Season: 1, Episode: 1},
			{SourcePath: "/tv/Show/raw21.mkv", TargetPath: "/tv/Show/Season 01/Show - S01E21.mkv", Season: 1, Episode: 21},
		},
		ProposedFileRenames: []RenameItem{
			{SourcePath: "/tv/Show/raw1.mkv", TargetPath: "/tv/Show/Season 01/Show - S01E01.mkv", AssetType: "video"},
			{SourcePath: "/tv/Show/raw1.jpg", TargetPath: "/tv/Show/Season 01/Show - S01E01.jpg", AssetType: "image"},
			{SourcePath: "/tv/Show/raw1.nfo", TargetPath: "/tv/Show/Season 01/Show - S01E01.nfo", AssetType: "nfo"},
			{SourcePath: "/tv/Show/raw21.mkv", TargetPath: "/tv/Show/Season 01/Show - S01E21.mkv", AssetType: "video"},
			{SourcePath: "/tv/Show/raw21.jpg", TargetPath: "/tv/Show/Season 01/Show - S01E21.jpg", AssetType: "image"},
			{SourcePath: "/tv/Show/raw21.nfo", TargetPath: "/tv/Show/Season 01/Show - S01E21.nfo", AssetType: "nfo"},
		},
	}
	detail := &tmdb.Detail{ID: 1, MediaType: "tv", Title: "Show"}
	err := expandEpisodeArtifacts(context.Background(), seasonCatalogStub{episodes: []tmdb.Episode{{ID: 11, Name: "Pilot", SeasonNumber: 1, EpisodeNumber: 1, StillURL: "https://image.example/e1.jpg"}}}, tmdb.Config{}, detail, &plan)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Ready || len(plan.Conflicts) != 0 {
		t.Fatalf("missing episode metadata must not block the plan: %#v", plan)
	}
	if len(plan.SkippedEpisodes) != 1 || plan.SkippedEpisodes[0].Episode != 21 {
		t.Fatalf("skipped episode was not recorded: %#v", plan.SkippedEpisodes)
	}
	if len(plan.EpisodeFiles) != 1 || plan.EpisodeFiles[0].Episode != 1 {
		t.Fatalf("unexpected remaining episode files: %#v", plan.EpisodeFiles)
	}
	if len(plan.ProposedFileRenames) != 3 || plan.ProposedFileRenames[0].SourcePath != "/tv/Show/raw1.mkv" || plan.ProposedFileRenames[1].SourcePath != "/tv/Show/raw1.jpg" || plan.ProposedFileRenames[2].SourcePath != "/tv/Show/raw1.nfo" {
		t.Fatalf("skipped episode renames were not dropped: %#v", plan.ProposedFileRenames)
	}
	if len(plan.Artifacts) != 2 || plan.Artifacts[0].Path != "/tv/Show/Season 01/Show - S01E01.nfo" {
		t.Fatalf("unexpected artifacts: %#v", plan.Artifacts)
	}
	if len(plan.Warnings) != 1 || plan.Warnings[0] != "episodes_skipped" {
		t.Fatalf("episodes_skipped warning was not added: %#v", plan.Warnings)
	}
}
