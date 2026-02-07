package edds

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/woozymasta/bcn"
)

func TestCompressRoundTrip(t *testing.T) {
	data := make([]byte, 128*1024)
	for i := range data {
		data[i] = byte((i*31 + 7) & 0xff)
	}

	block, err := compressBlock(data)
	if err != nil {
		t.Fatalf("compressBlock: %v", err)
	}

	out, err := decompressBlock(block, len(data))
	if err != nil {
		t.Fatalf("decompressBlock: %v", err)
	}

	if !bytes.Equal(out, data) {
		t.Fatalf("round-trip mismatch")
	}
}

func TestWriteRead(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x * 30), G: uint8(y * 30), B: 100, A: 255})
		}
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "test.edds")

	if err := WriteWithMipmaps(img, path, 0); err != nil {
		t.Fatalf("WriteWithMipmaps: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	gotImg, ok := got.(*image.NRGBA)
	if !ok {
		t.Fatalf("expected *image.NRGBA, got %T", got)
	}

	if gotImg.Bounds().Dx() != 8 || gotImg.Bounds().Dy() != 8 {
		t.Fatalf("unexpected size: %dx%d", gotImg.Bounds().Dx(), gotImg.Bounds().Dy())
	}

	if !bytes.Equal(gotImg.Pix, img.Pix) {
		// dump file for quick inspection when debugging
		_ = os.WriteFile(filepath.Join(dir, "got.raw"), gotImg.Pix, 0o644)
		_ = os.WriteFile(filepath.Join(dir, "want.raw"), img.Pix, 0o644)
		t.Fatalf("pixel mismatch")
	}
}

func TestWriteWithFormatAndOptions(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x * 10), G: uint8(y * 10), B: 100, A: 255})
		}
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "test_dxt5.edds")

	err := WriteWithOptions(img, path, &WriteOptions{
		Format:     bcn.FormatDXT5,
		MaxMipMaps: 1,
		Compress:   true,
		EncodeOptions: &bcn.EncodeOptions{
			QualityLevel: 8,
			Workers:      0,
		},
	})
	if err != nil {
		t.Fatalf("WriteWithOptions: %v", err)
	}

	cfg, err := ReadConfig(path)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if cfg.Width != 16 || cfg.Height != 16 {
		t.Fatalf("unexpected size: %dx%d", cfg.Width, cfg.Height)
	}
}

