// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/edds

package edds

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"

	"github.com/woozymasta/bcn"
)

const (
	defaultMaxReadMipMaps      = 32
	defaultMaxReadBlockBytes   = 1 << 30
	defaultMaxReadDecodedBytes = 1 << 30
	defaultMaxReadImageBytes   = 1 << 30
	defaultMaxReadInputBytes   = 2 << 30

	dx10ResourceDimensionTexture2D = 3
	dx10MiscTextureCube            = 0x4
)

// ReadOptions configures EDDS reading (e.g. BCn decode workers).
type ReadOptions struct {
	// DecodeOptions are passed to the BCn decoder (e.g. Workers).
	DecodeOptions *bcn.DecodeOptions
	// MaxMipMaps limits entries read from the EDDS block table. Zero uses the default.
	MaxMipMaps uint32
	// MaxBlockBytes limits one compressed or COPY block body. Zero uses the default.
	MaxBlockBytes int
	// MaxDecodedBytes limits one decoded mip payload. Zero uses the default.
	MaxDecodedBytes int
	// MaxImageBytes limits the decoded NRGBA image. Zero uses the default.
	MaxImageBytes int
	// MaxInputBytes limits buffered stream and legacy EDDS input. Zero uses the default.
	MaxInputBytes int64
}

type readLimits struct {
	maxMipMaps      uint32
	maxBlockBytes   int
	maxDecodedBytes int
	maxImageBytes   int
	maxInputBytes   int64
}

// limitedReader stops a sequential decode after the configured input limit.
type limitedReader struct {
	r         io.Reader
	remaining int64
}

// Read forwards up to the remaining input budget.
func (r *limitedReader) Read(p []byte) (int, error) {
	if r.remaining <= 0 {
		return 0, ErrReadLimitExceeded
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}

	n, err := r.r.Read(p)
	r.remaining -= int64(n)
	return n, err
}

// normalizeReadLimits applies default limits and validates caller-provided overrides.
func normalizeReadLimits(opts *ReadOptions) (readLimits, error) {
	limits := readLimits{
		maxMipMaps:      defaultMaxReadMipMaps,
		maxBlockBytes:   defaultMaxReadBlockBytes,
		maxDecodedBytes: defaultMaxReadDecodedBytes,
		maxImageBytes:   defaultMaxReadImageBytes,
		maxInputBytes:   min(int64(defaultMaxReadInputBytes), int64(maxInt-1)),
	}
	if opts == nil {
		return limits, nil
	}

	if opts.MaxMipMaps > defaultMaxReadMipMaps {
		return readLimits{}, fmt.Errorf("%w: MaxMipMaps must not exceed %d", ErrInvalidReadOptions, defaultMaxReadMipMaps)
	}
	if opts.MaxMipMaps > 0 {
		limits.maxMipMaps = opts.MaxMipMaps
	}
	if opts.MaxBlockBytes < 0 || opts.MaxDecodedBytes < 0 || opts.MaxImageBytes < 0 || opts.MaxInputBytes < 0 {
		return readLimits{}, fmt.Errorf("%w: limits must not be negative", ErrInvalidReadOptions)
	}
	if opts.MaxBlockBytes > maxInt32 {
		return readLimits{}, fmt.Errorf("%w: MaxBlockBytes exceeds EDDS block capacity", ErrInvalidReadOptions)
	}

	if opts.MaxBlockBytes > 0 {
		limits.maxBlockBytes = opts.MaxBlockBytes
	}
	if opts.MaxDecodedBytes > 0 {
		limits.maxDecodedBytes = opts.MaxDecodedBytes
	}
	if opts.MaxImageBytes > 0 {
		limits.maxImageBytes = opts.MaxImageBytes
	}
	if opts.MaxInputBytes > 0 {
		if opts.MaxInputBytes >= int64(maxInt) {
			return readLimits{}, fmt.Errorf("%w: MaxInputBytes exceeds platform capacity", ErrInvalidReadOptions)
		}
		limits.maxInputBytes = opts.MaxInputBytes
	}

	return limits, nil
}

