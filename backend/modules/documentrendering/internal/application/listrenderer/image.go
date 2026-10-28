package listexport

import (
	"bytes"
	"fmt"
	"image"
	"image/png"

	"github.com/signintech/gopdf"
)

// pdfImage normalizes PNGs before gopdf's deferred image parsing: its
// ImageHolderByBytes accepts Adam7 data, but drawing that holder fails.
// NRGBA reduces 16-bit channels to PDF-compatible 8-bit samples.
// Re-encoding preserves 8-bit pixels and alpha, including for previously stored
// logos (#5236). JPEGs retain their original bytes without lossy recompression.
func pdfImage(data []byte) (gopdf.ImageHolder, error) {
	decoded, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	if format == "png" {
		var normalized bytes.Buffer
		if err := png.Encode(&normalized, image8Bit(decoded)); err != nil {
			return nil, fmt.Errorf("normalize image: %w", err)
		}
		data = normalized.Bytes()
	}
	return gopdf.ImageHolderByBytes(data)
}

// image8Bit only converts PNG types whose 16-bit samples gopdf cannot read.
// Keep 8-bit and palette images intact, including their alpha values.
func image8Bit(source image.Image) image.Image {
	switch source.(type) {
	case *image.Gray16, *image.RGBA64, *image.NRGBA64:
		pixels := image.NewNRGBA(source.Bounds())
		for y := pixels.Bounds().Min.Y; y < pixels.Bounds().Max.Y; y++ {
			for x := pixels.Bounds().Min.X; x < pixels.Bounds().Max.X; x++ {
				pixels.Set(x, y, source.At(x, y))
			}
		}
		return pixels
	default:
		return source
	}
}
