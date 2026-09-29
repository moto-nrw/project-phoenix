package api

import (
	announcementAPI "github.com/moto-nrw/project-phoenix/modules/communication/http/parentannouncements"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
)

// declarationReports binds the Erklärung proof documents (#3430) of the
// staff report and the parent proof to the Document Rendering record
// renderer. Both consumers describe their document in the same unnamed
// shapes, so this one binding satisfies both ports. Every text is user or
// school input as far as the renderer is concerned and is sanitized.
type declarationReports struct {
	renderer lists.RecordRenderer
}

func newDeclarationReports() declarationReports {
	return declarationReports{renderer: lists.NewRecordRenderer()}
}

func (d declarationReports) RenderReport(doc announcementAPI.ReportDocument, filenameBase string) (announcementAPI.ReportFile, error) {
	out := lists.RecordDocument{
		Title: reportText(doc.Title), Subtitle: reportText(doc.Subtitle), GeneratedAt: doc.GeneratedAt, Footer: reportText(doc.Footer),
		Filters: make([]string, 0, len(doc.Filters)), Groups: make([]lists.RecordGroup, 0, len(doc.Sections)),
	}
	for _, filter := range doc.Filters {
		out.Filters = append(out.Filters, reportText(filter))
	}
	for _, section := range doc.Sections {
		group := lists.RecordGroup{Title: reportText(section.Title), Records: make([]lists.Record, 0, len(section.Cards))}
		for _, card := range section.Cards {
			group.Records = append(group.Records, reportRecord(card))
		}
		out.Groups = append(out.Groups, group)
	}
	file, err := d.renderer.RenderRecords(out, filenameBase)
	if err != nil {
		return announcementAPI.ReportFile{}, err
	}
	return announcementAPI.ReportFile{Data: file.Data, ContentType: file.ContentType, Filename: file.Filename}, nil
}

func reportRecord(card announcementAPI.ReportCard) lists.Record {
	record := lists.Record{Title: reportText(card.Title), Fields: reportFields(card.Fields)}
	for _, block := range card.Blocks {
		record.Subs = append(record.Subs, lists.SubRecord{Title: reportText(block.Title), Fields: reportFields(block.Fields)})
	}
	return record
}

func reportFields(fields []announcementAPI.ReportField) []lists.Field {
	out := make([]lists.Field, 0, len(fields))
	for _, f := range fields {
		out = append(out, lists.Field{Label: reportText(f.Label), Value: reportText(f.Value)})
	}
	return out
}

func reportText(text string) string { return lists.SanitizeUserText(text) }