// ReadConfig reads EDDS file configuration without decoding image data.
func ReadConfig(path string) (image.Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return image.Config{}, fmt.Errorf("%w: %q: %v", ErrOpenFile, path, err)
	}
	defer func() { _ = f.Close() }()

	header, dx10, err := readEDDSHeaders(f)
	if err != nil {
		return image.Config{}, err
	}
	if err := validateTextureType(header, dx10); err != nil {
		return image.Config{}, err
	}

	return image.Config{
		Width:      int(header.Width),
		Height:     int(header.Height),
		ColorModel: color.RGBAModel,
	}, nil
}

// Read reads and decodes an EDDS file into an image.
func Read(path string) (image.Image, error) {
	return ReadWithOptions(path, nil)
}

// Decode reads and decodes an EDDS stream into an image.
func Decode(r io.Reader) (image.Image, error) {
	return NewDecoder().Decode(r)
}

// DecodeWithOptions reads and decodes an EDDS stream with the given options.
func DecodeWithOptions(r io.Reader, opts *ReadOptions) (image.Image, error) {
	return NewDecoder().DecodeWithOptions(r, opts)
}

// ReadWithOptions reads and decodes an EDDS file with the given options.
// Nil opts uses default decoding (no DecodeOptions passed to bcn).
func ReadWithOptions(path string, opts *ReadOptions) (image.Image, error) {
	limits, err := normalizeReadLimits(opts)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %q: %v", ErrOpenFile, path, err)
	}
	defer func() { _ = f.Close() }()
	if err := validateInputFileSize(f, limits); err != nil {
		return nil, err
	}

	header, dx10, err := readEDDSHeaders(f)
	if err != nil {
		return nil, err
	}
	if err := validateTextureType(header, dx10); err != nil {
		return nil, err
	}

	format := detectFormat(header, dx10)

	mipMapCount, err := readMipMapCount(header, limits)
	if err != nil {
		return nil, err
	}

	hasBlockTable, err := hasBlockTableMagicAtCurrent(f)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrReadBlockTable, err)
	}

	var mipData []byte
	var mipWidth, mipHeight int
	if hasBlockTable {
		mipData, mipWidth, mipHeight, err = readLargestMipFromBlocks(f, header, format, mipMapCount, limits)
	} else {
		mipData, mipWidth, mipHeight, err = readLegacySingleBlock(f, header, dx10, format, limits)
		if err != nil {
			return nil, err
		}
	}
	if err != nil {
		return nil, err
	}

	decOpts := (*bcn.DecodeOptions)(nil)
	if opts != nil {
		decOpts = opts.DecodeOptions
	}
	rgbaData, err := bcn.DecodeImageWithOptions(mipData, mipWidth, mipHeight, format, decOpts)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecodeImage, err)
	}

	return rgbaData, nil
}

// Decoder decodes EDDS streams while reusing internal buffers across calls.
// A Decoder is NOT safe for concurrent use.
// The returned image shares the Decoder's reusable pixel buffer and is only valid
// until the next Decode call on the same Decoder.
type Decoder struct {
	img          *image.NRGBA
	blockTable   []blockHeader
	blockData    []byte
	raw          []byte
	decompressor blockDecompressor
}

// NewDecoder returns a ready-to-use Decoder.
func NewDecoder() *Decoder {
	return &Decoder{}
}

// Decode reads and decodes an EDDS stream into an image.
func (d *Decoder) Decode(r io.Reader) (image.Image, error) {
	return d.DecodeWithOptions(r, nil)
}

