package service

import "testing"

func TestJobPlanKeepsSourcesValidUntilRootRename(t *testing.T) {
	plan := PreviewPlan{
		SourcePath: "/tv/Show", ProposedDirectoryPath: "/tv/Show (2020) {tmdbid-1}",
		ProposedDirectoryRenames: []RenameItem{
			{SourcePath: "/tv/Show", TargetPath: "/tv/Show (2020) {tmdbid-1}", AssetType: "directory"},
			{SourcePath: "/tv/Show/S1", TargetPath: "/tv/Show (2020) {tmdbid-1}/Season 01", AssetType: "directory"},
		},
		ProposedFileRenames: []RenameItem{{SourcePath: "/tv/Show/S1/raw.S01E01.mkv", TargetPath: "/tv/Show (2020) {tmdbid-1}/Season 01/Show - S01E01.mkv", AssetType: "video"}},
		Artifacts:           []PreviewArtifact{{Path: "/tv/Show (2020) {tmdbid-1}/tvshow.nfo", Kind: "nfo", Content: "<tvshow/>"}},
	}
	operations, err := buildJobOperations(plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(operations) != 5 {
		t.Fatalf("unexpected operations: %#v", operations)
	}
	if operations[0].SourcePath != "/tv/Show/S1" || operations[0].TargetPath != "/tv/Show/Season 01" {
		t.Fatalf("season directory was not normalized under the current root first: %#v", operations)
	}
	if operations[1].SourcePath != "/tv/Show/Season 01/raw.S01E01.mkv" || operations[2].SourcePath != "/tv/Show" || operations[3].Type != "upload" || operations[4].Type != "marker" {
		t.Fatalf("root rename ordering is unsafe: %#v", operations)
	}
}

func TestFlatMovieJobPlanMovesThenRenames(t *testing.T) {
	plan := PreviewPlan{
		SourcePath: "/movies/Arrival.mkv", ProposedDirectoryPath: "/movies/Arrival (2016) {tmdbid-1}", OrganizeFlatMovie: true,
		ProposedDirectoryCreates: []string{"/movies/Arrival (2016) {tmdbid-1}"},
		ProposedFileRenames:      []RenameItem{{SourcePath: "/movies/Arrival.mkv", TargetPath: "/movies/Arrival (2016) {tmdbid-1}/Arrival (2016) {tmdbid-1}.mkv", AssetType: "video"}},
		Artifacts:                []PreviewArtifact{{Path: "/movies/Arrival (2016) {tmdbid-1}/Arrival (2016) {tmdbid-1}.nfo", Kind: "nfo", Content: "<movie/>"}},
	}
	operations, err := buildJobOperations(plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(operations) != 5 || operations[0].Type != "mkdir" || operations[1].Type != "move" || operations[2].Type != "rename" || operations[3].Type != "upload" || operations[4].Type != "marker" {
		t.Fatalf("unexpected flat movie operations: %#v", operations)
	}
}

func TestLooseMovieVersionsShareOneCreatedDirectory(t *testing.T) {
	root := "/movies/Arrival (2016) {tmdbid-1}"
	plan := PreviewPlan{
		SourcePath: "/movies/Arrival.2016.2160p.mkv", ProposedDirectoryPath: root, OrganizeFlatMovie: true,
		ProposedDirectoryCreates: []string{root},
		ProposedFileRenames: []RenameItem{
			{SourcePath: "/movies/Arrival.2016.2160p.mkv", TargetPath: root + "/Arrival (2016) {tmdbid-1} - 2160p.mkv", AssetType: "video"},
			{SourcePath: "/movies/Arrival.2016.1080p.mp4", TargetPath: root + "/Arrival (2016) {tmdbid-1} - 1080p.mp4", AssetType: "video"},
		},
		Artifacts: []PreviewArtifact{{Path: root + "/movie.nfo", Kind: "nfo", Content: "<movie/>"}},
	}
	operations, err := buildJobOperations(plan)
	if err != nil {
		t.Fatal(err)
	}
	if len(operations) != 7 || operations[0].Type != "mkdir" || operations[1].Type != "move" || operations[2].Type != "rename" || operations[3].Type != "move" || operations[4].Type != "rename" || operations[5].Type != "upload" || operations[6].Type != "marker" {
		t.Fatalf("unexpected loose multi-version operations: %#v", operations)
	}
}
