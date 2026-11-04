package listexport

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
	"time"

	listrecords "github.com/moto-nrw/project-phoenix/modules/documentrendering/records"
)

func signaturePNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for x := 0; x < w; x++ {
		img.Set(x, h/2, color.NRGBA{A: 255})
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// A field with an image (a signature, #4785) prints its label, places the
// image in the layout and still renders a PDF.
func TestRenderRecords_DrawsImageFields(t *testing.T) {
	t.Parallel()
	doc := listrecords.RecordDocument{
		Title: "Nachweis", GeneratedAt: time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC),
		Records: []listrecords.Record{{
			Title: "Zugestimmt",
			Fields: []listrecords.Field{
				{Label: "Von", Value: "Klaus Richter"},
				{Label: "Unterschrift", PNG: signaturePNG(t, 800, 300)},
			},
		}},
	}
	_, trace := traceRecords(t, doc)
	if !traceContains(trace, "Unterschrift") || !traceContains(trace, "[Bild]") {
		t.Fatalf("trace lacks the image field: %v", trace)
	}
	file, err := NewService().RenderRecords(doc, "nachweis")
	if err != nil {
		t.Fatalf("RenderRecords: %v", err)
	}
	if !bytes.HasPrefix(file.Data, []byte("%PDF-1.")) {
		t.Fatalf("data does not start with PDF header")
	}
	if !bytes.Contains(file.Data, []byte("/Subtype /Image")) {
		t.Fatalf("the PDF embeds no image")
	}
}

// The image keeps its aspect ratio inside the box.
func TestImageBoxKeepsAspectRatio(t *testing.T) {
	t.Parallel()
	w, h := imageBox(signaturePNG(t, 800, 300))
	if w > recImageMaxW+0.01 || h > recImageMaxH+0.01 {
		t.Fatalf("box %vx%v exceeds %vx%v", w, h, recImageMaxW, recImageMaxH)
	}
	if ratio := w / h; ratio < 2.66 || ratio > 2.67 {
		t.Fatalf("ratio %v, want 8:3", ratio)
	}
	if w, h := imageBox([]byte("not a png")); w != 0 || h != 0 {
		t.Fatalf("unreadable image takes space %vx%v", w, h)
	}
	truncated := signaturePNG(t, 800, 300)
	truncated = truncated[:len(truncated)-1]
	if _, err := png.DecodeConfig(bytes.NewReader(truncated)); err != nil {
		t.Fatalf("DecodeConfig rejects truncated PNG: %v", err)
	}
	if w, h := imageBox(truncated); w != 0 || h != 0 {
		t.Fatalf("corrupt image takes space %vx%v", w, h)
	}
}

func TestRenderRecordsSkipsCorruptImageFields(t *testing.T) {
	t.Parallel()
	truncated := signaturePNG(t, 800, 300)
	truncated = truncated[:len(truncated)-1]
	doc := listrecords.RecordDocument{
		Title: "Nachweis", GeneratedAt: time.Date(2026, 10, 24, 9, 0, 0, 0, time.UTC),
		Records: []listrecords.Record{{
			Title:  "Zugestimmt",
			Fields: []listrecords.Field{{Label: "Unterschrift", PNG: truncated}},
		}},
	}
	if _, err := NewService().RenderRecords(doc, "nachweis"); err != nil {
		t.Fatalf("RenderRecords corrupt image: %v", err)
	}
}

func TestRenderRecordsWithInterlacedImage(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../../../../testdata/interlaced-logo.png")
	if err != nil {
		t.Fatal(err)
	}
	doc := listrecords.RecordDocument{Title: "Nachweis", Records: []listrecords.Record{{Title: "Zugestimmt", Fields: []listrecords.Field{{Label: "Unterschrift", PNG: data}}}}}
	file, err := NewService().RenderRecords(doc, "nachweis")
	if err != nil {
		t.Fatalf("RenderRecords: %v", err)
	}
	if !bytes.HasPrefix(file.Data, []byte("%PDF")) || !bytes.Contains(file.Data, []byte("/Subtype /Image")) {
		t.Fatal("PDF must embed the interlaced image")
	}
}
