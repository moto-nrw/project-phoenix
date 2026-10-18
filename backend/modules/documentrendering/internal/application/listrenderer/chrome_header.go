package listexport

import (
	"fmt"
	"strings"

	listrecords "github.com/moto-nrw/project-phoenix/modules/documentrendering/records"

	"github.com/signintech/gopdf"
	"golang.org/x/text/unicode/norm"
)

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
func (c *pageChrome) bodyTop() float64 { return c.headerBottom() + headerBodyGap }

func (c *pageChrome) bodyBottom() float64 { return c.h - pageMargin - footerHeight }

// drawHeader draws the header block and returns the y it ended at, which
// headerBottom() predicts without drawing. Every advance below must use
// the header* constants so the prediction cannot drift from the drawing —
// TestDesignedPDFHeaderReserveMatchesDrawnHeader fails if it does.
func (c *pageChrome) drawHeader() (float64, error) {
	if err := c.drawHeaderBrand(); err != nil {
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

func (c *pageChrome) drawHeaderBrand() error {
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
	stamp := "Erstellt: " + listrecords.GeneratedAtLabel(c.generatedAt)
	sw, err := c.measure(stamp)
	if err != nil {
		return err
	}
	if err := c.text(c.w-pageMargin-sw, pageMargin+6, stamp); err != nil {
		return err
	}

	return nil
}
