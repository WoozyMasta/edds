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

type maxReadRequestReader struct {
	r   *bytes.Reader
	max int
}

func (r *maxReadRequestReader) Read(p []byte) (int, error) {
	if len(p) > r.max {
		return 0, errors.New("read request exceeds maximum")
	}

	return r.r.Read(p)
}

func TestCompressRoundTrip(t *testing.T) {
	data := make([]byte, 128*1024)
	for i := range data {
		data[i] = byte((i / 1024) & 0xff)
	}

	for _, compressionOpts := range []CompressionOptions{
		{Mode: CompressionNone},
		{Mode: CompressionLZ4},
		{Mode: CompressionLZ4HC},
		{Mode: CompressionLZ4HC, HCLevel: 9},
	} {
		compressionOpts := compressionOpts
		t.Run(compressionOpts.Mode.String(), func(t *testing.T) {
			compression, err := normalizeCompressionOptions(compressionOpts, true)
			if err != nil {
				t.Fatalf("normalizeCompressionOptions: %v", err)
			}

			var compressor blockCompressor
			block, _, err := compressor.compressBlock(nil, data, compression)
			if err != nil {
				t.Fatalf("compressBlockWithOptions: %v", err)
			}

			var decompressor blockDecompressor
			out, err := decompressor.decompressBlock(nil, block, len(data))
			if err != nil {
				t.Fatalf("decompressBlock: %v", err)
			}

			if !bytes.Equal(out, data) {
				t.Fatalf("round-trip mismatch")
			}
		})
	}
}

func TestCompressInvalidOptions(t *testing.T) {
	t.Parallel()

	tests := []CompressionOptions{
		{Mode: CompressionMode(999)},
		{Mode: CompressionLZ4, HCLevel: 1},
		{Mode: CompressionLZ4HC, HCLevel: 10},
		{Mode: CompressionLZ4, MinRatio: -1},
		{Mode: CompressionLZ4, ChunkSize: ChunkSize + 1},
	}

	for _, compressionOpts := range tests {
		_, err := normalizeCompressionOptions(compressionOpts, true)
		if !errors.Is(err, ErrInvalidCompressionOptions) {
			t.Fatalf("expected ErrInvalidCompressionOptions for %+v, got %v", compressionOpts, err)
		}
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

func TestStreamEncodeDecode(t *testing.T) {
	t.Parallel()

	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.NRGBA{
				R: uint8(x * 20), //nolint:gosec // bounded
				G: uint8(y * 20), //nolint:gosec // bounded
				B: 80,
				A: 255,
			})
		}
	}

	var buf bytes.Buffer
	if err := EncodeWithOptions(&buf, img, &WriteOptions{
		Format:     bcn.FormatBGRA8,
		MaxMipMaps: 1,
		Compression: CompressionOptions{
			Mode: CompressionNone,
		},
	}); err != nil {
		t.Fatalf("EncodeWithOptions: %v", err)
	}

	got, err := Decode(bytes.NewBuffer(buf.Bytes()))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	gotNRGBA, ok := got.(*image.NRGBA)
	if !ok {
		t.Fatalf("expected *image.NRGBA, got %T", got)
	}
	if !bytes.Equal(gotNRGBA.Pix, img.Pix) {
		t.Fatalf("stream round-trip pixel mismatch")
	}
}

func TestDecodeNonSeekableStream(t *testing.T) {
	t.Parallel()

	img := benchImage(128, 128)
	var buf bytes.Buffer
	if err := EncodeWithOptions(&buf, img, &WriteOptions{
		Format:      bcn.FormatBGRA8,
		MaxMipMaps:  1,
		Compression: CompressionOptions{Mode: CompressionNone},
	}); err != nil {
		t.Fatalf("EncodeWithOptions: %v", err)
	}

	got, err := Decode(&maxReadRequestReader{r: bytes.NewReader(buf.Bytes()), max: 64 * 1024})
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	gotNRGBA, ok := got.(*image.NRGBA)
	if !ok {
		t.Fatalf("expected *image.NRGBA, got %T", got)
	}
	if !bytes.Equal(gotNRGBA.Pix, img.Pix) {
		t.Fatalf("non-seekable stream round-trip pixel mismatch")
	}
}