// DecodeWithOptions reads and decodes an EDDS stream with the given options.
func (d *Decoder) DecodeWithOptions(r io.Reader, opts *ReadOptions) (image.Image, error) {
	limits, err := normalizeReadLimits(opts)
	if err != nil {
		return nil, err
	}

	if rs, ok := r.(io.ReadSeeker); ok {
		return d.decodeReadSeeker(rs, opts, limits)
	}

	stream := bufio.NewReader(&limitedReader{r: r, remaining: limits.maxInputBytes})
	return d.decodeStream(stream, opts, limits)
}

// decodeReadSeeker decodes an EDDS stream and supports seeking back for legacy input.
func (d *Decoder) decodeReadSeeker(r io.ReadSeeker, opts *ReadOptions, limits readLimits) (image.Image, error) {
	header, dx10, err := readEDDSHeaders(r)
	if err != nil {
		return nil, err
	}
	if err := validateTextureType(header, dx10); err != nil {
		return nil, err
	}

	format := detectFormat(header, dx10)

	mipMapCount, err := readMipMapCount(header, limits)
	if err != nil {
		return nil, err
	}

	hasBlockTable, err := hasBlockTableMagicAtCurrent(r)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrReadBlockTable, err)
	}

	var mipData []byte
	var mipWidth, mipHeight int
	if hasBlockTable {
		mipData, mipWidth, mipHeight, err = d.readLargestMipFromBlocks(r, header, format, mipMapCount, limits)
	} else {
		mipData, mipWidth, mipHeight, err = d.readLegacySingleBlock(r, header, dx10, format, limits)
		if err != nil {
			return nil, err
		}
	}
	if err != nil {
		return nil, err
	}

	return d.decodePayload(mipData, mipWidth, mipHeight, format, opts)
}

// decodeStream decodes a non-seekable EDDS stream without buffering the whole input.
func (d *Decoder) decodeStream(r *bufio.Reader, opts *ReadOptions, limits readLimits) (image.Image, error) {
	header, dx10, err := readEDDSHeaders(r)
	if err != nil {
		return nil, err
	}
	if err := validateTextureType(header, dx10); err != nil {
		return nil, err
	}

	format := detectFormat(header, dx10)
	mipMapCount, err := readMipMapCount(header, limits)
	if err != nil {
		return nil, err
	}

	hasBlockTable, err := hasBlockTableMagic(r)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrReadBlockTable, err)
	}
	if !hasBlockTable {
		mipData, mipWidth, mipHeight, err := d.readLegacySingleBlockFromReader(r, header, format, limits)
		if err != nil {
			return nil, err
		}
		return d.decodePayload(mipData, mipWidth, mipHeight, format, opts)
	}

	mipData, mipWidth, mipHeight, err := d.readLargestMipFromReader(r, header, format, mipMapCount, limits)
	if err != nil {
		return nil, err
	}

	return d.decodePayload(mipData, mipWidth, mipHeight, format, opts)
}

// decodePayload converts the selected EDDS mip payload into an NRGBA image.
func (d *Decoder) decodePayload(
	mipData []byte,
	mipWidth, mipHeight int,
	format bcn.Format,
	opts *ReadOptions,
) (image.Image, error) {
	decOpts := (*bcn.DecodeOptions)(nil)
	if opts != nil {
		decOpts = opts.DecodeOptions
	}
	rgbaData, err := bcn.DecodeImageInto(d.img, mipData, mipWidth, mipHeight, format, decOpts)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecodeImage, err)
	}
	d.img = rgbaData

	return rgbaData, nil
}

