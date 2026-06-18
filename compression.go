// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/edds

package edds

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/pierrec/lz4/v4"
)

const (
	// ChunkSize is the Enfusion chunk size for LZ4 streams.
	ChunkSize = 64 * 1024
)

const (
	// CompressionDefault preserves legacy Compress bool behavior.
	CompressionDefault CompressionMode = iota
	// CompressionNone stores raw blocks without LZ4 compression.
	CompressionNone
	// CompressionLZ4 uses the fast LZ4 block compressor.
	CompressionLZ4
	// CompressionLZ4HC uses the high-compression LZ4 block compressor.
	CompressionLZ4HC
)

// CompressionMode selects how EDDS block bodies are stored.
type CompressionMode int

// CompressionOptions configures EDDS block compression.
type CompressionOptions struct {
	// Mode selects compression algorithm. Default preserves legacy Compress behavior.
	Mode CompressionMode
	// HCLevel tunes CompressionLZ4HC. 0 = library default, 1..9 = explicit level.
	HCLevel int
	// MinRatio is raw/stored threshold. 0 = default threshold.
	MinRatio float64
	// ChunkSize controls LZ4 chunk size. 0 = EDDS default chunk size.
	ChunkSize int
}

// normalizedCompressionOptions contains validated compression settings.
type normalizedCompressionOptions struct {
	mode      CompressionMode
	hcLevel   lz4.CompressionLevel
	minRatio  float64
	chunkSize int
}

// String returns the stable display name for a compression mode.
func (m CompressionMode) String() string {
	switch m {
	case CompressionDefault:
		return "Default"
	case CompressionNone:
		return "None"
	case CompressionLZ4:
		return "LZ4"
	case CompressionLZ4HC:
		return "LZ4HC"
	default:
		return "Unknown"
	}
}

// isValid reports whether the compression mode is a concrete supported mode.
func (m CompressionMode) isValid() bool {
	switch m {
	case CompressionNone, CompressionLZ4, CompressionLZ4HC:
		return true
	default:
		return false
	}
}

// compressionModeFromBool maps the deprecated Compress bool to a concrete mode.
func compressionModeFromBool(compress bool) CompressionMode {
	if compress {
		return CompressionLZ4
	}

	return CompressionNone
}

// normalizeCompressionOptions validates user options and fills default values.
func normalizeCompressionOptions(opts CompressionOptions, compress bool) (normalizedCompressionOptions, error) {
	mode := opts.Mode
	if mode == CompressionDefault {
		mode = compressionModeFromBool(compress)
	}
	if !mode.isValid() {
		return normalizedCompressionOptions{}, ErrInvalidCompressionOptions
	}

	minRatio := opts.MinRatio
	if minRatio == 0 {
		minRatio = 1.0 / 0.85
	}
	if minRatio <= 0 {
		return normalizedCompressionOptions{}, fmt.Errorf("%w: MinRatio must be positive", ErrInvalidCompressionOptions)
	}

	chunkSize := opts.ChunkSize
	if chunkSize == 0 {
		chunkSize = ChunkSize
	}
	if chunkSize <= 0 || chunkSize > ChunkSize {
		return normalizedCompressionOptions{}, fmt.Errorf("%w: ChunkSize must be in 1..%d", ErrInvalidCompressionOptions, ChunkSize)
	}

	level, err := compressionHCLevel(opts.HCLevel)
	if err != nil {
		return normalizedCompressionOptions{}, err
	}
	if mode != CompressionLZ4HC && opts.HCLevel != 0 {
		return normalizedCompressionOptions{}, fmt.Errorf("%w: HCLevel requires LZ4HC", ErrInvalidCompressionOptions)
	}

	return normalizedCompressionOptions{
		mode:      mode,
		hcLevel:   level,
		minRatio:  minRatio,
		chunkSize: chunkSize,
	}, nil
}

// compressionHCLevel maps public HCLevel integers to lz4 compression levels.
func compressionHCLevel(level int) (lz4.CompressionLevel, error) {
	switch level {
	case 0:
		return 0, nil
	case 1:
		return lz4.Level1, nil
	case 2:
		return lz4.Level2, nil
	case 3:
		return lz4.Level3, nil
	case 4:
		return lz4.Level4, nil
	case 5:
		return lz4.Level5, nil
	case 6:
		return lz4.Level6, nil
	case 7:
		return lz4.Level7, nil
	case 8:
		return lz4.Level8, nil
	case 9:
		return lz4.Level9, nil
	default:
		return 0, fmt.Errorf("%w: HCLevel must be 0..9", ErrInvalidCompressionOptions)
	}
}

