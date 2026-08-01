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
	"strconv"
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
type styledLine struct {
	text   string
	style  LineStyle
	accent string
}

type renderRow struct {
	cells [][]styledLine // per column: wrapped, styled lines
	lines int            // max line count across cells
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
	// gutter is the left inset every body cell reserves for the accent bar.
	// It is derived from the document rather than passed in: the width has to
	// be the same for every cell, and a caller-supplied flag is one more thing
	// that can disagree with the rows it describes.
	gutter float64
}

const (
	accentBarW  = 2.5
	accentInset = 7.0
)

// documentHasAccents reports whether any cell opens an accent group.
func documentHasAccents(rows []Row) bool {
	for _, row := range rows {
		for _, value := range row.Values {
			if strings.Contains(value, accentMark) {
				return true
			}
		}
	}
	return false
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
	if documentHasAccents(doc.Rows) {
		r.gutter = accentInset
	}
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
		case ColumnPlanRowLabel:
			// A plan matrix inverts the child-list balance: the row label is
			// one name or one area, the day cells carry everything else.
			weight = 1.1
		case ColumnPlanMonday, ColumnPlanTuesday, ColumnPlanWednesday, ColumnPlanThursday,
			ColumnPlanFriday, ColumnPlanSaturday, ColumnPlanSunday:
			weight = 1.6
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

// headerBottom is the y just past the header block, summing the advances
// drawHeader() actually makes: an eyebrow only exists for two-part titles,
// a subtitle and filter pills only when the caller supplied them. This
// used to be a flat 104pt reserve sized for the fullest possible header,
// which left ~2cm unused on every "Tagesliste" (no eyebrow, often no
// filters) — the body simply started lower than the header ended.
func (c *pageChrome) headerBottom() float64 {
	return c.headerTop() + float64(len(c.pillRows))*pillRowH
}

// headerTop is headerBottom() without the filter pills — the y the pills
// start at, and the fixed part layoutPills sizes its row budget against.
func (c *pageChrome) headerTop() float64 {
	y := pageMargin + headerFirstY
	if titleEyebrow(c.title) != "" {
		y += headerEyebrowAdvance
	}
	y += headerTitleAdvance
	if c.subtitle != "" {
		y += headerSubtitleAdvance
	}
	return y
}

// maxPillRows is how many pill rows the header may grow by before the body
// shrinks below minBodyHeight. At least one row is always allowed.
func (c *pageChrome) maxPillRows() int {
	n := int((c.bodyBottom() - minBodyHeight - headerBodyGap - c.headerTop()) / pillRowH)
	if n < 1 {
		n = 1
	}
	return n
}

// bodyTop is the y where page content starts, below the header block;
// bodyBottom is the last usable y above the footer.
func (c *pageChrome) bodyTop() float64    { return c.headerBottom() + headerBodyGap }
func (c *pageChrome) bodyBottom() float64 { return c.h - pageMargin - footerHeight }

// groupChromeHeight is the vertical cost of a group heading inside the
// card, or 0 for an ungrouped document. Grouped documents start every
// group on a fresh page and carry the title onto continuation pages, so
// every page of a grouped document pays it and no page of an ungrouped
// one does — which is exactly what drawCard draws.
func (r *designRenderer) groupChromeHeight() float64 {
	for _, row := range r.doc.Rows {
		if row.GroupTitle != "" {
			return fontGroup + groupGap
		}
	}
	return 0
}

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
	// The heading is only deducted when the document actually has groups —
	// an ungrouped list (the Tagesliste builds its rows without a
	// GroupTitle) had 24pt reserved on every page for a heading drawCard
	// never draws, which cost a full row per page.
	avail := r.bodyBottom() - (r.bodyTop() + 2*cardPadY + r.groupChromeHeight() + r.tableHeaderHeight())
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

	// A page is only kept once it holds a body row: a group marker followed
	// by another marker (or by nothing) would otherwise print a heading-only
	// sheet.
	flush := func() {
		if wroteBody {
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

// buildRow wraps every cell of row to its column width, keeping each line's
// style. A segment is wrapped with its own font set, so a bold line is
// measured as bold and cannot overrun its column.
func (r *designRenderer) buildRow(row Row) renderRow {
	rr := renderRow{cells: make([][]styledLine, len(r.cols)), lines: 1}
	for i, col := range r.cols {
		rr.cells[i] = r.wrapStyled(norms(row.Values[col.ID]), r.widths[i]-cellPadX-r.gutter)
		if n := len(rr.cells[i]); n > rr.lines {
			rr.lines = n
		}
	}
	return rr
}

// wrapStyled splits a cell into its styled lines and wraps each to maxW.
func (r *designRenderer) wrapStyled(cell string, maxW float64) []styledLine {
	cell = strings.ReplaceAll(strings.ReplaceAll(cell, "\r\n", "\n"), "\r", "\n")
	out := []styledLine{}
	for _, segment := range strings.Split(cell, "\n") {
		decoded, text := DecodeLine(segment)
		// Measure in the face the line will be drawn in; a bold run is wider
		// than the same string in the regular face.
		if err := r.setFont(fontStyleFor(decoded.Style), fontBody); err != nil {
			return []styledLine{{text: text}}
		}
		for i, line := range r.wrapSegment(text, maxW) {
			wrapped := styledLine{text: line, style: decoded.Style}
			// Only the first wrapped line opens the accent group; the bar
			// then runs on until the next accented line, so a heading that
			// wraps does not restart its own bar.
			if i == 0 {
				wrapped.accent = decoded.Accent
			}
			out = append(out, wrapped)
		}
	}
	// Restore the body face so callers that measure afterwards are unaffected.
	_ = r.setFont(styleNormal, fontBody)
	if len(out) == 0 {
		return []styledLine{{}}
	}
	return out
}

func fontStyleFor(style LineStyle) string {
	if style == LineStrong {
		return styleBold
	}
	return styleNormal
}

func lineColorFor(style LineStyle) rgb {
	if style == LineMuted {
		return colorMuted
	}
	return colorBody
}

// splitRenderRow slices a row taller than maxLines into continuation
// rows of at most maxLines wrapped lines each — no line is dropped.
func splitRenderRow(rr renderRow, maxLines int) []renderRow {
	if rr.lines <= maxLines {
		return []renderRow{rr}
	}
	out := []renderRow{}
	for start := 0; start < rr.lines; start += maxLines {
		part := renderRow{cells: make([][]styledLine, len(rr.cells)), lines: 1}
		for i, lines := range rr.cells {
			end := start + maxLines
			if end > len(lines) {
				end = len(lines)
			}
			if start < len(lines) {
				part.cells[i] = append([]styledLine(nil), lines[start:end]...)
				if start > 0 && part.cells[i][0].accent == "" {
					for previous := start - 1; previous >= 0; previous-- {
						if lines[previous].accent != "" {
							part.cells[i][0].accent = lines[previous].accent
							break
						}
					}
				}
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

// maxHeaderLines caps how many wrapped lines the table header may occupy so
// the header band always leaves room for at least one body row. Column
// labels are administratively configured and unbounded: a label long enough
// to wrap past this cap in a narrow column would push the header past the
// footer and clip every row drawn beneath it (avail in paginate() goes
// negative). The header wraps freely up to the cap and only truncates in
// that pathological case — a normal header stays far below it.
func (r *designRenderer) maxHeaderLines() int {
	// The card region below the header band that the header and the first
	// body row must share (mirrors paginate()'s avail decomposition).
	region := r.bodyBottom() - r.bodyTop() - 2*cardPadY - r.groupChromeHeight()
	// Reserve the header band's own vertical padding (2*cellPadY + 2) plus one
	// body row (rowLineHt + 2*cellPadY); the rest is the header's line budget.
	n := int((region - (2*cellPadY + 2) - (rowLineHt + 2*cellPadY)) / rowLineHt)
	if n < 1 {
		n = 1
	}
	return n
}

func (r *designRenderer) headerLineCount() int {
	// Measure with the header font, then RESTORE the caller's font —
	// leaking fontTableHd into paginate() made body cells wrap narrower
	// in the measuring pass than in the drawing pass (lost lines).
	prevStyle, prevSize := r.curStyle, r.curSize
	_ = r.setFont(styleNormal, fontTableHd)
	lineCap := r.maxHeaderLines()
	maxLines := 1
	for i, col := range r.cols {
		if n := len(r.wrapCapped(col.Label, r.widths[i]-cellPadX, lineCap)); n > maxLines {
			maxLines = n
		}
	}
	if prevSize > 0 {
		_ = r.setFont(prevStyle, prevSize)
	}
	return maxLines
}

// wrap breaks text to fit maxW, honouring explicit newlines first and
// greedily wrapping each resulting segment. Callers that compose a cell
// from several facts (a plan cell is "07:30–14:00", the task, the room)
// separate them with "\n" and get one line each; everything else behaves
// exactly as before, since no other caller emits newlines.
func (c *pageChrome) wrap(s string, maxW float64) []string {
	// Normalize the line-break spellings a caller can realistically produce
	// before splitting, so a CRLF does not leave a stray \r inside a line.
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if strings.TrimSpace(s) == "" {
		return []string{""}
	}
	if !strings.Contains(s, "\n") {
		return c.wrapSegment(s, maxW)
	}
	lines := []string{}
	for _, segment := range strings.Split(s, "\n") {
		// An empty segment is a deliberate blank line (a cell ending in "\n"
		// would otherwise silently lose its trailing break). wrapSegment
		// returns []string{""} for it, which is exactly one blank line.
		lines = append(lines, c.wrapSegment(segment, maxW)...)
	}
	return lines
}

// wrapSegment greedily breaks one newline-free run of text to fit maxW,
// measured with the current font. A single word wider than maxW (long German
// compounds — "Schmetterlingsgruppe" in a narrow column) is hard-split at
// character level so no cell ever overflows into its neighbour.
func (c *pageChrome) wrapSegment(s string, maxW float64) []string {
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

// wrapCapped wraps like wrap() but never returns more than maxLines lines.
// When the label needs more, the last kept line ends in an ellipsis so the
// dropped text reads as deliberately cut, not silently gone — the same "no
// silent content loss" contract the body cells hold, applied to a header band
// that must stay bounded so it cannot draw past the footer.
func (c *pageChrome) wrapCapped(s string, maxW float64, maxLines int) []string {
	if maxLines < 1 {
		maxLines = 1
	}
	lines := c.wrap(s, maxW)
	if len(lines) <= maxLines {
		return lines
	}
	lines = lines[:maxLines]
	lines[maxLines-1] = c.ellipsize(lines[maxLines-1], maxW)
	return lines
}

// fitOneLine returns s unchanged when it already fits maxW, otherwise the
// longest prefix that fits with a trailing ellipsis. The header title band
// (eyebrow / headline / subtitle) reserves exactly one line each, so a
// user-supplied req.Title long enough to overrun the page is cut deliberately
// here rather than clipped at the page edge — the same "no silent content
// loss" contract wrapCapped applies to the table header, kept single-line so
// the header reserve TestDesignedPDFHeaderReserveMatchesDrawnHeader guards
// cannot drift. The active font must already be set (measure() reads it).
func (c *pageChrome) fitOneLine(s string, maxW float64) string {
	if wdt, err := c.measure(s); err == nil && wdt <= maxW {
		return s
	}
	return c.ellipsize(s, maxW)
}

// ellipsize trims runes off the end of line until it plus a trailing ellipsis
// fits maxW. A wrapped line already fills close to maxW, so the appended
// ellipsis usually needs a rune or two dropped to fit.
func (c *pageChrome) ellipsize(line string, maxW float64) string {
	runes := []rune(strings.TrimRight(line, " "))
	for {
		candidate := string(runes) + "…"
		if wdt, err := c.measure(candidate); err != nil || wdt <= maxW || len(runes) == 0 {
			return candidate
		}
		runes = runes[:len(runes)-1]
	}
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
	if _, err := r.drawHeader(); err != nil {
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

// drawHeader draws the header block and returns the y it ended at, which
// headerBottom() predicts without drawing. Every advance below must use
// the header* constants so the prediction cannot drift from the drawing —
// TestDesignedPDFHeaderReserveMatchesDrawnHeader fails if it does.
func (c *pageChrome) drawHeader() (float64, error) {
	if err := c.pdf.ImageByHolder(c.logo, pageMargin, pageMargin-6, &gopdf.Rect{
		W: logoHeight * logoAspect, H: logoHeight,
	}); err != nil {
		return 0, fmt.Errorf("draw logo: %w", err)
	}

	// Generated-at, right aligned.
	if err := c.setFont(styleNormal, fontMeta); err != nil {
		return 0, err
	}
	c.setText(colorMuted)
	stamp := "Erstellt: " + GeneratedAtLabel(c.generatedAt)
	sw, err := c.measure(stamp)
	if err != nil {
		return 0, err
	}
	if err := c.text(c.w-pageMargin-sw, pageMargin+6, stamp); err != nil {
		return 0, err
	}

	y := pageMargin + headerFirstY
	usable := c.w - 2*pageMargin

	// Eyebrow — only for two-part titles; single-part titles render the
	// headline alone instead of repeating themselves.
	if eyebrow := titleEyebrow(c.title); eyebrow != "" {
		if err := c.setFont(styleBold, fontEyebrow); err != nil {
			return 0, err
		}
		c.setText(colorEyebrow)
		if err := c.text(pageMargin, y, c.fitOneLine(strings.ToUpper(spaceOut(eyebrow)), usable)); err != nil {
			return 0, err
		}
		y += headerEyebrowAdvance
	}

	// Title.
	if err := c.setFont(styleBold, fontTitle); err != nil {
		return 0, err
	}
	c.setText(colorInk)
	if err := c.text(pageMargin, y, c.fitOneLine(titleHeadline(c.title), usable)); err != nil {
		return 0, err
	}
	y += headerTitleAdvance

	// Subtitle.
	if c.subtitle != "" {
		if err := c.setFont(styleNormal, fontSubtitle); err != nil {
			return 0, err
		}
		c.setText(colorMuted)
		if err := c.text(pageMargin, y, c.fitOneLine(c.subtitle, usable)); err != nil {
			return 0, err
		}
		y += headerSubtitleAdvance
	}

	// Filter pills.
	if err := c.drawFilterPills(pageMargin, y); err != nil {
		return 0, err
	}
	return y + float64(len(c.pillRows))*pillRowH, nil
}

// pillChunk is one drawn pill: a filter label, or one continuation chunk
// of a label too long for a full row. filter is the index of the label it
// came from, which is what the overflow marker counts.
type pillChunk struct {
	text   string
	width  float64 // pill width, text plus both insets
	filter int
}

// layoutPills packs the filter labels into rows that fit between the page
// margins; a label too long for even a full row is pre-wrapped into
// continuation chunks. Computed once — every page draws the same layout,
// and bodyTop() reserves space for the extra rows.
//
// The row count is capped at maxPillRows: filter text is unbounded (the
// care-usage export joins every care-offering name into one label) and an
// uncapped header eats the whole page body. Labels past the cap are
// summarised in a trailing "+N weitere Filter" pill rather than dropped
// without a trace.
func (c *pageChrome) layoutPills() error {
	if len(c.filters) == 0 {
		return nil
	}
	if err := c.setFont(styleNormal, fontMeta); err != nil {
		return err
	}
	usable := c.w - 2*pageMargin

	chunks, err := c.pillChunks(usable)
	if err != nil {
		return err
	}
	rows, kept := packPills(chunks, usable, c.maxPillRows())
	if kept < len(chunks) {
		rows, err = c.markPillOverflow(rows, chunks, kept, usable)
		if err != nil {
			return err
		}
	}
	c.pillRows = rows
	return nil
}

// pillChunks wraps every filter label to at most one full row.
func (c *pageChrome) pillChunks(usable float64) ([]pillChunk, error) {
	chunks := []pillChunk{}
	for i, f := range c.filters {
		for _, text := range c.wrap(norms(f), usable-2*pillPadX) {
			w, err := c.pillWidth(text)
			if err != nil {
				return nil, err
			}
			chunks = append(chunks, pillChunk{text: text, width: w, filter: i})
		}
	}
	return chunks, nil
}

func (c *pageChrome) pillWidth(text string) (float64, error) {
	tw, err := c.measure(text)
	if err != nil {
		return 0, err
	}
	return tw + 2*pillPadX, nil
}

// packPills greedily fills at most maxRows rows and reports how many
// chunks made it in; the rest is the caller's overflow.
func packPills(chunks []pillChunk, usable float64, maxRows int) (rows [][]string, kept int) {
	row := []string{}
	rowW := 0.0
	for _, ch := range chunks {
		if rowW+ch.width > usable && len(row) > 0 {
			if len(rows) == maxRows-1 {
				break // the current row is the last one allowed
			}
			rows = append(rows, row)
			row, rowW = []string{}, 0
		}
		row = append(row, ch.text)
		rowW += ch.width + pillGap
		kept++
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}
	return rows, kept
}

// markPillOverflow appends the "+N weitere Filter" marker to the last row,
// trimming trailing pills until it fits. Each trim grows N, so the label is
// re-measured every round instead of going stale.
func (c *pageChrome) markPillOverflow(rows [][]string, chunks []pillChunk, kept int, usable float64) ([][]string, error) {
	last := rows[len(rows)-1]
	rowW := 0.0
	for _, ch := range chunks[kept-len(last) : kept] {
		rowW += ch.width + pillGap
	}
	for {
		label := pillOverflowLabel(hiddenFilters(chunks, kept, len(c.filters)))
		w, err := c.pillWidth(label)
		if err != nil {
			return nil, err
		}
		if rowW+w <= usable || len(last) == 0 {
			rows[len(rows)-1] = append(last, label)
			return rows, nil
		}
		kept--
		rowW -= chunks[kept].width + pillGap
		last = last[:len(last)-1]
	}
}

// hiddenFilters counts the filters no visible chunk stands for. A filter whose
// earlier chunks are still on the page is truncated, not hidden, so it must not
// be counted — one long label alone never means a second filter exists.
func hiddenFilters(chunks []pillChunk, kept, total int) int {
	first := chunks[kept].filter
	if kept > 0 && chunks[kept-1].filter == first {
		first++
	}
	return total - first
}

// pillOverflowLabel names the filters the header had no room for. Zero hidden
// filters means every filter is on the page but the last one lost a chunk.
func pillOverflowLabel(n int) string {
	switch {
	case n <= 0:
		return "Filtertext gekürzt"
	case n == 1:
		return "+1 weiterer Filter"
	default:
		return fmt.Sprintf("+%d weitere Filter", n)
	}
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
			pw, err := c.pillWidth(label)
			if err != nil {
				return err
			}
			c.setFill(colorSurface)
			c.setStroke(colorBorder)
			c.pdf.SetLineWidth(0.5)
			if err := c.pdf.Rectangle(cx, y-2, cx+pw, y+12, "FD", pillRadius, 8); err != nil {
				return err
			}
			c.setText(colorMuted)
			if err := c.text(cx+pillPadX, y+8, label); err != nil {
				return err
			}
			cx += pw + pillGap
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
	// Cap the drawn labels at the same budget the band was sized with, or a
	// label that wrapped past the cap would draw more lines than the header
	// reserved and spill onto the rows below.
	lineCap := r.maxHeaderLines()
	cx := textLeft
	for i, col := range r.cols {
		ty := y + cellPadY + fontTableHd
		for _, ln := range r.wrapCapped(col.Label, r.widths[i]-cellPadX, lineCap) {
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
		for i := range r.cols {
			top := y + cellPadY
			r.drawAccentBars(cx, top, row.cells[i])
			ty := top + rowLineHt - 2
			for _, ln := range row.cells[i] {
				if err := r.setFont(fontStyleFor(ln.style), fontBody); err != nil {
					return err
				}
				r.setText(lineColorFor(ln.style))
				if err := r.text(cx+r.gutter, ty, ln.text); err != nil {
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

// drawAccentBars paints one bar per accent group: it starts at the accented
// line and runs down to the line before the next accented one, so a shift and
// the tasks under it read as one block. Lines with no accent above them get
// no bar.
func (r *designRenderer) drawAccentBars(x, top float64, lines []styledLine) {
	if r.gutter == 0 {
		return
	}
	start, color := -1, rgb{}
	flush := func(end int) {
		if start < 0 {
			return
		}
		barTop := top + float64(start)*rowLineHt + 1
		barBottom := top + float64(end)*rowLineHt - 1
		r.setFill(color)
		r.setStroke(color)
		_ = r.pdf.Rectangle(x, barTop, x+accentBarW, barBottom, "F", 1, 4)
		start = -1
	}
	for i, ln := range lines {
		if ln.accent == "" {
			continue
		}
		parsed, ok := parseHexColor(ln.accent)
		if !ok {
			continue
		}
		flush(i)
		start, color = i, parsed
	}
	flush(len(lines))
}

// parseHexColor reads "#RRGGBB" and the shorthand "#RGB" — both shapes the
// colour columns accept (activities.Category, schedule.ShiftType,
// facilities.Room all validate against the same pattern). Anything else is
// ignored rather than guessed at.
func parseHexColor(value string) (rgb, bool) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) == 3 {
		// "#abc" means "#aabbcc": each digit stands for both nibbles.
		var expanded strings.Builder
		for _, digit := range value {
			expanded.WriteRune(digit)
			expanded.WriteRune(digit)
		}
		value = expanded.String()
	}
	if len(value) != 6 {
		return rgb{}, false
	}
	var out [3]uint8
	for i := range out {
		component, err := strconv.ParseUint(value[i*2:i*2+2], 16, 8)
		if err != nil {
			return rgb{}, false
		}
		out[i] = uint8(component)
	}
	return rgb{out[0], out[1], out[2]}, true
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
//
// It normalizes first and then splits on normalization boundaries, not on
// runes. Titles are user-supplied and may arrive decomposed (NFD, common
// from macOS): splitting "ü" per rune inserts a space between the
// base letter and its combining diaeresis, which the NFC pass in text()
// can no longer recompose — the accent then renders detached or drops out.
func spaceOut(s string) string {
	var it norm.Iter
	it.InitString(norm.NFC, norms(s))
	parts := []string{}
	for !it.Done() {
		parts = append(parts, string(it.Next()))
	}
	return strings.Join(parts, " ")
}
