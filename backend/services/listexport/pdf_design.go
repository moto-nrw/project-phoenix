package listexport

// SPIKE (issue #1568): a branded PDF renderer built on signintech/gopdf.
// Renders listexport.Document in the app's design language: embedded Inter
// (real Unicode — no transliteration), moto logo, dotted page background,
// white content card, dark table header.
//
// Not wired into service.go — renderPDF (the hand-rolled writer) is still
// the production path.

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"strings"

	"github.com/signintech/gopdf"
	"golang.org/x/text/unicode/norm"
)

//go:embed assets/fonts/Inter-Regular.ttf
var interRegular []byte

//go:embed assets/fonts/Inter-SemiBold.ttf
var interSemiBold []byte

//go:embed assets/moto-logo.png
var motoLogoPNG []byte

const (
	fontFamily  = "inter"
	styleNormal = ""
	styleBold   = "B"
)

// norms normalizes to NFC. gopdf (like the whole gofpdf family) does no
// OpenType shaping: decomposed input (NFD — common from macOS) would drop
// its combining marks silently. Precomposed codepoints render correctly.
func norms(s string) string { return norm.NFC.String(s) }

type designPage struct {
	groupTitle string
	rows       []Row
}

type designRenderer struct {
	pdf    *gopdf.GoPdf
	doc    Document
	cols   []Column
	widths []float64
	w, h   float64
	total  int
	logo   gopdf.ImageHolder
	bgTpl  int
}

// newDesignRenderer prepares a renderer with fonts loaded but nothing
// drawn — split out so pagination decisions are unit-testable.
func newDesignRenderer(doc Document) (*designRenderer, error) {
	cols := doc.Columns
	if len(cols) == 0 {
		cols = ResolveColumns(nil, PresetOGSWeekly)
	}

	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4Landscape})

	if err := pdf.AddTTFFontData(fontFamily, interRegular); err != nil {
		return nil, fmt.Errorf("embed Inter Regular: %w", err)
	}
	if err := pdf.AddTTFFontDataWithOption(fontFamily, interSemiBold, gopdf.TtfOption{Style: gopdf.Bold}); err != nil {
		return nil, fmt.Errorf("embed Inter SemiBold: %w", err)
	}

	// One content-addressed holder per document: gopdf caches embedded
	// images by holder ID (an MD5 of the bytes), so the logo XObject is
	// written once and referenced from every page instead of being
	// re-embedded per page (~logo-size bytes saved per page).
	logo, err := gopdf.ImageHolderByBytes(motoLogoPNG)
	if err != nil {
		return nil, fmt.Errorf("logo holder: %w", err)
	}

	w, h := gopdf.PageSizeA4Landscape.W, gopdf.PageSizeA4Landscape.H
	bgTpl, err := importDotBackground(pdf, w, h)
	if err != nil {
		return nil, err
	}

	r := &designRenderer{pdf: pdf, doc: doc, cols: cols, w: w, h: h, logo: logo, bgTpl: bgTpl}
	r.widths = pdfColumnWidths(cols, w-2*pageMargin-2*cardPadX)
	return r, nil
}

func renderPDFDesigned(doc Document) ([]byte, error) {
	r, err := newDesignRenderer(doc)
	if err != nil {
		return nil, err
	}

	pages, err := r.paginate()
	if err != nil {
		return nil, err
	}
	r.total = len(pages)

	for i, p := range pages {
		if err := r.drawPage(p, i+1); err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer
	if _, err := r.pdf.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("write pdf: %w", err)
	}
	return buf.Bytes(), nil
}

// contentTop is the y where the card's table body starts, below the header
// block. bodyBottom is the last usable y inside the card.
func (r *designRenderer) cardTop() float64    { return pageMargin + 104 }
func (r *designRenderer) bodyBottom() float64 { return r.h - pageMargin - footerHeight }

