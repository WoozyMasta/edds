// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/edds

package edds

const (
	maxInt32  = int(^uint32(0) >> 1)
	maxUint32 = uint64(^uint32(0))
)

// i32FromInt converts an int to an int32.
func i32FromInt(n int) (int32, error) {
	if n < 0 || n > maxInt32 {
		return 0, ErrSizeOverflow
	}

	return int32(n), nil
}

// u32FromInt converts an int to a uint32.
func u32FromInt(n int) (uint32, error) {
	if n < 0 || uint64(n) > maxUint32 {
		return 0, ErrSizeOverflow
	}

	// #nosec G115 -- bounds checked above.
	return uint32(n), nil
}
