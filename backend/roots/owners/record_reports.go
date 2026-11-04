package owners

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
)

// The report documents of several owners (the Erklärung proofs #3430, Care
// Plan's proofs, the weekly protocol #4769) are described in the same
// unnamed shapes, so one binding to Document Rendering's record renderer
// satisfies every port. The API root and the Worker root both use it.
type (
	recordReportField = struct {
		Label, Value string
		PNG          []byte
	}
	recordReportBlock = struct {
		Title  string
		Fields []recordReportField
	}
	recordReportCard = struct {
		Title  string
		Fields []recordReportField
		Blocks []recordReportBlock
	}
	recordReportSection = struct {
		Title string
		Cards []recordReportCard
	}
	recordReportDocument = struct {
		Title       string
		Subtitle    string
		GeneratedAt time.Time
		Filters     []string
		Footer      string
		Sections    []recordReportSection
	}
	recordReportFile = struct {
		Data        []byte
		ContentType string
		Filename    string
	}
)

// RecordReports renders a report document as a PDF. Every text is user or
// school input as far as the renderer is concerned and is sanitized.
type RecordReports struct {
	renderer lists.RecordRenderer
}

// NewRecordReports binds the record renderer.
func NewRecordReports() RecordReports {
	return RecordReports{renderer: lists.NewRecordRenderer()}
}

// RenderReport renders doc as a PDF named after filenameBase.
func (r RecordReports) RenderReport(doc recordReportDocument, filenameBase string) (recordReportFile, error) {
	out := lists.RecordDocument{
		Title: ReportText(doc.Title), Subtitle: ReportText(doc.Subtitle), GeneratedAt: doc.GeneratedAt, Footer: ReportText(doc.Footer),
		Filters: make([]string, 0, len(doc.Filters)), Groups: make([]lists.RecordGroup, 0, len(doc.Sections)),
	}
	for _, filter := range doc.Filters {
		out.Filters = append(out.Filters, ReportText(filter))
	}
	for _, section := range doc.Sections {
		group := lists.RecordGroup{Title: ReportText(section.Title), Records: make([]lists.Record, 0, len(section.Cards))}
		for _, card := range section.Cards {
			group.Records = append(group.Records, recordOf(card))
		}
		out.Groups = append(out.Groups, group)
	}
	file, err := r.renderer.RenderRecords(out, filenameBase)
	if err != nil {
		return recordReportFile{}, err
	}
	return recordReportFile{Data: file.Data, ContentType: file.ContentType, Filename: file.Filename}, nil
}

func recordOf(card recordReportCard) lists.Record {
	record := lists.Record{Title: ReportText(card.Title), Fields: recordFieldsOf(card.Fields)}
	for _, block := range card.Blocks {
		record.Subs = append(record.Subs, lists.SubRecord{Title: ReportText(block.Title), Fields: recordFieldsOf(block.Fields)})
	}
	return record
}

func recordFieldsOf(fields []recordReportField) []lists.Field {
	out := make([]lists.Field, 0, len(fields))
	for _, f := range fields {
		out = append(out, lists.Field{Label: ReportText(f.Label), Value: ReportText(f.Value), PNG: f.PNG})
	}
	return out
}

// ReportText sanitizes a text for the record renderer.
func ReportText(text string) string { return lists.SanitizeUserText(text) }
