package service

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"time"

	"oscraper/internal/media"
	"oscraper/internal/model"
	"oscraper/internal/openlist"
	"oscraper/internal/provider/tmdb"
)

type renamePlanner struct {
	plan       *PreviewPlan
	existing   map[string]string
	claimed    map[string]string
	conflictID map[string]struct{}
}

func buildFullPreviewPlan(target *model.ScrapeTarget, candidate *model.MediaCandidate, detail *tmdb.Detail, entries, siblings []openlist.DirectoryEntry) PreviewPlan {
	return buildFullPreviewPlanWithVersionLabels(target, candidate, detail, entries, siblings, nil)
}

func buildFullPreviewPlanWithVersionLabels(target *model.ScrapeTarget, candidate *model.MediaCandidate, detail *tmdb.Detail, entries, siblings []openlist.DirectoryEntry, versionLabels map[string]string) PreviewPlan {
	standardName := safeMediaName(detail.Title)
	if detail.Year > 0 {
		standardName += fmt.Sprintf(" (%d)", detail.Year)
	}
	standardName += fmt.Sprintf(" {tmdbid-%d}", detail.ID)
	flatMovie := candidate.Kind == "movie" && media.IsVideoFile(path.Base(candidate.Path))
	if len(entries) == 0 && candidate.RepresentativeFile != "" {
		representativePath := candidate.Path
		if !flatMovie {
			representativePath = path.Join(candidate.Path, candidate.RepresentativeFile)
		}
		entries = []openlist.DirectoryEntry{{Name: path.Base(representativePath), Path: representativePath}}
	}
	proposedRoot := path.Join(path.Dir(candidate.Path), standardName)
	finalRoot := proposedRoot
	if !target.RenameEnabled {
		if flatMovie {
			finalRoot = path.Dir(candidate.Path)
		} else {
			finalRoot = candidate.Path
		}
	}
	plan := PreviewPlan{
		ReadOnly: true, RenameAllowed: target.RenameEnabled, OrganizeFlatMovie: flatMovie,
		SourcePath: candidate.Path, ProposedDirectoryName: path.Base(finalRoot), ProposedDirectoryPath: finalRoot,
		ProposedDirectoryCreates: []string{}, ProposedDirectoryRenames: []RenameItem{},
		ProposedFileRenames: []RenameItem{}, GeneratedFiles: []string{}, Artifacts: []PreviewArtifact{}, EpisodeFiles: []EpisodeFilePlan{}, SkippedEpisodes: []EpisodeFilePlan{}, MovieVersions: []MovieVersionPlan{}, Warnings: []string{}, Conflicts: []PlanConflict{},
	}
	plan.ScrapeMarkerPath = markerPathForPlan(plan)
	planner := newRenamePlanner(&plan, entries, siblings)
	if target.RenameEnabled {
		if flatMovie {
			planner.addCreate(proposedRoot, candidate.Path)
		} else {
			planner.addRename(candidate.Path, proposedRoot, "directory")
		}
	} else {
		plan.Warnings = append(plan.Warnings, "rename_disabled")
	}
	if detail.Year == 0 {
		plan.Warnings = append(plan.Warnings, "year_missing")
	}

	if candidate.Kind == "movie" {
		planner.planMovie(candidate, entries, siblings, standardName, finalRoot, target.RenameEnabled, versionLabels)
	} else {
		planner.planSeries(candidate, entries, detail.Title, finalRoot, target.RenameEnabled)
	}
	metadataDetail := *detail
	if candidate.Kind == "movie" {
		metadataDetail.MediaType = "movie"
	} else {
		metadataDetail.MediaType = "tv"
	}
	plan.Artifacts = buildMetadataArtifacts(&metadataDetail, finalRoot, standardName, time.Now().UTC())
	if candidate.Kind == "movie" && len(plan.MovieVersions) > 1 {
		for index := range plan.Artifacts {
			if plan.Artifacts[index].Kind == "nfo" {
				plan.Artifacts[index].Path = path.Join(finalRoot, "movie.nfo")
				break
			}
		}
	}
	for _, artifact := range plan.Artifacts {
		plan.GeneratedFiles = append(plan.GeneratedFiles, artifact.Path)
	}
	plan.Ready = detail.ID > 0 && detail.Title != "" && detail.Year > 0 && len(plan.Conflicts) == 0
	return plan
}

