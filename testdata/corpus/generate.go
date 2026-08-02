// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/edds

// Command generate creates the small Workbench conversion source fixture.
package main

import (
	"flag"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

const fixtureSize = 256

// main writes the deterministic RGBA PNG fixture next to this source file.
func main() {
	outDir := flag.String("out", ".", "output directory")
	flag.Parse()

	img := image.NewNRGBA(image.Rect(0, 0, fixtureSize, fixtureSize))
	for y := range fixtureSize {
		for x := range fixtureSize {
			pixel := color.NRGBA{
				R: uint8(x),     //nolint:gosec // bounded by fixtureSize
				G: uint8(y),     //nolint:gosec // bounded by fixtureSize
				B: uint8(x ^ y), //nolint:gosec // bounded by fixtureSize
				A: 192,
			}
			if x%32 < 2 || y%32 < 2 {
				pixel = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
			}
			img.SetNRGBA(x, y, pixel)
		}
	}

	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		panic(err)
	}
	f, err := os.Create(filepath.Join(*outDir, "mip-grid-256.png"))
	if err != nil {
		panic(err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}
