// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/edds

package edds

import (
	"fmt"

	"github.com/woozymasta/bcn"
)

// detectFormat detects the format of a DDS/EDDS file.
func detectFormat(header *bcn.DDSHeader, dx10 *bcn.DDSHeaderDX10) (bcn.Format, string) {
	if dx10 != nil {
		format := mapDxgiFormat(dx10.DXGIFormat)
		return format, fmt.Sprintf("DXGI %d", dx10.DXGIFormat)
	}

	pf := header.PixelFormat
	if (pf.Flags & bcn.DDSPFFourCC) != 0 {
		fourCCStr := intToFourCC(pf.FourCC)
		switch fourCCStr {
		case "DXT1":
			return bcn.FormatDXT1, fourCCStr
		case "DXT2", "DXT3":
			return bcn.FormatDXT3, fourCCStr
		case "DXT4", "DXT5":
			return bcn.FormatDXT5, fourCCStr
		case "ATI1", "BC4U", "BC4S":
			return bcn.FormatBC4, fourCCStr
		case "ATI2", "BC5U", "BC5S":
			return bcn.FormatBC5, fourCCStr
		default:
			return bcn.FormatUnknown, fourCCStr
		}
	}

	if (pf.Flags & bcn.DDSPFRGB) != 0 {
		if (pf.Flags&bcn.DDSPFAlphaPixels != 0) && pf.RGBBitCount == 32 {
			if pf.RBitMask == 0x000000ff && pf.GBitMask == 0x0000ff00 &&
				pf.BBitMask == 0x00ff0000 && pf.ABitMask == 0xff000000 {
				return bcn.FormatRGBA8, "RGBA8"
			}
			if pf.RBitMask == 0x00ff0000 && pf.GBitMask == 0x0000ff00 &&
				pf.BBitMask == 0x000000ff && pf.ABitMask == 0xff000000 {
				return bcn.FormatBGRA8, "BGRA8"
			}
		}
	}

	if (pf.Flags&bcn.DDSPFLuminance) != 0 && pf.RGBBitCount == 8 {
		return bcn.FormatRGBA8, "LUMINANCE8"
	}

	return bcn.FormatUnknown, "UNKNOWN"
}

// mapDxgiFormat maps a DXGI format to a BCn format.
func mapDxgiFormat(dxgiFormat uint32) bcn.Format {
	switch dxgiFormat {
	case 71:
		return bcn.FormatDXT1
	case 74:
		return bcn.FormatDXT3
	case 77:
		return bcn.FormatDXT5
	case 80:
		return bcn.FormatBC4
	case 83:
		return bcn.FormatBC5
	case 87:
		return bcn.FormatBGRA8
	case 28:
		return bcn.FormatRGBA8
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
	blocksW := (width + 3) / 4
	blocksH := (height + 3) / 4
	switch format {
	case bcn.FormatDXT1, bcn.FormatBC4:
		return blocksW * blocksH * 8
	case bcn.FormatDXT3, bcn.FormatDXT5, bcn.FormatBC5:
		return blocksW * blocksH * 16
	case bcn.FormatRGBA8, bcn.FormatBGRA8:
		return width * height * 4
	default:
		return -1
	}
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
	// set flags and caps
	flags := uint32(bcn.DDSFlagCaps | bcn.DDSFlagHeight | bcn.DDSFlagWidth | bcn.DDSFlagPixelFormat)
	caps := uint32(bcn.DDSCapsTexture)
	if mipMapCount > 1 {
		flags |= bcn.DDSFlagMipmapCount
		caps |= bcn.DDSCapsComplex | bcn.DDSCapsMipmap
	}

	// create DDS header
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

	// set pixel format
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
