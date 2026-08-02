// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/edds

package edds

import "github.com/woozymasta/bcn"

// detectFormat detects the format of a DDS/EDDS file.
func detectFormat(header *bcn.DDSHeader, dx10 *bcn.DDSHeaderDX10) bcn.Format {
	if dx10 != nil {
		return mapDxgiFormat(dx10.DXGIFormat)
	}

	pf := header.PixelFormat
	if (pf.Flags & bcn.DDSPFFourCC) != 0 {
		// Prefer FourCC before RGB masks; compressed DDS variants do not carry useful masks.
		fourCCStr := intToFourCC(pf.FourCC)
		switch fourCCStr {
		case "DXT1":
			return bcn.FormatDXT1
		case "DXT2", "DXT3":
			return bcn.FormatDXT3
		case "DXT4", "DXT5":
			return bcn.FormatDXT5
		case "ATI1", "BC4U":
			return bcn.FormatBC4
		case "BC4S":
			return bcn.FormatBC4S
		case "ATI2", "BC5U":
			return bcn.FormatBC5
		case "BC5S":
			return bcn.FormatBC5S
		default:
			return bcn.FormatUnknown
		}
	}

	if (pf.Flags & bcn.DDSPFRGB) != 0 {
		if pf.RGBBitCount == 8 && pf.RBitMask == 0x000000ff &&
			pf.GBitMask == 0 && pf.BBitMask == 0 && pf.ABitMask == 0 {
			return bcn.FormatR8
		}
		if (pf.Flags&bcn.DDSPFAlphaPixels != 0) && pf.RGBBitCount == 32 {
			if pf.RBitMask == 0x000000ff && pf.GBitMask == 0x0000ff00 &&
				pf.BBitMask == 0x00ff0000 && pf.ABitMask == 0xff000000 {
				return bcn.FormatRGBA8
			}
			if pf.RBitMask == 0x00ff0000 && pf.GBitMask == 0x0000ff00 &&
				pf.BBitMask == 0x000000ff && pf.ABitMask == 0xff000000 {
				return bcn.FormatBGRA8
			}
		}
	}

	if (pf.Flags & bcn.DDSPFLuminance) != 0 {
		if pf.RGBBitCount == 8 && pf.RBitMask == 0x000000ff &&
			pf.GBitMask == 0 && pf.BBitMask == 0 && pf.ABitMask == 0 {
			return bcn.FormatR8
		}
		if (pf.Flags&bcn.DDSPFAlphaPixels != 0) && pf.RGBBitCount == 16 &&
			pf.RBitMask == 0x000000ff && pf.GBitMask == 0 && pf.BBitMask == 0 &&
			pf.ABitMask == 0x0000ff00 {
			return bcn.FormatRG8
		}
	}

	if (pf.Flags&bcn.DDSPFAlpha) != 0 && pf.RGBBitCount == 8 &&
		pf.RBitMask == 0 && pf.GBitMask == 0 && pf.BBitMask == 0 && pf.ABitMask == 0x000000ff {
		return bcn.FormatA8
	}

	return bcn.FormatUnknown
}

// mapDxgiFormat maps a DXGI format to a BCn format.
func mapDxgiFormat(dxgiFormat uint32) bcn.Format {
	switch dxgiFormat {
	case 71, 72:
		return bcn.FormatDXT1
	case 74, 75:
		return bcn.FormatDXT3
	case 77, 78:
		return bcn.FormatDXT5
	case 80:
		return bcn.FormatBC4
	case 81:
		return bcn.FormatBC4S
	case 83:
		return bcn.FormatBC5
	case 84:
		return bcn.FormatBC5S
	case 87, 91:
		return bcn.FormatBGRA8
	case 88, 93:
		return bcn.FormatBGRX8
	case 98, 99:
		return bcn.FormatBC7
	case 24:
		return bcn.FormatRGB10A2
	case 28, 29:
		return bcn.FormatRGBA8
	case 49:
		return bcn.FormatRG8
	case 51:
		return bcn.FormatRG8S
	case 61:
		return bcn.FormatR8
	case 63:
		return bcn.FormatR8S
	case 65:
		return bcn.FormatA8
	case 85:
		return bcn.FormatRGB565
	case 86:
		return bcn.FormatRGBA5551
	case 115:
		return bcn.FormatRGBA4444
	default:
		return bcn.FormatUnknown
	}
}

// intToFourCC converts a uint32 to a four-character code string.
func intToFourCC(value uint32) string {
	return string([]byte{
		byte(value & 0xff),
		byte((value >> 8) & 0xff),
		byte((value >> 16) & 0xff),
		byte((value >> 24) & 0xff),
	})
}

// expectedDataLength calculates the expected data length for a given format and dimensions.
func expectedDataLength(format bcn.Format, width, height int) int {
	length, err := expectedDataLengthChecked(format, width, height)
	if err != nil {
		return -1
	}

	return length
}