// readLargestMipFromBlocks reads the largest mipmap from the blocks.
func readLargestMipFromBlocks(
	r io.ReadSeeker,
	header *bcn.DDSHeader,
	format bcn.Format,
	mipMapCount uint32,
	limits readLimits,
) ([]byte, int, int, error) {
	if mipMapCount == 0 {
		mipMapCount = 1
	}

	table, err := readBlockTable(r, mipMapCount)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%w: %v", ErrReadBlockTable, err)
	}
	if err := validateBlockTable(table, limits); err != nil {
		return nil, 0, 0, err
	}

	// EDDS writes the block table and payloads from smallest to largest mip.
	// The largest mip is therefore the last logical level and is selected here.
	for i := range mipMapCount {
		mipLevel := mipMapCount - i - 1
		if mipLevel != 0 {
			if _, err := r.Seek(int64(table[i].Size), io.SeekCurrent); err != nil {
				return nil, 0, 0, fmt.Errorf("%w: mipmap %d: %v", ErrSkipBlockBody, i, err)
			}
			continue
		}

		mipW := mipDimension(int(header.Width), int(mipLevel))
		mipH := mipDimension(int(header.Height), int(mipLevel))

		expectedSize, err := expectedReadDataLength(format, mipW, mipH, limits)
		if err != nil {
			return nil, 0, 0, err
		}

		block, err := readBlockBody(r, table[i])
		if err != nil {
			return nil, 0, 0, fmt.Errorf("%w: mipmap %d: %v", ErrReadBlockBody, i, err)
		}

		decompressed, err := decompressBlock(block, expectedSize)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("%w: mipmap %d: %v", ErrDecompressBlock, i, err)
		}
		if len(decompressed) != expectedSize {
			return nil, 0, 0, fmt.Errorf(
				"%w: expected %d, got %d",
				ErrLargestMipSizeMismatch,
				expectedSize,
				len(decompressed))
		}

		return decompressed, mipW, mipH, nil
	}

	return nil, 0, 0, fmt.Errorf("%w: mipmaps=%d", ErrPickLargestMip, mipMapCount)
}

// readLargestMipFromBlocksInto reads the largest mipmap using Decoder-owned buffers.
func (d *Decoder) readLargestMipFromBlocksInto(
	r io.ReadSeeker,
	header *bcn.DDSHeader,
	format bcn.Format,
	mipMapCount uint32,
	limits readLimits,
) ([]byte, int, int, error) {
	if mipMapCount == 0 {
		mipMapCount = 1
	}

	table, err := readBlockTableInto(d.blockTable, r, mipMapCount)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%w: %v", ErrReadBlockTable, err)
	}
	d.blockTable = table
	if err := validateBlockTable(table, limits); err != nil {
		return nil, 0, 0, err
	}

	// EDDS writes the block table and payloads from smallest to largest mip.
	// The largest mip is therefore the last logical level and is selected here.
	for i := range mipMapCount {
		mipLevel := mipMapCount - i - 1
		if mipLevel != 0 {
			if _, err := r.Seek(int64(table[i].Size), io.SeekCurrent); err != nil {
				return nil, 0, 0, fmt.Errorf("%w: mipmap %d: %v", ErrSkipBlockBody, i, err)
			}
			continue
		}

		mipW := mipDimension(int(header.Width), int(mipLevel))
		mipH := mipDimension(int(header.Height), int(mipLevel))

		expectedSize, err := expectedReadDataLength(format, mipW, mipH, limits)
		if err != nil {
			return nil, 0, 0, err
		}

		block, data, err := readBlockBodyInto(d.blockData, r, table[i])
		if err != nil {
			return nil, 0, 0, fmt.Errorf("%w: mipmap %d: %v", ErrReadBlockBody, i, err)
		}
		d.blockData = data

		decompressed, err := d.decompressor.decompressBlock(d.raw, block, expectedSize)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("%w: mipmap %d: %v", ErrDecompressBlock, i, err)
		}

		d.raw = decompressed
		if len(decompressed) != expectedSize {
			return nil, 0, 0, fmt.Errorf(
				"%w: expected %d, got %d",
				ErrLargestMipSizeMismatch,
				expectedSize,
				len(decompressed))
		}

		return decompressed, mipW, mipH, nil
	}

	return nil, 0, 0, fmt.Errorf("%w: mipmaps=%d", ErrPickLargestMip, mipMapCount)
}

