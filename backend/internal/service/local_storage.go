package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode"

	"oscraper/internal/openlist"

	"golang.org/x/sys/unix"
)

const defaultLocalMediaRoot = "/media"

type LocalStorageStatus struct {
	Root       string `json:"root"`
	Mounted    bool   `json:"mounted"`
	Readable   bool   `json:"readable"`
	Writable   bool   `json:"writable"`
	FreeBytes  uint64 `json:"free_bytes"`
	TotalBytes uint64 `json:"total_bytes"`
	UID        int    `json:"uid"`
	GID        int    `json:"gid"`
	Groups     []int  `json:"groups"`
}

type localStorage struct{ root string }

func newLocalStorage(root string) *localStorage {
	root = strings.TrimSpace(root)
	if root == "" {
		root = defaultLocalMediaRoot
	}
	abs, err := filepath.Abs(filepath.Clean(root))
	if err == nil {
		root = abs
	}
	return &localStorage{root: filepath.ToSlash(root)}
}

func (s *localStorage) Status() LocalStorageStatus {
	groups, _ := os.Getgroups()
	if groups == nil {
		groups = []int{}
	}
	status := LocalStorageStatus{Root: s.root, UID: os.Geteuid(), GID: os.Getegid(), Groups: groups}
	info, err := os.Lstat(filepath.FromSlash(s.root))
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return status
	}
	status.Mounted = true
	directory, err := os.Open(filepath.FromSlash(s.root))
	if err == nil {
		status.Readable = true
		_ = directory.Close()
	}
	status.Writable = unix.Access(filepath.FromSlash(s.root), unix.W_OK) == nil
	var stat unix.Statfs_t
	if err := unix.Statfs(filepath.FromSlash(s.root), &stat); err == nil {
		status.FreeBytes = uint64(stat.Bavail) * uint64(stat.Bsize)
		status.TotalBytes = uint64(stat.Blocks) * uint64(stat.Bsize)
	}
	return status
}

func (s *localStorage) Normalize(raw string) (string, error) {
	if raw == "" || strings.ContainsRune(raw, '\x00') || strings.Contains(raw, "\\") {
		return "", BadRequest("local.invalid_path", "Local media path is invalid")
	}
	for _, character := range raw {
		if unicode.IsControl(character) {
			return "", BadRequest("local.invalid_path", "Local media path contains control characters")
		}
	}
	cleaned := filepath.Clean(filepath.FromSlash(raw))
	if !filepath.IsAbs(cleaned) {
		return "", BadRequest("local.invalid_path", "Local media path must be absolute")
	}
	root := filepath.FromSlash(s.root)
	relative, err := filepath.Rel(root, cleaned)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", Forbidden("local.path_outside_root", "Local media path is outside /media")
	}
	return filepath.ToSlash(cleaned), nil
}

func (s *localStorage) ListDirectory(ctx context.Context, rawPath string, _ bool) ([]openlist.DirectoryEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	localPath, err := s.secureExisting(rawPath, true)
	if err != nil {
		return nil, mapLocalDirectoryError("local.read_failed", "Could not read local media directory", err)
	}
	items, err := os.ReadDir(localPath)
	if err != nil {
		return nil, mapLocalDirectoryError("local.read_failed", "Could not read local media directory", err)
	}
	entries := make([]openlist.DirectoryEntry, 0, len(items))
	parent, _ := s.Normalize(rawPath)
	for _, item := range items {
		if item.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, infoErr := item.Info()
		if infoErr != nil || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		identity := ""
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			identity = fmt.Sprintf("%d:%d", stat.Dev, stat.Ino)
		}
		entries = append(entries, openlist.DirectoryEntry{
			Name: item.Name(), Path: filepath.ToSlash(filepath.Join(filepath.FromSlash(parent), item.Name())),
			IsDir: info.IsDir(), Size: info.Size(), Modified: info.ModTime().UTC().Format(time.RFC3339Nano), Sign: identity,
		})
	}
	return entries, nil
}

func (s *localStorage) EntryState(rawPath string) (bool, bool, error) {
	normalized, err := s.Normalize(rawPath)
	if err != nil {
		return false, false, err
	}
	localPath, err := s.securePath(normalized, true)
	if err != nil {
		return false, false, err
	}
	info, err := os.Lstat(localPath)
	if errors.Is(err, fs.ErrNotExist) {
		return false, false, nil
	}
	if err != nil {
		return false, false, mapLocalFilesystemError("local.stat_failed", "Could not inspect local media path", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return false, false, Conflict("local.symlink_unsupported", "Symbolic links are not supported inside local media targets")
	}
	return true, info.IsDir(), nil
}

func (s *localStorage) CreateDirectory(rawPath string) error {
	normalized, err := s.Normalize(rawPath)
	if err != nil {
		return err
	}
	if _, err := s.secureExisting(filepath.ToSlash(filepath.Dir(filepath.FromSlash(normalized))), true); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.FromSlash(normalized), 0o755); err != nil {
		return mapLocalFilesystemError("local.mkdir_failed", "Could not create local media directory", err)
	}
	return nil
}