func newRenamePlanner(plan *PreviewPlan, groups ...[]openlist.DirectoryEntry) *renamePlanner {
	planner := &renamePlanner{plan: plan, existing: map[string]string{}, claimed: map[string]string{}, conflictID: map[string]struct{}{}}
	for _, entries := range groups {
		for _, entry := range entries {
			planner.existing[strings.ToLower(entry.Path)] = entry.Path
		}
	}
	return planner
}

func (p *renamePlanner) planMovie(candidate *model.MediaCandidate, entries, siblings []openlist.DirectoryEntry, standardName, finalRoot string, rename bool, versionLabels map[string]string) {
	assets := entries
	if media.IsVideoFile(path.Base(candidate.Path)) {
		if candidate.VideoCount <= 1 {
			assets = relatedFlatAssets(candidate.Path, siblings)
			if len(assets) == 0 {
				assets = entries
			}
		}
	}
	videos := movieMainVideoEntries(candidate, assets)
	if len(videos) == 0 {
		p.addConflict("video_missing", candidate.Path, "")
		return
	}
	multiVersion := len(videos) > 1
	if multiVersion && media.IsVideoFile(path.Base(candidate.Path)) && !rename {
		p.addConflict("movie_versions_require_rename", candidate.Path, finalRoot)
	}
	if len(videos) > 8 {
		p.addConflict("movie_version_limit_exceeded", candidate.Path, finalRoot)
	}
	if multiVersion && !media.IsVideoFile(path.Base(candidate.Path)) {
		for _, video := range videos {
			if path.Dir(video.Path) != candidate.Path {
				p.addConflict("movie_versions_not_colocated", video.Path, candidate.Path)
			}
		}
	}
	if multiVersion {
		for _, video := range videos {
			if media.IsMultipartMovieFile(video.Name) {
				p.addConflict("multipart_movie_unsupported", video.Path, "")
			}
		}
		if containsNFO(assets) {
			p.plan.Warnings = appendUnique(p.plan.Warnings, "legacy_movie_nfo_preserved")
		}
	}
	claimedAssets := map[string]struct{}{}
	claimedLabels := map[string]string{}
	for _, video := range videos {
		targetBase := standardName
		label, labelSource := "", ""
		labelValid := true
		if multiVersion {
			inferred := media.InferMovieVersionLabel(video.Name)
			label, labelSource = inferred.Label, inferred.Source
			if override, exists := versionLabels[video.Path]; exists {
				label = safeVersionLabel(override)
				labelSource = "user"
			}
			if label == "" {
				p.addConflict("movie_version_label_missing", video.Path, "")
				labelValid = false
			} else {
				key := strings.ToLower(label)
				if owner, exists := claimedLabels[key]; exists && owner != video.Path {
					p.addConflict("movie_version_label_duplicate", video.Path, owner)
					labelValid = false
				} else {
					claimedLabels[key] = video.Path
				}
				if labelValid {
					targetBase += " - " + label
				}
			}
		}
		targetVideo := video.Path
		if rename && labelValid {
			targetVideo = path.Join(finalRoot, targetBase+path.Ext(video.Name))
			p.addRename(video.Path, targetVideo, "video")
			p.planCompanions(video, assets, targetBase, finalRoot, claimedAssets, multiVersion)
		}
		if multiVersion {
			if !labelValid {
				targetVideo = ""
			}
			p.plan.MovieVersions = append(p.plan.MovieVersions, MovieVersionPlan{SourcePath: video.Path, TargetPath: targetVideo, Label: label, LabelSource: labelSource})
		}
	}
	if rename && media.IsVideoFile(path.Base(candidate.Path)) {
		flatMarkerPath := candidate.Path + scrapeMarkerSuffix
		for _, entry := range assets {
			if !entry.IsDir && entry.Path == flatMarkerPath {
				p.addRename(entry.Path, p.plan.ScrapeMarkerPath, "marker")
				break
			}
		}
	}
}

