// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/edds

package edds

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"

	"github.com/woozymasta/bcn"
)

// ReadOptions configures EDDS reading (e.g. BCn decode workers).
type ReadOptions struct {
	// DecodeOptions are passed to the BCn decoder (e.g. Workers).
	DecodeOptions *bcn.DecodeOptions
}

// ReadConfig reads EDDS file configuration without decoding image data.
func ReadConfig(path string) (image.Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return image.Config{}, fmt.Errorf("%w: %q: %v", ErrOpenFile, path, err)
	}
	defer func() { _ = f.Close() }()

	header, _, err := readEDDSHeaders(f)
	if err != nil {
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
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %q: %v", ErrOpenFile, path, err)
	}
	defer func() { _ = f.Close() }()

	return NewDecoder().DecodeWithOptions(f, opts)
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
	rs, err := ensureReadSeeker(r)
	if err != nil {
		return nil, err
	}

	header, dx10, err := readEDDSHeaders(rs)
	if err != nil {
		return nil, err
	}

	format, _ := detectFormat(header, dx10)

	mipMapCount := uint32(1)
	if (header.Caps&bcn.DDSCapsMipmap) != 0 && header.MipMapCount > 0 {
		mipMapCount = header.MipMapCount
	}

	mipData, mipWidth, mipHeight, err := d.readLargestMipFromBlocks(rs, header, format, mipMapCount)
	if err != nil {
		mipData, mipWidth, mipHeight, err = d.readLegacySingleBlock(rs, header, dx10, format)
		if err != nil {
			return nil, err
		}
	}

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

// readLargestMipFromBlocksInto reads the largest mipmap using Decoder-owned buffers.
func (d *Decoder) readLargestMipFromBlocksInto(
	r io.ReadSeeker,
	header *bcn.DDSHeader,
	format bcn.Format,
	mipMapCount uint32,
) ([]byte, int, int, error) {
	if mipMapCount == 0 {
		mipMapCount = 1
	}

	table, err := readBlockTableInto(d.blockTable, r, mipMapCount)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%w: %v", ErrReadBlockTable, err)
	}
	d.blockTable = table

	// EDDS writes the block table and payloads from smallest to largest mip.
	// The largest mip is therefore the last logical level and is selected here.
	for i := uint32(0); i < mipMapCount; i++ {
		mipLevel := mipMapCount - i - 1
		if mipLevel != 0 {
			if _, err := r.Seek(int64(table[i].Size), io.SeekCurrent); err != nil {
				return nil, 0, 0, fmt.Errorf("%w: mipmap %d: %v", ErrSkipBlockBody, i, err)
			}
			continue
		}

		block, data, err := readBlockBodyInto(d.blockData, r, table[i])
		if err != nil {
			return nil, 0, 0, fmt.Errorf("%w: mipmap %d: %v", ErrReadBlockBody, i, err)
		}
		d.blockData = data

		mipW := mipDimension(int(header.Width), int(mipLevel))
		mipH := mipDimension(int(header.Height), int(mipLevel))

		expectedSize := expectedDataLength(format, mipW, mipH)
		if expectedSize <= 0 {
			return nil, 0, 0, fmt.Errorf("%w: %s for mipmap %d", ErrUnknownFormat, format, i)
		}

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
) ([]byte, int, int, error) {
	return d.readLargestMipFromBlocksInto(r, header, format, mipMapCount)
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
) ([]byte, int, int, error) {
	headerSize := int64(4 + bcn.DDSHeaderSize)
	if dx10 != nil {
		headerSize += 20
	}
	if _, err := r.Seek(headerSize, io.SeekStart); err != nil {
		return nil, 0, 0, fmt.Errorf("%w: %v", ErrSeekDataStart, err)
	}

	remainingData, err := io.ReadAll(r)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%w: %v", ErrReadRemainingData, err)
	}

	expectedSize := expectedDataLength(format, int(header.Width), int(header.Height))
	if expectedSize <= 0 {
		return nil, 0, 0, fmt.Errorf("%w: %s", ErrUnknownFormat, format)
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

// ensureReadSeeker returns r as an io.ReadSeeker, buffering non-seekable streams.
func ensureReadSeeker(r io.Reader) (io.ReadSeeker, error) {
	if rs, ok := r.(io.ReadSeeker); ok {
		return rs, nil
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrReadRemainingData, err)
	}

	return bytes.NewReader(data), nil
}
