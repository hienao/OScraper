package metadata

import (
	"encoding/xml"
	"fmt"
	"time"

	"openlistscraper/internal/provider/tmdb"
)

type rating struct {
	Value float64 `xml:"value"`
	Votes int     `xml:"votes,omitempty"`
}

type common struct {
	Title         string   `xml:"title"`
	OriginalTitle string   `xml:"originaltitle,omitempty"`
	Plot          string   `xml:"plot,omitempty"`
	Runtime       int      `xml:"runtime,omitempty"`
	Rating        *rating  `xml:"rating,omitempty"`
	Year          int      `xml:"year,omitempty"`
	Genres        []string `xml:"genre,omitempty"`
	Studios       []string `xml:"studio,omitempty"`
	Thumb         string   `xml:"thumb,omitempty"`
	Fanart        string   `xml:"fanart,omitempty"`
	TMDBID        int      `xml:"tmdbid"`
	Country       string   `xml:"country,omitempty"`
	Language      string   `xml:"language,omitempty"`
	Status        string   `xml:"status,omitempty"`
	DateAdded     string   `xml:"dateadded"`
}

type movie struct {
	XMLName xml.Name `xml:"movie"`
	common
	Tagline     string `xml:"tagline,omitempty"`
	ReleaseDate string `xml:"releasedate,omitempty"`
	IMDBID      string `xml:"imdbid,omitempty"`
}

type tvShow struct {
	XMLName xml.Name `xml:"tvshow"`
	common
	Premiered string `xml:"premiered,omitempty"`
	Seasons   int    `xml:"season,omitempty"`
	Episodes  int    `xml:"episode,omitempty"`
}

type episodeDetails struct {
	XMLName   xml.Name `xml:"episodedetails"`
	Title     string   `xml:"title"`
	ShowTitle string   `xml:"showtitle"`
	Season    int      `xml:"season"`
	Episode   int      `xml:"episode"`
	Plot      string   `xml:"plot,omitempty"`
	Aired     string   `xml:"aired,omitempty"`
	Runtime   int      `xml:"runtime,omitempty"`
	Rating    *rating  `xml:"rating,omitempty"`
	Thumb     string   `xml:"thumb,omitempty"`
	TMDBID    int      `xml:"tmdbid"`
	DateAdded string   `xml:"dateadded"`
}

// BuildNFO creates a Kodi/Jellyfin/Emby-compatible UTF-8 metadata document.
func BuildNFO(detail *tmdb.Detail, generatedAt time.Time) string {
	base := common{
		Title: detail.Title, OriginalTitle: detail.OriginalTitle, Plot: detail.Overview, Runtime: detail.Runtime,
		Year: detail.Year, Genres: genreNames(detail.Genres), Studios: detail.Studios, Thumb: detail.PosterURL,
		Fanart: detail.BackdropURL, TMDBID: detail.ID, Country: detail.Country, Language: detail.OriginalLanguage,
		Status: detail.Status, DateAdded: generatedAt.UTC().Format("2006-01-02 15:04:05"),
	}
	if detail.VoteAverage > 0 || detail.VoteCount > 0 {
		base.Rating = &rating{Value: detail.VoteAverage, Votes: detail.VoteCount}
	}
	var document any
	if detail.MediaType == "tv" {
		document = tvShow{common: base, Premiered: detail.ReleaseDate, Seasons: detail.NumberOfSeasons, Episodes: detail.NumberOfEpisodes}
	} else {
		document = movie{common: base, Tagline: detail.Tagline, ReleaseDate: detail.ReleaseDate, IMDBID: detail.IMDBID}
	}
	encoded, err := xml.MarshalIndent(document, "", "  ")
	if err != nil {
		return ""
	}
	return fmt.Sprintf("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\n%s\n", encoded)
}

func BuildEpisodeNFO(showTitle string, episode tmdb.Episode, generatedAt time.Time) string {
	document := episodeDetails{
		Title: episode.Name, ShowTitle: showTitle, Season: episode.SeasonNumber, Episode: episode.EpisodeNumber,
		Plot: episode.Overview, Aired: episode.AirDate, Runtime: episode.Runtime, Thumb: episode.StillURL,
		TMDBID: episode.ID, DateAdded: generatedAt.UTC().Format("2006-01-02 15:04:05"),
	}
	if episode.VoteAverage > 0 || episode.VoteCount > 0 {
		document.Rating = &rating{Value: episode.VoteAverage, Votes: episode.VoteCount}
	}
	encoded, err := xml.MarshalIndent(document, "", "  ")
	if err != nil {
		return ""
	}
	return fmt.Sprintf("<?xml version=\"1.0\" encoding=\"UTF-8\" standalone=\"yes\"?>\n%s\n", encoded)
}

func genreNames(genres []tmdb.Genre) []string {
	result := make([]string, 0, len(genres))
	for _, genre := range genres {
		if genre.Name != "" {
			result = append(result, genre.Name)
		}
	}
	return result
}
