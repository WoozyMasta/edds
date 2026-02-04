package edds

import (
	"fmt"
	"image"
	"image/color"
	"io"
	"os"

	"github.com/woozymasta/bcn"
)

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
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %q: %v", ErrOpenFile, path, err)
	}
	defer func() { _ = f.Close() }()

	header, dx10, err := readEDDSHeaders(f)
	if err != nil {
		return nil, err
	}

	format, _ := detectFormat(header, dx10)

	mipMapCount := uint32(1)
	if (header.Caps&bcn.DDSCapsMipmap) != 0 && header.MipMapCount > 0 {
		mipMapCount = header.MipMapCount
	}

	mipData, mipWidth, mipHeight, err := readLargestMipFromBlocks(f, header, format, mipMapCount)
	if err != nil {
		mipData, mipWidth, mipHeight, err = readLegacySingleBlock(f, header, dx10, format)
		if err != nil {
			return nil, err
		}
	}

	rgbaData, err := bcn.DecodeImage(mipData, mipWidth, mipHeight, format)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecodeImage, err)
	}

	return rgbaData, nil
}

func readLargestMipFromBlocks(r io.ReadSeeker, header *bcn.DDSHeader, format bcn.Format, mipMapCount uint32) ([]byte, int, int, error) {
	if mipMapCount == 0 {
		mipMapCount = 1
	}

	table, err := readBlockTable(r, mipMapCount)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("%w: %v", ErrReadBlockTable, err)
	}

	for i := uint32(0); i < mipMapCount; i++ {
		mipLevel := mipMapCount - i - 1
		if mipLevel != 0 {
			if _, err := r.Seek(int64(table[i].Size), io.SeekCurrent); err != nil {
				return nil, 0, 0, fmt.Errorf("%w: mipmap %d: %v", ErrSkipBlockBody, i, err)
			}
			continue
		}

		block, err := readBlockBody(r, table[i])
		if err != nil {
			return nil, 0, 0, fmt.Errorf("%w: mipmap %d: %v", ErrReadBlockBody, i, err)
		}

		mipW := mipDimension(int(header.Width), int(mipLevel))
		mipH := mipDimension(int(header.Height), int(mipLevel))

		expectedSize := expectedDataLength(format, mipW, mipH)
		if expectedSize <= 0 {
			return nil, 0, 0, fmt.Errorf("%w: %s for mipmap %d", ErrUnknownFormat, format, i)
		}

		decompressed, err := decompressBlock(block, expectedSize)
		if err != nil {
			return nil, 0, 0, fmt.Errorf("%w: mipmap %d: %v", ErrDecompressBlock, i, err)
		}
		if len(decompressed) != expectedSize {
			return nil, 0, 0, fmt.Errorf("%w: expected %d, got %d", ErrLargestMipSizeMismatch, expectedSize, len(decompressed))
		}

		return decompressed, mipW, mipH, nil
	}

	return nil, 0, 0, fmt.Errorf("%w: mipmaps=%d", ErrPickLargestMip, mipMapCount)
}

func readLegacySingleBlock(r io.ReadSeeker, header *bcn.DDSHeader, dx10 *bcn.DDSHeaderDX10, format bcn.Format) ([]byte, int, int, error) {
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
	decompressed, err := decompressBlock(block, expectedSize)
	if err == nil {
		return decompressed, int(header.Width), int(header.Height), nil
	}

	if len(remainingData) == expectedSize {
		return remainingData, int(header.Width), int(header.Height), nil
	}

	return nil, 0, 0, fmt.Errorf("%w: %v", ErrParseSingleBlock, err)
}

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
