package edds

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/pierrec/lz4/v4"
)

const (
	// BlockMagicCOPY marks an uncompressed block.
	BlockMagicCOPY = "COPY"
	// BlockMagicLZ4 marks an LZ4-compressed block.
	BlockMagicLZ4 = "LZ4 "

	// ChunkSize is the Enfusion chunk size for LZ4 streams.
	ChunkSize = 64 * 1024
)

// Block represents one mipmap block body.
type Block struct {
	Magic            string
	Data             []byte
	Size             int32
	UncompressedSize int32
}

// writeBlockData writes the block payload (no table entry).
func writeBlockData(w io.Writer, block *Block) error {
	if block.Magic == BlockMagicLZ4 {
		if err := binary.Write(w, binary.LittleEndian, block.UncompressedSize); err != nil {
			return fmt.Errorf("%w: %v", ErrWriteUncompressedSize, err)
		}
		if _, err := w.Write(block.Data); err != nil {
			return fmt.Errorf("%w: %v", ErrWriteChunkStream, err)
		}
		return nil
	}
	if _, err := w.Write(block.Data); err != nil {
		return fmt.Errorf("%w: %v", ErrWriteBlockPayload, err)
	}
	return nil
}

// compressBlock compresses raw data into LZ4 chunk-stream or falls back to COPY.
func compressBlock(data []byte) (*Block, error) {
	if len(data) > maxInt32 {
		return nil, fmt.Errorf("%w: %d bytes", ErrInputTooLarge, len(data))
	}
	uncompressedSize, err := i32FromInt(len(data))
	if err != nil {
		return nil, err
	}

	if len(data) < 1024 {
		return &Block{Magic: BlockMagicCOPY, Size: uncompressedSize, Data: data}, nil
	}

	var chunkStream bytes.Buffer
	maxCompressedSize := lz4.CompressBlockBound(ChunkSize)
	compressBuf := make([]byte, maxCompressedSize)

	for i := 0; i < len(data); i += ChunkSize {
		end := i + ChunkSize
		if end > len(data) {
			end = len(data)
		}
		srcChunk := data[i:end]
		isLast := end == len(data)

		cn, err := lz4.CompressBlockHC(srcChunk, compressBuf, 0, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrLZ4Compress, err)
		}
		if cn == 0 || float64(cn) > float64(len(srcChunk))*0.85 {
			return &Block{Magic: BlockMagicCOPY, Size: uncompressedSize, Data: data}, nil
		}
		if cn > 0x7FFFFF {
			return nil, fmt.Errorf("%w: %d", ErrChunkTooLarge, cn)
		}

		chunkStream.WriteByte(byte(cn))
		chunkStream.WriteByte(byte(cn >> 8))
		chunkStream.WriteByte(byte(cn >> 16))
		if isLast {
			chunkStream.WriteByte(0x80)
		} else {
			chunkStream.WriteByte(0x00)
		}
		chunkStream.Write(compressBuf[:cn])
	}

	compressedData := chunkStream.Bytes()
	totalOverhead := 4 + len(compressedData)
	if totalOverhead > maxInt32 {
		return nil, fmt.Errorf("%w: %d bytes", ErrCompressedDataTooLarge, totalOverhead)
	}
	if float64(totalOverhead) > float64(len(data))*0.85 {
		return &Block{Magic: BlockMagicCOPY, Size: uncompressedSize, Data: data}, nil
	}

	size, err := i32FromInt(totalOverhead)
	if err != nil {
		return nil, err
	}

	return &Block{
		Magic:            BlockMagicLZ4,
		Size:             size,
		UncompressedSize: uncompressedSize,
		Data:             compressedData,
	}, nil
}

// decompressBlock inflates an EDDS block into raw data.
func decompressBlock(block *Block, expectedUncompressedSize int) ([]byte, error) {
	if block.Magic == BlockMagicCOPY {
		if len(block.Data) != expectedUncompressedSize {
			return nil, fmt.Errorf("%w: expected %d, got %d", ErrCopySizeMismatch, expectedUncompressedSize, len(block.Data))
		}
		out := make([]byte, len(block.Data))
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
		if (peek == expectedUncompressedSize || peek == targetSize) && c0 > 0 && c0 < (1<<20) {
			targetSize = peek
			data = data[4:]
		}
	}

	const dictCap = 64 * 1024
	dict := make([]byte, dictCap)
	dictSize := 0

	target := make([]byte, targetSize)
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
		want := ChunkSize
		if want > remaining {
			want = remaining
		}
		dst := target[outIdx : outIdx+want]

		n, err := lz4.UncompressBlockWithDict(compressed, dst, dict[:dictSize])
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrLZ4Decode, err)
		}

		outIdx += n

		decoded := target[outIdx-n : outIdx]
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

type blockHeader struct {
	Magic string
	Size  int32
}

func readBlockTable(r io.Reader, mipMapCount uint32) ([]blockHeader, error) {
	hdrs := make([]blockHeader, 0, mipMapCount)
	for i := uint32(0); i < mipMapCount; i++ {
		magicBytes := make([]byte, 4)
		if _, err := io.ReadFull(r, magicBytes); err != nil {
			return nil, fmt.Errorf("%w: %d: %v", ErrBlockTableMagicRead, i, err)
		}

		magic := string(magicBytes)
		var size int32
		if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
			return nil, fmt.Errorf("%w: %d: %v", ErrBlockTableSizeRead, i, err)
		}

		if magic != BlockMagicCOPY && magic != BlockMagicLZ4 {
			return nil, fmt.Errorf("%w: %d: %q", ErrBlockTableUnknownMagic, i, magic)
		}

		if size < 0 {
			return nil, fmt.Errorf("%w: %d: %d", ErrBlockTableInvalidSize, i, size)
		}

		hdrs = append(hdrs, blockHeader{Magic: magic, Size: size})
	}

	return hdrs, nil
}

func readBlockBody(r io.Reader, h blockHeader) (*Block, error) {
	if h.Size < 0 {
		return nil, fmt.Errorf("%w: %d", ErrBlockBodyInvalidSize, h.Size)
	}

	data := make([]byte, h.Size)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrBlockBodyRead, h.Magic, err)
	}

	return &Block{Magic: h.Magic, Size: h.Size, Data: data}, nil
}