// paginate splits rows into pages. Group titles start a new page, mirroring
// the current renderer's behaviour (each class gets its own sheet).
func (r *designRenderer) paginate() ([]designPage, error) {
	if err := r.pdf.SetFont(fontFamily, styleNormal, fontBody); err != nil {
		return nil, err
	}

	// Rows that fit inside one card: the card's own chrome (padding, an
	// optional group heading, the table header) comes off the top first.
	avail := r.bodyBottom() - (r.cardTop() + 2*cardPadY + fontGroup + groupGap + tableHeaderHeight())
	pages := []designPage{}
	cur := designPage{}
	used := 0.0
	wroteBody := false

	flush := func() {
		if wroteBody || cur.groupTitle != "" {
			pages = append(pages, cur)
		}
	}

	for _, row := range r.doc.Rows {
		if row.GroupTitle != "" {
			flush()
			cur = designPage{groupTitle: row.GroupTitle}
			used, wroteBody = 0, false
			continue
		}
		hgt := r.rowHeight(row)
		if used+hgt > avail && wroteBody {
			pages = append(pages, cur)
			cur = designPage{groupTitle: cur.groupTitle, rows: nil}
			used, wroteBody = 0, false
		}
		cur.rows = append(cur.rows, row)
		used += hgt
		wroteBody = true
	}
	flush()
	if len(pages) == 0 {
		pages = []designPage{{}}
	}
	return pages, nil
}

func tableHeaderHeight() float64 { return fontTableHd + 2*cellPadY + 2 }

func (r *designRenderer) rowHeight(row Row) float64 {
	maxLines := 1
	for i, col := range r.cols {
		lines := r.wrap(norms(row.Values[col.ID]), r.widths[i]-cellPadX)
		if len(lines) > maxLines {
			maxLines = len(lines)
		}
	}
	return float64(maxLines)*rowLineHt + 2*cellPadY
}

// wrap greedily breaks text to fit maxW, measured with the current font.
func (r *designRenderer) wrap(s string, maxW float64) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{""}
	}
	words := strings.Fields(s)
	lines := []string{}
	cur := ""
	for _, word := range words {
		try := word
		if cur != "" {
			try = cur + " " + word
		}
		if wdt, err := r.pdf.MeasureTextWidth(try); err == nil && wdt <= maxW {
			cur = try
			continue
		}
		if cur != "" {
			lines = append(lines, cur)
		}
		cur = word
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

func (r *designRenderer) setFill(c rgb)   { r.pdf.SetFillColor(c.R, c.G, c.B) }
func (r *designRenderer) setStroke(c rgb) { r.pdf.SetStrokeColor(c.R, c.G, c.B) }
func (r *designRenderer) setText(c rgb)   { r.pdf.SetTextColor(c.R, c.G, c.B) }

func (r *designRenderer) text(x, y float64, s string) error {
	r.pdf.SetXY(x, y)
	return r.pdf.Text(norms(s))
}

func (r *designRenderer) drawPage(p designPage, num int) error {
	r.pdf.AddPage()
	if err := r.drawBackground(); err != nil {
		return err
	}
	if err := r.drawHeader(); err != nil {
		return err
	}
	if err := r.drawCard(p); err != nil {
		return err
	}
	return r.drawFooter(num)
}

// dotBackgroundPDF renders the page background (gray-50 + the guide's dot
// grid) once, as a standalone single-page PDF held in memory. The real
// document imports that page as a Form XObject and stamps it per page —
// thousands of unique bezier circles compress terribly (~170 KB per page
// when drawn inline), a referenced template costs that once per document.
func dotBackgroundPDF(w, h float64) ([]byte, error) {
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: gopdf.Rect{W: w, H: h}})
	pdf.AddPage()

	pdf.SetFillColor(colorPageBg.R, colorPageBg.G, colorPageBg.B)
	pdf.RectFromUpperLeftWithStyle(0, 0, w, h, "F")

	// gopdf's Oval strokes only (its cache emits a bezier path ending in
	// "S"), so a filled dot is drawn as a stroked circle of half the radius
	// with the line width set to the full radius: the stroke then covers
	// r=0..dotRadiusPt, giving a solid disc.
	pdf.SetStrokeColor(colorDot.R, colorDot.G, colorDot.B)
	pdf.SetLineWidth(dotRadiusPt)
	half := dotRadiusPt / 2
	for x := dotOffsetPt; x < w; x += dotSpacingPt {
		for y := dotOffsetPt; y < h; y += dotSpacingPt {
			pdf.Oval(x-half, y-half, x+half, y+half)
		}
	}

	var buf bytes.Buffer
	if _, err := pdf.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("render dot background: %w", err)
	}
	return buf.Bytes(), nil
}

