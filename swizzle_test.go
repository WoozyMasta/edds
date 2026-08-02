// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/edds

package edds

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/woozymasta/bcn"
)

func TestSwizzleProfiles(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	src.SetNRGBA(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 40})

	tests := []struct {
		profile SwizzleProfile
		want    color.NRGBA
	}{
		{SwizzleProfileNone, color.NRGBA{R: 10, G: 20, B: 30, A: 40}},
		{SwizzleProfileAlphaToRGB, color.NRGBA{R: 40, G: 40, B: 40, A: 255}},
		{SwizzleProfileAmbientSpecularMapGA, color.NRGBA{R: 10, G: 20, B: 30, A: 40}},
		{SwizzleProfileNormalMapGA, color.NRGBA{G: 20, A: 10}},
		{SwizzleProfileNormalMapNOHQ, color.NRGBA{G: 20, A: 245}},
		{SwizzleProfileNormalSpecularMapXYZS, color.NRGBA{R: 10, G: 20, B: 30, A: 40}},
		{SwizzleProfileSMDIToGS, color.NRGBA{R: 30, G: 20, A: 255}},
		{SwizzleProfileTerrainLayerTexture, color.NRGBA{R: 10, G: 20, B: 30, A: 40}},
		{SwizzleProfileTerrainNormalSpecularSYxX, color.NRGBA{R: 40, G: 20, A: 10}},
		{SwizzleProfileTerrainSuperTexture, color.NRGBA{R: 10, G: 20, B: 30, A: 40}},
	}

	for _, tc := range tests {
		t.Run(tc.profile.String(), func(t *testing.T) {
			got, err := applySwizzleProfileInto(nil, src, tc.profile)
			if err != nil {
				t.Fatalf("applySwizzleProfileInto: %v", err)
			}
			if pixel := got.NRGBAAt(0, 0); pixel != tc.want {
				t.Fatalf("pixel = %#v, want %#v", pixel, tc.want)
			}
		})
	}

	if _, err := applySwizzleProfileInto(nil, src, SwizzleProfile(99)); !errors.Is(err, ErrInvalidSwizzleProfile) {
		t.Fatalf("invalid profile error = %v, want ErrInvalidSwizzleProfile", err)
	}
}

func TestEncodeWithSwizzleProfile(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 30), G: uint8(y * 30), B: 100, A: 200})
		}
	}

	var data bytes.Buffer
	if err := EncodeWithOptions(&data, src, &WriteOptions{
		Format:         bcn.FormatBGRA8,
		MaxMipMaps:     1,
		SwizzleProfile: SwizzleProfileTerrainNormalSpecularSYxX,
		Compression:    CompressionOptions{Mode: CompressionNone},
	}); err != nil {
		t.Fatalf("EncodeWithOptions: %v", err)
	}

	got, err := Decode(bytes.NewReader(data.Bytes()))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	want, err := applySwizzleProfileInto(nil, src, SwizzleProfileTerrainNormalSpecularSYxX)
	if err != nil {
		t.Fatalf("applySwizzleProfileInto: %v", err)
	}
	gotNRGBA := got.(*image.NRGBA)
	if !bytes.Equal(gotNRGBA.Pix, want.Pix) {
		t.Fatal("encoded pixels do not match swizzle profile")
	}

	path := filepath.Join(t.TempDir(), "swizzled.edds")
	if err := WriteWithOptions(src, path, &WriteOptions{
		Format:         bcn.FormatBGRA8,
		MaxMipMaps:     1,
		SwizzleProfile: SwizzleProfileTerrainNormalSpecularSYxX,
		Compression:    CompressionOptions{Mode: CompressionNone},
	}); err != nil {
		t.Fatalf("WriteWithOptions: %v", err)
	}
	fromFile, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(fromFile.(*image.NRGBA).Pix, want.Pix) {
		t.Fatal("written pixels do not match swizzle profile")
	}
}

