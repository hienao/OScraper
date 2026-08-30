package service

import (
	"path"
	"strings"

	"oscraper/internal/openlist"
)

const (
	scrapeMarkerName        = ".oscraper-scraped.v1"
	scrapeMarkerSuffix      = ".oscraper-scraped.v1"
	scrapeMarkerContent     = "OSCRAPER_SCRAPED\n"
	scrapeMarkerContentType = "text/plain; charset=utf-8"
)

func isScrapeMarkerName(name string) bool {
	return name == scrapeMarkerName || strings.HasSuffix(name, scrapeMarkerSuffix)
}

func hasDirectoryScrapeMarker(candidatePath string, entries []openlist.DirectoryEntry) bool {
	markerPath := path.Join(candidatePath, scrapeMarkerName)
	for _, entry := range entries {
		if !entry.IsDir && entry.Path == markerPath {
			return true
		}
	}
	return false
}

func hasFlatScrapeMarker(videoPath string, entries []openlist.DirectoryEntry) bool {
	markerPath := videoPath + scrapeMarkerSuffix
	for _, entry := range entries {
		if !entry.IsDir && entry.Path == markerPath {
			return true
		}
	}
	return false
}

func markerPathForPlan(plan PreviewPlan) string {
	if plan.ScrapeMarkerPath != "" {
		return plan.ScrapeMarkerPath
	}
	if plan.OrganizeFlatMovie && plan.ProposedDirectoryPath == path.Dir(plan.SourcePath) {
		return plan.SourcePath + scrapeMarkerSuffix
	}
	return path.Join(plan.ProposedDirectoryPath, scrapeMarkerName)
}
