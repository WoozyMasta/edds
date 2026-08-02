// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/edds

package edds

import (
	"fmt"
	"image"
)

// SwizzleProfile selects a Workbench-compatible channel transform before encoding.
// Profiles transform pixel storage only; EDDS files do not record the selected profile.
// Workbench ColorNoise is omitted because it generates alpha noise rather than remapping channels.
type SwizzleProfile uint8

const (
	// SwizzleProfileNone leaves channels unchanged.
	SwizzleProfileNone SwizzleProfile = iota
	// SwizzleProfileAlphaToRGB stores alpha in RGB and writes opaque alpha.
	SwizzleProfileAlphaToRGB
	// SwizzleProfileAmbientSpecularMapGA leaves RGBA channels unchanged.
	SwizzleProfileAmbientSpecularMapGA
	// SwizzleProfileNormalMapGA stores (0, G, 0, R).
	SwizzleProfileNormalMapGA
	// SwizzleProfileNormalMapNOHQ stores (0, G, 0, 1-R).
	SwizzleProfileNormalMapNOHQ
	// SwizzleProfileNormalSpecularMapXYZS leaves RGBA channels unchanged.
	SwizzleProfileNormalSpecularMapXYZS
	// SwizzleProfileSMDIToGS stores (B, G, 0, 1).
	SwizzleProfileSMDIToGS
	// SwizzleProfileTerrainLayerTexture leaves RGBA channels unchanged.
	SwizzleProfileTerrainLayerTexture
	// SwizzleProfileTerrainNormalSpecularSYxX stores (A, G, 0, R).
	SwizzleProfileTerrainNormalSpecularSYxX
	// SwizzleProfileTerrainSuperTexture leaves RGBA channels unchanged.
	SwizzleProfileTerrainSuperTexture
)

// String returns the Workbench profile name.
func (profile SwizzleProfile) String() string {
	switch profile {
	case SwizzleProfileNone:
		return "None"
	case SwizzleProfileAlphaToRGB:
		return "AlphaToRGB"
	case SwizzleProfileAmbientSpecularMapGA:
		return "AmbientSpecularMapGA"
	case SwizzleProfileNormalMapGA:
		return "NormalMapGA"
	case SwizzleProfileNormalMapNOHQ:
		return "NormalMap_NOHQ"
	case SwizzleProfileNormalSpecularMapXYZS:
		return "NormalSpecularMapXYZS"
	case SwizzleProfileSMDIToGS:
		return "SMDIToGS"
	case SwizzleProfileTerrainLayerTexture:
		return "TerrainLayerTexture"
	case SwizzleProfileTerrainNormalSpecularSYxX:
		return "TerrainNormalSpecular_SYxX"
	case SwizzleProfileTerrainSuperTexture:
		return "TerrainSuperTexture"
	default:
		return fmt.Sprintf("SwizzleProfile(%d)", profile)
	}
}

// validateSwizzleProfile reports whether profile is supported for writing.
func validateSwizzleProfile(profile SwizzleProfile) error {
	switch profile {
	case SwizzleProfileNone,
		SwizzleProfileAlphaToRGB,
		SwizzleProfileAmbientSpecularMapGA,
		SwizzleProfileNormalMapGA,
		SwizzleProfileNormalMapNOHQ,
		SwizzleProfileNormalSpecularMapXYZS,
		SwizzleProfileSMDIToGS,
		SwizzleProfileTerrainLayerTexture,
		SwizzleProfileTerrainNormalSpecularSYxX,
		SwizzleProfileTerrainSuperTexture:
		return nil
	default:
		return fmt.Errorf("%w: %d", ErrInvalidSwizzleProfile, profile)
	}
}

// applySwizzleProfileInto applies profile to src using dst when it has matching bounds.
func applySwizzleProfileInto(dst, src *image.NRGBA, profile SwizzleProfile) (*image.NRGBA, error) {
	if err := validateSwizzleProfile(profile); err != nil {
		return nil, err
	}
	if dst == nil || dst.Bounds() != src.Bounds() {
		dst = image.NewNRGBA(src.Bounds())
	}

	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			srcOffset := src.PixOffset(x, y)
			r, g, b, a := src.Pix[srcOffset], src.Pix[srcOffset+1], src.Pix[srcOffset+2], src.Pix[srcOffset+3]
			switch profile {
			case SwizzleProfileAlphaToRGB:
				r, g, b, a = a, a, a, 255
			case SwizzleProfileNormalMapGA:
				r, b, a = 0, 0, r
			case SwizzleProfileNormalMapNOHQ:
				r, b, a = 0, 0, 255-r
			case SwizzleProfileSMDIToGS:
				r, b, a = b, 0, 255
			case SwizzleProfileTerrainNormalSpecularSYxX:
				r, b, a = a, 0, r
			}
			dstOffset := dst.PixOffset(x, y)
			dst.Pix[dstOffset], dst.Pix[dstOffset+1], dst.Pix[dstOffset+2], dst.Pix[dstOffset+3] = r, g, b, a
		}
	}

	return dst, nil
}

// ensureImageSlots resizes image slots to n, allocating only when capacity is insufficient.
func ensureImageSlots(slots []*image.NRGBA, n int) []*image.NRGBA {
	if cap(slots) < n {
		next := make([]*image.NRGBA, n)
		copy(next, slots)
		return next
	}

	return slots[:n]
}
