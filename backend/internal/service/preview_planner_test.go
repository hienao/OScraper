package service

import (
	"testing"

	"oscraper/internal/model"
	"oscraper/internal/openlist"
	"oscraper/internal/provider/tmdb"
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

func TestFlatMoviePlanMigratesExistingScrapeMarker(t *testing.T) {
	target := &model.ScrapeTarget{RootPath: "/movies", LibraryType: "movie", RenameEnabled: true}
	candidate := &model.MediaCandidate{Path: "/movies/Arrival.mkv", Kind: "movie", RepresentativeFile: "Arrival.mkv", Scraped: true}
	video := openlist.DirectoryEntry{Name: "Arrival.mkv", Path: candidate.Path}
	marker := openlist.DirectoryEntry{Name: "Arrival.mkv" + scrapeMarkerSuffix, Path: candidate.Path + scrapeMarkerSuffix, Size: int64(len(scrapeMarkerContent))}
	plan := buildFullPreviewPlan(target, candidate, &tmdb.Detail{ID: 329865, Title: "Arrival", Year: 2016}, []openlist.DirectoryEntry{video, marker}, []openlist.DirectoryEntry{video, marker})
	if !plan.Ready || plan.ScrapeMarkerPath != "/movies/Arrival (2016) {tmdbid-329865}/"+scrapeMarkerName {
		t.Fatalf("unexpected marker plan: %#v", plan)
	}
	found := false
	for _, rename := range plan.ProposedFileRenames {
		if rename.SourcePath == marker.Path && rename.TargetPath == plan.ScrapeMarkerPath && rename.AssetType == "marker" {
			found = true
		}
	}
	if !found {
		t.Fatalf("flat scrape marker was not migrated: %#v", plan.ProposedFileRenames)
	}
}

func TestMoviePlanSupportsMultipleVersionsAndCompanions(t *testing.T) {
	target := &model.ScrapeTarget{RootPath: "/movies", LibraryType: "movie", RenameEnabled: true}
	candidate := &model.MediaCandidate{Path: "/movies/碟中谍5：神秘国度 (2015) {tmdbid=177677}", Kind: "movie", RepresentativeFile: "碟中谍5：神秘国度 (2015) - 2160p.x265.AAC.mp4", VideoCount: 2}
	entries := []openlist.DirectoryEntry{
		{Name: "碟中谍5：神秘国度 (2015) - 2160p.x265.AAC.mp4", Path: candidate.Path + "/碟中谍5：神秘国度 (2015) - 2160p.x265.AAC.mp4"},
		{Name: "碟中谍5：神秘国度 (2015) - 2160p.x265.AAC.nfo", Path: candidate.Path + "/碟中谍5：神秘国度 (2015) - 2160p.x265.AAC.nfo"},
		{Name: "碟中谍5：神秘国度 (2015) - 720p.rmvb", Path: candidate.Path + "/碟中谍5：神秘国度 (2015) - 720p.rmvb"},
		{Name: "碟中谍5：神秘国度 (2015) - 720p.zh.srt", Path: candidate.Path + "/碟中谍5：神秘国度 (2015) - 720p.zh.srt"},
		{Name: "碟中谍5：神秘国度 (2015) - 720p.nfo", Path: candidate.Path + "/碟中谍5：神秘国度 (2015) - 720p.nfo"},
	}
	detail := &tmdb.Detail{ID: 177677, Title: "碟中谍5：神秘国度", Year: 2015}
	plan := buildFullPreviewPlan(target, candidate, detail, entries, nil)
	if !plan.Ready || len(plan.MovieVersions) != 2 || len(plan.Conflicts) != 0 {
		t.Fatalf("unexpected multi-version plan: %#v", plan)
	}
	if plan.MovieVersions[0].Label != "2160p" || plan.MovieVersions[1].Label != "720p" {
		t.Fatalf("unexpected labels: %#v", plan.MovieVersions)
	}
	wantSubtitle := "/movies/碟中谍5：神秘国度 (2015) {tmdbid-177677}/碟中谍5：神秘国度 (2015) {tmdbid-177677} - 720p.zh.srt"
	foundSubtitle, renamedNFO := false, false
	for _, rename := range plan.ProposedFileRenames {
		foundSubtitle = foundSubtitle || rename.TargetPath == wantSubtitle
		renamedNFO = renamedNFO || rename.AssetType == "nfo"
	}
	if !foundSubtitle || renamedNFO {
		t.Fatalf("companions were not planned safely: %#v", plan.ProposedFileRenames)
	}
	if len(plan.Artifacts) == 0 || plan.Artifacts[0].Path != "/movies/碟中谍5：神秘国度 (2015) {tmdbid-177677}/movie.nfo" {
		t.Fatalf("multi-version movie did not use shared metadata: %#v", plan.Artifacts)
	}
	if len(plan.Warnings) != 1 || plan.Warnings[0] != "legacy_movie_nfo_preserved" {
		t.Fatalf("legacy NFO warning missing: %#v", plan.Warnings)
	}
}

func TestMoviePlanAllowsVersionLabelOverrides(t *testing.T) {
	target := &model.ScrapeTarget{RootPath: "/movies", LibraryType: "movie", RenameEnabled: true}
	candidate := &model.MediaCandidate{Path: "/movies/Film", Kind: "movie", VideoCount: 2}
	entries := []openlist.DirectoryEntry{
		{Name: "Film-a.mkv", Path: "/movies/Film/Film-a.mkv"},
		{Name: "Film-b.mkv", Path: "/movies/Film/Film-b.mkv"},
	}
	detail := &tmdb.Detail{ID: 1, Title: "Film", Year: 2020}
	blocked := buildFullPreviewPlan(target, candidate, detail, entries, nil)
	if blocked.Ready || len(blocked.Conflicts) != 2 {
		t.Fatalf("missing labels did not block: %#v", blocked)
	}
	labels := map[string]string{entries[0].Path: "Theatrical", entries[1].Path: "Director's Cut"}
	plan := buildFullPreviewPlanWithVersionLabels(target, candidate, detail, entries, nil, labels)
	if !plan.Ready || len(plan.Conflicts) != 0 || plan.MovieVersions[1].LabelSource != "user" {
		t.Fatalf("version overrides were not applied: %#v", plan)
	}
}

func TestMoviePlanRejectsDuplicateVersionsAndMultipart(t *testing.T) {
	target := &model.ScrapeTarget{RootPath: "/movies", LibraryType: "movie", RenameEnabled: true}
	candidate := &model.MediaCandidate{Path: "/movies/Film", Kind: "movie", VideoCount: 2}
	entries := []openlist.DirectoryEntry{
		{Name: "Film.1080p.CD1.mkv", Path: "/movies/Film/Film.1080p.CD1.mkv"},
		{Name: "Film.1080p.CD2.mp4", Path: "/movies/Film/Film.1080p.CD2.mp4"},
	}
	plan := buildFullPreviewPlan(target, candidate, &tmdb.Detail{ID: 1, Title: "Film", Year: 2020}, entries, nil)
	codes := map[string]bool{}
	for _, conflict := range plan.Conflicts {
		codes[conflict.Code] = true
	}
	if plan.Ready || !codes["multipart_movie_unsupported"] || !codes["movie_version_label_duplicate"] {
		t.Fatalf("unsafe multipart plan was not blocked: %#v", plan)
	}
}

func TestMoviePlanAssignsCompanionToLongestMatchingVersionName(t *testing.T) {
	target := &model.ScrapeTarget{RootPath: "/movies", LibraryType: "movie", RenameEnabled: true}
	candidate := &model.MediaCandidate{Path: "/movies/Film", Kind: "movie", VideoCount: 2}
	entries := []openlist.DirectoryEntry{
		{Name: "Film (2020).mkv", Path: "/movies/Film/Film (2020).mkv"},
		{Name: "Film (2020) - 2160p.mkv", Path: "/movies/Film/Film (2020) - 2160p.mkv"},
		{Name: "Film (2020) - 2160p.zh.srt", Path: "/movies/Film/Film (2020) - 2160p.zh.srt"},
	}
	labels := map[string]string{entries[0].Path: "1080p", entries[1].Path: "2160p"}
	plan := buildFullPreviewPlanWithVersionLabels(target, candidate, &tmdb.Detail{ID: 1, Title: "Film", Year: 2020}, entries, nil, labels)
	want := "/movies/Film (2020) {tmdbid-1}/Film (2020) {tmdbid-1} - 2160p.zh.srt"
	for _, rename := range plan.ProposedFileRenames {
		if rename.SourcePath == entries[2].Path {
			if rename.TargetPath != want {
				t.Fatalf("subtitle assigned to the wrong version: %#v", rename)
			}
			return
		}
	}
	t.Fatalf("subtitle was not planned: %#v", plan.ProposedFileRenames)
}

func TestMoviePlanDoesNotTreatTrailerAsAVersion(t *testing.T) {
	target := &model.ScrapeTarget{RootPath: "/movies", LibraryType: "movie", RenameEnabled: true}
	candidate := &model.MediaCandidate{Path: "/movies/Film", Kind: "movie", VideoCount: 2}
	entries := []openlist.DirectoryEntry{
		{Name: "Film.mkv", Path: "/movies/Film/Film.mkv"},
		{Name: "trailers", Path: "/movies/Film/trailers", IsDir: true},
		{Name: "Film-trailer.mkv", Path: "/movies/Film/trailers/Film-trailer.mkv"},
	}
	plan := buildFullPreviewPlan(target, candidate, &tmdb.Detail{ID: 1, Title: "Film", Year: 2020}, entries, nil)
	if !plan.Ready || len(plan.MovieVersions) != 0 || len(plan.ProposedFileRenames) != 1 {
		t.Fatalf("trailer was treated as a movie version: %#v", plan)
	}
}
