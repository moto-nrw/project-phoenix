package api

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	signatureWidth    = 600
	signatureHeight   = 180
	signatureBaseline = 112.0
	signatureMargin   = 34.0
	signatureWordGap  = 22.0
)

var signatureInk = color.NRGBA{R: 23, G: 37, B: 84}

type signaturePoint struct{ x, y float64 }

// demoSignaturePNG draws a handwritten-looking signature of the name as the
// portal's canvas would send it: a transparent PNG with dark ink, base64
// encoded. Each word is one stroke of connected loops in the rhythm of its
// letters (a tall loop for the capital, taller loops for ascenders, a loop
// below the line for descenders), slanted to the right and underlined. The
// same name always gives the same image.
func demoSignaturePNG(name string) string {
	img := image.NewNRGBA(image.Rect(0, 0, signatureWidth, signatureHeight))
	for _, stroke := range signatureStrokes(name) {
		inkStroke(img, stroke)
	}
	// Two colours, transparent and ink, keep the PNG to a few kilobytes: the
	// parents portal limits a contact's request body to 8 KB, signature
	// included (#4780).
	ink := signatureInk
	ink.A = 255
	paletted := image.NewPaletted(img.Bounds(), color.Palette{color.Transparent, ink})
	for y := img.Rect.Min.Y; y < img.Rect.Max.Y; y++ {
		for x := img.Rect.Min.X; x < img.Rect.Max.X; x++ {
			if img.NRGBAAt(x, y).A >= 96 {
				paletted.SetColorIndex(x, y, 1)
			}
		}
	}
	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoder.Encode(&buf, paletted); err != nil {
		panic(fmt.Sprintf("encode demo signature: %v", err))
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func signatureStrokes(name string) [][]signaturePoint {
	words := strings.Fields(name)
	if len(words) == 0 {
		words = []string{"Unterschrift"}
	}
	// A capital takes two letter widths; the letters share the canvas.
	units := 0
	for _, word := range words {
		units += utf8.RuneCountInString(word) + 1
	}
	room := signatureWidth - 2*signatureMargin - 30 - signatureWordGap*float64(len(words)-1)
	advance := math.Min(30, room/float64(units))
	x := signatureMargin
	strokes := make([][]signaturePoint, 0, len(words)+1)
	for _, word := range words {
		stroke := make([]signaturePoint, 0, 48*utf8.RuneCountInString(word))
		for i, r := range []rune(word) {
			width := advance
			if i == 0 {
				width = 2 * advance
			}
			stroke = append(stroke, signatureLetter(x, width, r, i == 0, signatureJitter(word, i))...)
			x += width
		}
		strokes = append(strokes, stroke)
		x += signatureWordGap
	}
	return append(strokes, signatureUnderline(signatureMargin+advance, x-signatureWordGap))
}

// signatureLetter is one loop of a prolate cycloid: the pen moves forward
// by the letter's width, rises to the letter's height, loops back and comes
// down to the line, which joins the letters like cursive writing.
func signatureLetter(x0, width float64, r rune, capital bool, jitter float64) []signaturePoint {
	height, below, loop := 26.0, false, 1.7
	switch lower := unicode.ToLower(r); {
	case capital:
		height, loop = 70, 1.4
	case strings.ContainsRune("bdfhklt", lower):
		height = 58
	case strings.ContainsRune("gjpqy", lower):
		height, below = 40, true
	}
	height *= 0.75 + 0.5*jitter
	radius := width / (2 * math.Pi)
	const steps = 48
	points := make([]signaturePoint, 0, steps+1)
	for step := 0; step <= steps; step++ {
		theta := 2 * math.Pi * float64(step) / steps
		lift := height * (1 - math.Cos(theta)) / 2
		y := signatureBaseline - lift
		if below {
			y = signatureBaseline + 0.9*lift
		}
		// The pen runs back at the turning point, which closes the loop at
		// the top of the letter (below the line for a descender).
		x := x0 + radius*theta + loop*radius*math.Sin(theta) + 0.28*(signatureBaseline-y)
		// Handwriting climbs a little towards the end of the line.
		y -= 0.03 * (x - signatureMargin)
		points = append(points, signaturePoint{x: x, y: y})
	}
	return points
}

// signatureUnderline swings back from the end of the name under it.
func signatureUnderline(from, to float64) []signaturePoint {
	const steps = 80
	points := make([]signaturePoint, 0, steps+1)
	for step := 0; step <= steps; step++ {
		t := float64(step) / steps
		x := to - (to-from)*t
		points = append(points, signaturePoint{
			x: x,
			y: signatureBaseline + 16 + 10*math.Sin(t*math.Pi) - 0.03*(x-signatureMargin),
		})
	}
	return points
}

// signatureJitter varies the letters' heights a little, the same way for
// the same name.
func signatureJitter(word string, index int) float64 {
	h := fnv.New32a()
	_, _ = fmt.Fprintf(h, "%s/%d", word, index)
	return float64(h.Sum32()%1000) / 1000
}

// inkStroke draws a stroke with a round pen that presses harder on the way
// down, as a hand does.
func inkStroke(img *image.NRGBA, stroke []signaturePoint) {
	for i := 1; i < len(stroke); i++ {
		from, to := stroke[i-1], stroke[i]
		length := math.Hypot(to.x-from.x, to.y-from.y)
		radius := 1.4
		if length > 0 && to.y > from.y {
			radius += 0.9 * (to.y - from.y) / length
		}
		samples := max(1, int(length/0.6))
		for s := 0; s <= samples; s++ {
			t := float64(s) / float64(samples)
			inkDot(img, from.x+(to.x-from.x)*t, from.y+(to.y-from.y)*t, radius)
		}
	}
}

func inkDot(img *image.NRGBA, cx, cy, radius float64) {
	for y := int(cy - radius - 1); y <= int(cy+radius+1); y++ {
		for x := int(cx - radius - 1); x <= int(cx+radius+1); x++ {
			if !image.Pt(x, y).In(img.Rect) {
				continue
			}
			cover := radius + 0.5 - math.Hypot(float64(x)+0.5-cx, float64(y)+0.5-cy)
			if cover <= 0 {
				continue
			}
			alpha := uint8(math.Min(cover, 1) * 255)
			if img.NRGBAAt(x, y).A < alpha {
				ink := signatureInk
				ink.A = alpha
				img.SetNRGBA(x, y, ink)
			}
		}
	}
}
