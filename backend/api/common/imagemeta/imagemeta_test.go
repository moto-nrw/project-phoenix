package imagemeta

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/image/webp"
)

// gpsMarker stands for the GPS coordinates a phone writes into Exif.
const gpsMarker = "GPS-51.2277N-6.7735E"

// exifTIFF builds a little-endian TIFF block with an orientation tag (when
// orientation > 0) and a GPS IFD pointer, followed by a GPS marker string.
func exifTIFF(orientation uint16) []byte {
	order := binary.LittleEndian
	var entries [][]byte
	entry := func(tag, typ uint16, count, value uint32) []byte {
		e := make([]byte, 12)
		order.PutUint16(e[0:2], tag)
		order.PutUint16(e[2:4], typ)
		order.PutUint32(e[4:8], count)
		order.PutUint32(e[8:12], value)
		return e
	}
	if orientation > 0 {
		entries = append(entries, entry(exifOrientationTag, 3, 1, uint32(orientation)))
	}
	gpsOffset := uint32(8 + 2 + 12*(len(entries)+1) + 4)
	entries = append(entries, entry(0x8825, 4, 1, gpsOffset))
	out := []byte{'I', 'I', 42, 0, 8, 0, 0, 0}
	out = order.AppendUint16(out, uint16(len(entries)))
	for _, e := range entries {
		out = append(out, e...)
	}
	out = append(out, 0, 0, 0, 0)
	return append(out, gpsMarker...)
}

func testJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for x := range 4 {
		img.Set(x, 0, color.RGBA{R: 200, A: 255})
		img.Set(x, 1, color.RGBA{B: 200, A: 255})
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, nil))
	return buf.Bytes()
}

// jpegWithMetadata inserts Exif (with GPS), XMP and a comment after SOI and
// appends a secondary image after EOI, like phone cameras do.
func jpegWithMetadata(t *testing.T, orientation uint16) []byte {
	t.Helper()
	plain := testJPEG(t)
	out := []byte{0xFF, 0xD8}
	out = appendJPEGSegment(out, 0xE1, append([]byte("Exif\x00\x00"), exifTIFF(orientation)...))
	out = appendJPEGSegment(out, 0xE1, []byte("http://ns.adobe.com/xap/1.0/\x00<x:xmpmeta>"+gpsMarker+"</x:xmpmeta>"))
	out = appendJPEGSegment(out, 0xFE, []byte("comment "+gpsMarker))
	out = append(out, plain[2:]...)
	return append(out, plain...) // trailing MPF-style image
}

func jpegSegments(t *testing.T, data []byte) map[byte][][]byte {
	t.Helper()
	segments := map[byte][][]byte{}
	pos := 2
	for {
		marker, next, err := nextJPEGMarker(data, pos)
		require.NoError(t, err)
		if marker == 0xDA || marker == 0xD9 {
			return segments
		}
		length := int(binary.BigEndian.Uint16(data[next : next+2]))
		segments[marker] = append(segments[marker], data[next+2:next+length])
		pos = next + length
	}
}

