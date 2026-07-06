package listexport

import (
	"fmt"
	"strings"
)

// RenderRecords renders one block per record as a print-friendly PDF
// (A4 portrait). Use this instead of the flat-table Render when each
// record carries hierarchical/variable data (e.g. one enrollment with
// N children, offerings, consents and per-phase custom fields) that a
// fixed-column table can't lay out legibly. It reuses the same low-level PDF primitives as the flat-table
// renderer (writePDFText, drawPDFRect, buildPDF, the WinAnsi font) so
// there is a single PDF code path in this package.
//
// Layout per record: a heading with an underline rule, the record's
// own label/value fields, then each sub-record indented beneath it.
// A record that overflows the page continues on the next page rather
// than being clipped; the page header + footer repeat on every page.
func (s *RendererService) RenderRecords(doc RecordDocument, filenameBase string) (File, error) {
	base := safeFilename(filenameBase)
	if base == "" {
		base = "export"
	}
	data := renderRecordsPDF(doc)
	return File{Data: data, ContentType: "application/pdf", Filename: base + ".pdf"}, nil
}

func (s *RendererService) RenderRecordsDOCX(doc RecordDocument, filenameBase string) (File, error) {
	base := safeFilename(filenameBase)
	if base == "" {
		base = "export"
	}
	data, err := renderRecordsDOCX(doc)
	if err != nil {
		return File{}, err
	}
	return File{
		Data:        data,
		ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		Filename:    base + ".docx",
	}, nil
}

// Vertical-rhythm constants (PDF points). Kept here so the measure
// pass (recordHeight/subHeight) and the draw pass stay in lock-step —
// any spacing change is made once and both passes follow.
const (
	nameSize         = 12  // record heading (guardian name)
	nameToRule       = 6   // gap below the name before its underline
	ruleToFields     = 14  // gap below the underline before the first field
	fieldLineH       = 11  // height of one (wrapped) field line
	subTopGap        = 9   // gap above each child sub-block
	subTitleSize     = 9.5 // child heading
	subTitleToFields = 12  // gap below the child heading before its fields
	interRecordGap   = 20  // gap between two registration blocks
	interGroupGap    = 24  // gap between two status groups
	groupTitleSize   = 11  // status group heading
	groupTitleGap    = 14  // gap below the group heading
	headerToBody     = 20  // gap below the page-header rule before the first block
)

// recordPager streams record content across pages, finalising a page
// and opening a fresh one whenever the cursor would cross the bottom
// margin.
type recordPager struct {
	doc        RecordDocument
	pageWidth  float64
	pageHeight float64
	margin     float64
	bottom     float64 // y below which content must not be drawn
	topY       float64 // y at the top of the body (just under the page header)

	pages []string
	cur   *strings.Builder
	y     float64
}

func renderRecordsPDF(doc RecordDocument) []byte {
	p := &recordPager{
		doc:        doc,
		pageWidth:  595.0,
		pageHeight: 842.0,
		margin:     40.0,
	}
	p.bottom = p.margin + 24 // leave room for the footer line
	p.newPage()
	// Tallest block we can keep together on one page. Blocks taller
	// than this are allowed to split (no single page could hold them).
	maxContent := p.topY - p.bottom

	groups := recordDocumentGroups(doc)
	if recordGroupCount(groups) == 0 {
		writePDFText(p.cur, p.margin, p.y, 9, "Keine Anmeldungen vorhanden.")
		p.y -= fieldLineH
	}
	for groupIdx, group := range groups {
		if len(group.Records) == 0 {
			continue
		}
		if groupIdx > 0 {
			p.y -= interGroupGap
		}
		p.writeGroupTitle(group.Title)
		for i, rec := range group.Records {
			p.writeRecord(rec, maxContent)
			if i < len(group.Records)-1 {
				// Plain vertical gap between records. Each record already
				// carries an underline rule under its heading, so no
				// separator line is needed.
				p.y -= interRecordGap
			}
		}
	}
	p.pages = append(p.pages, p.cur.String())

	return assembleRecordPDF(p.pages, p.pageWidth, p.pageHeight)
}

func recordDocumentGroups(doc RecordDocument) []RecordGroup {
	if len(doc.Groups) > 0 {
		return doc.Groups
	}
	if len(doc.Records) == 0 {
		return nil
	}
	return []RecordGroup{{Records: doc.Records}}
}

func recordGroupCount(groups []RecordGroup) int {
	count := 0
	for _, group := range groups {
		count += len(group.Records)
	}
	return count
}

// usableWidth is the horizontal space available for text at the given
// left indent.
func (p *recordPager) usableWidth(x float64) float64 {
	return p.pageWidth - p.margin - x
}

func (p *recordPager) newPage() {
	p.cur = &strings.Builder{}
	p.y = p.pageHeight - p.margin

	// Page header: title, subtitle, generated-at.
	writePDFText(p.cur, p.margin, p.y, 15, p.doc.Title)
	p.y -= 19
	if p.doc.Subtitle != "" {
		writePDFText(p.cur, p.margin, p.y, 9, p.doc.Subtitle)
		p.y -= 12
	}
	writePDFText(p.cur, p.margin, p.y, 8, "Erstellt: "+GeneratedAtLabel(p.doc.GeneratedAt))
	p.y -= 8
	// Applied filters (e.g. "Status: Angenommen"), one per line, repeated
	// on every page so a filtered printout is self-describing.
	for _, filter := range p.doc.Filters {
		p.y -= 10
		writePDFText(p.cur, p.margin, p.y, 8, filter)
	}
	p.y -= headerToBody
	p.topY = p.y

	// Footer (bottom of page) + moto wordmark.
	if p.doc.Footer != "" {
		writePDFText(p.cur, p.margin, p.margin-2, 7, p.doc.Footer)
	}
	writePDFText(p.cur, p.pageWidth-p.margin-28, p.margin-2, 7, "moto")
}

