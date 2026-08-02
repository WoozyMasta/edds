// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/edds

package edds

import (
	"fmt"
	"os"
	"path/filepath"
)

// writeFileAtomic writes to a sibling temporary file and replaces path on success.
func writeFileAtomic(path string, write func(*os.File) error) (err error) {
	dir, base := filepath.Dir(path), filepath.Base(path)
	temp, err := os.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return fmt.Errorf("%w: %q: %v", ErrCreateFile, path, err)
	}

	tempPath := temp.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = temp.Close()
			_ = os.Remove(tempPath)
		}
	}()

	if err := write(temp); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("%w: sync temporary file: %v", ErrAtomicWrite, err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("%w: close temporary file: %v", ErrAtomicWrite, err)
	}
	if err := replaceFile(tempPath, path); err != nil {
		return fmt.Errorf("%w: replace %q: %v", ErrAtomicWrite, path, err)
	}

	removeTemp = false
	return nil
}