// importDotBackground registers the shared background template on pdf.
func importDotBackground(pdf *gopdf.GoPdf, w, h float64) (int, error) {
	bg, err := dotBackgroundPDF(w, h)
	if err != nil {
		return 0, err
	}
	rs := io.ReadSeeker(bytes.NewReader(bg))
	// gofpdi template IDs are zero-based — the first import returns 0,
	// negative values signal failure.
	tpl := pdf.ImportPageStream(&rs, 1, "/MediaBox")
	if tpl < 0 {
		return 0, fmt.Errorf("import dot background template failed")
	}
	return tpl, nil
}

func (r *designRenderer) drawBackground() error {
	r.pdf.UseImportedTemplate(r.bgTpl, 0, 0, r.w, r.h)
	return nil
}

func (r *designRenderer) drawHeader() error {
	if err := r.pdf.ImageByHolder(r.logo, pageMargin, pageMargin-6, &gopdf.Rect{
		W: logoHeight * logoAspect, H: logoHeight,
	}); err != nil {
		return fmt.Errorf("draw logo: %w", err)
	}

	// Generated-at, right aligned.
	if err := r.pdf.SetFont(fontFamily, styleNormal, fontMeta); err != nil {
		return err
	}
	r.setText(colorMuted)
	stamp := "Erstellt: " + GeneratedAtLabel(r.doc.GeneratedAt)
	sw, err := r.pdf.MeasureTextWidth(stamp)
	if err != nil {
		return err
	}
	if err := r.text(r.w-pageMargin-sw, pageMargin+6, stamp); err != nil {
		return err
	}

	y := pageMargin + 32

	// Eyebrow — only for two-part titles; single-part titles render the
	// headline alone instead of repeating themselves.
	if eyebrow := docEyebrow(r.doc); eyebrow != "" {
		if err := r.pdf.SetFont(fontFamily, styleBold, fontEyebrow); err != nil {
			return err
		}
		r.setText(colorEyebrow)
		if err := r.text(pageMargin, y, strings.ToUpper(spaceOut(eyebrow))); err != nil {
			return err
		}
		y += 16
	}

	// Title.
	if err := r.pdf.SetFont(fontFamily, styleBold, fontTitle); err != nil {
		return err
	}
	r.setText(colorInk)
	if err := r.text(pageMargin, y, docHeadline(r.doc)); err != nil {
		return err
	}
	y += 14

	// Subtitle.
	if r.doc.Subtitle != "" {
		if err := r.pdf.SetFont(fontFamily, styleNormal, fontSubtitle); err != nil {
			return err
		}
		r.setText(colorMuted)
		if err := r.text(pageMargin, y, r.doc.Subtitle); err != nil {
			return err
		}
		y += 14
	}

	// Filter pills.
	return r.drawFilterPills(pageMargin, y)
}

func (r *designRenderer) drawFilterPills(x, y float64) error {
	if len(r.doc.Filters) == 0 {
		return nil
	}
	if err := r.pdf.SetFont(fontFamily, styleNormal, fontMeta); err != nil {
		return err
	}
	cx := x
	for _, f := range r.doc.Filters {
		f = norms(f)
		tw, err := r.pdf.MeasureTextWidth(f)
		if err != nil {
			return err
		}
		pw := tw + 14
		r.setFill(colorSurface)
		r.setStroke(colorBorder)
		r.pdf.SetLineWidth(0.5)
		if err := r.pdf.Rectangle(cx, y-2, cx+pw, y+12, "FD", pillRadius, 8); err != nil {
			return err
		}
		r.setText(colorMuted)
		if err := r.text(cx+7, y+8, f); err != nil {
			return err
		}
		cx += pw + 6
	}
	return nil
}

