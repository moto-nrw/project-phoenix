package listexport

// The letter layout (#4669): one listrecords.Letter on A4 portrait pages in
// the layout of its sender, not in moto's list design. A thin band in the
// sender's colour, the logo top left and the letterhead top right, the
// address field and the facts beside it at the height DIN 5008 puts them,
// then subject, salutation, body and closing. The footer repeats on every
// page; a body that does not fit continues on the next page.
//
// Like the record layout it runs twice: a measuring pass counts the pages
// and records the trace the tests assert against, the drawing pass paints.

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg" // logos are PNG or JPEG
	_ "image/png"
	"strings"

	listrecords "github.com/moto-nrw/project-phoenix/modules/documentrendering/records"
	"github.com/signintech/gopdf"
)

// Geometry of the letter in PDF points (A4 is 595 × 842).
const (
	letterMarginLeft  = 70.0  // 25 mm
	letterMarginRight = 57.0  // 20 mm
	letterTop         = 34.0  // top of logo and letterhead
	letterBand        = 6.0   // height of the colour band
	letterLogoMaxW    = 150.0 // logo box
	letterLogoMaxH    = 56.0
	letterSenderW     = 190.0 // width of the letterhead and the facts column
	letterAddressTop  = 127.0 // 45 mm: return line, then the address field
	letterAddressW    = 240.0
	letterFactsTop    = 142.0
	letterBodyTop     = 270.0 // first line of the subject on the first page
	letterNextTop     = 60.0  // top of the body on every further page
	letterFooterGap   = 14.0  // space between body and footer rule
	letterFooterLine  = 9.0
	letterFooterSize  = 7.0
	letterReturnSize  = 6.5
	letterSmallSize   = 8.5
	letterBodySize    = 10.0
	letterBodyLine    = 14.0
	letterSubjectSize = 11.5
	letterParagraph   = 9.0  // space after a paragraph
	letterFactGap     = 3.0  // space before a table
	letterHeadGap     = 12.0 // least space between the head and the subject
	letterFactsGap    = 10.0 // least space between the letterhead and the facts
)

type letterLayout struct {
	*pageChrome
	letter listrecords.Letter
	accent rgb
	logo   gopdf.ImageHolder
	logoW  float64
	logoH  float64
	y      float64
	// headBottom is the lowest line of the head; a long block of facts
	// pushes the subject down instead of running into it.
	headBottom float64
	// senderBottom is the line below the letterhead; the facts start
	// beneath it when the letterhead runs long.
	senderBottom float64
	// footerLines are the footer wrapped to the page width: a long line,
	// such as the bank account, wraps instead of losing its end.
	footerLines []string
	page        int
	total       int
	draw        bool
	trace       []string
}

// renderLetterPDF renders one letter.
func renderLetterPDF(letter listrecords.Letter) ([]byte, error) {
	chrome, err := newLetterChrome(letter.Title)
	if err != nil {
		return nil, err
	}
	measure, err := newLetterLayout(chrome, letter)
	if err != nil {
		return nil, err
	}
	if err := measure.run(); err != nil {
		return nil, err
	}
	paint := *measure
	paint.draw, paint.total, paint.trace = true, measure.page, nil
	if err := paint.run(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if _, err := chrome.pdf.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("write letter pdf: %w", err)
	}
	return buf.Bytes(), nil
}

// newLetterChrome starts a white A4 page set with the fonts of the list
// design and their glyph fallback; it draws no moto frame.
func newLetterChrome(title string) (*pageChrome, error) {
	size := *gopdf.PageSizeA4
	pdf := &gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: size})
	pdf.SetInfo(gopdf.PdfInfo{Title: norms(title)})
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
	return &pageChrome{pdf: pdf, w: size.W, h: size.H, title: title, primaryCov: primaryCov, fallbackCov: fallbackCov}, nil
}

func newLetterLayout(chrome *pageChrome, letter listrecords.Letter) (*letterLayout, error) {
	l := &letterLayout{pageChrome: chrome, letter: letter, accent: colorInk}
	if accent, ok := parseHexColor(letter.AccentColor); ok {
		l.accent = accent
	}
	if len(letter.Logo) > 0 {
		if _, _, ok := imageSize(letter.Logo); !ok {
			return nil, errors.New("letter logo: not a PNG or JPEG image")
		}
		holder, err := gopdf.ImageHolderByBytes(letter.Logo)
		if err != nil {
			return nil, fmt.Errorf("letter logo: %w", err)
		}
		l.logo = holder
		l.logoW, l.logoH = logoBox(letter.Logo)
	}
	return l, nil
}

// logoBox scales the logo into its box, keeping the aspect ratio.
func logoBox(data []byte) (float64, float64) {
	width, height, _ := imageSize(data)
	scale := min(letterLogoMaxW/width, letterLogoMaxH/height, 1)
	return width * scale, height * scale
}

// imageSize reads the pixel size of a PNG or JPEG image.
func imageSize(data []byte) (float64, float64, bool) {
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 {
		return 0, 0, false
	}
	return float64(config.Width), float64(config.Height), true
}

