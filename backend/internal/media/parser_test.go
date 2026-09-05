package media

import "testing"

func TestMovieUsesCandidateDirectory(t *testing.T) {
	result := ParseCandidate("Inception (2010)", "Inception.1080p.mkv", "movie")
	if result.Title != "Inception" || result.Year == nil || *result.Year != 2010 || result.Confidence < 70 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestTVUsesSeasonAndEpisodeMarkers(t *testing.T) {
	result := ParseCandidate("Breaking Bad", "Season 05/Breaking.Bad.S05E14.1080p.mkv", "tv")
	if result.Title != "Breaking Bad" || result.Season == nil || *result.Season != 5 || result.Episode == nil || *result.Episode != 14 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestAnimeSupportsAbsoluteEpisodes(t *testing.T) {
	result := ParseCandidate("葬送的芙莉莲", "Season 01/[ANi] Sousou no Frieren - 05 [1080P].mkv", "anime")
	if result.Season == nil || *result.Season != 1 || result.Episode == nil || *result.Episode != 5 {
		t.Fatalf("unexpected result: %#v", result)
	}
	result = ParseCandidate("海贼王", "[1071].mkv", "anime")
	if result.Season == nil || *result.Season != 1 || result.Episode == nil || *result.Episode != 1071 {
		t.Fatalf("unexpected absolute episode result: %#v", result)
	}
}

func TestSeasonAliasesAndTMDBID(t *testing.T) {
	checks := map[string]int{"Season 1": 1, "S02": 2, "第3季": 3, "第十二季": 12, "Specials": 0, "特别篇": 0}
	for value, expected := range checks {
		actual := ParseSeasonNumber(value)
		if actual == nil || *actual != expected {
			t.Fatalf("season %q: got %v, want %d", value, actual, expected)
		}
	}
	id := ExtractTMDBID("Movies/流浪地球 (2019) {tmdbid-535167}")
	if id == nil || *id != 535167 {
		t.Fatalf("unexpected tmdb id: %v", id)
	}
}

func TestMovieVersionLabelInference(t *testing.T) {
	tests := map[string]string{
		"Movie (2020) - 2160p.x265.AAC.mkv":                 "2160p",
		"Movie.2020.1080p.WEB-DL.mkv":                       "1080p WEB-DL",
		"Movie (2020) - Director's Cut.2160p.HDR.Remux.mkv": "Director's Cut 2160p HDR Remux",
		"Movie (2020) - Open.Matte.mkv":                     "Open Matte",
	}
	for name, expected := range tests {
		if result := InferMovieVersionLabel(name); result.Label != expected {
			t.Fatalf("%q: got %#v, want %q", name, result, expected)
		}
	}
	if result := InferMovieVersionLabel("Movie (2020).mkv"); result.Label != "" {
		t.Fatalf("plain movie unexpectedly received a version: %#v", result)
	}
}

func TestMovieExtraAndMultipartClassification(t *testing.T) {
	if !IsMovieExtra("/movies/Film", "/movies/Film/trailers/Film-trailer.mkv") {
		t.Fatal("trailer directory was not classified as an extra")
	}
	if IsMovieExtra("/movies/Film", "/movies/Film/Film - 2160p.mkv") {
		t.Fatal("main version was classified as an extra")
	}
	if !IsMultipartMovieFile("Film.CD1.mkv") || IsMultipartMovieFile("Film.2160p.mkv") {
		t.Fatal("multipart classification failed")
	}
}

func TestVideoExtensionsAreCaseInsensitive(t *testing.T) {
	if !IsVideoFile("Film.MKV") || IsVideoFile("poster.jpg") {
		t.Fatal("unexpected video extension classification")
	}
}

func TestContainedSeasonDirectorySemantics(t *testing.T) {
	checks := map[string]int{
		"黑袍纠察队 第四季 1080p Remux":     4,
		"The Boys Season 4 Remux":   4,
		"The.Boys.S04.2160p":        4,
		"The Boys Specials":         0,
		"The Boys S04 Season 4 第四季": 4,
	}
	for value, expected := range checks {
		result := ParseSeasonDirectoryName(value)
		if result.Status != "matched" || result.Season == nil || *result.Season != expected {
			t.Fatalf("%q: unexpected result %#v", value, result)
		}
	}
	if result := ParseSeasonDirectoryName("S03 第四季"); result.Status != "ambiguous" {
		t.Fatalf("expected ambiguity, got %#v", result)
	}
	for _, value := range []string{"四季酒店 1080p Remux", "The Boys S04E01 1080p"} {
		if result := ParseSeasonDirectoryName(value); result.Status != "no_match" {
			t.Fatalf("%q should not match: %#v", value, result)
		}
	}
	parsed := ParseCandidate("The Boys", "黑袍纠察队 第四季 1080p Remux/E01.mkv", "tv")
	if parsed.Season == nil || *parsed.Season != 4 || parsed.Episode == nil || *parsed.Episode != 1 {
		t.Fatalf("compound season directory did not inform episode parsing: %#v", parsed)
	}
}