func (s *localStorage) MoveNoReplace(sourcePath, targetPath string) error {
	source, err := s.secureExisting(sourcePath, false)
	if err != nil {
		return err
	}
	target, err := s.Normalize(targetPath)
	if err != nil {
		return err
	}
	if _, err := s.secureExisting(filepath.ToSlash(filepath.Dir(filepath.FromSlash(target))), true); err != nil {
		return err
	}
	if strings.EqualFold(filepath.ToSlash(source), target) && filepath.ToSlash(source) != target {
		temporary := filepath.Join(filepath.Dir(source), fmt.Sprintf(".oscraper-rename-%d", time.Now().UnixNano()))
		if err := renameLocalNoReplace(source, temporary); err != nil {
			return mapLocalRenameError(err)
		}
		if err := renameLocalNoReplace(temporary, filepath.FromSlash(target)); err != nil {
			_ = os.Rename(temporary, source)
			return mapLocalRenameError(err)
		}
		return nil
	}
	if err := renameLocalNoReplace(source, filepath.FromSlash(target)); err != nil {
		return mapLocalRenameError(err)
	}
	return nil
}

func (s *localStorage) PutMetadata(rawPath string, expectedSize int64, content io.Reader) error {
	normalized, err := s.Normalize(rawPath)
	if err != nil {
		return err
	}
	parent, err := s.secureExisting(filepath.ToSlash(filepath.Dir(filepath.FromSlash(normalized))), true)
	if err != nil {
		return err
	}
	if info, statErr := os.Lstat(filepath.FromSlash(normalized)); statErr == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return Conflict("job.target_exists", "A non-regular file occupies the metadata path")
		}
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return mapLocalFilesystemError("local.stat_failed", "Could not inspect local metadata path", statErr)
	}
	temporary, err := os.CreateTemp(parent, ".oscraper-metadata-*")
	if err != nil {
		return mapLocalFilesystemError("local.write_failed", "Could not create local metadata file", err)
	}
	temporaryName := temporary.Name()
	removeTemporary := true
	defer func() {
		_ = temporary.Close()
		if removeTemporary {
			_ = os.Remove(temporaryName)
		}
	}()
	if err := temporary.Chmod(0o644); err != nil {
		return mapLocalFilesystemError("local.write_failed", "Could not set local metadata permissions", err)
	}
	written, err := io.Copy(temporary, content)
	if err != nil || (expectedSize >= 0 && written != expectedSize) {
		if err == nil {
			err = io.ErrUnexpectedEOF
		}
		return mapLocalFilesystemError("local.write_failed", "Could not write local metadata file", err)
	}
	if err := temporary.Sync(); err != nil {
		return mapLocalFilesystemError("local.write_failed", "Could not flush local metadata file", err)
	}
	if err := temporary.Close(); err != nil {
		return mapLocalFilesystemError("local.write_failed", "Could not close local metadata file", err)
	}
	if err := os.Rename(temporaryName, filepath.FromSlash(normalized)); err != nil {
		return mapLocalFilesystemError("local.write_failed", "Could not finalize local metadata file", err)
	}
	removeTemporary = false
	return nil
}

func (s *localStorage) secureExisting(rawPath string, requireDirectory bool) (string, error) {
	localPath, err := s.securePath(rawPath, false)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(localPath)
	if err != nil {
		return "", mapLocalFilesystemError("local.path_unavailable", "Local media path is unavailable", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", Conflict("local.symlink_unsupported", "Symbolic links are not supported inside local media targets")
	}
	if requireDirectory && !info.IsDir() {
		return "", BadRequest("local.not_directory", "Local media path is not a directory")
	}
	return localPath, nil
}

func (s *localStorage) securePath(rawPath string, allowMissingLeaf bool) (string, error) {
	normalized, err := s.Normalize(rawPath)
	if err != nil {
		return "", err
	}
	root := filepath.FromSlash(s.root)
	relative, _ := filepath.Rel(root, filepath.FromSlash(normalized))
	current := root
	parts := strings.Split(relative, string(filepath.Separator))
	if relative == "." {
		parts = nil
	}
	rootInfo, err := os.Lstat(root)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return "", Conflict("local.not_mounted", "Local media root is not mounted as a real directory")
	}
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, fs.ErrNotExist) && allowMissingLeaf && index == len(parts)-1 {
			return current, nil
		}
		if statErr != nil {
			return "", mapLocalFilesystemError("local.path_unavailable", "Local media path is unavailable", statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", Conflict("local.symlink_unsupported", "Symbolic links are not supported inside local media targets")
		}
	}
	return filepath.FromSlash(normalized), nil
}

func mapLocalRenameError(err error) error {
	if errors.Is(err, fs.ErrExist) {
		return Conflict("job.target_exists", "Local media destination already exists")
	}
	if errors.Is(err, syscall.EXDEV) {
		return Conflict("local.cross_device_move", "Local media cannot be moved across filesystems")
	}
	return mapLocalFilesystemError("local.rename_failed", "Could not move local media path", err)
}

func mapLocalFilesystemError(code, message string, err error) error {
	if errors.Is(err, fs.ErrPermission) {
		return Forbidden("local.permission_denied", "Local media path permission was denied")
	}
	if errors.Is(err, fs.ErrNotExist) {
		return NotFound("local.path_unavailable", "Local media path does not exist")
	}
	return Internal(code, message, err)
}

func mapLocalDirectoryError(code, message string, err error) error {
	var serviceErr *Error
	if (errors.As(err, &serviceErr) && serviceErr.Code == "local.path_unavailable") || errors.Is(err, fs.ErrNotExist) {
		return NotFound("local.path_unavailable", "The directory does not exist. Please select a directory again")
	}
	return mapLocalFilesystemError(code, message, err)
}
