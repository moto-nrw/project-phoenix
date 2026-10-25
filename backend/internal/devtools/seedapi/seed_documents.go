package api

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"math"
)

// demoIllustratedPDF builds a one-page PDF with a title, a picture and the
// text lines below it. The picture travels as raw RGB, so the file has the
// size of a real flyer (tens of kilobytes) and the attachment list does not
// show a few hundred bytes. The same input always gives the same file.
func demoIllustratedPDF(title string, picture *image.NRGBA, lines []string) []byte {
	bounds := picture.Bounds()
	pixels := make([]byte, 0, 3*bounds.Dx()*bounds.Dy())
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := picture.NRGBAAt(x, y)
			pixels = append(pixels, c.R, c.G, c.B)
		}
	}

	var content bytes.Buffer
	content.WriteString("q 420 0 0 280 88 470 cm /Im1 Do Q\n")
	content.WriteString("BT\n/F1 22 Tf\n72 780 Td\n(" + pdfString(title) + ") Tj\n/F1 12 Tf\n0 -340 Td\n")
	for _, line := range lines {
		content.WriteString("0 -20 Td\n(" + pdfString(line) + ") Tj\n")
	}
	content.WriteString("ET")

	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R >> /XObject << /Im1 6 0 R >> >> /Contents 5 0 R >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", content.Len(), content.String()),
		fmt.Sprintf("<< /Type /XObject /Subtype /Image /Width %d /Height %d /ColorSpace /DeviceRGB /BitsPerComponent 8 /Length %d >>\nstream\n%s\nendstream",
			bounds.Dx(), bounds.Dy(), len(pixels), pixels),
	}

	var out bytes.Buffer
	out.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, object := range objects {
		offsets[i] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n%s\nendobj\n", i+1, object)
	}
	startxref := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&out, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, startxref)
	return out.Bytes()
}

// lanternPicture paints the flyer picture of the Laternenfest: three glowing
// lanterns on a stick under an evening sky, above a dark hill.
func lanternPicture() *image.NRGBA {
	const width, height = 168, 112
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	lanterns := []struct{ x, y, r float64 }{{48, 54, 13}, {86, 62, 15}, {124, 52, 12}}
	for y := range height {
		for x := range width {
			img.SetNRGBA(x, y, lanternPixel(float64(x), float64(y), width, height, lanterns))
		}
	}
	return img
}

func lanternPixel(x, y float64, width, height int, lanterns []struct{ x, y, r float64 }) color.NRGBA {
	t := y / float64(height)
	c := color.NRGBA{R: uint8(18 + 40*t), G: uint8(28 + 30*t), B: uint8(78 + 50*t), A: 255}
	// A few fixed stars.
	if (int(x)*7+int(y)*13)%211 == 0 && y < 40 {
		c = color.NRGBA{R: 250, G: 240, B: 200, A: 255}
	}
	// The stick and the strings the lanterns hang from.
	if math.Abs(y-(26+0.05*x)) < 1 && x > 30 && x < 142 {
		c = color.NRGBA{R: 92, G: 64, B: 40, A: 255}
	}
	for _, l := range lanterns {
		if math.Abs(x-l.x) < 0.6 && y > 26+0.05*x && y < l.y-l.r {
			c = color.NRGBA{R: 200, G: 200, B: 200, A: 255}
		}
		d := math.Hypot(x-l.x, (y-l.y)*1.15)
		switch {
		case d < l.r:
			glow := 1 - d/l.r
			c = color.NRGBA{R: 255, G: uint8(140 + 90*glow), B: uint8(30 + 120*glow), A: 255}
		case d < l.r+9:
			// The warm halo around the lantern.
			mix := (l.r + 9 - d) / 9 * 0.6
			c = blend(c, color.NRGBA{R: 255, G: 190, B: 90, A: 255}, mix)
		}
	}
	// The hill the procession walks over.
	if y > float64(height)-18-8*math.Sin(x/27) {
		c = color.NRGBA{R: 22, G: 48, B: 34, A: 255}
	}
	return c
}

func blend(a, b color.NRGBA, mix float64) color.NRGBA {
	lerp := func(p, q uint8) uint8 { return uint8(float64(p)*(1-mix) + float64(q)*mix) }
	return color.NRGBA{R: lerp(a.R, b.R), G: lerp(a.G, b.G), B: lerp(a.B, b.B), A: 255}
}
