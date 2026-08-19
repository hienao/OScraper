package metadata

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"

	"oscraper/internal/provider/tmdb"
)

func TestMovieNFOIsEscapedAndKodiCompatible(t *testing.T) {
	detail := &tmdb.Detail{
		ID: 329865, MediaType: "movie", Title: "Arrival & 降临", OriginalTitle: "Arrival", Year: 2016,
		ReleaseDate: "2016-11-10", Overview: "Humans < visitors", Tagline: "Why are they here?", Runtime: 116,
		VoteAverage: 7.6, VoteCount: 18000, Genres: []tmdb.Genre{{Name: "Science Fiction"}},
		Studios: []string{"Paramount Pictures"}, IMDBID: "tt2543164", OriginalLanguage: "en",
	}
	content := BuildNFO(detail, time.Date(2026, 8, 19, 1, 2, 3, 0, time.UTC))
	if !strings.Contains(content, "<movie>") || !strings.Contains(content, "<tmdbid>329865</tmdbid>") || !strings.Contains(content, "Arrival &amp; 降临") {
		t.Fatalf("unexpected movie NFO:\n%s", content)
	}
	var parsed struct {
		Title string `xml:"title"`
		Plot  string `xml:"plot"`
	}
	if err := xml.Unmarshal([]byte(content), &parsed); err != nil || parsed.Title != detail.Title || parsed.Plot != detail.Overview {
		t.Fatalf("NFO is not valid XML: %#v %v", parsed, err)
	}
}

func TestEpisodeNFOContainsSeasonEpisodeAndShow(t *testing.T) {
	content := BuildEpisodeNFO("Show", tmdb.Episode{ID: 99, Name: "Pilot", SeasonNumber: 1, EpisodeNumber: 2, Overview: "Plot"}, time.Now())
	if !strings.Contains(content, "<episodedetails>") || !strings.Contains(content, "<showtitle>Show</showtitle>") || !strings.Contains(content, "<episode>2</episode>") {
		t.Fatalf("unexpected episode NFO:\n%s", content)
	}
}
