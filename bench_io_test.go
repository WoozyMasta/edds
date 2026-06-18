package edds

import (
	"image"
	"image/color"
	"path/filepath"
	"testing"

	"github.com/woozymasta/bcn"
)

const (
	benchImageWidth  = 1024
	benchImageHeight = 1024
)

type benchCompressionCase struct {
	name string
	opts CompressionOptions
}

// benchImage builds a deterministic image used by IO benchmarks.
func benchImage(width, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.NRGBA{
				R: uint8((x*7 + y*3) & 0xff),        //nolint:gosec // bounded by mask
				G: uint8((x*13 + y*5) & 0xff),       //nolint:gosec // bounded by mask
				B: uint8((x ^ y ^ (x >> 2)) & 0xff), //nolint:gosec // bounded by mask
				A: 255,
			})
		}
	}

	return img
}

// benchWriteOptionsDXT5 returns the representative DXT5 write configuration.
func benchWriteOptionsDXT5() WriteOptions {
	return WriteOptions{
		Format:      bcn.FormatDXT5,
		MaxMipMaps:  0,
		Compression: CompressionOptions{Mode: CompressionLZ4},
		EncodeOptions: &bcn.EncodeOptions{
			QualityLevel: bcn.QualityLevelFast,
		},
	}
}

// benchWriteOptionsBGRA8 returns the representative BGRA8 write configuration.
func benchWriteOptionsBGRA8() WriteOptions {
	return WriteOptions{
		Format:      bcn.FormatBGRA8,
		MaxMipMaps:  0,
		Compression: CompressionOptions{Mode: CompressionLZ4},
	}
}

// benchCompressionCases returns the compression matrix used by write benchmarks.
func benchCompressionCases() []benchCompressionCase {
	return []benchCompressionCase{
		{name: CompressionNone.String(), opts: CompressionOptions{Mode: CompressionNone}},
		{name: CompressionLZ4.String(), opts: CompressionOptions{Mode: CompressionLZ4}},
		{name: CompressionLZ4HC.String(), opts: CompressionOptions{Mode: CompressionLZ4HC}},
	}
}

// benchInputPath writes a benchmark EDDS file for read benchmarks.
func benchInputPath(b *testing.B, img image.Image, opts WriteOptions) string {
	b.Helper()

	path := filepath.Join(b.TempDir(), "input.edds")
	if err := WriteWithOptions(img, path, &opts); err != nil {
		b.Fatalf("prepare input file: %v", err)
	}

	return path
}

// benchMipmaps generates mipmaps with the same bcn API used by the writer.
func benchMipmaps(b *testing.B, img image.Image, maxMipMaps int) []*image.NRGBA {
	b.Helper()

	mips := bcn.GenerateMipmapsN(img, maxMipMaps, false)
	if len(mips) == 0 {
		b.Fatal("generate mipmaps: empty result")
	}

	return mips
}

// benchPayloads encodes all mip levels into the requested texture format.
func benchPayloads(b *testing.B, mips []*image.NRGBA, format bcn.Format, encOpts *bcn.EncodeOptions) [][]byte {
	b.Helper()

	payloads := make([][]byte, len(mips))
	for i, mip := range mips {
		data, _, _, err := bcn.EncodeImageWithOptions(mip, format, encOpts)
		if err != nil {
			b.Fatalf("encode mipmap %d: %v", i, err)
		}
		payloads[i] = data
	}

	return payloads
}

// benchPayloadBytes computes total encoded payload bytes for throughput reporting.
func benchPayloadBytes(payloads [][]byte) int64 {
	var total int64
	for _, p := range payloads {
		total += int64(len(p))
	}

	return total
}

// benchImageBytes computes total source image bytes for throughput reporting.
func benchImageBytes(mips []*image.NRGBA) int64 {
	var total int64
	for _, mip := range mips {
		total += int64(len(mip.Pix))
	}

	return total
}

// benchStoredPayloadBytes computes total EDDS block table size for one compression mode.
func benchStoredPayloadBytes(b *testing.B, payloads [][]byte, compressionOpts CompressionOptions) int64 {
	b.Helper()

	compression, err := normalizeCompressionOptions(compressionOpts, true)
	if err != nil {
		b.Fatalf("normalize compression %s: %v", compressionOpts.Mode, err)
	}

	var total int64
	for i, payload := range payloads {
		block, err := compressBlockWithOptions(payload, compression)
		if err != nil {
			b.Fatalf("compress mipmap %d with %s: %v", i, compressionOpts.Mode, err)
		}
		total += int64(block.Size)
	}

	return total
}

// benchReportCompressionRatio reports raw/stored payload ratio for matrix benchmarks.
func benchReportCompressionRatio(b *testing.B, rawBytes, storedBytes int64) {
	b.Helper()

	if storedBytes <= 0 {
		b.Fatal("stored payload is empty")
	}
	b.ReportMetric(float64(rawBytes)/float64(storedBytes), "raw/stored")
}

func BenchmarkStageGenerateMipmaps(b *testing.B) {
	img := benchImage(benchImageWidth, benchImageHeight)

	b.ReportAllocs()
	b.SetBytes(int64(len(img.Pix)))
	b.ResetTimer()

	for b.Loop() {
		benchMipmaps(b, img, 0)
	}
}