func (p *renamePlanner) planSeries(candidate *model.MediaCandidate, entries []openlist.DirectoryEntry, title, finalRoot string, rename bool) {
	seasonDirectories := map[int]string{}
	for _, entry := range entries {
		if !entry.IsDir {
			continue
		}
		result := media.ParseSeasonDirectoryName(entry.Name)
		switch result.Status {
		case "ambiguous":
			p.addConflict("season_ambiguous", entry.Path, "")
		case "invalid":
			p.addConflict("season_invalid", entry.Path, "")
		case "matched":
			season := *result.Season
			if existing, ok := seasonDirectories[season]; ok && existing != entry.Path {
				p.addConflict("duplicate_season_directory", entry.Path, path.Join(finalRoot, seasonName(season)))
			} else {
				seasonDirectories[season] = entry.Path
			}
			targetPath := path.Join(finalRoot, seasonName(season))
			if rename && entry.Name != seasonName(season) {
				p.addRename(entry.Path, targetPath, "directory")
			}
		}
	}

	videos := videoEntries(entries)
	claimedAssets := map[string]struct{}{}
	createdSeasons := map[int]struct{}{}
	for _, video := range videos {
		relative := strings.TrimPrefix(video.Path, strings.TrimRight(candidate.Path, "/")+"/")
		info := media.ParseCandidate(path.Base(candidate.Path), relative, candidate.Kind)
		if info.Season == nil || info.Episode == nil {
			p.addConflict("episode_unrecognized", video.Path, "")
			continue
		}
		season, episode := *info.Season, *info.Episode
		seasonDirectory := path.Join(finalRoot, seasonName(season))
		if !rename {
			seasonDirectory = path.Dir(video.Path)
		}
		if rename {
			if _, hasDirectory := seasonDirectories[season]; !hasDirectory {
				if _, created := createdSeasons[season]; !created {
					p.addCreate(seasonDirectory, video.Path)
					createdSeasons[season] = struct{}{}
				}
			}
		}
		targetBase := fmt.Sprintf("%s - S%02dE%02d", safeMediaName(title), season, episode)
		targetVideo := path.Join(seasonDirectory, targetBase+path.Ext(video.Name))
		if !rename {
			targetVideo = video.Path
		}
		p.plan.EpisodeFiles = append(p.plan.EpisodeFiles, EpisodeFilePlan{SourcePath: video.Path, TargetPath: targetVideo, Season: season, Episode: episode})
		if rename {
			p.addRename(video.Path, targetVideo, "video")
			p.planCompanions(video, entries, targetBase, seasonDirectory, claimedAssets)
		}
	}
}

func (p *renamePlanner) planCompanions(video openlist.DirectoryEntry, entries []openlist.DirectoryEntry, targetBase, targetDirectory string, claimed map[string]struct{}, skipNFO ...bool) {
	videoBase := strings.TrimSuffix(video.Name, path.Ext(video.Name))
	for _, entry := range entries {
		if entry.IsDir || entry.Path == video.Path || path.Dir(entry.Path) != path.Dir(video.Path) || media.IsVideoFile(entry.Name) || isScrapeMarkerName(entry.Name) {
			continue
		}
		if len(skipNFO) > 0 && skipNFO[0] && strings.EqualFold(path.Ext(entry.Name), ".nfo") {
			continue
		}
		key := strings.ToLower(entry.Path)
		if _, exists := claimed[key]; exists {
			continue
		}
		entryBase := strings.TrimSuffix(entry.Name, path.Ext(entry.Name))
		if entryBase != videoBase && !strings.HasPrefix(entryBase, videoBase+".") && !strings.HasPrefix(entryBase, videoBase+"-") && !strings.HasPrefix(entryBase, videoBase+"_") {
			continue
		}
		if len(skipNFO) > 0 && skipNFO[0] && movieCompanionOwner(entry, entries) != video.Path {
			continue
		}
		suffix := strings.TrimPrefix(entryBase, videoBase)
		targetName := targetBase + suffix + path.Ext(entry.Name)
		p.addRename(entry.Path, path.Join(targetDirectory, targetName), assetType(entry.Name))
		claimed[key] = struct{}{}
	}
}

func movieCompanionOwner(companion openlist.DirectoryEntry, entries []openlist.DirectoryEntry) string {
	companionBase := strings.TrimSuffix(companion.Name, path.Ext(companion.Name))
	owner, longest := "", -1
	for _, candidate := range entries {
		if candidate.IsDir || !media.IsVideoFile(candidate.Name) || path.Dir(candidate.Path) != path.Dir(companion.Path) {
			continue
		}
		videoBase := strings.TrimSuffix(candidate.Name, path.Ext(candidate.Name))
		if companionBase != videoBase && !strings.HasPrefix(companionBase, videoBase+".") && !strings.HasPrefix(companionBase, videoBase+"-") && !strings.HasPrefix(companionBase, videoBase+"_") {
			continue
		}
		if len(videoBase) > longest {
			owner, longest = candidate.Path, len(videoBase)
		}
	}
	return owner
}

