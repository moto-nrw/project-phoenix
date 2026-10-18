package listexport

import (
	"fmt"
	"strings"
	"time"

	"github.com/signintech/gopdf"
)

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
	for _, part := range c.segmentParts(s, maxW) {
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

func (c *pageChrome) setFill(col rgb) { c.pdf.SetFillColor(col.R, col.G, col.B) }

func (c *pageChrome) setStroke(col rgb) { c.pdf.SetStrokeColor(col.R, col.G, col.B) }

func (c *pageChrome) setText(col rgb) { c.pdf.SetTextColor(col.R, col.G, col.B) }

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

func (c *pageChrome) segmentParts(segment string, maxW float64) []string {
	var parts []string
	for _, word := range strings.Fields(segment) {
		parts = append(parts, c.splitLongWord(word, maxW)...)
	}
	return parts
}
