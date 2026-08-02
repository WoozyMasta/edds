// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/edds

//go:build !windows

package edds

import "os"

// replaceFile atomically replaces path with tempPath on POSIX filesystems.
func replaceFile(tempPath, path string) error {
	return os.Rename(tempPath, path)
}