// readLargestMipFromBlocks reads the largest mipmap through the reusable Decoder path.
func (d *Decoder) readLargestMipFromBlocks(
	r io.ReadSeeker,
	header *bcn.DDSHeader,
	format bcn.Format,
	mipMapCount uint32,
	limits readLimits,
) ([]byte, int, int, error) {
	return d.readLargestMipFromBlocksInto(r, header, format, mipMapCount, limits)
}

// readLargestMipFromReader reads the largest mipmap from a sequential EDDS stream.
func (d *Decoder) readLargestMipFromReader(
	r io.Reader,
	header *bcn.DDSHeader,
	format bcn.Format,
	mipMapCount uint32,
	limits readLimits,
) ([]byte, int, int, error) {
	table, err := readBlockTable(r, mipMapCount)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%w: %v", ErrReadBlockTable, err)
	}
	if err := validateBlockTable(table, limits); err != nil {
		return nil, 0, 0, err
	}

	for i := range mipMapCount {
		mipLevel := mipMapCount - i - 1
		if mipLevel != 0 {
			if err := discardBlockBody(r, table[i].Size); err != nil {
				return nil, 0, 0, fmt.Errorf("%w: mipmap %d: %w", ErrSkipBlockBody, i, err)
			}
			continue
		}

		mipW := mipDimension(int(header.Width), int(mipLevel))
		mipH := mipDimension(int(header.Height), int(mipLevel))
		expectedSize, err := expectedReadDataLength(format, mipW, mipH, limits)
		if err != nil {
			return nil, 0, 0, err
		}

		block, data, err := readBlockBodyInto(d.blockData, r, table[i])
		if err != nil {
			return nil, 0, 0, fmt.Errorf("%w: mipmap %d: %v", ErrReadBlockBody, i, err)
		}
		d.blockData = data

		decompressed, err := d.decompressor.decompressBlock(d.raw, block, expectedSize)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("%w: mipmap %d: %v", ErrDecompressBlock, i, err)
		}
		d.raw = decompressed
		return decompressed, mipW, mipH, nil
	}

	return nil, 0, 0, fmt.Errorf("%w: mipmaps=%d", ErrPickLargestMip, mipMapCount)
}

// hasBlockTableMagic reports whether the next bytes begin a current EDDS block table.
func hasBlockTableMagic(r *bufio.Reader) (bool, error) {
	magic, err := r.Peek(4)
	if err != nil {
		return false, err
	}

	return isBlockTableMagic(magic), nil
}

// hasBlockTableMagicAtCurrent reports whether an io.ReadSeeker is positioned at a current block table.
func hasBlockTableMagicAtCurrent(r io.ReadSeeker) (bool, error) {
	position, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return false, err
	}

	var magic [4]byte
	_, readErr := io.ReadFull(r, magic[:])
	_, seekErr := r.Seek(position, io.SeekStart)
	if readErr != nil {
		return false, readErr
	}
	if seekErr != nil {
		return false, seekErr
	}

	return isBlockTableMagic(magic[:]), nil
}

// isBlockTableMagic reports whether magic is a supported current EDDS block marker.
func isBlockTableMagic(magic []byte) bool {
	return bytes.Equal(magic, []byte(BlockMagicCOPY)) || bytes.Equal(magic, []byte(BlockMagicLZ4))
}

// discardBlockBody consumes one block body from a sequential EDDS stream.
func discardBlockBody(r io.Reader, size int32) error {
	_, err := io.CopyN(io.Discard, r, int64(size))
	return err
}

