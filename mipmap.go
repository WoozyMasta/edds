package edds

// calculateMipMapCount calculates the number of mipmap levels for a given width and height.
func calculateMipMapCount(width, height int) (int, error) {
	count := 1
	w, err := u32FromInt(width)
	if err != nil {
		return 0, err
	}

	h, err := u32FromInt(height)
	if err != nil {
		return 0, err
	}

	for w > 1 || h > 1 {
		count++
		if w > 1 {
			w /= 2
		}
		if h > 1 {
			h /= 2
		}
	}

	if count > 11 {
		count = 11
	}

	return count, nil
}

// mipDimension calculates the dimension of a mipmap level.
func mipDimension(base, level int) int {
	result := base >> level
	if result < 1 {
		return 1
	}

	return result
}