func (l *letterLayout) run() error {
	l.page, l.headBottom, l.senderBottom = 0, 0, 0
	if err := l.wrapFooter(); err != nil {
		return err
	}
	if err := l.newPage(); err != nil {
		return err
	}
	if err := l.writeHead(); err != nil {
		return err
	}
	l.y = max(letterBodyTop, l.headBottom+letterHeadGap)
	if err := l.writeLine(l.letter.Subject, styleBold, letterSubjectSize, l.accent); err != nil {
		return err
	}
	l.y += letterBodyLine
	if l.letter.Salutation != "" {
		if err := l.writeParagraph(l.letter.Salutation, styleNormal); err != nil {
			return err
		}
	}
	for _, block := range l.letter.Blocks {
		if err := l.writeBlock(block); err != nil {
			return err
		}
	}
	for _, line := range l.letter.Closing {
		if err := l.writeLine(line, styleNormal, letterBodySize, colorBody); err != nil {
			return err
		}
	}
	return nil
}

// emit draws in the drawing pass and records the text in both.
func (l *letterLayout) emit(x, y float64, s string) error {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	l.trace = append(l.trace, s)
	if !l.draw {
		return nil
	}
	return l.text(x, y, s)
}

func (l *letterLayout) style(style string, size float64, col rgb) error {
	if err := l.setFont(style, size); err != nil {
		return err
	}
	l.setText(col)
	return nil
}

func (l *letterLayout) contentWidth() float64 {
	return l.w - letterMarginLeft - letterMarginRight
}

// wrapFooter breaks the footer lines to the width of the page.
func (l *letterLayout) wrapFooter() error {
	if err := l.style(styleNormal, letterFooterSize, colorMuted); err != nil {
		return err
	}
	l.footerLines = nil
	for _, line := range l.letter.Footer {
		l.footerLines = append(l.footerLines, l.wrap(line, l.contentWidth())...)
	}
	return nil
}

func (l *letterLayout) bodyBottom() float64 {
	lines := float64(len(l.footerLines))
	return l.h - letterTop - lines*letterFooterLine - letterFooterGap
}

// newPage opens a page with the colour band and the footer; the head
// belongs to the first page only.
func (l *letterLayout) newPage() error {
	l.page++
	l.y = letterNextTop
	if l.draw {
		l.pdf.AddPage()
		l.setFill(l.accent)
		l.pdf.RectFromUpperLeftWithStyle(0, 0, l.w, letterBand, "F")
	}
	return l.writeFooter()
}

// ensure starts a new page when height does not fit above the footer.
func (l *letterLayout) ensure(height float64) error {
	if l.y+height <= l.bodyBottom() {
		return nil
	}
	return l.newPage()
}

// writeHead prints logo, letterhead, return line, address field and facts.
func (l *letterLayout) writeHead() error {
	if l.draw && l.logoW > 0 {
		if err := l.pdf.ImageByHolder(l.logo, letterMarginLeft, letterTop, &gopdf.Rect{W: l.logoW, H: l.logoH}); err != nil {
			return fmt.Errorf("letter logo: %w", err)
		}
	}
	if l.logoW > 0 {
		l.trace = append(l.trace, "[Logo]")
	}
	if err := l.writeSender(); err != nil {
		return err
	}
	if err := l.writeAddress(); err != nil {
		return err
	}
	return l.writeFacts()
}

func (l *letterLayout) writeSender() error {
	x := l.w - letterMarginRight - letterSenderW
	y := letterTop + letterSmallSize
	for i, line := range l.letter.Sender {
		style, col := styleNormal, colorMuted
		if i == 0 {
			style, col = styleBold, l.accent
		}
		if err := l.style(style, letterSmallSize, col); err != nil {
			return err
		}
		for _, wrapped := range l.wrap(line, letterSenderW) {
			if err := l.emit(x, y, wrapped); err != nil {
				return err
			}
			y += letterSmallSize + 3
		}
	}
	l.senderBottom = y
	return nil
}

// writeAddress prints the return line (the sender in one line) above the
// address field.
func (l *letterLayout) writeAddress() error {
	y := letterAddressTop
	if len(l.letter.Sender) > 0 {
		if err := l.style(styleNormal, letterReturnSize, colorMuted); err != nil {
			return err
		}
		returnLine := l.fitOneLine(strings.Join(nonEmpty(l.letter.Sender), " · "), letterAddressW)
		if err := l.emit(letterMarginLeft, y, returnLine); err != nil {
			return err
		}
	}
	y += 20
	if err := l.style(styleNormal, letterBodySize, colorBody); err != nil {
		return err
	}
	for _, line := range l.letter.Recipient {
		if err := l.emit(letterMarginLeft, y, l.fitOneLine(line, letterAddressW)); err != nil {
			return err
		}
		y += letterBodyLine
	}
	l.headBottom = max(l.headBottom, y)
	return nil
}

