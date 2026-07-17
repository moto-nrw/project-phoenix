package listexport

// The designed PDF renderer (issue #1568), built on signintech/gopdf.
// Renders listexport.Document in the app's design language: embedded Inter
// (real Unicode — no transliteration), moto logo, dotted page background,
// white content card, DataTable chrome. The shared page frame (pageChrome)
// is also used by the record/block renderer in records_design.go.

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"strings"
	"time"

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

// renderRow is one table row with every cell already wrapped to its
// column width, so pagination, splitting, and drawing all work on the
// same line counts and cannot disagree.
type renderRow struct {
	cells [][]string // per column: wrapped lines
	lines int        // max line count across cells
}

func (rr renderRow) height() float64 {
	return float64(rr.lines)*rowLineHt + 2*cellPadY
}

type designPage struct {
	groupTitle string
	rows       []renderRow
}

// pageChrome is the shared page frame every designed PDF draws through:
// dotted background, logo header with title/subtitle/filter pills, and
// the footer carrying the confidentiality note + page numbers. Both the
// table renderer (Document) and the record/block renderer (RecordDocument)
// use it, so the two layouts cannot drift apart.
type pageChrome struct {
	pdf         *gopdf.GoPdf
	w, h        float64
	logo        gopdf.ImageHolder
	bgTpl       int
	title       string
	subtitle    string
	generatedAt time.Time
	filters     []string
	footer      string

	// Glyph fallback (see font_fallback.go): coverage of the primary and
	// fallback fonts, plus the current font state so fallback runs can be
	// drawn at the right size and the primary font restored afterwards.
	primaryCov  *fontCoverage
	fallbackCov *fontCoverage
	curStyle    string
	curSize     float64

	// pillRows is the pre-computed filter-pill layout (computed once in
	// newPageChrome): pills wrap onto additional rows instead of running
	// off the right page edge, and bodyTop grows with the extra rows.
	pillRows [][]string
}

func newPageChrome(landscape bool, title, subtitle string, generatedAt time.Time, filters []string, footer string) (*pageChrome, error) {
	size := *gopdf.PageSizeA4
	if landscape {
		size = *gopdf.PageSizeA4Landscape
	}

	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: size})

	if err := pdf.AddTTFFontData(fontFamily, interRegular); err != nil {
		return nil, fmt.Errorf("embed Inter Regular: %w", err)
	}
	if err := pdf.AddTTFFontDataWithOption(fontFamily, interSemiBold, gopdf.TtfOption{Style: gopdf.Bold}); err != nil {
		return nil, fmt.Errorf("embed Inter SemiBold: %w", err)
	}
	if err := pdf.AddTTFFontData(fallbackFontFamily, unifontRegular); err != nil {
		return nil, fmt.Errorf("embed fallback font: %w", err)
	}

	primaryCov, err := newFontCoverage(interRegular)
	if err != nil {
		return nil, err
	}
	fallbackCov, err := newFontCoverage(unifontRegular)
	if err != nil {
		return nil, err
	}

	// One content-addressed holder per document: gopdf caches embedded
	// images by holder ID (an MD5 of the bytes), so the logo XObject is
	// written once and referenced from every page instead of being
	// re-embedded per page (~logo-size bytes saved per page).
	logo, err := gopdf.ImageHolderByBytes(motoLogoPNG)
	if err != nil {
		return nil, fmt.Errorf("logo holder: %w", err)
	}

	bgTpl, err := importDotBackground(pdf, size.W, size.H)
	if err != nil {
		return nil, err
	}

	c := &pageChrome{
		pdf: pdf, w: size.W, h: size.H, logo: logo, bgTpl: bgTpl,
		title: title, subtitle: subtitle, generatedAt: generatedAt,
		filters: filters, footer: footer,
		primaryCov: primaryCov, fallbackCov: fallbackCov,
	}
	if err := c.layoutPills(); err != nil {
		return nil, err
	}
	return c, nil
}

// setFont sets the primary font and remembers style/size so fallback
// runs (and post-fallback restores) use the same size.
func (c *pageChrome) setFont(style string, size float64) error {
	c.curStyle, c.curSize = style, size
	return c.pdf.SetFont(fontFamily, style, size)
}