// readLegacySingleBlock is a backward-compatibility fallback for older EDDS files.
// Some legacy files do not have a valid block table after the DDS header
// and instead store a single payload blob.
func readLegacySingleBlock(
	r io.ReadSeeker,
	header *bcn.DDSHeader,
	dx10 *bcn.DDSHeaderDX10,
	format bcn.Format,
	limits readLimits,
) ([]byte, int, int, error) {
	expectedSize, err := expectedReadDataLength(format, int(header.Width), int(header.Height), limits)
	if err != nil {
		return nil, 0, 0, err
	}

	headerSize := int64(4 + bcn.DDSHeaderSize)
	if dx10 != nil {
		headerSize += 20
	}
	if _, err := r.Seek(headerSize, io.SeekStart); err != nil {
		return nil, 0, 0, fmt.Errorf("%w: %v", ErrSeekDataStart, err)
	}

	remainingData, err := readAllWithLimit(r, int64(limits.maxBlockBytes))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%w: %w", ErrReadRemainingData, err)
	}

	size, err := i32FromInt(len(remainingData))
	if err != nil {
		return nil, 0, 0, err
	}

	block := &Block{Magic: BlockMagicLZ4, Size: size, Data: remainingData}
	decompressed, err := decompressBlock(block, expectedSize)
	if err == nil {
		return decompressed, int(header.Width), int(header.Height), nil
	}

	if len(remainingData) == expectedSize {
		// Older uncompressed files may contain only raw mip payload after DDS headers.
		return remainingData, int(header.Width), int(header.Height), nil
	}

	return nil, 0, 0, fmt.Errorf("%w: %v", ErrParseSingleBlock, err)
}

// readLegacySingleBlock reads old EDDS payloads
// without a block table using Decoder-owned buffers.
// Some legacy files do not have a valid block table after the DDS header
// and instead store a single payload blob.
// We treat that blob as an LZ4 block first, and if decompression fails
// but the size already matches the expected mip size,
// we accept it as raw uncompressed data.
func (d *Decoder) readLegacySingleBlock(
	r io.ReadSeeker,
	header *bcn.DDSHeader,
	dx10 *bcn.DDSHeaderDX10,
	format bcn.Format,
	limits readLimits,
) ([]byte, int, int, error) {
	headerSize := int64(4 + bcn.DDSHeaderSize)
	if dx10 != nil {
		headerSize += 20
	}
	if _, err := r.Seek(headerSize, io.SeekStart); err != nil {
		return nil, 0, 0, fmt.Errorf("%w: %v", ErrSeekDataStart, err)
	}

	return d.readLegacySingleBlockFromReader(r, header, format, limits)
}

// readLegacySingleBlockFromReader reads a bounded legacy EDDS payload from r.
func (d *Decoder) readLegacySingleBlockFromReader(
	r io.Reader,
	header *bcn.DDSHeader,
	format bcn.Format,
	limits readLimits,
) ([]byte, int, int, error) {
	expectedSize, err := expectedReadDataLength(format, int(header.Width), int(header.Height), limits)
	if err != nil {
		return nil, 0, 0, err
	}

	remainingData, err := readAllWithLimit(r, int64(limits.maxBlockBytes))
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%w: %w", ErrReadRemainingData, err)
	}

	size, err := i32FromInt(len(remainingData))
	if err != nil {
		return nil, 0, 0, err
	}

	block := &Block{Magic: BlockMagicLZ4, Size: size, Data: remainingData}
	decompressed, err := d.decompressor.decompressBlock(d.raw, block, expectedSize)
	if err == nil {
		d.raw = decompressed
		return decompressed, int(header.Width), int(header.Height), nil
	}

	if len(remainingData) == expectedSize {
		// Older uncompressed files may contain only raw mip payload after DDS headers.
		return remainingData, int(header.Width), int(header.Height), nil
	}

	return nil, 0, 0, fmt.Errorf("%w: %v", ErrParseSingleBlock, err)
}

// readEDDSHeaders reads the EDDS headers from the reader.
func readEDDSHeaders(r io.Reader) (*bcn.DDSHeader, *bcn.DDSHeaderDX10, error) {
	header, err := bcn.ReadDDSHeader(r)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrDDSHeaderRead, err)
	}

	dx10, err := bcn.ReadDDSHeaderDX10(r, header)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrDDSDX10Read, err)
	}

	return header, dx10, nil
}

