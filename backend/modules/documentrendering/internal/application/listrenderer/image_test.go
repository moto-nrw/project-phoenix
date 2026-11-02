package listexport

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"testing"
)

func TestPDFImageKeepsCompatibleImagesUnchanged(t *testing.T) {
	t.Parallel()
	// The ancillary chunk would disappear if this valid PNG were re-encoded.
	compatible, err := os.ReadFile("../../../../../testdata/compatible-logo.png")
	if err != nil {
		t.Fatal(err)
	}
	var jpegData bytes.Buffer
	if err := jpeg.Encode(&jpegData, image.NewRGBA(image.Rect(0, 0, 2, 2)), nil); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{"PNG with metadata": compatible, "moto": motoLogoPNG, "JPEG": jpegData.Bytes()} {
		t.Run(name, func(t *testing.T) {
			holder, err := pdfImage(data)
			if err != nil {
				t.Fatal(err)
			}
			got, err := io.ReadAll(holder)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, data) {
				t.Fatal("compatible image was re-encoded")
			}
		})
	}
}

func BenchmarkPDFImageCompatible(b *testing.B) {
	img := image.NewNRGBA(image.Rect(0, 0, 2000, 2000))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 33, 122, 120, 96
	}
	var large bytes.Buffer
	if err := png.Encode(&large, img); err != nil {
		b.Fatal(err)
	}
	for name, data := range map[string][]byte{"moto": motoLogoPNG, "2000x2000": large.Bytes()} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := pdfImage(data); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
