package auditlog

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func encodePNG(t *testing.T, img image.Image) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func inkedImage(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, h/2, color.NRGBA{R: 20, G: 20, B: 20, A: 255})
	}
	return img
}

func TestParseSignatureImageAcceptsADrawnAndATypedSignature(t *testing.T) {
	t.Parallel()
	drawn, err := ParseSignatureImage(SignatureMethodDrawn, encodePNG(t, inkedImage(600, 200)), "ignored")
	require.NoError(t, err)
	require.Equal(t, SignatureMethodDrawn, drawn.Method)
	require.Empty(t, drawn.TypedName, "a drawn signature keeps no typed name")

	typed, err := ParseSignatureImage(SignatureMethodTyped, encodePNG(t, inkedImage(600, 200)), "  Klaus Richter ")
	require.NoError(t, err)
	require.Equal(t, "Klaus Richter", typed.TypedName)
}

func TestParseSignatureImageRefusesWhatIsNoSignature(t *testing.T) {
	t.Parallel()
	blank := image.NewNRGBA(image.Rect(0, 0, 600, 200))
	white := image.NewNRGBA(image.Rect(0, 0, 600, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 600; x++ {
			white.Set(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	cases := map[string]struct{ method, data, typed string }{
		"unknown method":     {"stamp", encodePNG(t, inkedImage(100, 50)), ""},
		"not base64":         {SignatureMethodDrawn, "%%%", ""},
		"not a png":          {SignatureMethodDrawn, base64.StdEncoding.EncodeToString([]byte("GIF89a not a png")), ""},
		"empty canvas":       {SignatureMethodDrawn, encodePNG(t, blank), ""},
		"white canvas":       {SignatureMethodDrawn, encodePNG(t, white), ""},
		"too wide":           {SignatureMethodDrawn, encodePNG(t, inkedImage(SignatureImageMaxWidth+1, 100)), ""},
		"too large":          {SignatureMethodDrawn, strings.Repeat("A", SignatureImageMaxBytes*2), ""},
		"typed without name": {SignatureMethodTyped, encodePNG(t, inkedImage(100, 50)), " "},
	}
	for name, c := range cases {
		_, err := ParseSignatureImage(c.method, c.data, c.typed)
		require.ErrorIs(t, err, ErrSignatureInvalid, name)
	}
}