func TestWorkbenchSwizzleCorpus(t *testing.T) {
	source := loadPNGNRGBA(t, filepath.Join("testdata", "corpus", "mip-grid-256.png"))
	tests := []struct {
		name    string
		profile SwizzleProfile
		format  bcn.Format
		noise   bool
	}{
		{name: "AlphaToRGB", profile: SwizzleProfileAlphaToRGB, format: bcn.FormatDXT1},
		{name: "AmbientSpecularMapGA", profile: SwizzleProfileAmbientSpecularMapGA, format: bcn.FormatDXT5},
		{name: "ColorNoise", format: bcn.FormatDXT5, noise: true},
		{name: "NormalMapGA", profile: SwizzleProfileNormalMapGA, format: bcn.FormatDXT5},
		{name: "NormalMap_NOHQ", profile: SwizzleProfileNormalMapNOHQ, format: bcn.FormatDXT5},
		{name: "NormalSpecularMapXYZS", profile: SwizzleProfileNormalSpecularMapXYZS, format: bcn.FormatDXT5},
		{name: "SMDIToGS", profile: SwizzleProfileSMDIToGS, format: bcn.FormatDXT1},
		{name: "TerrainLayerTexture", profile: SwizzleProfileTerrainLayerTexture, format: bcn.FormatDXT5},
		{name: "TerrainNormalSpecular_SYxX", profile: SwizzleProfileTerrainNormalSpecularSYxX, format: bcn.FormatDXT5},
		{name: "TerrainSuperTexture", profile: SwizzleProfileTerrainSuperTexture, format: bcn.FormatDXT5},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join("testdata", "swizzle", "mip-grid-256-Swizzle-"+tc.name+".edds")
			f, err := os.Open(path)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			header, dx10, err := readEDDSHeaders(f)
			_ = f.Close()
			if err != nil {
				t.Fatalf("readEDDSHeaders: %v", err)
			}
			if header.Width != 256 || header.Height != 256 || header.MipMapCount != 9 {
				t.Fatalf("header = %dx%d, %d mips; want 256x256, 9 mips", header.Width, header.Height, header.MipMapCount)
			}
			if got := detectFormat(header, dx10); got != tc.format {
				t.Fatalf("detectFormat() = %v, want %v", got, tc.format)
			}

			decoded, err := Read(path)
			if err != nil {
				t.Fatalf("Read: %v", err)
			}
			got, ok := decoded.(*image.NRGBA)
			if !ok {
				t.Fatalf("Read type = %T, want *image.NRGBA", decoded)
			}

			expected, err := applySwizzleProfileInto(nil, source, tc.profile)
			if err != nil {
				t.Fatalf("applySwizzleProfileInto: %v", err)
			}
			channels := 4
			if tc.noise {
				channels = 3
				if alphaValueCount(got) <= 16 {
					t.Fatal("ColorNoise alpha is not sufficiently varied")
				}
			}
			for channel, meanError := range meanAbsoluteChannelError(expected, got, channels) {
				if meanError > 2 {
					t.Fatalf("channel %d mean absolute error = %.2f, want <= 2", channel, meanError)
				}
			}
		})
	}
}

func loadPNGNRGBA(t *testing.T, path string) *image.NRGBA {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open %q: %v", path, err)
	}
	defer func() { _ = f.Close() }()

	decoded, err := png.Decode(f)
	if err != nil {
		t.Fatalf("Decode %q: %v", path, err)
	}
	result := image.NewNRGBA(decoded.Bounds())
	draw.Draw(result, result.Bounds(), decoded, decoded.Bounds().Min, draw.Src)
	return result
}

func meanAbsoluteChannelError(expected, got *image.NRGBA, channels int) []float64 {
	errors := make([]float64, channels)
	pixels := expected.Bounds().Dx() * expected.Bounds().Dy()
	for y := expected.Bounds().Min.Y; y < expected.Bounds().Max.Y; y++ {
		for x := expected.Bounds().Min.X; x < expected.Bounds().Max.X; x++ {
			expectedOffset := expected.PixOffset(x, y)
			gotOffset := got.PixOffset(x, y)
			for channel := range channels {
				delta := int(expected.Pix[expectedOffset+channel]) - int(got.Pix[gotOffset+channel])
				if delta < 0 {
					delta = -delta
				}
				errors[channel] += float64(delta)
			}
		}
	}
	for channel := range errors {
		errors[channel] /= float64(pixels)
	}
	return errors
}

func alphaValueCount(img *image.NRGBA) int {
	values := [256]bool{}
	for y := img.Bounds().Min.Y; y < img.Bounds().Max.Y; y++ {
		for x := img.Bounds().Min.X; x < img.Bounds().Max.X; x++ {
			values[img.Pix[img.PixOffset(x, y)+3]] = true
		}
	}

	count := 0
	for _, found := range values {
		if found {
			count++
		}
	}
	return count
}
