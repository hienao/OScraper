package service

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestScrapeMarkerPayloadHashIsStable(t *testing.T) {
	const expectedHash = "25d181f5ee54ac6de5da8693f7cb3da0f078b39e5c2d9fcd3109d8b57577dea0"
	actualHash := fmt.Sprintf("%x", sha256.Sum256([]byte(scrapeMarkerContent)))
	if actualHash != expectedHash {
		t.Fatalf("scrape marker payload hash changed: %s", actualHash)
	}
}