func (p *renamePlanner) addCreate(targetPath, sourceContext string) {
	key := strings.ToLower(targetPath)
	if existing, exists := p.existing[key]; exists {
		p.addConflict("target_exists", sourceContext, existing)
		return
	}
	if owner, exists := p.claimed[key]; exists && owner != sourceContext {
		p.addConflict("duplicate_target", sourceContext, targetPath)
		return
	}
	p.claimed[key] = sourceContext
	p.plan.ProposedDirectoryCreates = appendUnique(p.plan.ProposedDirectoryCreates, targetPath)
}

func (p *renamePlanner) addRename(sourcePath, targetPath, assetTypeValue string) {
	if sourcePath == targetPath {
		return
	}
	key := strings.ToLower(targetPath)
	if existing, exists := p.existing[key]; exists && !strings.EqualFold(existing, sourcePath) {
		p.addConflict("target_exists", sourcePath, targetPath)
	}
	if owner, exists := p.claimed[key]; exists && owner != sourcePath {
		p.addConflict("duplicate_target", sourcePath, targetPath)
	}
	p.claimed[key] = sourcePath
	item := RenameItem{SourcePath: sourcePath, TargetPath: targetPath, AssetType: assetTypeValue}
	if assetTypeValue == "directory" {
		p.plan.ProposedDirectoryRenames = append(p.plan.ProposedDirectoryRenames, item)
	} else {
		p.plan.ProposedFileRenames = append(p.plan.ProposedFileRenames, item)
	}
}

func (p *renamePlanner) addConflict(code, sourcePath, targetPath string) {
	key := code + "\x00" + strings.ToLower(sourcePath) + "\x00" + strings.ToLower(targetPath)
	if _, exists := p.conflictID[key]; exists {
		return
	}
	p.conflictID[key] = struct{}{}
	p.plan.Conflicts = append(p.plan.Conflicts, PlanConflict{Code: code, SourcePath: sourcePath, TargetPath: targetPath})
}

func videoEntries(entries []openlist.DirectoryEntry) []openlist.DirectoryEntry {
	result := make([]openlist.DirectoryEntry, 0)
	for _, entry := range entries {
		if !entry.IsDir && media.IsVideoFile(entry.Name) {
			result = append(result, entry)
		}
	}
	sort.Slice(result, func(left, right int) bool { return result[left].Path < result[right].Path })
	return result
}

func movieMainVideoEntries(candidate *model.MediaCandidate, entries []openlist.DirectoryEntry) []openlist.DirectoryEntry {
	result := make([]openlist.DirectoryEntry, 0)
	root := candidate.Path
	if media.IsVideoFile(path.Base(candidate.Path)) {
		root = path.Dir(candidate.Path)
	}
	for _, entry := range videoEntries(entries) {
		if !media.IsMovieExtra(root, entry.Path) {
			result = append(result, entry)
		}
	}
	return result
}

func containsNFO(entries []openlist.DirectoryEntry) bool {
	for _, entry := range entries {
		if !entry.IsDir && strings.EqualFold(path.Ext(entry.Name), ".nfo") {
			return true
		}
	}
	return false
}

func safeVersionLabel(value string) string {
	value = safeMediaName(value)
	value = strings.Join(strings.Fields(strings.NewReplacer(".", " ", "_", " ").Replace(value)), " ")
	return strings.Trim(value, " -")
}

func relatedFlatAssets(videoPath string, siblings []openlist.DirectoryEntry) []openlist.DirectoryEntry {
	videoName := path.Base(videoPath)
	videoBase := strings.TrimSuffix(videoName, path.Ext(videoName))
	result := make([]openlist.DirectoryEntry, 0)
	for _, entry := range siblings {
		entryBase := strings.TrimSuffix(entry.Name, path.Ext(entry.Name))
		if entry.Path == videoPath || entryBase == videoBase || strings.HasPrefix(entryBase, videoBase+".") || strings.HasPrefix(entryBase, videoBase+"-") || strings.HasPrefix(entryBase, videoBase+"_") {
			result = append(result, entry)
		}
	}
	return result
}

func seasonName(season int) string { return fmt.Sprintf("Season %02d", season) }

func assetType(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".srt", ".ass", ".ssa", ".sub", ".vtt":
		return "subtitle"
	case ".jpg", ".jpeg", ".png", ".webp":
		return "image"
	case ".nfo":
		return "nfo"
	default:
		return "sidecar"
	}
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
