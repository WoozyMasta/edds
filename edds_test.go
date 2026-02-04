package edds

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
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
