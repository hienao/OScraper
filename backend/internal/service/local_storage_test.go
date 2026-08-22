package service

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStorageKeepsPathsInsideRootAndSkipsSymlinks(t *testing.T) {
	root := t.TempDir()
	movies := filepath.Join(root, "movies")
	if err := os.Mkdir(movies, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(movies, "Arrival.mkv"), []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(movies, "escape")); err != nil {
		t.Skipf("symlinks are unavailable: %v", err)
	}
	storage := newLocalStorage(root)
	entries, err := storage.ListDirectory(context.Background(), filepath.ToSlash(movies), false)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "Arrival.mkv" {
		t.Fatalf("unexpected local entries: %#v", entries)
	}
	if _, err := storage.Normalize(filepath.ToSlash(filepath.Join(root, "..", filepath.Base(outside)))); err == nil {
		t.Fatal("expected a path outside the local root to be rejected")
	}
	if _, err := storage.ListDirectory(context.Background(), filepath.ToSlash(filepath.Join(movies, "escape")), false); err == nil {
		t.Fatal("expected a symlink directory to be rejected")
	}
}

func TestLocalStorageMovesWithoutOverwriteAndWritesMetadataAtomically(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "movies"), 0o755); err != nil {
		t.Fatal(err)
	}
	storage := newLocalStorage(root)
	source := filepath.Join(root, "movies", "Arrival.mkv")
	target := filepath.Join(root, "movies", "Arrival (2016).mkv")
	if err := os.WriteFile(source, []byte("video"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := storage.MoveNoReplace(filepath.ToSlash(source), filepath.ToSlash(target)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatal(err)
	}
	conflictSource := filepath.Join(root, "movies", "Other.mkv")
	if err := os.WriteFile(conflictSource, []byte("other"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := storage.MoveNoReplace(filepath.ToSlash(conflictSource), filepath.ToSlash(target)); err == nil {
		t.Fatal("expected an existing destination to block the move")
	}
	if _, err := os.Stat(conflictSource); err != nil {
		t.Fatalf("source was changed after a blocked move: %v", err)
	}
	nfo := filepath.Join(root, "movies", "Arrival (2016).nfo")
	content := "<movie><title>Arrival</title></movie>"
	if err := storage.PutMetadata(filepath.ToSlash(nfo), int64(len(content)), strings.NewReader(content)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(nfo)
	if err != nil || string(data) != content {
		t.Fatalf("metadata was not written: %q %v", data, err)
	}
	entries, err := os.ReadDir(filepath.Dir(nfo))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".oscraper-metadata-") {
			t.Fatalf("temporary metadata file was left behind: %s", entry.Name())
		}
	}
	if _, err := os.Stat(source); !errorsIsNotExist(err) {
		t.Fatalf("moved source still exists: %v", err)
	}
}

func errorsIsNotExist(err error) bool {
	return err != nil && (os.IsNotExist(err) || err == fs.ErrNotExist)
}
