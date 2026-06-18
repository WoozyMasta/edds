// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/edds

package edds

import (
	"encoding/binary"
	"fmt"
	"image"
	"io"
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
	return WriteWithOptions(img, path, nil)
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

// Encode writes an EDDS stream with default options.
func Encode(w io.Writer, img image.Image) error {
	return NewEncoder().Encode(w, img)
}

// EncodeWithOptions writes an EDDS stream with fully customizable options.
func EncodeWithOptions(w io.Writer, img image.Image, opts *WriteOptions) error {
	return NewEncoder().EncodeWithOptions(w, img, opts)
}

// EncodeFromBlocks writes an EDDS stream from pre-encoded mip payloads.
// The mipmaps slice must be ordered from largest to smallest.
func EncodeFromBlocks(w io.Writer, format bcn.Format, width, height int, mipmaps [][]byte) error {
	compression, err := normalizeCompressionOptions(
		CompressionOptions{Mode: CompressionLZ4},
		true)
	if err != nil {
		return err
	}

	return NewEncoder().writeFromBlocks(w, format, width, height, mipmaps, compression)
}

// EncodeFromBlocksWithCompression writes an EDDS stream from pre-encoded mip payloads.
// The mipmaps slice must be ordered from largest to smallest.
// compress=false stores COPY blocks (no LZ4).
func EncodeFromBlocksWithCompression(
	w io.Writer,
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

	return NewEncoder().writeFromBlocks(w, format, width, height, mipmaps, compression)
}

// EncodeFromBlocksWithCompressionOptions writes an EDDS stream
// from pre-encoded mip payloads.
// The mipmaps slice must be ordered from largest to smallest.
func EncodeFromBlocksWithCompressionOptions(
	w io.Writer,
	format bcn.Format,
	width, height int,
	mipmaps [][]byte,
	compressionOpts CompressionOptions,
) error {
	compression, err := normalizeCompressionOptions(compressionOpts, true)
	if err != nil {
		return err
	}

	return NewEncoder().writeFromBlocks(w, format, width, height, mipmaps, compression)
}

// Encoder encodes EDDS streams while reusing internal buffers across calls.
// An Encoder is NOT safe for concurrent use; create one per worker goroutine.
type Encoder struct {
	mips          []*image.NRGBA
	payloads      [][]byte
	blockPayloads [][]byte
	blocks        []*Block
	compressor    blockCompressor
}

// NewEncoder returns a ready-to-use Encoder.
func NewEncoder() *Encoder {
	return &Encoder{}
}

// Encode writes an EDDS stream with default options.
func (e *Encoder) Encode(w io.Writer, img image.Image) error {
	return e.EncodeWithOptions(w, img, nil)
}

// EncodeWithOptions writes an EDDS stream with fully customizable options.
func (e *Encoder) EncodeWithOptions(w io.Writer, img image.Image, opts *WriteOptions) error {
	return e.writeWithOptions(w, img, opts)
}

// EncodeFromBlocks writes an EDDS stream from pre-encoded mip payloads.
// The mipmaps slice must be ordered from largest to smallest.
func (e *Encoder) EncodeFromBlocks(w io.Writer, format bcn.Format, width, height int, mipmaps [][]byte) error {
	compression, err := normalizeCompressionOptions(
		CompressionOptions{Mode: CompressionLZ4},
		true)
	if err != nil {
		return err
	}

	return e.writeFromBlocks(w, format, width, height, mipmaps, compression)
}

// EncodeFromBlocksWithCompression writes an EDDS stream from pre-encoded mip payloads.
// The mipmaps slice must be ordered from largest to smallest.
// compress=false stores COPY blocks (no LZ4).
func (e *Encoder) EncodeFromBlocksWithCompression(
	w io.Writer,
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

	return e.writeFromBlocks(w, format, width, height, mipmaps, compression)
}

// EncodeFromBlocksWithCompressionOptions writes an EDDS stream
// from pre-encoded mip payloads.
// The mipmaps slice must be ordered from largest to smallest.
func (e *Encoder) EncodeFromBlocksWithCompressionOptions(
	w io.Writer,
	format bcn.Format,
	width, height int,
	mipmaps [][]byte,
	compressionOpts CompressionOptions,
) error {
	compression, err := normalizeCompressionOptions(compressionOpts, true)
	if err != nil {
		return err
	}

	return e.writeFromBlocks(w, format, width, height, mipmaps, compression)
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

	mips := bcn.GenerateMipmapsN(img, mipMapCount, false)

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

// writeWithOptions writes img to w using Encoder-owned reusable buffers.
func (e *Encoder) writeWithOptions(
	w io.Writer,
	img image.Image,
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

	// BCn owns mip generation; using the Into variant lets batch encoders retain buffers.
	e.mips = bcn.GenerateMipmapsInto(e.mips, img, mipMapCount, false)

	e.payloads = ensurePayloadSlots(e.payloads, len(e.mips))
	payloads := e.payloads[:len(e.mips)]
	for i, mip := range e.mips {
		data, _, _, err := bcn.EncodeImageInto(payloads[i], mip, cfg.Format, cfg.EncodeOptions)
		if err != nil {
			return fmt.Errorf("%w: mipmap %d: %v", ErrCompressMipmap, i, err)
		}
		payloads[i] = data
	}

	compression, err := normalizeCompressionOptions(cfg.Compression, cfg.Compress)
	if err != nil {
		return err
	}

	return e.writeFromBlocks(w, cfg.Format, width, height, payloads, compression)
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

	// Build all block descriptors before opening the output file because the table precedes payload data.
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

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("%w: %q: %v", ErrCreateFile, path, err)
	}
	defer func() { _ = f.Close() }()

	if err := bcn.WriteDDSMagic(f); err != nil {
		return fmt.Errorf("%w: %v", ErrWriteDDSMagic, err)
	}
	if err := bcn.WriteDDSHeader(f, header); err != nil {
		return fmt.Errorf("%w: %v", ErrWriteDDSHeader, err)
	}

	// EDDS stores mip table entries from smallest to largest mip.
	for i, v := range slices.Backward(blocks) {
		block := v
		if _, err := f.Write([]byte(block.Magic)); err != nil {
			return fmt.Errorf("%w: mipmap %d: %v", ErrWriteBlockMagic, i, err)
		}
		if err := binary.Write(f, binary.LittleEndian, block.Size); err != nil {
			return fmt.Errorf("%w: mipmap %d: %v", ErrWriteBlockSize, i, err)
		}
	}

	// Payload order mirrors the table order.
	for i, v := range slices.Backward(blocks) {
		if err := writeBlockData(f, v); err != nil {
			return fmt.Errorf("%w: mipmap %d: %v", ErrWriteBlockData, i, err)
		}
	}

	return nil
}

// writeFromBlocks validates pre-encoded mipmaps and writes an EDDS stream.
func (e *Encoder) writeFromBlocks(
	w io.Writer,
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

	header, err := makeDDSHeader(w32, h32, mip32, format)
	if err != nil {
		return err
	}

	// Build all block descriptors before writing because the table precedes payload data.
	e.blocks = ensureBlockSlots(e.blocks, len(mipmaps))
	e.blockPayloads = ensurePayloadSlots(e.blockPayloads, len(mipmaps))
	blocks := e.blocks[:len(mipmaps)]
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
			block, payload, err := e.compressor.compressBlock(e.blockPayloads[i], mip, compression)
			if err != nil {
				return fmt.Errorf("%w: mipmap %d: %v", ErrCompressMipmap, i, err)
			}
			e.blockPayloads[i] = payload
			blocks[i] = block
		} else {
			size, err := i32FromInt(len(mip))
			if err != nil {
				return err
			}
			blocks[i] = &Block{Magic: BlockMagicCOPY, Size: size, Data: mip}
		}
	}

	if err := bcn.WriteDDSMagic(w); err != nil {
		return fmt.Errorf("%w: %v", ErrWriteDDSMagic, err)
	}
	if err := bcn.WriteDDSHeader(w, header); err != nil {
		return fmt.Errorf("%w: %v", ErrWriteDDSHeader, err)
	}

	// EDDS stores mip table entries from smallest to largest mip.
	for i, v := range slices.Backward(blocks) {
		block := v
		if _, err := w.Write([]byte(block.Magic)); err != nil {
			return fmt.Errorf("%w: mipmap %d: %v", ErrWriteBlockMagic, i, err)
		}
		if err := binary.Write(w, binary.LittleEndian, block.Size); err != nil {
			return fmt.Errorf("%w: mipmap %d: %v", ErrWriteBlockSize, i, err)
		}
	}

	// Payload order mirrors the table order.
	for i, v := range slices.Backward(blocks) {
		if err := writeBlockData(w, v); err != nil {
			return fmt.Errorf("%w: mipmap %d: %v", ErrWriteBlockData, i, err)
		}
	}

	return nil
}

// ensurePayloadSlots returns slots resized to n,
// allocating only when capacity is insufficient.
func ensurePayloadSlots(slots [][]byte, n int) [][]byte {
	if cap(slots) < n {
		next := make([][]byte, n)
		copy(next, slots)
		return next
	}

	return slots[:n]
}

// ensureBlockSlots returns slots resized to n,
// allocating only when capacity is insufficient.
func ensureBlockSlots(slots []*Block, n int) []*Block {
	if cap(slots) < n {
		next := make([]*Block, n)
		copy(next, slots)
		return next
	}

	return slots[:n]
}
