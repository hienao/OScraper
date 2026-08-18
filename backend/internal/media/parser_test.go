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

func TestVideoExtensionsAreCaseInsensitive(t *testing.T) {
	if !IsVideoFile("Film.MKV") || IsVideoFile("poster.jpg") {
		t.Fatal("unexpected video extension classification")
	}
}
