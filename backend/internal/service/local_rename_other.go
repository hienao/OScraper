//go:build !linux

package service

import (
	"io/fs"
	"os"
)

func renameLocalNoReplace(source, target string) error {
	if _, err := os.Lstat(target); err == nil {
		return fs.ErrExist
	} else if !os.IsNotExist(err) {
		return err
	}
	return os.Rename(source, target)
}