func BenchmarkStageGenerateMipmapsByLimit(b *testing.B) {
	img := benchImage(benchImageWidth, benchImageHeight)

	for _, tc := range []struct {
		name       string
		maxMipMaps int
	}{
		{name: "Max4", maxMipMaps: 4},
		{name: "Max1", maxMipMaps: 1},
	} {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(len(img.Pix)))
			b.ResetTimer()

			for b.Loop() {
				benchMipmaps(b, img, tc.maxMipMaps)
			}
		})
	}
}

func BenchmarkStageEncodeMipChainDXT5(b *testing.B) {
	img := benchImage(benchImageWidth, benchImageHeight)
	opts := benchWriteOptionsDXT5()
	mips := benchMipmaps(b, img, opts.MaxMipMaps)

	b.ReportAllocs()
	b.SetBytes(benchImageBytes(mips))
	b.ResetTimer()

	for b.Loop() {
		benchPayloads(b, mips, opts.Format, opts.EncodeOptions)
	}
}

func BenchmarkStageCompressBlocksDXT5(b *testing.B) {
	img := benchImage(benchImageWidth, benchImageHeight)
	opts := benchWriteOptionsDXT5()
	mips := benchMipmaps(b, img, opts.MaxMipMaps)
	payloads := benchPayloads(b, mips, opts.Format, opts.EncodeOptions)
	payloadBytes := benchPayloadBytes(payloads)

	for _, tc := range benchCompressionCases() {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			compression, err := normalizeCompressionOptions(tc.opts, true)
			if err != nil {
				b.Fatalf("normalize compression %s: %v", tc.name, err)
			}
			storedBytes := benchStoredPayloadBytes(b, payloads, tc.opts)

			b.ReportAllocs()
			b.SetBytes(payloadBytes)
			b.ResetTimer()

			for b.Loop() {
				for i, payload := range payloads {
					if _, err := compressBlockWithOptions(payload, compression); err != nil {
						b.Fatalf("compress mipmap %d with %s: %v", i, tc.name, err)
					}
				}
			}
			benchReportCompressionRatio(b, payloadBytes, storedBytes)
		})
	}
}

func BenchmarkContainerWriteFromBlocksDXT5(b *testing.B) {
	img := benchImage(benchImageWidth, benchImageHeight)
	opts := benchWriteOptionsDXT5()
	mips := benchMipmaps(b, img, opts.MaxMipMaps)
	payloads := benchPayloads(b, mips, opts.Format, opts.EncodeOptions)
	payloadBytes := benchPayloadBytes(payloads)
	path := filepath.Join(b.TempDir(), "container_write.edds")
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()

	for _, tc := range benchCompressionCases() {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			storedBytes := benchStoredPayloadBytes(b, payloads, tc.opts)

			b.ReportAllocs()
			b.SetBytes(payloadBytes)
			b.ResetTimer()

			for b.Loop() {
				if err := WriteFromBlocksWithCompressionOptions(path, opts.Format, width, height, payloads, tc.opts); err != nil {
					b.Fatalf("write from blocks with %s: %v", tc.name, err)
				}
			}
			benchReportCompressionRatio(b, payloadBytes, storedBytes)
		})
	}
}

func BenchmarkMainFlowWriteDXT5(b *testing.B) {
	img := benchImage(benchImageWidth, benchImageHeight)
	path := filepath.Join(b.TempDir(), "main_flow_write_dxt5.edds")
	opts := benchWriteOptionsDXT5()

	b.ReportAllocs()
	b.SetBytes(int64(len(img.Pix)))
	b.ResetTimer()

	for b.Loop() {
		if err := WriteWithOptions(img, path, &opts); err != nil {
			b.Fatalf("write: %v", err)
		}
	}
}

func BenchmarkMainFlowWriteBGRA8(b *testing.B) {
	img := benchImage(benchImageWidth, benchImageHeight)
	path := filepath.Join(b.TempDir(), "main_flow_write_bgra8.edds")
	opts := benchWriteOptionsBGRA8()

	b.ReportAllocs()
	b.SetBytes(int64(len(img.Pix)))
	b.ResetTimer()

	for b.Loop() {
		if err := WriteWithOptions(img, path, &opts); err != nil {
			b.Fatalf("write: %v", err)
		}
	}
}

func BenchmarkMainFlowReadDXT5(b *testing.B) {
	img := benchImage(benchImageWidth, benchImageHeight)
	opts := benchWriteOptionsDXT5()
	path := benchInputPath(b, img, opts)

	b.ReportAllocs()
	b.SetBytes(int64(len(img.Pix)))
	b.ResetTimer()

	for b.Loop() {
		if _, err := Read(path); err != nil {
			b.Fatalf("read: %v", err)
		}
	}
}

func BenchmarkMainFlowReadBGRA8(b *testing.B) {
	img := benchImage(benchImageWidth, benchImageHeight)
	opts := benchWriteOptionsBGRA8()
	path := benchInputPath(b, img, opts)

	b.ReportAllocs()
	b.SetBytes(int64(len(img.Pix)))
	b.ResetTimer()

	for b.Loop() {
		if _, err := Read(path); err != nil {
			b.Fatalf("read: %v", err)
		}
	}
}