// applyFont switches between the primary font (current style/size) and
// the fallback font (regular only — Unifont has a single weight).
func (c *pageChrome) applyFont(fallback bool) error {
	if fallback {
		return c.pdf.SetFont(fallbackFontFamily, styleNormal, c.curSize)
	}
	return c.pdf.SetFont(fontFamily, c.curStyle, c.curSize)
}

// measure returns the drawn width of s: per-run widths under the font
// each run will actually use, matching text() exactly.
func (c *pageChrome) measure(s string) (float64, error) {
	s = norms(s)
	runs := splitFontRuns(s, c.primaryCov, c.fallbackCov)
	// Fast path: pure primary-font text measures under the font already
	// set — no state churn.
	if len(runs) == 1 && !runs[0].fallback {
		return c.pdf.MeasureTextWidth(runs[0].text)
	}
	total := 0.0
	usedFallback := false
	for _, run := range runs {
		if run.fallback {
			usedFallback = true
		}
		if err := c.applyFont(run.fallback); err != nil {
			return 0, err
		}
		w, err := c.pdf.MeasureTextWidth(run.text)
		if err != nil {
			return 0, err
		}
		total += w
	}
	if usedFallback {
		if err := c.applyFont(false); err != nil {
			return 0, err
		}
	}
	return total, nil
}

type designRenderer struct {
	*pageChrome
	doc    Document
	cols   []Column
	widths []float64
	total  int
}

// newDesignRenderer prepares a renderer with fonts loaded but nothing
// drawn — split out so pagination decisions are unit-testable.
func newDesignRenderer(doc Document) (*designRenderer, error) {
	cols := doc.Columns
	if len(cols) == 0 {
		cols = ResolveColumns(nil, PresetOGSWeekly)
	}

	chrome, err := newPageChrome(true, doc.Title, doc.Subtitle, doc.GeneratedAt, doc.Filters, doc.Footer)
	if err != nil {
		return nil, err
	}

	r := &designRenderer{pageChrome: chrome, doc: doc, cols: cols}
	r.widths = pdfColumnWidths(cols, r.w-2*pageMargin-2*cardPadX)
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

// pdfColumnWidths distributes the usable width across columns by
// per-column weight.
func pdfColumnWidths(cols []Column, total float64) []float64 {
	weights := make([]float64, len(cols))
	sum := 0.0
	for i, col := range cols {
		weight := 1.0
		switch col.ID {
		case ColumnName:
			weight = 1.4
		case ColumnSchoolClass:
			weight = 0.65
		case ColumnGroup:
			weight = 1.2
		case ColumnWeeklyMonday, ColumnWeeklyTuesday, ColumnWeeklyWednesday, ColumnWeeklyThursday, ColumnWeeklyFriday:
			weight = 0.9
		case ColumnDeparture:
			weight = 1.5
		case ColumnGuardianContacts:
			weight = 2.7
		}
		weights[i] = weight
		sum += weight
	}
	widths := make([]float64, len(cols))
	for i, weight := range weights {
		widths[i] = total * weight / sum
	}
	return widths
}

// bodyTop is the y where page content starts, below the header block;
// bodyBottom is the last usable y above the footer.
func (c *pageChrome) bodyTop() float64 {
	extra := 0.0
	if n := len(c.pillRows); n > 1 {
		extra = float64(n-1) * pillRowH
	}
	return pageMargin + 104 + extra
}
func (c *pageChrome) bodyBottom() float64 { return c.h - pageMargin - footerHeight }

// paginate wraps every cell, splits rows into pages, and slices rows
// taller than a page body into continuation rows. Group titles start a
// new page, mirroring the old renderer's behaviour (each class gets its
// own sheet).
func (r *designRenderer) paginate() ([]designPage, error) {
	if err := r.setFont(styleNormal, fontBody); err != nil {
		return nil, err
	}

	// Rows that fit inside one card: the card's own chrome (padding, an
	// optional group heading, the table header) comes off the top first.
	avail := r.bodyBottom() - (r.bodyTop() + 2*cardPadY + fontGroup + groupGap + r.tableHeaderHeight())
	// The tallest single row a fresh page can hold; taller rows are
	// sliced so no content draws past the page body (they used to run
	// through the footer and off the page).
	maxRowLines := int((avail - 2*cellPadY) / rowLineHt)
	if maxRowLines < 1 {
		maxRowLines = 1
	}

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
		for _, rr := range splitRenderRow(r.buildRow(row), maxRowLines) {
			hgt := rr.height()
			if used+hgt > avail && wroteBody {
				pages = append(pages, cur)
				cur = designPage{groupTitle: cur.groupTitle, rows: nil}
				used, wroteBody = 0, false
			}
			cur.rows = append(cur.rows, rr)
			used += hgt
			wroteBody = true
		}
	}
	flush()
	if len(pages) == 0 {
		pages = []designPage{{}}
	}
	return pages, nil
}

