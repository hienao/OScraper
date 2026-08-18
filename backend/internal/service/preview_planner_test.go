package service

import (
	"testing"

	"openlistscraper/internal/model"
	"openlistscraper/internal/openlist"
	"openlistscraper/internal/provider/tmdb"
)

func TestSeriesPlanRenamesSeasonEpisodesAndCompanions(t *testing.T) {
	target := &model.ScrapeTarget{RootPath: "/tv", LibraryType: "tv", RenameEnabled: true}
	candidate := &model.MediaCandidate{Path: "/tv/Show", Kind: "tv", RepresentativeFile: "S1/raw.S01E01.mkv"}
	entries := []openlist.DirectoryEntry{
		{Name: "S1", Path: "/tv/Show/S1", IsDir: true},
		{Name: "raw.S01E01.mkv", Path: "/tv/Show/S1/raw.S01E01.mkv"},
		{Name: "raw.S01E01.zh-CN.ass", Path: "/tv/Show/S1/raw.S01E01.zh-CN.ass"},
		{Name: "raw.S01E01-thumb.jpg", Path: "/tv/Show/S1/raw.S01E01-thumb.jpg"},
	}
	plan := buildFullPreviewPlan(target, candidate, &tmdb.Detail{ID: 1399, Title: "Show", Year: 2011}, entries, []openlist.DirectoryEntry{{Name: "Show", Path: "/tv/Show", IsDir: true}})
	if !plan.Ready || len(plan.Conflicts) != 0 || len(plan.ProposedDirectoryRenames) != 2 || len(plan.ProposedFileRenames) != 3 {
		t.Fatalf("unexpected plan: %#v", plan)
	}
	wantVideo := "/tv/Show (2011) {tmdbid-1399}/Season 01/Show - S01E01.mkv"
	if plan.ProposedFileRenames[0].TargetPath != wantVideo {
		t.Fatalf("unexpected video target: %#v", plan.ProposedFileRenames)
	}
	if plan.ProposedFileRenames[1].TargetPath != "/tv/Show (2011) {tmdbid-1399}/Season 01/Show - S01E01.zh-CN.ass" {
		t.Fatalf("subtitle association was not preserved: %#v", plan.ProposedFileRenames)
	}
}

func TestSeriesPlanBlocksAmbiguousSeasonAndDuplicateEpisode(t *testing.T) {
	target := &model.ScrapeTarget{RootPath: "/tv", LibraryType: "tv", RenameEnabled: true}
	candidate := &model.MediaCandidate{Path: "/tv/Show", Kind: "tv", RepresentativeFile: "S03 第四季/raw.S03E01.mkv"}
	entries := []openlist.DirectoryEntry{
		{Name: "S03 第四季", Path: "/tv/Show/S03 第四季", IsDir: true},
		{Name: "a.S03E01.mkv", Path: "/tv/Show/S03 第四季/a.S03E01.mkv"},
		{Name: "b.S03E01.mkv", Path: "/tv/Show/S03 第四季/b.S03E01.mkv"},
	}
	plan := buildFullPreviewPlan(target, candidate, &tmdb.Detail{ID: 1, Title: "Show", Year: 2020}, entries, nil)
	if plan.Ready || len(plan.Conflicts) < 2 {
		t.Fatalf("expected blocking conflicts: %#v", plan)
	}
}

func TestMoviePlanDetectsExistingDestination(t *testing.T) {
	target := &model.ScrapeTarget{RootPath: "/movies", LibraryType: "movie", RenameEnabled: true}
	candidate := &model.MediaCandidate{Path: "/movies/Arrival.mkv", Kind: "movie", RepresentativeFile: "Arrival.mkv"}
	video := openlist.DirectoryEntry{Name: "Arrival.mkv", Path: candidate.Path}
	siblings := []openlist.DirectoryEntry{video, {Name: "Arrival (2016) {tmdbid-329865}", Path: "/movies/Arrival (2016) {tmdbid-329865}", IsDir: true}}
	plan := buildFullPreviewPlan(target, candidate, &tmdb.Detail{ID: 329865, Title: "Arrival", Year: 2016}, []openlist.DirectoryEntry{video}, siblings)
	if plan.Ready || len(plan.Conflicts) == 0 || plan.Conflicts[0].Code != "target_exists" {
		t.Fatalf("existing target was not blocked: %#v", plan)
	}
}