func TestStrip_JPEGRemovesExifGPSAndKeepsPixels(t *testing.T) {
	t.Parallel()
	original := jpegWithMetadata(t, 0)

	stripped, err := Strip(original, "image/jpeg")
	require.NoError(t, err)

	assert.NotContains(t, string(stripped), gpsMarker)
	assert.NotContains(t, string(stripped), "Exif")
	assert.Empty(t, jpegSegments(t, stripped)[0xE1], "no APP1 segment may remain")
	assert.Equal(t, testJPEG(t), stripped, "plain image data stays byte-identical")

	want, err := jpeg.Decode(bytes.NewReader(testJPEG(t)))
	require.NoError(t, err)
	got, err := jpeg.Decode(bytes.NewReader(stripped))
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestStrip_JPEGKeepsOnlyOrientation(t *testing.T) {
	t.Parallel()
	stripped, err := Strip(jpegWithMetadata(t, 6), "image/jpeg")
	require.NoError(t, err)

	assert.NotContains(t, string(stripped), gpsMarker)
	app1 := jpegSegments(t, stripped)[0xE1]
	require.Len(t, app1, 1)
	require.True(t, bytes.HasPrefix(app1[0], exifHeader))
	tiff := app1[0][len(exifHeader):]
	assert.Equal(t, uint16(6), exifOrientation(tiff))
	assert.Equal(t, orientationOnlyTIFF(6), tiff, "IFD0 holds the orientation tag only")
	_, err = jpeg.Decode(bytes.NewReader(stripped))
	require.NoError(t, err)
}

func TestStrip_JPEGKeepsColourSegments(t *testing.T) {
	t.Parallel()
	plain := testJPEG(t)
	icc := append([]byte("ICC_PROFILE\x00\x01\x01"), "profile"...)
	in := appendJPEGSegment([]byte{0xFF, 0xD8}, 0xE2, icc)
	in = appendJPEGSegment(in, 0xE2, []byte("MPF\x00"+gpsMarker))
	in = append(in, plain[2:]...)

	stripped, err := Strip(in, "image/jpeg")
	require.NoError(t, err)
	assert.Equal(t, [][]byte{icc}, jpegSegments(t, stripped)[0xE2])
	assert.NotContains(t, string(stripped), gpsMarker)
}

func TestStrip_RejectsTruncatedJPEG(t *testing.T) {
	t.Parallel()
	original := jpegWithMetadata(t, 0)
	_, err := Strip(original[:40], "image/jpeg")
	assert.ErrorIs(t, err, ErrInvalidImage)
}

func pngChunks(t *testing.T, data []byte) []string {
	t.Helper()
	var types []string
	for pos := 8; pos+12 <= len(data); {
		length := int(binary.BigEndian.Uint32(data[pos:]))
		types = append(types, string(data[pos+4:pos+8]))
		pos += 12 + length
	}
	return types
}

func TestStrip_PNGDropsTextAndExifChunks(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewGray(image.Rect(0, 0, 2, 2))))
	plain := buf.Bytes()
	in := append([]byte{}, plain[:33]...) // signature + IHDR
	in = appendPNGChunk(in, "tEXt", []byte("Location\x00"+gpsMarker))
	in = appendPNGChunk(in, "eXIf", exifTIFF(8))
	in = appendPNGChunk(in, "gAMA", []byte{0, 0, 0xB1, 0x8F})
	in = append(in, plain[33:]...)

	stripped, err := Strip(in, "image/png")
	require.NoError(t, err)
	assert.NotContains(t, string(stripped), gpsMarker)
	assert.Equal(t, []string{"IHDR", "eXIf", "gAMA", "IDAT", "IEND"}, pngChunks(t, stripped))
	_, err = png.Decode(bytes.NewReader(stripped))
	require.NoError(t, err, "chunk CRCs stay valid")
}

// lossless1x1WebP is a valid 1x1 lossless WebP (simple format).
const lossless1x1WebP = "UklGRhoAAABXRUJQVlA4TA0AAAAvAAAAEAcQERGIiP4HAA=="

func webpChunk(fourCC string, payload []byte) []byte {
	out := append([]byte(fourCC), binary.LittleEndian.AppendUint32(nil, uint32(len(payload)))...)
	out = append(out, payload...)
	if len(payload)%2 == 1 {
		out = append(out, 0)
	}
	return out
}

func TestStrip_WebPDropsExifAndXMP(t *testing.T) {
	t.Parallel()
	simple, err := base64.StdEncoding.DecodeString(lossless1x1WebP)
	require.NoError(t, err)
	vp8l := simple[12:]
	vp8x := []byte{webpFlagEXIF | webpFlagXMP, 0, 0, 0, 0, 0, 0, 0, 0, 0} // 1x1 canvas
	body := append([]byte("WEBP"), webpChunk("VP8X", vp8x)...)
	body = append(body, vp8l...)
	body = append(body, webpChunk("EXIF", append([]byte("Exif\x00\x00"), exifTIFF(3)...))...)
	body = append(body, webpChunk("XMP ", []byte(gpsMarker))...)
	in := append([]byte("RIFF"), binary.LittleEndian.AppendUint32(nil, uint32(len(body)))...)
	in = append(in, body...)

	stripped, err := Strip(in, "image/webp")
	require.NoError(t, err)
	assert.NotContains(t, string(stripped), gpsMarker)
	assert.NotContains(t, string(stripped), "XMP ")
	assert.Equal(t, uint32(len(stripped)-8), binary.LittleEndian.Uint32(stripped[4:8]))
	assert.Equal(t, byte(webpFlagEXIF), stripped[20], "XMP flag cleared, EXIF kept for orientation")
	exifAt := bytes.Index(stripped, []byte("EXIF"))
	require.Positive(t, exifAt)
	assert.Equal(t, orientationOnlyTIFF(3), stripped[exifAt+8:])
	_, err = webp.Decode(bytes.NewReader(stripped))
	require.NoError(t, err)
}