func TestCurrentBlockTableErrorsDoNotFallbackToLegacy(t *testing.T) {
	t.Parallel()

	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	var buf bytes.Buffer
	if err := EncodeWithOptions(&buf, img, &WriteOptions{
		Format:      bcn.FormatBGRA8,
		MaxMipMaps:  1,
		Compression: CompressionOptions{Mode: CompressionNone},
	}); err != nil {
		t.Fatalf("EncodeWithOptions: %v", err)
	}

	data := buf.Bytes()
	blockSizeOffset := 4 + bcn.DDSHeaderSize + 4
	binary.LittleEndian.PutUint32(data[blockSizeOffset:blockSizeOffset+4], 0)

	assertCurrentBlockTableError := func(t *testing.T, err error) {
		t.Helper()
		if !errors.Is(err, ErrDecompressBlock) {
			t.Fatalf("error = %v, want ErrDecompressBlock", err)
		}
		if errors.Is(err, ErrParseSingleBlock) {
			t.Fatalf("error unexpectedly fell back to legacy parser: %v", err)
		}
	}

	_, err := Decode(bytes.NewReader(data))
	assertCurrentBlockTableError(t, err)

	path := filepath.Join(t.TempDir(), "corrupt.edds")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	_, err = Read(path)
	assertCurrentBlockTableError(t, err)
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

func TestEncodeFromBlocksWithCompression(t *testing.T) {
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

	payload, _, _, err := bcn.EncodeImageWithOptions(img, bcn.FormatBGRA8, nil)
	if err != nil {
		t.Fatalf("EncodeImageWithOptions: %v", err)
	}

	var buf bytes.Buffer
	if err := EncodeFromBlocksWithCompression(&buf, bcn.FormatBGRA8, 8, 8, [][]byte{payload}, false); err != nil {
		t.Fatalf("EncodeFromBlocksWithCompression: %v", err)
	}

	got, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	gotNRGBA, ok := got.(*image.NRGBA)
	if !ok {
		t.Fatalf("expected *image.NRGBA, got %T", got)
	}
	if !bytes.Equal(gotNRGBA.Pix, img.Pix) {
		t.Fatalf("stream blocks round-trip pixel mismatch")
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
			name: "dxgi-bc7",
			dx10: &bcn.DDSHeaderDX10{DXGIFormat: 98},
			want: bcn.FormatBC7,
		},
		{
			name: "dxgi-bc4s",
			dx10: &bcn.DDSHeaderDX10{DXGIFormat: 81},
			want: bcn.FormatBC4S,
		},
		{
			name: "dxgi-bc5s",
			dx10: &bcn.DDSHeaderDX10{DXGIFormat: 84},
			want: bcn.FormatBC5S,
		},
		{
			name: "fourcc-bc4s",
			header: &bcn.DDSHeader{
				PixelFormat: bcn.DDSPixelFormat{
					Flags:  bcn.DDSPFFourCC,
					FourCC: makeFourCC('B', 'C', '4', 'S'),
				},
			},
			want: bcn.FormatBC4S,
		},
		{
			name: "fourcc-bc5s",
			header: &bcn.DDSHeader{
				PixelFormat: bcn.DDSPixelFormat{
					Flags:  bcn.DDSPFFourCC,
					FourCC: makeFourCC('B', 'C', '5', 'S'),
				},
			},
			want: bcn.FormatBC5S,
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

			got := detectFormat(tc.header, tc.dx10)
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
		{name: "bc4s-5x7", format: bcn.FormatBC4S, w: 5, h: 7, want: 32},
		{name: "bc5s-5x7", format: bcn.FormatBC5S, w: 5, h: 7, want: 64},
		{name: "bc7-5x7", format: bcn.FormatBC7, w: 5, h: 7, want: 64},
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

func TestCalculateMipMapCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		width, height int
		want          int
	}{
		{name: "1x1", width: 1, height: 1, want: 1},
		{name: "1024x1024", width: 1024, height: 1024, want: 11},
		{name: "8192x8192", width: 8192, height: 8192, want: 14},
		{name: "16384x16384", width: 16384, height: 16384, want: 15},
		{name: "16384x1", width: 16384, height: 1, want: 15},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := calculateMipMapCount(tc.width, tc.height)
			if err != nil {
				t.Fatalf("calculateMipMapCount: %v", err)
			}
			if got != tc.want {
				t.Fatalf("calculateMipMapCount(%d, %d) = %d, want %d", tc.width, tc.height, got, tc.want)
			}
		})
	}
}

