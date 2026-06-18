// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/edds

package edds

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	// BlockMagicCOPY marks an uncompressed block.
	BlockMagicCOPY = "COPY"
	// BlockMagicLZ4 marks an LZ4-compressed block.
	BlockMagicLZ4 = "LZ4 "
)

// Block represents one mipmap block body.
type Block struct {
	Magic            string // COPY or LZ4
	Data             []byte // compressed or uncompressed data
	Size             int32  // compressed size
	UncompressedSize int32  // uncompressed size
}

// blockHeader represents one mipmap block table entry.
type blockHeader struct {
	Magic string // COPY or LZ4
	Size  int32  // compressed size
}

// writeBlockData writes a block payload without its block table entry.
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

// readBlockTableInto reads block headers into a reusable slice.
func readBlockTableInto(dst []blockHeader, r io.Reader, mipMapCount uint32) ([]blockHeader, error) {
	hdrs := ensureBlockHeaderSlots(dst, int(mipMapCount))[:0]
	for i := range mipMapCount {
		var magicBytes [4]byte
		if _, err := io.ReadFull(r, magicBytes[:]); err != nil {
			return nil, fmt.Errorf("%w: %d: %v", ErrBlockTableMagicRead, i, err)
		}

		var magic string
		switch magicBytes {
		case [4]byte{'C', 'O', 'P', 'Y'}:
			magic = BlockMagicCOPY
		case [4]byte{'L', 'Z', '4', ' '}:
			magic = BlockMagicLZ4
		default:
			// Keep the original bytes in the error path without allocating for known magic values.
			magic = string(magicBytes[:])
		}
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

// readBlockBodyInto reads one block body into a reusable buffer.
func readBlockBodyInto(dst []byte, r io.Reader, h blockHeader) (*Block, []byte, error) {
	if h.Size < 0 {
		return nil, dst, fmt.Errorf("%w: %d", ErrBlockBodyInvalidSize, h.Size)
	}

	data := ensureLen(dst, int(h.Size))
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, data, fmt.Errorf("%w: %s: %v", ErrBlockBodyRead, h.Magic, err)
	}

	return &Block{Magic: h.Magic, Size: h.Size, Data: data}, data, nil
}

// ensureBlockHeaderSlots returns slots resized to n, allocating only when capacity is insufficient.
func ensureBlockHeaderSlots(slots []blockHeader, n int) []blockHeader {
	if cap(slots) < n {
		next := make([]blockHeader, n)
		copy(next, slots)
		return next
	}

	return slots[:n]
}
