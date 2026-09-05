package media

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

var (
	movieVersionResolutionPattern = regexp.MustCompile(`(?i)(?:^|[ ._\-\[(])(4320p|2160p|1080p|720p|576p|480p|8k|4k|uhd)(?:$|[ ._\-\])])`)
	movieVersionMultipartPattern  = regexp.MustCompile(`(?i)(?:^|[ ._\-\[(])(?:cd|disc|disk|part|pt)[ ._\-]*\d+(?:$|[ ._\-\])])`)
	movieVersionTechnicalPattern  = regexp.MustCompile(`(?i)^(?:4320p|2160p|1080p|720p|576p|480p|8k|4k|uhd|hdr10\+?|hdr|dolby[ ._-]*vision|dv|bluray|blu-ray|web[ ._-]*dl|webrip|hdtv|remux|x26[45]|h26[45]|hevc|avc|aac|dts|flac|10bit)$`)
	movieVersionDVPattern         = regexp.MustCompile(`(?i)(?:^|[ ._\-])dv(?:$|[ ._\-])`)
	movieVersionHDRPattern        = regexp.MustCompile(`(?i)(?:^|[ ._\-])hdr(?:$|[ ._\-])`)
)

type MovieVersionLabel struct {
	Label  string
	Source string
}

// MovieIdentityKey returns a conservative identity used to group loose movie
// files. A provider id wins; otherwise both a parsed title and year are
// required so unrelated root-level files are never grouped by title alone.
func MovieIdentityKey(info Info, uniqueFallback string) string {
	if info.TMDBID != nil && *info.TMDBID > 0 {
		return "tmdb:" + strconv.Itoa(*info.TMDBID)
	}
	if strings.TrimSpace(info.Title) != "" && info.Year != nil {
		return "title:" + strings.ToLower(strings.Join(strings.Fields(info.Title), " ")) + ":" + strconv.Itoa(*info.Year)
	}
	return "file:" + strings.ToLower(uniqueFallback)
}

func IsMovieExtra(candidateRoot string, entryPath string) bool {
	relative := strings.TrimPrefix(entryPath, strings.TrimRight(candidateRoot, "/")+"/")
	for _, component := range strings.Split(strings.ToLower(path.Dir(relative)), "/") {
		normalized := strings.NewReplacer("_", " ", "-", " ").Replace(component)
		switch strings.Join(strings.Fields(normalized), " ") {
		case "extras", "extra", "trailers", "trailer", "featurettes", "featurette", "samples", "sample", "behind the scenes", "deleted scenes", "interviews", "shorts":
			return true
		}
	}
	base := strings.ToLower(strings.TrimSuffix(path.Base(entryPath), path.Ext(entryPath)))
	for _, suffix := range []string{"-trailer", ".trailer", "_trailer", "-sample", ".sample", "_sample", "-featurette", ".featurette", "_featurette"} {
		if strings.HasSuffix(base, suffix) {
			return true
		}
	}
	return false
}

func IsMultipartMovieFile(name string) bool {
	return movieVersionMultipartPattern.MatchString(strings.TrimSuffix(path.Base(name), path.Ext(name)))
}

// InferMovieVersionLabel creates stable, user-facing labels from common
// edition, resolution, HDR, source and remux tokens. Unknown text after an
// explicit " - " separator is retained as an explicit edition label.
func InferMovieVersionLabel(name string) MovieVersionLabel {
	base := strings.TrimSpace(strings.TrimSuffix(path.Base(name), path.Ext(name)))
	lower := strings.ToLower(base)
	parts := make([]string, 0, 5)
	appendPart := func(value string) {
		for _, existing := range parts {
			if strings.EqualFold(existing, value) {
				return
			}
		}
		parts = append(parts, value)
	}

	editions := []struct {
		patterns []string
		label    string
	}{
		{[]string{"director's cut", "directors cut", "director cut"}, "Director's Cut"},
		{[]string{"extended cut", "extended edition", "extended"}, "Extended"},
		{[]string{"theatrical cut", "theatrical edition", "theatrical"}, "Theatrical"},
		{[]string{"unrated cut", "unrated"}, "Unrated"},
		{[]string{"open matte", "open.matte", "open_matte"}, "Open Matte"},
		{[]string{"criterion"}, "Criterion"},
		{[]string{"imax"}, "IMAX"},
	}
	for _, edition := range editions {
		for _, marker := range edition.patterns {
			if strings.Contains(lower, marker) {
				appendPart(edition.label)
				break
			}
		}
	}
	if match := movieVersionResolutionPattern.FindStringSubmatch(base); len(match) == 2 {
		resolution := strings.ToLower(match[1])
		switch resolution {
		case "4k", "uhd":
			resolution = "2160p"
		case "8k":
			resolution = "4320p"
		}
		appendPart(resolution)
	}
	if strings.Contains(lower, "dolby vision") || movieVersionDVPattern.MatchString(base) {
		appendPart("Dolby Vision")
	} else if strings.Contains(lower, "hdr10+") {
		appendPart("HDR10+")
	} else if strings.Contains(lower, "hdr10") {
		appendPart("HDR10")
	} else if movieVersionHDRPattern.MatchString(base) {
		appendPart("HDR")
	}
	if strings.Contains(lower, "remux") {
		appendPart("Remux")
	} else if strings.Contains(lower, "blu-ray") || strings.Contains(lower, "bluray") {
		appendPart("BluRay")
	} else if strings.Contains(lower, "web-dl") || strings.Contains(lower, "web.dl") || strings.Contains(lower, "web_dl") {
		appendPart("WEB-DL")
	} else if strings.Contains(lower, "webrip") || strings.Contains(lower, "web-rip") {
		appendPart("WEBRip")
	} else if strings.Contains(lower, "hdtv") {
		appendPart("HDTV")
	}
	if len(parts) > 0 {
		source := "derived"
		if strings.Contains(base, " - ") {
			source = "explicit"
		} else if len(parts) == 1 && movieVersionResolutionPattern.MatchString(base) {
			source = "resolution"
		}
		return MovieVersionLabel{Label: strings.Join(parts, " "), Source: source}
	}

	if index := strings.LastIndex(base, " - "); index >= 0 {
		suffix := strings.TrimSpace(base[index+3:])
		words := strings.Fields(strings.NewReplacer(".", " ", "_", " ").Replace(suffix))
		kept := make([]string, 0, len(words))
		for _, word := range words {
			if !movieVersionTechnicalPattern.MatchString(word) {
				kept = append(kept, word)
			}
		}
		if len(kept) > 0 {
			return MovieVersionLabel{Label: strings.Join(kept, " "), Source: "explicit"}
		}
	}
	return MovieVersionLabel{}
}
