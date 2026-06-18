package edds

import (
	"image"
	"image/color"
	"path/filepath"
	"testing"

	"github.com/woozymasta/bcn"
)

// benchMainFlowImage builds a deterministic image used by IO benchmarks.
func benchMainFlowImage(width, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Deterministic pattern with mixed low/high frequencies.
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

// benchMainFlowWriteOptionsDXT5 defines a representative DXT5 write configuration.
func benchMainFlowWriteOptionsDXT5() *WriteOptions {
	return &WriteOptions{
		Format:     bcn.FormatDXT5,
		MaxMipMaps: 0,
		Compress:   true,
		EncodeOptions: &bcn.EncodeOptions{
			QualityLevel: bcn.QualityLevelFast,
		},
	}
}

// benchMainFlowWriteOptionsBGRA8 defines a representative BGRA8 write configuration.
func benchMainFlowWriteOptionsBGRA8() *WriteOptions {
	return &WriteOptions{
		Format:     bcn.FormatBGRA8,
		MaxMipMaps: 0,
		Compress:   true,
	}
}

// benchMainFlowInputPath prepares a benchmark EDDS file for read benchmarks.
func benchMainFlowInputPath(b *testing.B, img image.Image, opts *WriteOptions) string {
	b.Helper()

	path := filepath.Join(b.TempDir(), "main_flow_input.edds")
	if err := WriteWithOptions(img, path, opts); err != nil {
		b.Fatalf("prepare input file: %v", err)
	}

	return path
}

// benchMainFlowPayloads pre-encodes mip payloads used by container-only benchmarks.
func benchMainFlowPayloads(
	b *testing.B,
	img image.Image,
	format bcn.Format,
	maxMipMaps int,
	encOpts *bcn.EncodeOptions,
) [][]byte {
	b.Helper()

	mips := bcn.GenerateMipmaps(img, false)
	if maxMipMaps > 0 && len(mips) > maxMipMaps {
		mips = mips[:maxMipMaps]
	}

	payloads := make([][]byte, len(mips))
	for i, mip := range mips {
		data, _, _, err := bcn.EncodeImageWithOptions(mip, format, encOpts)
		if err != nil {
			b.Fatalf("prepare payloads (mipmap %d): %v", i, err)
		}

		payloads[i] = data
	}

	return payloads
}

// benchPayloadBytes computes total payload bytes for throughput reporting.
func benchPayloadBytes(payloads [][]byte) int64 {
	var total int64
	for _, p := range payloads {
		total += int64(len(p))
	}

	return total
}

func benchCompressionOptions() []CompressionOptions {
	return []CompressionOptions{
		{Mode: CompressionNone},
		{Mode: CompressionLZ4},
		{Mode: CompressionLZ4HC},
	}
}

func benchCompressedPayloadBytes(b *testing.B, payloads [][]byte, compressionOpts CompressionOptions) int64 {
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

func benchReportCompressionRatio(b *testing.B, rawBytes, storedBytes int64) {
	b.Helper()

	if storedBytes <= 0 {
		b.Fatal("compressed payload is empty")
	}
	b.ReportMetric(float64(rawBytes)/float64(storedBytes), "raw/stored")
}

// benchImageBytes computes total source image bytes for throughput reporting.
func benchImageBytes(mips []*image.NRGBA) int64 {
	var total int64
	for _, mip := range mips {
		total += int64(len(mip.Pix))
	}

	return total
}

func BenchmarkStageGenerateMipmaps(b *testing.B) {
	img := benchMainFlowImage(1024, 1024)

	b.ReportAllocs()
	b.SetBytes(int64(len(img.Pix)))
	b.ResetTimer()

	for b.Loop() {
		mips := bcn.GenerateMipmaps(img, false)
		if len(mips) == 0 {
			b.Fatal("generate mipmaps: empty result")
		}
	}
}

func BenchmarkStageEncodeMipChainDXT5(b *testing.B) {
	img := benchMainFlowImage(1024, 1024)
	opts := benchMainFlowWriteOptionsDXT5()
	mips := bcn.GenerateMipmaps(img, false)

	if opts.MaxMipMaps > 0 && len(mips) > opts.MaxMipMaps {
		mips = mips[:opts.MaxMipMaps]
	}

	b.ReportAllocs()
	b.SetBytes(benchImageBytes(mips))
	b.ResetTimer()

	for b.Loop() {
		for i, mip := range mips {
			if _, _, _, err := bcn.EncodeImageWithOptions(mip, opts.Format, opts.EncodeOptions); err != nil {
				b.Fatalf("encode mipmap %d: %v", i, err)
			}
		}
	}
}

func BenchmarkStageCompressBlocksDXT5(b *testing.B) {
	img := benchMainFlowImage(1024, 1024)
	opts := benchMainFlowWriteOptionsDXT5()
	payloads := benchMainFlowPayloads(b, img, opts.Format, opts.MaxMipMaps, opts.EncodeOptions)
	payloadBytes := benchPayloadBytes(payloads)

	for _, compressionOpts := range benchCompressionOptions() {
		compressionOpts := compressionOpts
		b.Run(compressionOpts.Mode.String(), func(b *testing.B) {
			compression, err := normalizeCompressionOptions(compressionOpts, true)
			if err != nil {
				b.Fatalf("normalize compression: %v", err)
			}
			storedBytes := benchCompressedPayloadBytes(b, payloads, compressionOpts)

			b.ReportAllocs()
			b.SetBytes(payloadBytes)
			b.ResetTimer()

			for b.Loop() {
				for i, payload := range payloads {
					if _, err := compressBlockWithOptions(payload, compression); err != nil {
						b.Fatalf("compress mipmap %d with %s: %v", i, compressionOpts.Mode, err)
					}
				}
			}
			benchReportCompressionRatio(b, payloadBytes, storedBytes)
		})
	}
}

func BenchmarkMainFlowWriteDXT5(b *testing.B) {
	img := benchMainFlowImage(1024, 1024)
	path := filepath.Join(b.TempDir(), "main_flow_write_dxt5.edds")
	opts := benchMainFlowWriteOptionsDXT5()

	b.ReportAllocs()
	b.SetBytes(int64(len(img.Pix)))
	b.ResetTimer()

	for b.Loop() {
		if err := WriteWithOptions(img, path, opts); err != nil {
			b.Fatalf("write: %v", err)
		}
	}
}

func BenchmarkMainFlowWriteDXT5ByCompression(b *testing.B) {
	img := benchMainFlowImage(1024, 1024)
	opts := benchMainFlowWriteOptionsDXT5()
	payloads := benchMainFlowPayloads(b, img, opts.Format, opts.MaxMipMaps, opts.EncodeOptions)
	payloadBytes := benchPayloadBytes(payloads)

	for _, compressionOpts := range benchCompressionOptions() {
		compressionOpts := compressionOpts
		b.Run(compressionOpts.Mode.String(), func(b *testing.B) {
			path := filepath.Join(b.TempDir(), "main_flow_write_dxt5.edds")
			writeOpts := *opts
			writeOpts.Compress = compressionOpts.Mode != CompressionNone
			writeOpts.Compression = compressionOpts

			storedBytes := benchCompressedPayloadBytes(b, payloads, compressionOpts)

			b.ReportAllocs()
			b.SetBytes(int64(len(img.Pix)))
			b.ResetTimer()

			for b.Loop() {
				if err := WriteWithOptions(img, path, &writeOpts); err != nil {
					b.Fatalf("write with %s: %v", compressionOpts.Mode, err)
				}
			}
			benchReportCompressionRatio(b, payloadBytes, storedBytes)
		})
	}
}

func BenchmarkMainFlowWriteBGRA8(b *testing.B) {
	img := benchMainFlowImage(1024, 1024)
	path := filepath.Join(b.TempDir(), "main_flow_write_bgra8.edds")
	opts := benchMainFlowWriteOptionsBGRA8()

	b.ReportAllocs()
	b.SetBytes(int64(len(img.Pix)))
	b.ResetTimer()

	for b.Loop() {
		if err := WriteWithOptions(img, path, opts); err != nil {
			b.Fatalf("write: %v", err)
		}
	}
}

func BenchmarkContainerWriteFromBlocksDXT5(b *testing.B) {
	img := benchMainFlowImage(1024, 1024)
	opts := benchMainFlowWriteOptionsDXT5()
	payloads := benchMainFlowPayloads(b, img, opts.Format, opts.MaxMipMaps, opts.EncodeOptions)
	payloadBytes := benchPayloadBytes(payloads)
	path := filepath.Join(b.TempDir(), "container_write.edds")
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()

	for _, compressionOpts := range benchCompressionOptions() {
		compressionOpts := compressionOpts
		b.Run(compressionOpts.Mode.String(), func(b *testing.B) {
			storedBytes := benchCompressedPayloadBytes(b, payloads, compressionOpts)

			b.ReportAllocs()
			b.SetBytes(payloadBytes)
			b.ResetTimer()

			for b.Loop() {
				if err := WriteFromBlocksWithCompressionOptions(path, opts.Format, width, height, payloads, compressionOpts); err != nil {
					b.Fatalf("write from blocks (%s): %v", compressionOpts.Mode, err)
				}
			}
			benchReportCompressionRatio(b, payloadBytes, storedBytes)
		})
	}
}

func BenchmarkMainFlowReadDXT5(b *testing.B) {
	img := benchMainFlowImage(1024, 1024)
	opts := benchMainFlowWriteOptionsDXT5()
	path := benchMainFlowInputPath(b, img, opts)

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
	img := benchMainFlowImage(1024, 1024)
	opts := benchMainFlowWriteOptionsBGRA8()
	path := benchMainFlowInputPath(b, img, opts)

	b.ReportAllocs()
	b.SetBytes(int64(len(img.Pix)))
	b.ResetTimer()

	for b.Loop() {
		if _, err := Read(path); err != nil {
			b.Fatalf("read: %v", err)
		}
	}
}
