package listexport

import (
	"strings"

	listrecords "github.com/moto-nrw/project-phoenix/modules/documentrendering/records"
)

// RenderRecords renders one block per record as a print-friendly PDF
// (A4 portrait). Use this instead of the flat-table Render when each
// record carries hierarchical/variable data (e.g. one enrollment with
// N children, offerings, consents and per-phase custom fields) that a
// fixed-column table can't lay out legibly. It draws through the same
// pageChrome frame as the flat-table renderer (records_design.go), so
// both PDF layouts share one design.
//
// Layout per record: a white card with the heading, a hairline rule,
// the record's own label/value fields, then each sub-record indented
// beneath it. A record taller than a page flows across pages as a bare
// block instead; the page header + footer repeat on every page.
func (s *RendererService) RenderRecords(doc listrecords.RecordDocument, filenameBase string) (listrecords.File, error) {
	base := safeFilename(filenameBase)
	if base == "" {
		base = "export"
	}
	data, err := renderRecordsPDFDesigned(doc)
	if err != nil {
		return listrecords.File{}, err
	}
	return listrecords.File{Data: data, ContentType: "application/pdf", Filename: base + ".pdf"}, nil
}

func (s *RendererService) RenderRecordsDOCX(doc listrecords.RecordDocument, filenameBase string) (listrecords.File, error) {
	base := safeFilename(filenameBase)
	if base == "" {
		base = "export"
	}
	data, err := renderRecordsDOCX(doc)
	if err != nil {
		return listrecords.File{}, err
	}
	return listrecords.File{
		Data:        data,
		ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		Filename:    base + ".docx",
	}, nil
}

func recordDocumentGroups(doc listrecords.RecordDocument) []listrecords.RecordGroup {
	if len(doc.Groups) > 0 {
		return doc.Groups
	}
	if len(doc.Records) == 0 {
		return nil
	}
	return []listrecords.RecordGroup{{Records: doc.Records}}
}

func recordGroupCount(groups []listrecords.RecordGroup) int {
	count := 0
	for _, group := range groups {
		count += len(group.Records)
	}
	return count
}

// fieldText builds the "Label: value" string for a field, or reports
// ok=false when the value is empty (such fields are skipped entirely).
func fieldText(f listrecords.Field) (string, bool) {
	value := strings.TrimSpace(f.Value)
	if value == "" {
		return "", false
	}
	if label := strings.TrimSpace(f.Label); label != "" {
		return label + ": " + value, true
	}
	return value, true
}