// validateTextureType ensures the DDS resource maps to one NRGBA image.
func validateTextureType(header *bcn.DDSHeader, dx10 *bcn.DDSHeaderDX10) error {
	if (header.Caps2 & bcn.DDSCaps2Cubemap) != 0 {
		return fmt.Errorf("%w: cubemaps are not supported", ErrUnsupportedTextureType)
	}
	if dx10 == nil {
		return nil
	}
	if dx10.ResourceDimension != dx10ResourceDimensionTexture2D {
		return fmt.Errorf("%w: DX10 resource dimension %d", ErrUnsupportedTextureType, dx10.ResourceDimension)
	}
	if dx10.ArraySize != 1 {
		return fmt.Errorf("%w: DX10 array size %d", ErrUnsupportedTextureType, dx10.ArraySize)
	}
	if (dx10.MiscFlag & dx10MiscTextureCube) != 0 {
		return fmt.Errorf("%w: DX10 cubemaps are not supported", ErrUnsupportedTextureType)
	}

	return nil
}

// readMipMapCount returns the header mip count after enforcing the configured limit.
func readMipMapCount(header *bcn.DDSHeader, limits readLimits) (uint32, error) {
	mipMapCount := uint32(1)
	if (header.Caps&bcn.DDSCapsMipmap) != 0 && header.MipMapCount > 0 {
		mipMapCount = header.MipMapCount
	}
	if mipMapCount > limits.maxMipMaps {
		return 0, fmt.Errorf("%w: mipmaps %d exceeds %d", ErrReadLimitExceeded, mipMapCount, limits.maxMipMaps)
	}

	return mipMapCount, nil
}

// validateBlockTable ensures every block body fits within the configured limit.
func validateBlockTable(table []blockHeader, limits readLimits) error {
	for i, block := range table {
		if int64(block.Size) > int64(limits.maxBlockBytes) {
			return fmt.Errorf("%w: block %d size %d exceeds %d", ErrReadLimitExceeded, i, block.Size, limits.maxBlockBytes)
		}
	}

	return nil
}

// expectedReadDataLength validates the raw mip payload and decoded image sizes.
func expectedReadDataLength(format bcn.Format, width, height int, limits readLimits) (int, error) {
	imageSize, err := expectedDataLengthChecked(bcn.FormatRGBA8, width, height)
	if err != nil {
		return 0, err
	}
	if imageSize > limits.maxImageBytes {
		return 0, fmt.Errorf("%w: decoded image %d bytes exceeds %d", ErrReadLimitExceeded, imageSize, limits.maxImageBytes)
	}

	size, err := expectedDataLengthChecked(format, width, height)
	if err != nil {
		if err == ErrInvalidFormat {
			return 0, fmt.Errorf("%w: %s", ErrUnknownFormat, format)
		}
		return 0, err
	}
	if size > limits.maxDecodedBytes {
		return 0, fmt.Errorf("%w: decoded mip %d bytes exceeds %d", ErrReadLimitExceeded, size, limits.maxDecodedBytes)
	}

	return size, nil
}

// validateInputFileSize rejects files that exceed the configured buffered-input limit.
func validateInputFileSize(f *os.File, limits readLimits) error {
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrReadRemainingData, err)
	}
	if info.Size() > limits.maxInputBytes {
		return fmt.Errorf("%w: input %d bytes exceeds %d", ErrReadLimitExceeded, info.Size(), limits.maxInputBytes)
	}

	return nil
}

// readAllWithLimit reads at most maxBytes bytes and reports a read-limit error otherwise.
func readAllWithLimit(r io.Reader, maxBytes int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("%w: input exceeds %d bytes", ErrReadLimitExceeded, maxBytes)
	}

	return data, nil
}