func TestWriteFromBlocksWithCompressionValidation(t *testing.T) {
	t.Parallel()

	validDXT1 := make([]byte, 8) // 4x4 DXT1 = 1 block = 8 bytes

	tests := []struct {
		name    string
		format  bcn.Format
		width   int
		height  int
		mips    [][]byte
		wantErr error
	}{
		{name: "empty-mips", format: bcn.FormatDXT1, width: 4, height: 4, mips: nil, wantErr: ErrEmptyMipmaps},
		{name: "unknown-format", format: bcn.FormatUnknown, width: 4, height: 4, mips: [][]byte{validDXT1}, wantErr: ErrInvalidFormat},
		{name: "mipmap-size-mismatch", format: bcn.FormatDXT1, width: 4, height: 4, mips: [][]byte{make([]byte, 7)}, wantErr: ErrMipmapSizeMismatch},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "out.edds")
			err := WriteFromBlocksWithCompression(path, tc.format, tc.width, tc.height, tc.mips, true)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected error %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestDetectFormatTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header *bcn.DDSHeader
		dx10   *bcn.DDSHeaderDX10
		want   bcn.Format
	}{
		{
			name: "fourcc-dxt1",
			header: &bcn.DDSHeader{
				PixelFormat: bcn.DDSPixelFormat{
					Flags:  bcn.DDSPFFourCC,
					FourCC: makeFourCC('D', 'X', 'T', '1'),
				},
			},
			want: bcn.FormatDXT1,
		},
		{
			name: "rgb-bgra8",
			header: &bcn.DDSHeader{
				PixelFormat: bcn.DDSPixelFormat{
					Flags:       bcn.DDSPFRGB | bcn.DDSPFAlphaPixels,
					RGBBitCount: 32,
					RBitMask:    0x00ff0000,
					GBitMask:    0x0000ff00,
					BBitMask:    0x000000ff,
					ABitMask:    0xff000000,
				},
			},
			want: bcn.FormatBGRA8,
		},
		{
			name: "dxgi-dxt5",
			dx10: &bcn.DDSHeaderDX10{DXGIFormat: 77},
			want: bcn.FormatDXT5,
		},
		{
			name: "unknown",
			header: &bcn.DDSHeader{
				PixelFormat: bcn.DDSPixelFormat{
					Flags:  bcn.DDSPFFourCC,
					FourCC: makeFourCC('X', 'X', 'X', 'X'),
				},
			},
			want: bcn.FormatUnknown,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, _ := detectFormat(tc.header, tc.dx10)
			if got != tc.want {
				t.Fatalf("detectFormat() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExpectedDataLengthTable(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		format bcn.Format
		w      int
		h      int
		want   int
	}{
		{name: "dxt1-4x4", format: bcn.FormatDXT1, w: 4, h: 4, want: 8},
		{name: "dxt1-5x7", format: bcn.FormatDXT1, w: 5, h: 7, want: 32},
		{name: "dxt5-4x4", format: bcn.FormatDXT5, w: 4, h: 4, want: 16},
		{name: "bgra8-1x1", format: bcn.FormatBGRA8, w: 1, h: 1, want: 4},
		{name: "bgra8-5x7", format: bcn.FormatBGRA8, w: 5, h: 7, want: 140},
		{name: "unknown", format: bcn.FormatUnknown, w: 4, h: 4, want: -1},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := expectedDataLength(tc.format, tc.w, tc.h)
			if got != tc.want {
				t.Fatalf("expectedDataLength(%v,%d,%d) = %d, want %d", tc.format, tc.w, tc.h, got, tc.want)
			}
		})
	}
}

func TestReadBlockTableErrors(t *testing.T) {
	t.Parallel()

	t.Run("unknown-magic", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		_, _ = buf.WriteString("ABCD")
		_ = binary.Write(&buf, binary.LittleEndian, int32(8))

		_, err := readBlockTable(bytes.NewReader(buf.Bytes()), 1)
		if !errors.Is(err, ErrBlockTableUnknownMagic) {
			t.Fatalf("expected ErrBlockTableUnknownMagic, got %v", err)
		}
	})

	t.Run("negative-size", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		_, _ = buf.WriteString(BlockMagicCOPY)
		_ = binary.Write(&buf, binary.LittleEndian, int32(-1))

		_, err := readBlockTable(bytes.NewReader(buf.Bytes()), 1)
		if !errors.Is(err, ErrBlockTableInvalidSize) {
			t.Fatalf("expected ErrBlockTableInvalidSize, got %v", err)
		}
	})
}

func TestWriteWithFormatAndCompressionCOPYPath(t *testing.T) {
	t.Parallel()

	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.NRGBA{
				R: uint8(x * 20), //nolint:gosec // bounded
				G: uint8(y * 20), //nolint:gosec // bounded
				B: 90,
				A: 255,
			})
		}
	}

	path := filepath.Join(t.TempDir(), "copy.edds")
	if err := WriteWithFormatAndCompression(img, path, bcn.FormatBGRA8, 1, false); err != nil {
		t.Fatalf("WriteWithFormatAndCompression: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	gotNRGBA, ok := got.(*image.NRGBA)
	if !ok {
		t.Fatalf("expected *image.NRGBA, got %T", got)
	}
	if !bytes.Equal(gotNRGBA.Pix, img.Pix) {
		t.Fatalf("COPY path pixel mismatch")
	}
}
