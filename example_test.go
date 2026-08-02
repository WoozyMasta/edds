// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/edds

package edds_test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"

	"github.com/woozymasta/bcn"
	"github.com/woozymasta/edds"
)

func Example() {
	dir, err := os.MkdirTemp("", "edds-example-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	path := filepath.Join(dir, "texture.edds")

	if err := edds.Write(img, path); err != nil {
		panic(err)
	}

	decoded, err := edds.Read(path)
	if err != nil {
		panic(err)
	}

	fmt.Println(decoded.Bounds())
	// Output:
	// (0,0)-(2,2)
}

func ExampleEncode() {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})

	var data bytes.Buffer
	if err := edds.Encode(&data, img); err != nil {
		panic(err)
	}

	decoded, err := edds.Decode(&data)
	if err != nil {
		panic(err)
	}

	fmt.Println(decoded.Bounds())
	// Output:
	// (0,0)-(2,2)
}

func ExampleWriteWithOptions() {
	dir, err := os.MkdirTemp("", "edds-example-")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	path := filepath.Join(dir, "texture.edds")
	if err := edds.WriteWithOptions(img, path, &edds.WriteOptions{
		Format:      bcn.FormatDXT5,
		MaxMipMaps:  1,
		Compression: edds.CompressionOptions{Mode: edds.CompressionLZ4},
	}); err != nil {
		panic(err)
	}

	config, err := edds.ReadConfig(path)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%dx%d\n", config.Width, config.Height)
	// Output:
	// 4x4
}
