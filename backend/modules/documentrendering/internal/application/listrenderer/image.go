package listexport

import (
	"bytes"
	"fmt"
	"image"
	"image/png"

	"github.com/signintech/gopdf"
)

// pdfImage only normalizes PNG variants that gopdf cannot draw. Compatible
// PNGs (including the moto graphic) and JPEGs retain their original bytes;
// do not decode and re-encode them for every letter of a billing run.
func pdfImage(data []byte) (gopdf.ImageHolder, error) {
	if !pngNeedsNormalization(data) {
		return gopdf.ImageHolderByBytes(data)
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	var normalized bytes.Buffer
	if err := png.Encode(&normalized, image8Bit(decoded)); err != nil {
		return nil, fmt.Errorf("normalize image: %w", err)
	}
	return gopdf.ImageHolderByBytes(normalized.Bytes())
}

// The first PNG chunk is the 13-byte IHDR. Its bit depth and interlace
// method are at byte offsets 24 and 28. gopdf refuses 16-bit and Adam7 PNGs.
// Callers keep their existing image validation; gopdf parses at draw time.
func pngNeedsNormalization(data []byte) bool {
	const header = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"
	return len(data) >= 33 && bytes.HasPrefix(data, []byte(header)) && (data[24] > 8 || data[28] != 0)
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