func (l *letterLayout) writeFacts() error {
	x := l.w - letterMarginRight - letterSenderW
	y := max(letterFactsTop, l.senderBottom+letterFactsGap)
	for _, fact := range l.letter.Facts {
		if err := l.style(styleNormal, letterReturnSize+0.5, colorMuted); err != nil {
			return err
		}
		if err := l.emit(x, y, fact.Label); err != nil {
			return err
		}
		y += letterSmallSize + 1
		if err := l.style(styleNormal, letterSmallSize+0.5, colorBody); err != nil {
			return err
		}
		for _, line := range l.wrapCapped(fact.Value, letterSenderW, 2) {
			if err := l.emit(x, y, line); err != nil {
				return err
			}
			y += letterSmallSize + 3
		}
		y += 3
	}
	l.headBottom = max(l.headBottom, y)
	return nil
}

// writeLine prints one line of the body, wrapped, and moves below it.
func (l *letterLayout) writeLine(s, style string, size float64, col rgb) error {
	if err := l.style(style, size, col); err != nil {
		return err
	}
	for _, line := range l.wrap(s, l.contentWidth()) {
		if err := l.ensure(letterBodyLine); err != nil {
			return err
		}
		if err := l.style(style, size, col); err != nil {
			return err
		}
		if err := l.emit(letterMarginLeft, l.y+size, line); err != nil {
			return err
		}
		l.y += letterBodyLine
	}
	return nil
}

// writeParagraph prints a paragraph with the space after it. A blank line
// in the text keeps its height.
func (l *letterLayout) writeParagraph(s, style string) error {
	for _, segment := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		if strings.TrimSpace(segment) == "" {
			l.y += letterBodyLine
			continue
		}
		if err := l.writeLine(segment, style, letterBodySize, colorBody); err != nil {
			return err
		}
	}
	l.y += letterParagraph
	return nil
}

func (l *letterLayout) writeBlock(block listrecords.LetterBlock) error {
	if block.Heading != "" {
		if err := l.ensure(3 * letterBodyLine); err != nil {
			return err
		}
		if err := l.writeLine(block.Heading, styleBold, letterBodySize, l.accent); err != nil {
			return err
		}
	}
	if strings.TrimSpace(block.Text) != "" {
		style := styleNormal
		if block.Bold {
			style = styleBold
		}
		if err := l.writeParagraph(block.Text, style); err != nil {
			return err
		}
	}
	if len(block.Facts) == 0 {
		return nil
	}
	l.y += letterFactGap - letterParagraph
	for _, fact := range block.Facts {
		if err := l.writeFactRow(fact); err != nil {
			return err
		}
	}
	l.y += letterParagraph + letterFactGap
	return nil
}

// writeFactRow prints a label on the left and its value right-aligned; a
// long label wraps beside the value.
func (l *letterLayout) writeFactRow(fact listrecords.LetterFact) error {
	style := styleNormal
	if fact.Bold {
		style = styleBold
	}
	if err := l.style(style, letterBodySize, colorBody); err != nil {
		return err
	}
	valueW, err := l.measure(fact.Value)
	if err != nil {
		return err
	}
	labelW := max(l.contentWidth()-valueW-16, l.contentWidth()/2)
	lines := l.wrap(fact.Label, labelW)
	if err := l.ensure(float64(len(lines)) * letterBodyLine); err != nil {
		return err
	}
	if err := l.style(style, letterBodySize, colorBody); err != nil {
		return err
	}
	if fact.Bold && l.draw {
		l.setStroke(colorBorder)
		l.pdf.Line(letterMarginLeft, l.y, l.w-letterMarginRight, l.y)
	}
	if err := l.emit(l.w-letterMarginRight-valueW, l.y+letterBodySize, fact.Value); err != nil {
		return err
	}
	for _, line := range lines {
		if err := l.emit(letterMarginLeft, l.y+letterBodySize, line); err != nil {
			return err
		}
		l.y += letterBodyLine
	}
	return nil
}

// writeFooter prints the footer lines below a hairline at the foot of the
// page, and the page number when the letter has more than one page.
func (l *letterLayout) writeFooter() error {
	if err := l.style(styleNormal, letterFooterSize, colorMuted); err != nil {
		return err
	}
	y := l.bodyBottom() + letterFooterGap/2
	if l.draw && len(l.footerLines) > 0 {
		l.setStroke(colorBorder)
		l.pdf.Line(letterMarginLeft, y, l.w-letterMarginRight, y)
	}
	for _, line := range l.footerLines {
		y += letterFooterLine
		if err := l.emit(letterMarginLeft, y, line); err != nil {
			return err
		}
	}
	if !l.draw || l.total < 2 {
		return nil
	}
	label := fmt.Sprintf("Seite %d von %d", l.page, l.total)
	width, err := l.measure(label)
	if err != nil {
		return err
	}
	return l.text(l.w-letterMarginRight-width, l.h-letterTop/2, label)
}

func nonEmpty(lines []string) []string {
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			kept = append(kept, line)
		}
	}
	return kept
}
