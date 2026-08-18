package service

import (
	"path"
	"time"

	"openlistscraper/internal/metadata"
	"openlistscraper/internal/provider/tmdb"
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