// copyBlock creates an uncompressed COPY block for raw data.
func copyBlock(data []byte) (*Block, error) {
	if len(data) > maxInt32 {
		return nil, fmt.Errorf("%w: %d bytes", ErrInputTooLarge, len(data))
	}
	uncompressedSize, err := i32FromInt(len(data))
	if err != nil {
		return nil, err
	}

	return &Block{Magic: BlockMagicCOPY, Size: uncompressedSize, Data: data}, nil
}

// blockCompressor keeps temporary LZ4 buffers for repeated block compression.
type blockCompressor struct {
	compressBuf []byte
}

// compressBlock compresses raw data into dst when possible
// and returns the retained output buffer.
func (c *blockCompressor) compressBlock(dst []byte, data []byte, opts normalizedCompressionOptions) (*Block, []byte, error) {
	if !opts.mode.isValid() {
		return nil, dst, ErrInvalidCompressionOptions
	}

	if opts.mode == CompressionNone {
		block, err := copyBlock(data)
		return block, dst, err
	}
	if len(data) > maxInt32 {
		return nil, dst, fmt.Errorf("%w: %d bytes", ErrInputTooLarge, len(data))
	}

	uncompressedSize, err := i32FromInt(len(data))
	if err != nil {
		return nil, dst, err
	}
	if len(data) < 1024 {
		block, err := copyBlock(data)
		return block, dst, err
	}

	// Reserve for the expected stream size, not the LZ4 upper bound.
	// If compression later fails the ratio check, dst is retained for reuse.
	if cap(dst) < compressedStreamCapacity(len(data), opts.chunkSize, opts.minRatio) {
		dst = make([]byte, 0, compressedStreamCapacity(len(data), opts.chunkSize, opts.minRatio))
	} else {
		dst = dst[:0]
	}
	maxCompressedSize := lz4.CompressBlockBound(opts.chunkSize)
	if cap(c.compressBuf) < maxCompressedSize {
		c.compressBuf = make([]byte, maxCompressedSize)
	}
	compressBuf := c.compressBuf[:maxCompressedSize]
	var fastCompressor lz4.Compressor
	var hcCompressor *lz4.CompressorHC
	if opts.mode == CompressionLZ4HC && opts.hcLevel != 0 {
		hcCompressor = &lz4.CompressorHC{Level: opts.hcLevel}
	}

	for i := 0; i < len(data); i += opts.chunkSize {
		end := min(i+opts.chunkSize, len(data))
		srcChunk := data[i:end]
		isLast := end == len(data)

		var cn int
		var err error
		switch opts.mode {
		case CompressionLZ4:
			cn, err = fastCompressor.CompressBlock(srcChunk, compressBuf)
		case CompressionLZ4HC:
			if hcCompressor != nil {
				cn, err = hcCompressor.CompressBlock(srcChunk, compressBuf)
			} else {
				cn, err = lz4.CompressBlockHC(srcChunk, compressBuf, 0, nil, nil)
			}
		}
		if err != nil {
			return nil, dst, fmt.Errorf("%w: %v", ErrLZ4Compress, err)
		}

		if cn == 0 || float64(len(srcChunk))/float64(cn) < opts.minRatio {
			block, err := copyBlock(data)
			return block, dst, err
		}
		if cn > 0x7FFFFF {
			return nil, dst, fmt.Errorf("%w: %d", ErrChunkTooLarge, cn)
		}

		// EDDS stores each LZ4 chunk as a 24-bit compressed size plus flags.
		dst = append(dst, byte(cn), byte(cn>>8), byte(cn>>16))
		if isLast {
			dst = append(dst, 0x80)
		} else {
			dst = append(dst, 0x00)
		}
		dst = append(dst, compressBuf[:cn]...)
	}

	totalOverhead := 4 + len(dst)
	if totalOverhead > maxInt32 {
		return nil, dst, fmt.Errorf("%w: %d bytes", ErrCompressedDataTooLarge, totalOverhead)
	}

	if float64(len(data))/float64(totalOverhead) < opts.minRatio {
		block, err := copyBlock(data)
		return block, dst, err
	}

	size, err := i32FromInt(totalOverhead)
	if err != nil {
		return nil, dst, err
	}

	return &Block{
		Magic:            BlockMagicLZ4,
		Size:             size,
		UncompressedSize: uncompressedSize,
		Data:             dst,
	}, dst, nil
}

// compressedStreamCapacity estimates the final LZ4 chunk-stream size.
func compressedStreamCapacity(dataLen, chunkSize int, minRatio float64) int {
	if dataLen <= 0 || chunkSize <= 0 {
		return 0
	}

	targetRatio := max(minRatio, 2.0)
	var capacity int
	for i := 0; i < dataLen; i += chunkSize {
		chunkLen := min(chunkSize, dataLen-i)
		capacity += int(float64(chunkLen)/targetRatio) + 5
	}

	if capacity > dataLen {
		return dataLen
	}

	return capacity
}