// buildRow wraps every cell of row to its column width (body font).
func (r *designRenderer) buildRow(row Row) renderRow {
	rr := renderRow{cells: make([][]string, len(r.cols)), lines: 1}
	for i, col := range r.cols {
		lines := r.wrap(norms(row.Values[col.ID]), r.widths[i]-cellPadX)
		rr.cells[i] = lines
		if len(lines) > rr.lines {
			rr.lines = len(lines)
		}
	}
	return rr
}

// splitRenderRow slices a row taller than maxLines into continuation
// rows of at most maxLines wrapped lines each — no line is dropped.
func splitRenderRow(rr renderRow, maxLines int) []renderRow {
	if rr.lines <= maxLines {
		return []renderRow{rr}
	}
	out := []renderRow{}
	for start := 0; start < rr.lines; start += maxLines {
		part := renderRow{cells: make([][]string, len(rr.cells)), lines: 1}
		for i, lines := range rr.cells {
			end := start + maxLines
			if end > len(lines) {
				end = len(lines)
			}
			if start < len(lines) {
				part.cells[i] = lines[start:end]
			} else {
				part.cells[i] = nil
			}
			if n := len(part.cells[i]); n > part.lines {
				part.lines = n
			}
		}
		out = append(out, part)
	}
	return out
}

// tableHeaderHeight wraps every column label to its column width (with
// the header font pinned) and sizes the header band for the tallest one —
// long labels ("Betreuungs-/Anmeldestatus") stack instead of overlapping
// their neighbour column.
func (r *designRenderer) tableHeaderHeight() float64 {
	return float64(r.headerLineCount())*rowLineHt + 2*cellPadY + 2
}

func (r *designRenderer) headerLineCount() int {
	// Measure with the header font, then RESTORE the caller's font —
	// leaking fontTableHd into paginate() made body cells wrap narrower
	// in the measuring pass than in the drawing pass (lost lines).
	prevStyle, prevSize := r.curStyle, r.curSize
	_ = r.setFont(styleNormal, fontTableHd)
	maxLines := 1
	for i, col := range r.cols {
		if n := len(r.wrap(col.Label, r.widths[i]-cellPadX)); n > maxLines {
			maxLines = n
		}
	}
	if prevSize > 0 {
		_ = r.setFont(prevStyle, prevSize)
	}
	return maxLines
}