func TestReadLimits(t *testing.T) {
	t.Parallel()

	defaults, err := normalizeReadLimits(nil)
	if err != nil {
		t.Fatalf("normalizeReadLimits: %v", err)
	}
	wantInputBytes := min(int64(2<<30), int64(maxInt-1))
	if defaults.maxBlockBytes != 1<<30 || defaults.maxDecodedBytes != 1<<30 || defaults.maxImageBytes != 1<<30 || defaults.maxInputBytes != wantInputBytes {
		t.Fatalf("unexpected defaults: %+v", defaults)
	}

	if _, err := normalizeReadLimits(&ReadOptions{MaxBlockBytes: -1}); !errors.Is(err, ErrInvalidReadOptions) {
		t.Fatalf("negative limit error = %v, want ErrInvalidReadOptions", err)
	}
	if _, err := normalizeReadLimits(&ReadOptions{MaxMipMaps: defaultMaxReadMipMaps + 1}); !errors.Is(err, ErrInvalidReadOptions) {
		t.Fatalf("mipmap option error = %v, want ErrInvalidReadOptions", err)
	}

	header := &bcn.DDSHeader{Caps: bcn.DDSCapsMipmap, MipMapCount: defaultMaxReadMipMaps + 1}
	if _, err := readMipMapCount(header, defaults); !errors.Is(err, ErrReadLimitExceeded) {
		t.Fatalf("mipmap limit error = %v, want ErrReadLimitExceeded", err)
	}

	if _, err := expectedReadDataLength(bcn.FormatRGBA8, 16*1024, 16*1024, defaults); err != nil {
		t.Fatalf("16K RGBA default limit: %v", err)
	}
	if _, err := expectedReadDataLength(bcn.FormatRGBA8, 4, 4, readLimits{maxDecodedBytes: 1}); !errors.Is(err, ErrReadLimitExceeded) {
		t.Fatalf("decoded size limit error = %v, want ErrReadLimitExceeded", err)
	}
	if _, err := expectedReadDataLength(bcn.FormatDXT1, 4, 4, readLimits{maxDecodedBytes: 1024, maxImageBytes: 1}); !errors.Is(err, ErrReadLimitExceeded) {
		t.Fatalf("image size limit error = %v, want ErrReadLimitExceeded", err)
	}
	if _, err := expectedDataLengthChecked(bcn.FormatBC7, int(^uint32(0)), int(^uint32(0))); !errors.Is(err, ErrSizeOverflow) {
		t.Fatalf("overflow error = %v, want ErrSizeOverflow", err)
	}

	var blockTable bytes.Buffer
	_, _ = blockTable.WriteString(BlockMagicCOPY)
	if err := binary.Write(&blockTable, binary.LittleEndian, int32(1025)); err != nil {
		t.Fatalf("write block size: %v", err)
	}
	_, _, _, err = readLargestMipFromBlocks(
		bytes.NewReader(blockTable.Bytes()),
		&bcn.DDSHeader{Width: 4, Height: 4},
		bcn.FormatDXT1,
		1,
		readLimits{maxMipMaps: 1, maxBlockBytes: 1024, maxDecodedBytes: 1024},
	)
	if !errors.Is(err, ErrReadLimitExceeded) {
		t.Fatalf("block size limit error = %v, want ErrReadLimitExceeded", err)
	}

	if _, err := readAllWithLimit(bytes.NewBuffer(make([]byte, 17)), 16); !errors.Is(err, ErrReadLimitExceeded) {
		t.Fatalf("stream input limit error = %v, want ErrReadLimitExceeded", err)
	}
}

func TestReadBlockTableErrors(t *testing.T) {
	t.Parallel()

	t.Run("unknown-magic", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		_, _ = buf.WriteString("ABCD")
		_ = binary.Write(&buf, binary.LittleEndian, int32(8))

		_, err := readBlockTableInto(nil, bytes.NewReader(buf.Bytes()), 1)
		if !errors.Is(err, ErrBlockTableUnknownMagic) {
			t.Fatalf("expected ErrBlockTableUnknownMagic, got %v", err)
		}
	})

	t.Run("negative-size", func(t *testing.T) {
		t.Parallel()

		var buf bytes.Buffer
		_, _ = buf.WriteString(BlockMagicCOPY)
		_ = binary.Write(&buf, binary.LittleEndian, int32(-1))

		_, err := readBlockTableInto(nil, bytes.NewReader(buf.Bytes()), 1)
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
