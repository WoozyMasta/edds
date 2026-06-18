// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/edds

package edds

import (
	"encoding/binary"
	"fmt"
	"image"
	"os"
	"slices"

	"github.com/woozymasta/bcn"
)

// WriteOptions configures fully customizable EDDS writing.
type WriteOptions struct {
	// EncodeOptions are passed directly to BCn encoder (quality/workers/etc.).
	EncodeOptions *bcn.EncodeOptions
	// Format selects output texture format.
	Format bcn.Format
	// MaxMipMaps limits written mipmaps (0 = full chain).
	MaxMipMaps int
	// Compression configures EDDS block compression.
	Compression CompressionOptions
	// Compress controls EDDS block compression (LZ4 if true, COPY if false).
	//
	// Deprecated: use Compression.Mode.
	Compress bool
}

// Write writes an EDDS file with a full mip chain.
func Write(img image.Image, path string) error {
	return writeWithOptions(img, path, nil)
}

// WriteWithMipmaps writes an EDDS file with a mipmap limit.
// maxMipMaps=0 means full chain.
func WriteWithMipmaps(img image.Image, path string, maxMipMaps int) error {
	return writeWithOptions(img, path, &WriteOptions{
		Format:     bcn.FormatBGRA8,
		MaxMipMaps: maxMipMaps,
		Compress:   true,
	})
}

// WriteWithFormat writes an EDDS file with the requested format.
// maxMipMaps=0 means full chain.
func WriteWithFormat(img image.Image, path string, format bcn.Format, maxMipMaps int) error {
	return writeWithOptions(img, path, &WriteOptions{
		Format:     format,
		MaxMipMaps: maxMipMaps,
		Compress:   true,
	})
}

// WriteWithFormatAndCompression writes an EDDS file with the requested format.
// maxMipMaps=0 means full chain. compress=false stores COPY blocks.
func WriteWithFormatAndCompression(img image.Image, path string, format bcn.Format, maxMipMaps int, compress bool) error {
	return writeWithOptions(img, path, &WriteOptions{
		Format:     format,
		MaxMipMaps: maxMipMaps,
		Compress:   compress,
	})
}

// WriteWithOptions writes EDDS with fully customizable options.
// Nil opts uses defaults: BGRA8, full mip chain, LZ4 compression.
func WriteWithOptions(img image.Image, path string, opts *WriteOptions) error {
	return writeWithOptions(img, path, opts)
}

// normalizeWriteOptions normalizes the write options.
func normalizeWriteOptions(opts *WriteOptions) WriteOptions {
	cfg := WriteOptions{
		Format:      bcn.FormatBGRA8,
		MaxMipMaps:  0,
		Compression: CompressionOptions{Mode: CompressionLZ4},
		Compress:    true,
	}
	if opts == nil {
		return cfg
	}

	if opts.Format != bcn.FormatUnknown {
		cfg.Format = opts.Format
	}
	cfg.MaxMipMaps = opts.MaxMipMaps
	cfg.Compress = opts.Compress
	cfg.Compression = opts.Compression
	cfg.EncodeOptions = opts.EncodeOptions

	return cfg
}

// WriteFromBlocks writes an EDDS file from pre-encoded mip payloads.
// The mipmaps slice must be ordered from largest to smallest.
func WriteFromBlocks(path string, format bcn.Format, width, height int, mipmaps [][]byte) error {
	compression, err := normalizeCompressionOptions(
		CompressionOptions{Mode: CompressionLZ4},
		true)
	if err != nil {
		return err
	}

	return writeFromBlocks(path, format, width, height, mipmaps, compression)
}

// WriteFromBlocksWithCompression writes an EDDS file from pre-encoded mip payloads.
// The mipmaps slice must be ordered from largest to smallest.
// compress=false stores COPY blocks (no LZ4).
func WriteFromBlocksWithCompression(
	path string,
	format bcn.Format,
	width, height int,
	mipmaps [][]byte,
	compress bool,
) error {
	compression, err := normalizeCompressionOptions(
		CompressionOptions{Mode: compressionModeFromBool(compress)},
		compress)
	if err != nil {
		return err
	}

	return writeFromBlocks(path, format, width, height, mipmaps, compression)
}

// WriteFromBlocksWithCompressionOptions writes an EDDS file from pre-encoded mip payloads.
// The mipmaps slice must be ordered from largest to smallest.
func WriteFromBlocksWithCompressionOptions(
	path string,
	format bcn.Format,
	width, height int,
	mipmaps [][]byte,
	compressionOpts CompressionOptions,
) error {
	compression, err := normalizeCompressionOptions(compressionOpts, true)
	if err != nil {
		return err
	}

	return writeFromBlocks(path, format, width, height, mipmaps, compression)
}