// blockDecompressor keeps the rolling LZ4 dictionary for repeated block decompression.
type blockDecompressor struct {
	dict []byte
}

// decompressBlock inflates block into dst when possible
// and preserves the LZ4 dictionary buffer.
func (d *blockDecompressor) decompressBlock(dst []byte, block *Block, expectedUncompressedSize int) ([]byte, error) {
	if block.Magic == BlockMagicCOPY {
		if len(block.Data) != expectedUncompressedSize {
			return nil, fmt.Errorf("%w: expected %d, got %d", ErrCopySizeMismatch, expectedUncompressedSize, len(block.Data))
		}
		out := ensureLen(dst, len(block.Data))
		copy(out, block.Data)
		return out, nil
	}

	if block.Magic != BlockMagicLZ4 {
		return nil, fmt.Errorf("%w: %q", ErrUnknownBlockMagic, block.Magic)
	}

	targetSize := expectedUncompressedSize
	if block.UncompressedSize > 0 {
		targetSize = int(block.UncompressedSize)
	}
	if targetSize <= 0 {
		return nil, fmt.Errorf("%w: %d", ErrInvalidTargetSize, targetSize)
	}

	data := block.Data
	if len(data) >= 8 {
		peek := int(binary.LittleEndian.Uint32(data[:4]))
		c0 := int(data[4]) | (int(data[5]) << 8) | (int(data[6]) << 16)
		// Some legacy writers include the uncompressed size in the block payload.
		// New files keep it outside Block.Data via writeBlockData.
		if (peek == expectedUncompressedSize || peek == targetSize) && c0 > 0 && c0 < (1<<20) {
			targetSize = peek
			data = data[4:]
		}
	}

	const dictCap = 64 * 1024
	if cap(d.dict) < dictCap {
		d.dict = make([]byte, dictCap)
	}
	dict := d.dict[:dictCap]
	dictSize := 0

	target := ensureLen(dst, targetSize)
	outIdx := 0

	r := bytes.NewReader(data)
	for {
		if r.Len() < 4 {
			return nil, fmt.Errorf("%w: need 4 bytes header, have %d", ErrChunkStreamTruncated, r.Len())
		}

		var hdr [4]byte
		if _, err := io.ReadFull(r, hdr[:]); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrChunkHeaderRead, err)
		}

		cSize := int(hdr[0]) | (int(hdr[1]) << 8) | (int(hdr[2]) << 16)
		flags := hdr[3]
		if (flags &^ 0x80) != 0 {
			return nil, fmt.Errorf("%w: 0x%02x", ErrUnknownLZ4Flags, flags)
		}
		if cSize <= 0 || cSize > r.Len() {
			return nil, fmt.Errorf("%w: %d (remaining %d)", ErrInvalidChunkSize, cSize, r.Len())
		}

		compressed := make([]byte, cSize)
		if _, err := io.ReadFull(r, compressed); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrChunkDataRead, err)
		}

		remaining := targetSize - outIdx
		if remaining <= 0 {
			return nil, ErrDecodeOverrun
		}
		want := min(ChunkSize, remaining)
		dst := target[outIdx : outIdx+want]

		n, err := lz4.UncompressBlockWithDict(compressed, dst, dict[:dictSize])
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrLZ4Decode, err)
		}

		outIdx += n

		decoded := target[outIdx-n : outIdx]
		// LZ4 block mode uses the previous 64 KiB of decoded bytes as dictionary.
		if len(decoded) >= dictCap {
			copy(dict, decoded[len(decoded)-dictCap:])
			dictSize = dictCap
		} else {
			avail := dictCap - dictSize
			if len(decoded) <= avail {
				copy(dict[dictSize:], decoded)
				dictSize += len(decoded)
			} else {
				shift := len(decoded) - avail
				copy(dict, dict[shift:dictSize])
				copy(dict[dictCap-len(decoded):], decoded)
				dictSize = dictCap
			}
		}

		if (flags & 0x80) != 0 {
			break
		}
	}

	if outIdx != targetSize {
		return nil, fmt.Errorf("%w: expected %d, got %d", ErrDecodedSizeMismatch, targetSize, outIdx)
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("%w: %d bytes left after decode", ErrBlockLengthMismatch, r.Len())
	}

	return target, nil
}

// ensureLen returns b resized to n, allocating only when capacity is insufficient.
func ensureLen(b []byte, n int) []byte {
	if cap(b) < n {
		return make([]byte, n)
	}

	return b[:n]
}
