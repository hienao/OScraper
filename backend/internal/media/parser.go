package media

import (
	"path"
	"regexp"
	"strconv"
	"strings"
)

var (
	tmdbIDPattern                 = regexp.MustCompile(`(?i)\{tmdbid-(\d+)\}`)
	yearPattern                   = regexp.MustCompile(`(?i)^(.*?)[\s._\-\[(]+((?:19|20)\d{2})[\])]?(?:\D.*)?$`)
	seasonEpisodePattern          = regexp.MustCompile(`(?i)(?:^|[\s._\-\[])S(\d{1,2})[\s._-]*E(\d{1,4})(?:\D|$)`)
	xEpisodePattern               = regexp.MustCompile(`(?i)(?:^|\D)(\d{1,2})x(\d{1,4})(?:\D|$)`)
	episodePattern                = regexp.MustCompile(`(?i)(?:^|[\s._\-\[])E(?:PISODE|P)?[\s._-]*(\d{1,4})(?:\D|$)`)
	absolutePattern               = regexp.MustCompile(`(?i)(?:^|[\s._\-\[])(\d{1,4})(?:v\d+)?(?:[\s._\-\]]|$)`)
	seasonPattern                 = regexp.MustCompile(`(?i)^(?:season|s)[\s._-]*(\d{1,2})$`)
	chineseSeasonPattern          = regexp.MustCompile(`^第([0-9零〇一二两三四五六七八九十]+)季$`)
	specialsPattern               = regexp.MustCompile(`(?i)^(?:specials?|special episodes?|特别篇|特别季|特典)$`)
	containedChineseSeasonPattern = regexp.MustCompile(`第\s*([0-9零〇一二两三四五六七八九十]+)\s*季`)
	containedEnglishSeasonPattern = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])season[\s._-]*(\d{1,3})`)
	containedShortSeasonPattern   = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])s[\s._-]*(\d{1,3})`)
	containedSpecialsPattern      = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:specials?|special[\s._-]+episodes?|特别篇|特别季|特典)(?:$|[^a-z0-9])`)
	releaseTagPattern             = regexp.MustCompile(`(?i)\b(?:2160p|1080p|720p|480p|4k|uhd|hdr|bluray|web-?dl|webrip|hdtv|x26[45]|h26[45]|hevc|avc|aac|dts|flac|10bit|remux)\b`)
	separatorPattern              = regexp.MustCompile(`[._\-\[\]()]+`)
)

var videoExtensions = map[string]struct{}{
	".mp4": {}, ".avi": {}, ".mkv": {}, ".mov": {}, ".wmv": {}, ".flv": {},
	".webm": {}, ".m4v": {}, ".rmvb": {}, ".ts": {}, ".vob": {}, ".3gp": {},
}

type Info struct {
	Title      string
	Year       *int
	Season     *int
	Episode    *int
	TMDBID     *int
	Confidence int
}

type SeasonDirectoryResult struct {
	Status  string
	Season  *int
	Markers []string
}

func IsVideoFile(name string) bool {
	_, ok := videoExtensions[strings.ToLower(path.Ext(strings.TrimSpace(name)))]
	return ok
}

// ParseCandidate ports the task-oriented parsing rules used by ostrm. The
// candidate name supplies the movie/show title while the representative file
// and its relative path supply season and episode markers.
func ParseCandidate(candidateName, representativePath, libraryType string) Info {
	title, year := parseTitle(candidateName)
	result := Info{Title: title, Year: year, TMDBID: ExtractTMDBID(candidateName + "/" + representativePath)}

	if libraryType == "movie" {
		result.Confidence = 60
		if result.Title != "" {
			result.Confidence += 20
		}
		if year != nil {
			result.Confidence += 20
		}
		return result
	}

	for _, component := range strings.Split(path.Dir(representativePath), "/") {
		season := ParseSeasonNumber(component)
		if season == nil {
			contained := ParseSeasonDirectoryName(component)
			if contained.Status == "matched" {
				season = contained.Season
			}
		}
		if season != nil {
			result.Season = season
			break
		}
	}
	result.Season, result.Episode = parseEpisode(path.Base(representativePath), result.Season, libraryType == "anime")
	result.Confidence = 20
	if result.Title != "" {
		result.Confidence += 40
	}
	if result.Season != nil {
		result.Confidence += 10
	}
	if result.Episode != nil {
		result.Confidence += 20
	}
	if result.Year != nil {
		result.Confidence += 10
	}
	return result
}

func ExtractTMDBID(value string) *int {
	match := tmdbIDPattern.FindStringSubmatch(value)
	if len(match) != 2 {
		return nil
	}
	valueInt, err := strconv.Atoi(match[1])
	if err != nil || valueInt <= 0 {
		return nil
	}
	return &valueInt
}

func ParseSeasonNumber(value string) *int {
	value = strings.TrimSpace(value)
	if match := seasonPattern.FindStringSubmatch(value); len(match) == 2 {
		return integerPointer(match[1])
	}
	if match := chineseSeasonPattern.FindStringSubmatch(value); len(match) == 2 {
		if number, ok := parseChineseNumber(match[1]); ok {
			return &number
		}
	}
	if specialsPattern.MatchString(value) {
		zero := 0
		return &zero
	}
	return nil
}

// ParseSeasonDirectoryName ports ostrm's contained season marker semantics.
// It accepts compound directory names but rejects ambiguous season numbers and
// does not treat an SxxExx episode marker as a season directory.
func ParseSeasonDirectoryName(value string) SeasonDirectoryResult {
	value = strings.TrimSpace(value)
	if value == "" {
		return SeasonDirectoryResult{Status: "no_match", Markers: []string{}}
	}
	seasons := map[int]struct{}{}
	markers := make([]string, 0)
	invalid := false
	collect := func(pattern *regexp.Regexp, chinese, short bool) {
		for _, indices := range pattern.FindAllStringSubmatchIndex(value, -1) {
			if len(indices) < 4 {
				continue
			}
			if short {
				remainder := value[indices[1]:]
				remainder = strings.TrimLeft(remainder, " ._-")
				if len(remainder) > 1 && (remainder[0] == 'e' || remainder[0] == 'E') && remainder[1] >= '0' && remainder[1] <= '9' {
					continue
				}
			}
			marker := strings.TrimSpace(value[indices[0]:indices[1]])
			markers = append(markers, marker)
			var number int
			var ok bool
			if chinese {
				number, ok = parseChineseNumber(value[indices[2]:indices[3]])
			} else {
				parsed := integerPointer(value[indices[2]:indices[3]])
				ok = parsed != nil
				if ok {
					number = *parsed
				}
			}
			if !ok || number < 0 || number > 99 {
				invalid = true
				continue
			}
			seasons[number] = struct{}{}
		}
	}
	collect(containedChineseSeasonPattern, true, false)
	collect(containedEnglishSeasonPattern, false, false)
	collect(containedShortSeasonPattern, false, true)
	for _, marker := range containedSpecialsPattern.FindAllString(value, -1) {
		markers = append(markers, strings.TrimSpace(marker))
		seasons[0] = struct{}{}
	}
	if invalid {
		return SeasonDirectoryResult{Status: "invalid", Markers: markers}
	}
	if len(seasons) == 0 {
		return SeasonDirectoryResult{Status: "no_match", Markers: markers}
	}
	if len(seasons) > 1 {
		return SeasonDirectoryResult{Status: "ambiguous", Markers: markers}
	}
	for number := range seasons {
		season := number
		return SeasonDirectoryResult{Status: "matched", Season: &season, Markers: markers}
	}
	return SeasonDirectoryResult{Status: "no_match", Markers: markers}
}

func parseEpisode(fileName string, directorySeason *int, anime bool) (*int, *int) {
	name := strings.TrimSuffix(fileName, path.Ext(fileName))
	if match := seasonEpisodePattern.FindStringSubmatch(name); len(match) == 3 {
		return integerPointer(match[1]), integerPointer(match[2])
	}
	if match := xEpisodePattern.FindStringSubmatch(name); len(match) == 3 {
		return integerPointer(match[1]), integerPointer(match[2])
	}
	if match := episodePattern.FindStringSubmatch(name); len(match) == 2 {
		return directorySeason, integerPointer(match[1])
	}
	if anime {
		for _, match := range absolutePattern.FindAllStringSubmatch(name, -1) {
			candidate, err := strconv.Atoi(match[1])
			if err == nil && candidate > 0 && candidate < 10000 && candidate != 480 && candidate != 720 && candidate != 1080 && candidate != 2160 {
				if directorySeason == nil {
					one := 1
					directorySeason = &one
				}
				return directorySeason, &candidate
			}
		}
	}
	return directorySeason, nil
}

func parseTitle(value string) (string, *int) {
	value = tmdbIDPattern.ReplaceAllString(strings.TrimSpace(value), " ")
	var year *int
	if match := yearPattern.FindStringSubmatch(value); len(match) == 3 {
		value = match[1]
		year = integerPointer(match[2])
	}
	value = releaseTagPattern.ReplaceAllString(value, " ")
	value = separatorPattern.ReplaceAllString(value, " ")
	value = strings.Join(strings.Fields(value), " ")
	return value, year
}

func integerPointer(value string) *int {
	number, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	return &number
}

func parseChineseNumber(value string) (int, bool) {
	if number, err := strconv.Atoi(value); err == nil {
		return number, number >= 0 && number <= 99
	}
	value = strings.NewReplacer("〇", "零", "两", "二").Replace(value)
	digit := func(r rune) (int, bool) {
		for index, candidate := range []rune("零一二三四五六七八九") {
			if r == candidate {
				return index, true
			}
		}
		return 0, false
	}
	parts := strings.Split(value, "十")
	if len(parts) == 1 {
		if len([]rune(value)) != 1 {
			return 0, false
		}
		return digit([]rune(value)[0])
	}
	if len(parts) != 2 {
		return 0, false
	}
	tens := 1
	if parts[0] != "" {
		var ok bool
		tens, ok = digit([]rune(parts[0])[0])
		if !ok || len([]rune(parts[0])) != 1 {
			return 0, false
		}
	}
	units := 0
	if parts[1] != "" {
		var ok bool
		units, ok = digit([]rune(parts[1])[0])
		if !ok || len([]rune(parts[1])) != 1 {
			return 0, false
		}
	}
	return tens*10 + units, true
}