// wrap greedily breaks text to fit maxW, measured with the current font.
// A single word wider than maxW (long German compounds — "Schmetterlings-
// gruppe" in a narrow column) is hard-split at character level so no cell
// ever overflows into its neighbour.
func (c *pageChrome) wrap(s string, maxW float64) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return []string{""}
	}
	lines := []string{}
	cur := ""
	for _, word := range strings.Fields(s) {
		for _, part := range c.splitLongWord(word, maxW) {
			try := part
			if cur != "" {
				try = cur + " " + part
			}
			if wdt, err := c.measure(try); err == nil && wdt <= maxW {
				cur = try
				continue
			}
			if cur != "" {
				lines = append(lines, cur)
			}
			cur = part
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

// splitLongWord chunks a word that alone exceeds maxW into rune slices
// that fit; words that fit are returned unchanged.
func (c *pageChrome) splitLongWord(word string, maxW float64) []string {
	if wdt, err := c.measure(word); err != nil || wdt <= maxW {
		return []string{word}
	}
	parts := []string{}
	cur := ""
	for _, r := range word {
		try := cur + string(r)
		if wdt, err := c.measure(try); err == nil && wdt > maxW && cur != "" {
			parts = append(parts, cur)
			cur = string(r)
			continue
		}
		cur = try
	}
	if cur != "" {
		parts = append(parts, cur)
	}
	return parts
}

func (c *pageChrome) setFill(col rgb)   { c.pdf.SetFillColor(col.R, col.G, col.B) }
func (c *pageChrome) setStroke(col rgb) { c.pdf.SetStrokeColor(col.R, col.G, col.B) }
func (c *pageChrome) setText(col rgb)   { c.pdf.SetTextColor(col.R, col.G, col.B) }

// text draws s at (x, y), switching to the fallback font for runs the
// primary font has no glyphs for — nothing is silently dropped.
func (c *pageChrome) text(x, y float64, s string) error {
	s = norms(s)
	runs := splitFontRuns(s, c.primaryCov, c.fallbackCov)
	if len(runs) == 1 && !runs[0].fallback {
		c.pdf.SetXY(x, y)
		return c.pdf.Text(runs[0].text)
	}
	cx := x
	for _, run := range runs {
		if err := c.applyFont(run.fallback); err != nil {
			return err
		}
		c.pdf.SetXY(cx, y)
		if err := c.pdf.Text(run.text); err != nil {
			return err
		}
		w, err := c.pdf.MeasureTextWidth(run.text)
		if err != nil {
			return err
		}
		cx += w
	}
	return c.applyFont(false)
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
	return r.drawFooter(num, r.total)
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

func (c *pageChrome) drawBackground() error {
	c.pdf.UseImportedTemplate(c.bgTpl, 0, 0, c.w, c.h)
	return nil
}

func (c *pageChrome) drawHeader() error {
	if err := c.pdf.ImageByHolder(c.logo, pageMargin, pageMargin-6, &gopdf.Rect{
		W: logoHeight * logoAspect, H: logoHeight,
	}); err != nil {
		return fmt.Errorf("draw logo: %w", err)
	}

	// Generated-at, right aligned.
	if err := c.setFont(styleNormal, fontMeta); err != nil {
		return err
	}
	c.setText(colorMuted)
	stamp := "Erstellt: " + GeneratedAtLabel(c.generatedAt)
	sw, err := c.measure(stamp)
	if err != nil {
		return err
	}
	if err := c.text(c.w-pageMargin-sw, pageMargin+6, stamp); err != nil {
		return err
	}

	y := pageMargin + 32

	// Eyebrow — only for two-part titles; single-part titles render the
	// headline alone instead of repeating themselves.
	if eyebrow := titleEyebrow(c.title); eyebrow != "" {
		if err := c.setFont(styleBold, fontEyebrow); err != nil {
			return err
		}
		c.setText(colorEyebrow)
		if err := c.text(pageMargin, y, strings.ToUpper(spaceOut(eyebrow))); err != nil {
			return err
		}
		y += 16
	}

	// Title.
	if err := c.setFont(styleBold, fontTitle); err != nil {
		return err
	}
	c.setText(colorInk)
	if err := c.text(pageMargin, y, titleHeadline(c.title)); err != nil {
		return err
	}
	y += 14

	// Subtitle.
	if c.subtitle != "" {
		if err := c.setFont(styleNormal, fontSubtitle); err != nil {
			return err
		}
		c.setText(colorMuted)
		if err := c.text(pageMargin, y, c.subtitle); err != nil {
			return err
		}
		y += 14
	}

	// Filter pills.
	return c.drawFilterPills(pageMargin, y)
}

// layoutPills packs the filter labels into rows that fit between the
// page margins; a label too long for even a full row is pre-wrapped into
// continuation chunks. Computed once — every page draws the same layout,
// and bodyTop() reserves space for the extra rows.
func (c *pageChrome) layoutPills() error {
	if len(c.filters) == 0 {
		return nil
	}
	if err := c.setFont(styleNormal, fontMeta); err != nil {
		return err
	}
	usable := c.w - 2*pageMargin
	rows := [][]string{}
	row := []string{}
	rowW := 0.0
	for _, f := range c.filters {
		for _, chunk := range c.wrap(norms(f), usable-14) {
			tw, err := c.measure(chunk)
			if err != nil {
				return err
			}
			pw := tw + 14
			if rowW+pw > usable && len(row) > 0 {
				rows = append(rows, row)
				row, rowW = []string{}, 0
			}
			row = append(row, chunk)
			rowW += pw + 6
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	c.pillRows = rows
	return nil
}

func (c *pageChrome) drawFilterPills(x, y float64) error {
	if len(c.pillRows) == 0 {
		return nil
	}
	if err := c.setFont(styleNormal, fontMeta); err != nil {
		return err
	}
	for _, row := range c.pillRows {
		cx := x
		for _, label := range row {
			tw, err := c.measure(label)
			if err != nil {
				return err
			}
			pw := tw + 14
			c.setFill(colorSurface)
			c.setStroke(colorBorder)
			c.pdf.SetLineWidth(0.5)
			if err := c.pdf.Rectangle(cx, y-2, cx+pw, y+12, "FD", pillRadius, 8); err != nil {
				return err
			}
			c.setText(colorMuted)
			if err := c.text(cx+7, y+8, label); err != nil {
				return err
			}
			cx += pw + 6
		}
		y += pillRowH
	}
	return nil
}

func (r *designRenderer) drawCard(p designPage) error {
	top := r.bodyTop()
	left := pageMargin
	right := r.w - pageMargin

	// The card hugs its content (as the guide's cards do) instead of
	// stretching to the page foot, so a short class list doesn't render as
	// a mostly-empty sheet.
	// rowHeight measures with the *current* font — pin it to the body font
	// before measuring, or the pill font (set in drawHeader) is used.
	if err := r.setFont(styleNormal, fontBody); err != nil {
		return err
	}
	content := 2*cardPadY + r.tableHeaderHeight()
	if p.groupTitle != "" {
		content += fontGroup + groupGap
	}
	for _, row := range p.rows {
		content += row.height()
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
		if err := r.setFont(styleBold, fontGroup); err != nil {
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
	// Labels wrap to their column width, top-aligned in the band.
	hh := r.tableHeaderHeight()
	if err := r.setFont(styleNormal, fontTableHd); err != nil {
		return err
	}
	r.setText(colorHeaderText)
	cx := textLeft
	for i, col := range r.cols {
		ty := y + cellPadY + fontTableHd
		for _, ln := range r.wrap(col.Label, r.widths[i]-cellPadX) {
			if err := r.text(cx, ty, ln); err != nil {
				return err
			}
			ty += rowLineHt
		}
		cx += r.widths[i]
	}
	y += hh
	r.setStroke(colorHeaderRule)
	r.pdf.SetLineWidth(0.7)
	r.pdf.Line(textLeft, y, textRight, y)

	// Body rows.
	if err := r.setFont(styleNormal, fontBody); err != nil {
		return err
	}
	for _, row := range p.rows {
		cx = textLeft
		r.setText(colorBody)
		for i := range r.cols {
			ty := y + cellPadY + rowLineHt - 2
			for _, ln := range row.cells[i] {
				if err := r.text(cx, ty, ln); err != nil {
					return err
				}
				ty += rowLineHt
			}
			cx += r.widths[i]
		}
		y += row.height()
		r.setStroke(colorRowLine)
		r.pdf.SetLineWidth(0.5)
		r.pdf.Line(textLeft, y, textRight, y)
	}
	return nil
}

func (c *pageChrome) drawFooter(num, total int) error {
	if err := c.setFont(styleNormal, fontFooter); err != nil {
		return err
	}
	c.setText(colorMuted)
	y := c.h - pageMargin + 4

	// Confidentiality note, centred. The old hand-rolled table writer
	// dropped doc.Footer entirely — this path honours it (as
	// xlsx.go/docx.go already do).
	if c.footer != "" {
		fw, err := c.measure(norms(c.footer))
		if err != nil {
			return err
		}
		if err := c.text((c.w-fw)/2, y, c.footer); err != nil {
			return err
		}
	}

	page := fmt.Sprintf("Seite %d von %d", num, total)
	pw, err := c.measure(page)
	if err != nil {
		return err
	}
	return c.text(c.w-pageMargin-pw, y, page)
}

// titleEyebrow derives the small green kicker above the title from a
// two-part title ("Kinderliste — OGS Wochenübersicht" → "Kinderliste").
// Production titles are usually single-part ("Klassenliste", user-supplied
// req.Title) — those get NO eyebrow rather than repeating the title.
func titleEyebrow(title string) string {
	title = strings.TrimSpace(title)
	for _, sep := range []string{" — ", " – ", " - "} {
		if i := strings.Index(title, sep); i > 0 {
			return strings.TrimSpace(title[:i])
		}
	}
	return ""
}

// titleHeadline is the title minus the eyebrow part, so the two don't
// repeat each other ("Kinderliste — OGS Wochenübersicht" → "OGS
// Wochenübersicht").
func titleHeadline(title string) string {
	title = strings.TrimSpace(title)
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