// ensureSpace finalises the current page and starts a new one when
// fewer than `needed` points remain above the bottom margin.
func (p *recordPager) ensureSpace(needed float64) {
	if p.y-needed < p.bottom {
		p.pages = append(p.pages, p.cur.String())
		p.newPage()
	}
}

func (p *recordPager) rule(y float64) {
	fmt.Fprintf(p.cur, "%.1f %.1f m %.1f %.1f l S\n", p.margin, y, p.pageWidth-p.margin, y)
}

func (p *recordPager) writeGroupTitle(title string) {
	title = strings.TrimSpace(title)
	if title == "" {
		return
	}
	p.ensureSpace(groupTitleGap + fieldLineH)
	writePDFText(p.cur, p.margin, p.y, groupTitleSize, title)
	p.y -= groupTitleGap
}

// writeRecord draws one registration block. If the whole block fits on
// a page it is kept together (page-break before, never mid-block);
// otherwise only the heading + first line is guaranteed together and
// the body flows, with each child sub-block still kept together.
func (p *recordPager) writeRecord(rec Record, maxContent float64) {
	if h := p.recordHeight(rec); h <= maxContent {
		p.ensureSpace(h)
	} else {
		p.ensureSpace(nameToRule + ruleToFields + fieldLineH)
	}
	writePDFText(p.cur, p.margin, p.y, nameSize, rec.Title)
	p.y -= nameToRule
	p.rule(p.y)
	p.y -= ruleToFields

	for _, f := range rec.Fields {
		p.field(p.margin+4, f)
	}
	for _, sub := range rec.Subs {
		p.writeSub(sub, maxContent)
	}
}

// writeSub draws one child sub-block, kept together when it fits a page.
func (p *recordPager) writeSub(sub SubRecord, maxContent float64) {
	if h := p.subHeight(sub); h <= maxContent {
		p.ensureSpace(h)
	}
	p.y -= subTopGap
	writePDFText(p.cur, p.margin+12, p.y, subTitleSize, sub.Title)
	p.y -= subTitleToFields
	for _, f := range sub.Fields {
		p.field(p.margin+24, f)
	}
}

// recordHeight / subHeight measure a block's drawn height using the
// same constants + wrap logic as the draw pass, so keep-together
// reservations match what is actually rendered.
func (p *recordPager) recordHeight(rec Record) float64 {
	h := float64(nameToRule + ruleToFields)
	for _, f := range rec.Fields {
		h += p.fieldHeight(p.margin+4, f)
	}
	for _, sub := range rec.Subs {
		h += p.subHeight(sub)
	}
	return h
}

func (p *recordPager) subHeight(sub SubRecord) float64 {
	h := float64(subTopGap + subTitleToFields)
	for _, f := range sub.Fields {
		h += p.fieldHeight(p.margin+24, f)
	}
	return h
}

func (p *recordPager) fieldHeight(x float64, f Field) float64 {
	text, ok := fieldText(f)
	if !ok {
		return 0
	}
	return float64(len(wrapPDFText(text, p.usableWidth(x)))) * fieldLineH
}

// fieldText builds the "Label: value" string for a field, or reports
// ok=false when the value is empty (such fields are skipped entirely).
func fieldText(f Field) (string, bool) {
	value := strings.TrimSpace(f.Value)
	if value == "" {
		return "", false
	}
	if label := strings.TrimSpace(f.Label); label != "" {
		return label + ": " + value, true
	}
	return value, true
}

// field writes "Label: value", wrapping to the available width and
// page-breaking line by line. Empty values are skipped entirely.
func (p *recordPager) field(x float64, f Field) {
	text, ok := fieldText(f)
	if !ok {
		return
	}
	for _, line := range wrapPDFText(text, p.usableWidth(x)) {
		p.ensureSpace(fieldLineH)
		writePDFText(p.cur, x, p.y, 8, line)
		p.y -= fieldLineH
	}
}

func assembleRecordPDF(pages []string, pageWidth, pageHeight float64) []byte {
	objects := []pdfObject{
		{id: 1, data: "<< /Type /Catalog /Pages 2 0 R >>"},
		{id: 3, data: "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>"},
	}
	pageRefs := make([]string, 0, len(pages))
	nextID := 4
	for _, content := range pages {
		pageID := nextID
		contentID := nextID + 1
		nextID += 2
		pageRefs = append(pageRefs, fmt.Sprintf("%d 0 R", pageID))
		pageObj := fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %.0f %.0f] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", pageWidth, pageHeight, contentID)
		contentObj := fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content)
		objects = append(objects, pdfObject{id: pageID, data: pageObj}, pdfObject{id: contentID, data: contentObj})
	}
	objects = append(objects[:1], append([]pdfObject{{id: 2, data: fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(pageRefs, " "), len(pageRefs))}}, objects[1:]...)...)
	return buildPDF(objects)
}
