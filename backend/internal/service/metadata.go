package service

import (
	"context"
	"path"
	"strings"
	"time"

	"oscraper/internal/metadata"
	"oscraper/internal/provider/tmdb"
)

func buildMetadataArtifacts(detail *tmdb.Detail, finalRoot, standardName string, generatedAt time.Time) []PreviewArtifact {
	nfoPath := path.Join(finalRoot, standardName+".nfo")
	if detail.MediaType == "tv" {
		nfoPath = path.Join(finalRoot, "tvshow.nfo")
	}
	artifacts := []PreviewArtifact{{Path: nfoPath, Kind: "nfo", Content: metadata.BuildNFO(detail, generatedAt)}}
	if detail.PosterURL != "" {
		posterName := standardName + "-poster.jpg"
		if detail.MediaType == "tv" {
			posterName = "poster.jpg"
		}
		artifacts = append(artifacts, PreviewArtifact{Path: path.Join(finalRoot, posterName), Kind: "poster", SourceURL: detail.PosterURL})
	}
	if detail.BackdropURL != "" {
		backdropName := standardName + "-backdrop.jpg"
		if detail.MediaType == "tv" {
			backdropName = "fanart.jpg"
		}
		artifacts = append(artifacts, PreviewArtifact{Path: path.Join(finalRoot, backdropName), Kind: "backdrop", SourceURL: detail.BackdropURL})
	}
	return artifacts
}

func expandEpisodeArtifacts(ctx context.Context, provider TMDBSeasonCatalog, config tmdb.Config, detail *tmdb.Detail, plan *PreviewPlan) error {
	seasons := make(map[int]struct{})
	for _, file := range plan.EpisodeFiles {
		seasons[file.Season] = struct{}{}
	}
	episodes := make(map[[2]int]tmdb.Episode)
	for season := range seasons {
		items, err := provider.Season(ctx, config, detail.ID, season)
		if err != nil {
			return err
		}
		for _, episode := range items {
			episodes[[2]int{episode.SeasonNumber, episode.EpisodeNumber}] = episode
		}
	}
	generatedAt := time.Now().UTC()
	kept := make([]EpisodeFilePlan, 0, len(plan.EpisodeFiles))
	for _, file := range plan.EpisodeFiles {
		episode, found := episodes[[2]int{file.Season, file.Episode}]
		if !found {
			// Episodes TMDB does not know about are still renamed, but no
			// episode metadata is generated for them.
			plan.SkippedEpisodes = append(plan.SkippedEpisodes, file)
			continue
		}
		kept = append(kept, file)
		base := strings.TrimSuffix(file.TargetPath, path.Ext(file.TargetPath))
		nfo := PreviewArtifact{Path: base + ".nfo", Kind: "episode_nfo", Content: metadata.BuildEpisodeNFO(detail.Title, episode, generatedAt)}
		plan.Artifacts = append(plan.Artifacts, nfo)
		plan.GeneratedFiles = append(plan.GeneratedFiles, nfo.Path)
		if episode.StillURL != "" {
			thumb := PreviewArtifact{Path: base + "-thumb.jpg", Kind: "episode_thumb", SourceURL: episode.StillURL}
			plan.Artifacts = append(plan.Artifacts, thumb)
			plan.GeneratedFiles = append(plan.GeneratedFiles, thumb.Path)
		}
	}
	plan.EpisodeFiles = kept
	if len(plan.SkippedEpisodes) > 0 {
		plan.Warnings = append(plan.Warnings, "episodes_skipped")
	}
	if len(plan.Conflicts) > 0 {
		plan.Ready = false
	}
	return nil
}

func isNFOArtifact(kind string) bool { return kind == "nfo" || strings.HasSuffix(kind, "_nfo") }
