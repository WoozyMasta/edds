package edds

import (
	"encoding/binary"
	"fmt"
	"image"
	"os"

	"github.com/woozymasta/bcn"
)

// Write writes an EDDS file with a full mip chain.
func Write(img image.Image, path string) error {
	return WriteWithFormat(img, path, bcn.FormatBGRA8, 0)
}

// WriteWithMipmaps writes an EDDS file with a mipmap limit.
// maxMipMaps=0 means full chain.
func WriteWithMipmaps(img image.Image, path string, maxMipMaps int) error {
	return WriteWithFormat(img, path, bcn.FormatBGRA8, maxMipMaps)
}

// WriteWithFormat writes an EDDS file with the requested format.
// maxMipMaps=0 means full chain.
func WriteWithFormat(img image.Image, path string, format bcn.Format, maxMipMaps int) error {
	return WriteWithFormatAndCompression(img, path, format, maxMipMaps, true)
}

// WriteWithFormatAndCompression writes an EDDS file with the requested format.
// maxMipMaps=0 means full chain. compress=false stores COPY blocks.
func WriteWithFormatAndCompression(img image.Image, path string, format bcn.Format, maxMipMaps int, compress bool) error {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	mipMapCount, err := calculateMipMapCount(width, height)
	if err != nil {
		return err
	}
	if maxMipMaps > 0 && maxMipMaps < mipMapCount {
		mipMapCount = maxMipMaps
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
		data, _, _, err := bcn.EncodeImageWithOptions(mip, format, nil)
		if err != nil {
			return fmt.Errorf("%w: mipmap %d: %v", ErrCompressMipmap, i, err)
		}
		payloads[i] = data
	}

	return WriteFromBlocksWithCompression(path, format, width, height, payloads, compress)
}

// WriteFromBlocks writes an EDDS file from pre-encoded mip payloads.
// The mipmaps slice must be ordered from largest to smallest.
func WriteFromBlocks(path string, format bcn.Format, width, height int, mipmaps [][]byte) error {
	return WriteFromBlocksWithCompression(path, format, width, height, mipmaps, true)
}

// WriteFromBlocksWithCompression writes an EDDS file from pre-encoded mip payloads.
// The mipmaps slice must be ordered from largest to smallest.
// compress=false stores COPY blocks (no LZ4).
func WriteFromBlocksWithCompression(path string, format bcn.Format, width, height int, mipmaps [][]byte, compress bool) error {
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

		if compress {
			block, err := compressBlock(mip)
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

	for i := len(blocks) - 1; i >= 0; i-- {
		block := blocks[i]
		if _, err := f.Write([]byte(block.Magic)); err != nil {
			return fmt.Errorf("%w: mipmap %d: %v", ErrWriteBlockMagic, i, err)
		}
		if err := binary.Write(f, binary.LittleEndian, block.Size); err != nil {
			return fmt.Errorf("%w: mipmap %d: %v", ErrWriteBlockSize, i, err)
		}
	}

	for i := len(blocks) - 1; i >= 0; i-- {
		if err := writeBlockData(f, blocks[i]); err != nil {
			return fmt.Errorf("%w: mipmap %d: %v", ErrWriteBlockData, i, err)
		}
	}

	return nil
}