func (r *designRenderer) drawCard(p designPage) error {
	top := r.cardTop()
	left := pageMargin
	right := r.w - pageMargin

	// The card hugs its content (as the guide's cards do) instead of
	// stretching to the page foot, so a short class list doesn't render as
	// a mostly-empty sheet.
	// rowHeight measures with the *current* font — pin it to the body font
	// before measuring, or the pill font (set in drawHeader) is used.
	if err := r.pdf.SetFont(fontFamily, styleNormal, fontBody); err != nil {
		return err
	}
	content := 2*cardPadY + tableHeaderHeight()
	if p.groupTitle != "" {
		content += fontGroup + groupGap
	}
	for _, row := range p.rows {
		content += r.rowHeight(row)
	}
	bottom := top + content
	if maxY := r.bodyBottom(); bottom > maxY {
		bottom = maxY
	}

	// Card surface.
	r.setFill(colorSurface)
	r.setStroke(colorBorder)
	r.pdf.SetLineWidth(0.6)
	if err := r.pdf.Rectangle(left, top, right, bottom, "FD", cardRadius, 12); err != nil {
		return err
	}

	y := top + cardPadY
	textLeft := left + cardPadX
	textRight := right - cardPadX

	// Group heading — the card's title: InfoCard's "text-base font-semibold
	// text-gray-900" with an mb-4 gap under it. Plain text, no filled band
	// (the app never fills one).
	if p.groupTitle != "" {
		if err := r.pdf.SetFont(fontFamily, styleBold, fontGroup); err != nil {
			return err
		}
		r.setText(colorInk)
		if err := r.text(textLeft, y+fontGroup-2, p.groupTitle); err != nil {
			return err
		}
		y += fontGroup + groupGap
	}

	// Table header: muted labels over a hairline — DataTable's
	// "border-b border-gray-100 text-xs font-medium text-gray-500".
	hh := tableHeaderHeight()
	if err := r.pdf.SetFont(fontFamily, styleNormal, fontTableHd); err != nil {
		return err
	}
	r.setText(colorHeaderText)
	cx := textLeft
	for i, col := range r.cols {
		if err := r.text(cx, y+hh-cellPadY-1, col.Label); err != nil {
			return err
		}
		cx += r.widths[i]
	}
	y += hh
	r.setStroke(colorHeaderRule)
	r.pdf.SetLineWidth(0.7)
	r.pdf.Line(textLeft, y, textRight, y)

	// Body rows.
	if err := r.pdf.SetFont(fontFamily, styleNormal, fontBody); err != nil {
		return err
	}
	for _, row := range p.rows {
		rh := r.rowHeight(row)
		cx = textLeft
		r.setText(colorBody)
		for i, col := range r.cols {
			lines := r.wrap(norms(row.Values[col.ID]), r.widths[i]-cellPadX)
			ty := y + cellPadY + rowLineHt - 2
			for _, ln := range lines {
				if err := r.text(cx, ty, ln); err != nil {
					return err
				}
				ty += rowLineHt
			}
			cx += r.widths[i]
		}
		y += rh
		r.setStroke(colorRowLine)
		r.pdf.SetLineWidth(0.5)
		r.pdf.Line(textLeft, y, textRight, y)
	}
	return nil
}

func (r *designRenderer) drawFooter(num int) error {
	if err := r.pdf.SetFont(fontFamily, styleNormal, fontFooter); err != nil {
		return err
	}
	r.setText(colorMuted)
	y := r.h - pageMargin + 4

	// Confidentiality note, centred. renderPDF drops doc.Footer entirely —
	// this path honours it (as xlsx.go/docx.go/records.go already do).
	if r.doc.Footer != "" {
		fw, err := r.pdf.MeasureTextWidth(norms(r.doc.Footer))
		if err != nil {
			return err
		}
		if err := r.text((r.w-fw)/2, y, r.doc.Footer); err != nil {
			return err
		}
	}

	page := fmt.Sprintf("Seite %d von %d", num, r.total)
	pw, err := r.pdf.MeasureTextWidth(page)
	if err != nil {
		return err
	}
	return r.text(r.w-pageMargin-pw, y, page)
}

// docEyebrow derives the small green kicker above the title from a
// two-part title ("Kinderliste — OGS Wochenübersicht" → "Kinderliste").
// Production titles are usually single-part ("Klassenliste", user-supplied
// req.Title) — those get NO eyebrow rather than repeating the title.
func docEyebrow(doc Document) string {
	title := strings.TrimSpace(doc.Title)
	for _, sep := range []string{" — ", " – ", " - "} {
		if i := strings.Index(title, sep); i > 0 {
			return strings.TrimSpace(title[:i])
		}
	}
	return ""
}

// docHeadline is the title minus the eyebrow part, so the two don't repeat
// each other ("Kinderliste — OGS Wochenübersicht" → "OGS Wochenübersicht").
func docHeadline(doc Document) string {
	title := strings.TrimSpace(doc.Title)
	for _, sep := range []string{" — ", " – ", " - "} {
		if i := strings.Index(title, sep); i > 0 {
			return strings.TrimSpace(title[i+len(sep):])
		}
	}
	return title
}

// spaceOut letterspaces a short label (the guide's eyebrow style).
func spaceOut(s string) string {
	parts := strings.Split(s, "")
	return strings.Join(parts, " ")
}
