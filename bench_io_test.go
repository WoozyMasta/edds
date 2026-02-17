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

// benchMainFlowWriteOptions defines a representative write configuration.
func benchMainFlowWriteOptions() *WriteOptions {
	return &WriteOptions{
		Format:     bcn.FormatDXT5,
		MaxMipMaps: 0,
		Compress:   true,
		EncodeOptions: &bcn.EncodeOptions{
			QualityLevel: 8,
		},
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

func BenchmarkMainFlowWrite(b *testing.B) {
	img := benchMainFlowImage(1024, 1024)
	path := filepath.Join(b.TempDir(), "main_flow_write.edds")
	opts := benchMainFlowWriteOptions()

	b.ReportAllocs()
	b.SetBytes(int64(len(img.Pix)))
	b.ResetTimer()

	for b.Loop() {
		if err := WriteWithOptions(img, path, opts); err != nil {
			b.Fatalf("write: %v", err)
		}
	}
}

func BenchmarkMainFlowRead(b *testing.B) {
	img := benchMainFlowImage(1024, 1024)
	opts := benchMainFlowWriteOptions()
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