// expectedDataLengthChecked calculates the expected data length without integer overflow.
func expectedDataLengthChecked(format bcn.Format, width, height int) (int, error) {
	if width <= 0 || height <= 0 {
		return 0, ErrSizeOverflow
	}

	w := uint64(width)
	h := uint64(height)
	var size uint64

	switch format {
	case bcn.FormatDXT1, bcn.FormatBC4, bcn.FormatBC4S:
		size = checkedDataLength((w+3)/4, (h+3)/4, 8)
	case bcn.FormatDXT3, bcn.FormatDXT5, bcn.FormatBC5, bcn.FormatBC5S, bcn.FormatBC7:
		size = checkedDataLength((w+3)/4, (h+3)/4, 16)
	case bcn.FormatRGBA8, bcn.FormatBGRA8, bcn.FormatBGRX8, bcn.FormatRGB10A2:
		size = checkedDataLength(w, h, 4)
	case bcn.FormatR8, bcn.FormatR8S, bcn.FormatA8:
		size = checkedDataLength(w, h)
	case bcn.FormatRG8, bcn.FormatRG8S, bcn.FormatRGB565, bcn.FormatRGBA5551, bcn.FormatRGBA4444:
		size = checkedDataLength(w, h, 2)
	default:
		return 0, ErrInvalidFormat
	}

	if size == 0 || size > uint64(maxInt) {
		return 0, ErrSizeOverflow
	}

	return int(size), nil
}

// checkedDataLength returns a positive byte length or zero on uint64 overflow.
func checkedDataLength(factors ...uint64) uint64 {
	product := uint64(1)
	for _, factor := range factors {
		if factor == 0 || product > ^uint64(0)/factor {
			return 0
		}
		product *= factor
	}

	return product
}

// makeFourCC creates a four-character code from four bytes.
func makeFourCC(a, b, c, d byte) uint32 {
	return uint32(a) | uint32(b)<<8 | uint32(c)<<16 | uint32(d)<<24
}

// enfusionReserved1 returns the reserved1 field value for Enfusion files.
func enfusionReserved1() [11]uint32 {
	return [11]uint32{
		0,
		0x31464e45, // "ENF1"
		0, 0, 0, 0, 0, 0, 0, 0, 0,
	}
}

// makeDDSHeader creates a DDS header for a given format and dimensions.
func makeDDSHeader(width, height, mipMapCount uint32, format bcn.Format) (*bcn.DDSHeader, error) {
	// Set only the caps/flags used by Enfusion readers; extra DDS fields stay zeroed.
	flags := uint32(bcn.DDSFlagCaps | bcn.DDSFlagHeight | bcn.DDSFlagWidth | bcn.DDSFlagPixelFormat)
	caps := uint32(bcn.DDSCapsTexture)
	if mipMapCount > 1 {
		flags |= bcn.DDSFlagMipmapCount
		caps |= bcn.DDSCapsComplex | bcn.DDSCapsMipmap
	}

	hdr := &bcn.DDSHeader{
		Size:        bcn.DDSHeaderSize,
		Flags:       flags,
		Height:      height,
		Width:       width,
		Depth:       1,
		MipMapCount: mipMapCount,
		Reserved1:   enfusionReserved1(),
		Caps:        caps,
	}
	hdr.PixelFormat.Size = bcn.DDSPixelFormatSize

	// Keep legacy FourCC headers for BCn formats; EDDS does not need DX10 headers here.
	switch format {
	case bcn.FormatDXT1:
		hdr.Flags |= bcn.DDSFlagLinearSize
		hdr.PixelFormat.Flags = bcn.DDSPFFourCC
		hdr.PixelFormat.FourCC = makeFourCC('D', 'X', 'T', '1')
	case bcn.FormatDXT3:
		hdr.Flags |= bcn.DDSFlagLinearSize
		hdr.PixelFormat.Flags = bcn.DDSPFFourCC
		hdr.PixelFormat.FourCC = makeFourCC('D', 'X', 'T', '3')
	case bcn.FormatDXT5:
		hdr.Flags |= bcn.DDSFlagLinearSize
		hdr.PixelFormat.Flags = bcn.DDSPFFourCC
		hdr.PixelFormat.FourCC = makeFourCC('D', 'X', 'T', '5')
	case bcn.FormatBC4:
		hdr.Flags |= bcn.DDSFlagLinearSize
		hdr.PixelFormat.Flags = bcn.DDSPFFourCC
		hdr.PixelFormat.FourCC = makeFourCC('A', 'T', 'I', '1')
	case bcn.FormatBC5:
		hdr.Flags |= bcn.DDSFlagLinearSize
		hdr.PixelFormat.Flags = bcn.DDSPFFourCC
		hdr.PixelFormat.FourCC = makeFourCC('A', 'T', 'I', '2')
	case bcn.FormatRGBA8:
		hdr.Flags |= bcn.DDSFlagPitch
		hdr.PixelFormat.Flags = bcn.DDSPFRGB | bcn.DDSPFAlphaPixels
		hdr.PixelFormat.RGBBitCount = 32
		hdr.PixelFormat.RBitMask = 0x000000ff
		hdr.PixelFormat.GBitMask = 0x0000ff00
		hdr.PixelFormat.BBitMask = 0x00ff0000
		hdr.PixelFormat.ABitMask = 0xff000000
		hdr.PitchOrLinearSize = width * 4
	case bcn.FormatBGRA8:
		hdr.Flags |= bcn.DDSFlagPitch
		hdr.PixelFormat.Flags = bcn.DDSPFRGB | bcn.DDSPFAlphaPixels
		hdr.PixelFormat.RGBBitCount = 32
		hdr.PixelFormat.RBitMask = 0x00ff0000
		hdr.PixelFormat.GBitMask = 0x0000ff00
		hdr.PixelFormat.BBitMask = 0x000000ff
		hdr.PixelFormat.ABitMask = 0xff000000
		hdr.PitchOrLinearSize = width * 4
	default:
		return nil, ErrInvalidFormat
	}

	return hdr, nil
}
