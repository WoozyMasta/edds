package edds

import (
	"bytes"
	"testing"
)

func FuzzReadEDDSHeaders(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("DDS "))
	f.Add([]byte{
		'D', 'D', 'S', ' ',
		124, 0, 0, 0,
	})

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 4096 {
			t.Skip()
		}

		_, _, _ = readEDDSHeaders(bytes.NewReader(data))
	})
}

func FuzzDecompressBlock(f *testing.F) {
	f.Add(BlockMagicCOPY, []byte{}, 0)
	f.Add(BlockMagicCOPY, []byte{1, 2, 3}, 3)
	f.Add(BlockMagicCOPY, []byte{1, 2, 3}, 2)
	f.Add(BlockMagicLZ4, []byte{}, 16)
	f.Add("BAD!", []byte{1, 2, 3, 4}, 4)

	raw := make([]byte, 4096)
	for i := range raw {
		raw[i] = byte(i / 64)
	}
	block, err := compressBlockWithOptions(raw, normalizedCompressionOptions{
		mode:      CompressionLZ4,
		minRatio:  1.0 / 0.85,
		chunkSize: ChunkSize,
	})
	if err == nil {
		f.Add(block.Magic, block.Data, len(raw))
	}

	f.Fuzz(func(t *testing.T, magic string, data []byte, expectedSize int) {
		if len(magic) > 16 || len(data) > 64*1024 {
			t.Skip()
		}
		if expectedSize < 0 || expectedSize > 64*1024 {
			t.Skip()
		}

		block := &Block{
			Magic: magic,
			Size:  int32(len(data)), //nolint:gosec // bounded above.
			Data:  data,
		}

		_, _ = decompressBlock(block, expectedSize)
	})
}