// writeWithOptions writes an EDDS file with full low-level options.
func writeWithOptions(
	img image.Image,
	path string,
	opts *WriteOptions,
) error {
	cfg := normalizeWriteOptions(opts)

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	mipMapCount, err := calculateMipMapCount(width, height)
	if err != nil {
		return err
	}
	if cfg.MaxMipMaps > 0 && cfg.MaxMipMaps < mipMapCount {
		mipMapCount = cfg.MaxMipMaps
	}
	if mipMapCount < 1 {
		mipMapCount = 1
	}

	mips := bcn.GenerateMipmaps(img, false)
	if len(mips) > mipMapCount {
		mips = mips[:mipMapCount]
	}

	payloads := make([][]byte, len(mips))
	for i, mip := range mips {
		data, _, _, err := bcn.EncodeImageWithOptions(mip, cfg.Format, cfg.EncodeOptions)
		if err != nil {
			return fmt.Errorf("%w: mipmap %d: %v", ErrCompressMipmap, i, err)
		}
		payloads[i] = data
	}

	compression, err := normalizeCompressionOptions(cfg.Compression, cfg.Compress)
	if err != nil {
		return err
	}

	return writeFromBlocks(path, cfg.Format, width, height, payloads, compression)
}

// writeFromBlocks validates pre-encoded mipmaps and writes an EDDS container.
func writeFromBlocks(
	path string,
	format bcn.Format,
	width, height int,
	mipmaps [][]byte,
	compression normalizedCompressionOptions,
) error {
	if len(mipmaps) == 0 {
		return ErrEmptyMipmaps
	}
	if format == bcn.FormatUnknown {
		return ErrInvalidFormat
	}

	// convert dimensions to uint32
	w32, err := u32FromInt(width)
	if err != nil {
		return err
	}
	h32, err := u32FromInt(height)
	if err != nil {
		return err
	}
	mip32, err := u32FromInt(len(mipmaps))
	if err != nil {
		return err
	}

	// create DDS header
	header, err := makeDDSHeader(w32, h32, mip32, format)
	if err != nil {
		return err
	}

	// create blocks
	blocks := make([]*Block, len(mipmaps))
	for i, mip := range mipmaps {
		mipW := mipDimension(width, i)
		mipH := mipDimension(height, i)
		expected := expectedDataLength(format, mipW, mipH)
		if expected <= 0 {
			return ErrInvalidFormat
		}
		if len(mip) != expected {
			return fmt.Errorf("%w: mipmap %d: expected %d, got %d", ErrMipmapSizeMismatch, i, expected, len(mip))
		}

		if compression.mode != CompressionNone {
			block, err := compressBlockWithOptions(mip, compression)
			if err != nil {
				return fmt.Errorf("%w: mipmap %d: %v", ErrCompressMipmap, i, err)
			}
			blocks[i] = block
		} else {
			size, err := i32FromInt(len(mip))
			if err != nil {
				return err
			}
			blocks[i] = &Block{Magic: BlockMagicCOPY, Size: size, Data: mip}
		}
	}

	// create file
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("%w: %q: %v", ErrCreateFile, path, err)
	}
	defer func() { _ = f.Close() }()

	// write DDS magic
	if err := bcn.WriteDDSMagic(f); err != nil {
		return fmt.Errorf("%w: %v", ErrWriteDDSMagic, err)
	}
	if err := bcn.WriteDDSHeader(f, header); err != nil {
		return fmt.Errorf("%w: %v", ErrWriteDDSHeader, err)
	}

	// write block table
	for i, v := range slices.Backward(blocks) {
		block := v
		if _, err := f.Write([]byte(block.Magic)); err != nil {
			return fmt.Errorf("%w: mipmap %d: %v", ErrWriteBlockMagic, i, err)
		}
		if err := binary.Write(f, binary.LittleEndian, block.Size); err != nil {
			return fmt.Errorf("%w: mipmap %d: %v", ErrWriteBlockSize, i, err)
		}
	}

	// write block data
	for i, v := range slices.Backward(blocks) {
		if err := writeBlockData(f, v); err != nil {
			return fmt.Errorf("%w: mipmap %d: %v", ErrWriteBlockData, i, err)
		}
	}

	return nil
}
