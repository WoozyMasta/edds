package edds

const (
	maxInt32  = int(^uint32(0) >> 1)
	maxUint32 = uint64(^uint32(0))
)

func i32FromInt(n int) (int32, error) {
	if n < 0 || n > maxInt32 {
		return 0, ErrSizeOverflow
	}

	return int32(n), nil
}

func u32FromInt(n int) (uint32, error) {
	if n < 0 || uint64(n) > maxUint32 {
		return 0, ErrSizeOverflow
	}

	// #nosec G115 -- bounds checked above.
	return uint32(n), nil
}
