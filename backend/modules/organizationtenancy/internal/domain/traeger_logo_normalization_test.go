package domain

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"os"
	"testing"
)

func TestCheckLogoKeepsCompatibleOptimizedPNGAtUploadLimit(t *testing.T) {
	t.Parallel()
	const limit = 1 << 20
	// An optimized indexed PNG just below the real 1 MiB limit grows past
	// that limit with the default encoder. Generate it instead of committing
	// a megabyte-sized binary fixture.
	palette := color.Palette{}
	for i := 0; i < 8; i++ {
		palette = append(palette, color.Gray{Y: uint8(i * 255 / 7)})
	}
	img := image.NewPaletted(image.Rect(0, 0, 2000, 1390), palette)
	rng := rand.New(rand.NewSource(5236)) // deterministic non-secret image pixels
	for i := range img.Pix {
		img.Pix[i] = byte(rng.Intn(len(palette)))
	}
	var original bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoder.Encode(&original, img); err != nil {
		t.Fatal(err)
	}
	if original.Len() >= limit || original.Len() < limit-4096 {
		t.Fatalf("fixture must be just below 1 MiB, got %d", original.Len())
	}
	decoded, err := png.Decode(bytes.NewReader(original.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	var recoded bytes.Buffer
	if err := png.Encode(&recoded, decoded); err != nil {
		t.Fatal(err)
	}
	if recoded.Len() <= limit {
		t.Fatalf("fixture must grow past 1 MiB on re-encoding, got %d", recoded.Len())
	}
	checked, ok := CheckLogo(original.Bytes(), limit, 2000)
	if !ok {
		t.Fatal("previously valid optimized PNG refused")
	}
	if !bytes.Equal(checked.Data, original.Bytes()) {
		t.Fatal("compatible PNG must retain its original bytes")
	}
	if checked.ContentType != "image/png" || checked.Width != 2000 || checked.Height != 1390 {
		t.Fatalf("wrong metadata: %+v", checked)
	}
}

func TestCheckLogoKeepsValidOriginalWhenNormalizationExceedsLimit(t *testing.T) {
	t.Parallel()
	// Use a small configurable boundary for the incompatible-image fallback;
	// the optimized-image test above exercises the real upload limit as well.
	original, err := os.ReadFile("../../../../testdata/interlaced-small-logo.png")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := png.Decode(bytes.NewReader(original))
	if err != nil {
		t.Fatal(err)
	}
	var recoded bytes.Buffer
	if err := png.Encode(&recoded, decoded); err != nil {
		t.Fatal(err)
	}
	if recoded.Len() <= len(original) {
		t.Fatal("fixture must grow on normalization")
	}
	checked, ok := CheckLogo(original, len(original), 2000)
	if !ok {
		t.Fatal("valid interlaced PNG refused when its normalization exceeded the byte limit")
	}
	if !bytes.Equal(checked.Data, original) {
		t.Fatal("oversized normalization must fall back to the valid original")
	}
	if checked.ContentType != "image/png" || checked.Width != 1 || checked.Height != 1 {
		t.Fatalf("wrong metadata: %+v", checked)
	}
}
