package domain

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"testing"
)

func TestValidIBANChecksLengthCountryAndCheckDigits(t *testing.T) {
	t.Parallel()
	cases := map[string]bool{
		"DE89370400440532013000": true,
		"GB82WEST12345698765432": true,
		"DE89370400440532013001": false, // wrong check digits
		"DE5112345678901":        false, // check digits fit, but too short for Germany
		"XX82WEST12345698765432": false, // no SEPA country
		"DE89370400440532A13000": false, // letter in a German account number
	}
	for iban, want := range cases {
		if got := ValidIBAN(iban); got != want {
			t.Errorf("ValidIBAN(%q) = %v, want %v", iban, got, want)
		}
	}
}

func TestCheckLogoNormalizesInterlacedPNGWithinLimits(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../../../testdata/interlaced-logo.png")
	if err != nil {
		t.Fatal(err)
	}
	checked, ok := CheckLogo(data, 2*1024*1024, 2000)
	if !ok {
		t.Fatal("valid interlaced PNG refused")
	}
	if checked.ContentType != "image/png" || checked.Width != 32 || checked.Height != 16 {
		t.Fatalf("metadata: %+v", checked)
	}
	// The upload's input limit and dimension limit still apply.
	if _, ok := CheckLogo(data, len(data)-1, 2000); ok {
		t.Fatal("oversized input accepted")
	}
	if _, ok := CheckLogo(data, 2*1024*1024, 31); ok {
		t.Fatal("oversized dimensions accepted")
	}
	original, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	stored, err := png.Decode(bytes.NewReader(checked.Data))
	if err != nil {
		t.Fatal(err)
	}
	if checked.Data[28] != 0 {
		t.Fatal("checked logo still interlaced")
	}
	for y := 0; y < 16; y++ {
		for x := 0; x < 32; x++ {
			if original.At(x, y) != stored.At(x, y) {
				t.Fatalf("pixel %d,%d changed", x, y)
			}
		}
	}
}

func TestCheckLogoNormalizes16BitPNGAndKeepsJPEG(t *testing.T) {
	t.Parallel()
	img := image.NewNRGBA64(image.Rect(0, 0, 32, 16))
	img.SetNRGBA64(1, 1, color.NRGBA64{R: 0x2121, G: 0x7a7a, B: 0x7878, A: 0xffff})
	var pngData, jpegData bytes.Buffer
	if err := png.Encode(&pngData, img); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&jpegData, img, nil); err != nil {
		t.Fatal(err)
	}
	checked, ok := CheckLogo(pngData.Bytes(), 1024*1024, 2000)
	if !ok {
		t.Fatal("16-bit PNG refused")
	}
	if checked.Data[24] != 8 || checked.Data[28] != 0 {
		t.Fatal("normalized PNG must be 8-bit and non-interlaced")
	}
	jpegChecked, ok := CheckLogo(jpegData.Bytes(), 1024*1024, 2000)
	if !ok || !bytes.Equal(jpegData.Bytes(), jpegChecked.Data) {
		t.Fatal("JPEG bytes changed")
	}
}
