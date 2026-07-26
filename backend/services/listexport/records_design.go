package listexport

// The designed record/block renderer (issue #1568): RecordDocument as
// A4-portrait pages in the app's design language, one white card per
// record (mirroring the guide's step cards), drawn through the shared
// pageChrome frame.
//
// Layout rules carried over from the old hand-rolled record pager:
//   - a record that fits on one page is kept together (page-break before,
//     never mid-card); an oversized record flows across pages as a bare
//     block instead of a card
//   - each child sub-block is kept together when it fits a page
//   - group headings introduce sections; the page header and footer
//     repeat on every page
//   - empty documents render a placeholder line
//
// The layout runs twice: a measuring pass counts pages (the footer prints
// "Seite N von M", so M must be known before the first page is drawn) and
// records a text trace the tests assert against; the drawing pass then
// paints. Both passes share one code path — recordLayout.run — so they
// cannot disagree.

import (
	"bytes"
	"fmt"
	"strings"
)

// Vertical rhythm (PDF points) for the record card layout.
const (
	recCardPad       = 12.0 // card inner padding
	recTitleSize     = 11.0 // record heading (e.g. guardian name)
	recTitleToRule   = 8.0  // gap below the heading before its hairline
	recRuleToFields  = 12.0 // gap below the hairline before the first field
	recFieldLineH    = 11.0 // height of one (wrapped) field line
	recSubTopGap     = 9.0  // gap above each child sub-block
	recSubTitleSize  = 9.5  // child heading
	recSubTitleGap   = 6.0  // gap below the child heading before its fields
	recSubIndent     = 12.0 // sub-block indent inside the card
	recInterRecord   = 12.0 // gap between two record cards
	recInterGroup    = 18.0 // gap between two groups
	recGroupTitleGap = 12.0 // gap below a group heading
	recEmptyLine     = "Keine Anmeldungen vorhanden."
)

type recordLayout struct {
	*pageChrome
	doc   RecordDocument
	y     float64
	page  int
	total int  // known during the draw pass; 0 while measuring
	draw  bool // false = measuring/tracing pass

	// trace collects every text string the layout emits, in order. The
	// old writer's tests asserted content via string search in the raw
	// PDF bytes; compressed streams + subset fonts make that impossible,
	// so tests assert against this trace instead.
	trace []string
}

func renderRecordsPDFDesigned(doc RecordDocument) ([]byte, error) {
	chrome, err := newPageChrome(false, doc.Title, doc.Subtitle, doc.GeneratedAt, doc.Filters, doc.Footer)
	if err != nil {
		return nil, err
	}

	// Pass 1: measure — count pages, no drawing.
	measure := &recordLayout{pageChrome: chrome, doc: doc}
	if err := measure.run(); err != nil {
		return nil, err
	}

	// Pass 2: draw, with the total known.
	paint := &recordLayout{pageChrome: chrome, doc: doc, draw: true, total: measure.page}
	if err := paint.run(); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if _, err := chrome.pdf.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("write records pdf: %w", err)
	}
	return buf.Bytes(), nil
}

func (l *recordLayout) run() error {
	l.page = 0
	if err := l.newPage(); err != nil {
		return err
	}

	groups := recordDocumentGroups(l.doc)
	if recordGroupCount(groups) == 0 {
		if err := l.setFont(styleNormal, fontBody); err != nil {
			return err
		}
		l.setText(colorMuted)
		if err := l.emit(pageMargin, l.y+fontBody, recEmptyLine); err != nil {
			return err
		}
		return nil
	}

	for groupIdx, group := range groups {
		if len(group.Records) == 0 {
			continue
		}
		if groupIdx > 0 {
			l.y += recInterGroup
		}
		if err := l.writeGroupTitle(group.Title); err != nil {
			return err
		}
		for i, rec := range group.Records {
			if err := l.writeRecord(rec); err != nil {
				return err
			}
			if i < len(group.Records)-1 {
				l.y += recInterRecord
			}
		}
	}
	return nil
}

// emit draws text at (x, baselineY) in draw mode and always records it in
// the trace. norms() is applied by pageChrome.text.
func (l *recordLayout) emit(x, y float64, s string) error {
	l.trace = append(l.trace, s)
	if !l.draw {
		return nil
	}
	return l.text(x, y, s)
}

func (l *recordLayout) newPage() error {
	l.page++
	l.y = l.bodyTop()
	if !l.draw {
		return nil
	}
	l.pdf.AddPage()
	if err := l.drawBackground(); err != nil {
		return err
	}
	if _, err := l.drawHeader(); err != nil {
		return err
	}
	return l.drawFooter(l.page, l.total)
}

// ensureSpace opens a new page when fewer than `needed` points remain.
func (l *recordLayout) ensureSpace(needed float64) error {
	if l.y+needed > l.bodyBottom() {
		return l.newPage()
	}
	return nil
}

// maxContent is the tallest block that can be kept together on one page.
func (l *recordLayout) maxContent() float64 {
	return l.bodyBottom() - l.bodyTop()
}

func (l *recordLayout) writeGroupTitle(title string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil
	}
	if err := l.ensureSpace(fontGroup + recGroupTitleGap + recTitleSize); err != nil {
		return err
	}
	if l.draw {
		if err := l.setFont(styleBold, fontGroup); err != nil {
			return err
		}
		l.setText(colorInk)
	}
	if err := l.emit(pageMargin, l.y+fontGroup, title); err != nil {
		return err
	}
	l.y += fontGroup + recGroupTitleGap
	return nil
}

