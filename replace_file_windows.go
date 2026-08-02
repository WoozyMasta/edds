// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/edds

//go:build windows

package edds

import "golang.org/x/sys/windows"

// replaceFile atomically replaces path with tempPath when the destination exists.
func replaceFile(tempPath, path string) error {
	return windows.Rename(tempPath, path)
}