// writeRecord draws one record. A record that fits a page renders as a
// kept-together card; an oversized one falls back to a bare flowing block
// (the card surface cannot span a page break).
func (l *recordLayout) writeRecord(rec Record) error {
	h := l.recordHeight(rec)
	if h <= l.maxContent() {
		if err := l.ensureSpace(h); err != nil {
			return err
		}
		return l.drawRecordCard(rec, h)
	}
	return l.flowRecordBare(rec)
}

// drawRecordCard paints the card surface at its measured height, then the
// record content inside it.
func (l *recordLayout) drawRecordCard(rec Record, h float64) error {
	if l.draw {
		l.setFill(colorSurface)
		l.setStroke(colorBorder)
		l.pdf.SetLineWidth(0.6)
		if err := l.pdf.Rectangle(pageMargin, l.y, l.w-pageMargin, l.y+h, "FD", cardRadius, 12); err != nil {
			return err
		}
	}

	inner := pageMargin + recCardPad
	l.y += recCardPad
	if err := l.writeRecordContent(rec, inner, false); err != nil {
		return err
	}
	l.y += recCardPad
	return nil
}

// flowRecordBare streams an oversized record across pages: heading with
// hairline, fields line by line, sub-blocks still kept together when they
// fit a page.
func (l *recordLayout) flowRecordBare(rec Record) error {
	if err := l.ensureSpace(recTitleSize + recTitleToRule + recRuleToFields + recFieldLineH); err != nil {
		return err
	}
	return l.writeRecordContent(rec, pageMargin, true)
}

// writeRecordContent renders heading, hairline, fields, and sub-blocks at
// the given left edge. paged=true allows page breaks between lines.
func (l *recordLayout) writeRecordContent(rec Record, left float64, paged bool) error {
	right := l.w - left

	// Heading.
	if l.draw {
		if err := l.setFont(styleBold, recTitleSize); err != nil {
			return err
		}
		l.setText(colorInk)
	}
	if err := l.emit(left, l.y+recTitleSize, rec.Title); err != nil {
		return err
	}
	l.y += recTitleSize + recTitleToRule

	// Hairline under the heading — DataTable's gray-100 rule.
	if l.draw {
		l.setStroke(colorHeaderRule)
		l.pdf.SetLineWidth(0.7)
		l.pdf.Line(left, l.y, right, l.y)
	}
	l.y += recRuleToFields

	for _, f := range rec.Fields {
		if err := l.writeField(left, f, paged); err != nil {
			return err
		}
	}
	for _, sub := range rec.Subs {
		if err := l.writeSub(sub, left, paged); err != nil {
			return err
		}
	}
	return nil
}

// writeSub renders one child sub-block, kept together when it fits a page
// (only relevant in paged/bare mode — inside a card the whole record
// already fits).
func (l *recordLayout) writeSub(sub SubRecord, left float64, paged bool) error {
	if paged {
		if h := l.subHeight(sub, left); h <= l.maxContent() {
			if err := l.ensureSpace(h); err != nil {
				return err
			}
		}
	}
	l.y += recSubTopGap
	if l.draw {
		if err := l.setFont(styleBold, recSubTitleSize); err != nil {
			return err
		}
		l.setText(colorInk)
	}
	if err := l.emit(left+recSubIndent, l.y+recSubTitleSize, sub.Title); err != nil {
		return err
	}
	l.y += recSubTitleSize + recSubTitleGap
	for _, f := range sub.Fields {
		if err := l.writeField(left+2*recSubIndent, f, paged); err != nil {
			return err
		}
	}
	return nil
}

// writeField renders "Label: value", wrapped to the available width. In
// paged mode each line may page-break.
func (l *recordLayout) writeField(x float64, f Field, paged bool) error {
	text, ok := fieldText(f)
	if !ok {
		return nil
	}
	width := l.w - pageMargin - recCardPad - x
	for _, line := range l.wrapMeasured(text, width) {
		if paged {
			if err := l.ensureSpace(recFieldLineH); err != nil {
				return err
			}
		}
		// Body styling is applied per line: ensureSpace may have opened a
		// new page, whose footer leaves the footer font and muted color set.
		if l.draw {
			if err := l.setFont(styleNormal, fontBody); err != nil {
				return err
			}
			l.setText(colorBody)
		}
		if err := l.emit(x, l.y+fontBody, line); err != nil {
			return err
		}
		l.y += recFieldLineH
	}
	return nil
}

// wrapMeasured wraps with the body font pinned, so the measuring and
// drawing passes agree even when the current font differs.
func (l *recordLayout) wrapMeasured(s string, width float64) []string {
	// SetFont only errors on unknown family/style; the family is embedded
	// in newPageChrome, so the error cannot fire here.
	_ = l.setFont(styleNormal, fontBody)
	return l.wrap(norms(s), width)
}

// recordHeight measures a record's card height using the same constants
// as the draw path.
func (l *recordLayout) recordHeight(rec Record) float64 {
	h := 2*recCardPad + recTitleSize + recTitleToRule + recRuleToFields
	inner := pageMargin + recCardPad
	for _, f := range rec.Fields {
		h += l.fieldHeight(inner, f)
	}
	for _, sub := range rec.Subs {
		h += l.subHeight(sub, inner)
	}
	return h
}

func (l *recordLayout) subHeight(sub SubRecord, left float64) float64 {
	h := recSubTopGap + recSubTitleSize + recSubTitleGap
	for _, f := range sub.Fields {
		h += l.fieldHeight(left+2*recSubIndent, f)
	}
	return h
}

func (l *recordLayout) fieldHeight(x float64, f Field) float64 {
	text, ok := fieldText(f)
	if !ok {
		return 0
	}
	width := l.w - pageMargin - recCardPad - x
	return float64(len(l.wrapMeasured(text, width))) * recFieldLineH
}
