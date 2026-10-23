package enrollmenthttp

import (
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The three enrollment exports (#4750) share the parts below instead of
// building their documents each on their own: Anmeldungen der Phase,
// Anmeldungen eines Kindes and the Anmelde-Auswertung. Each export reads its
// own data and decides which fields it shows; the parts fix the header and
// footer, the format dispatch, the block of one child with its enrollment
// details (PDF/DOCX) and the column catalog of the tables (XLSX). The
// compact Auswertung and the Klassenliste of a phase share the class-list
// table of Document Rendering (enrollmentClassRosterDocument).

// enrollmentExportHead is the page header every enrollment export prints.
// Both layouts carry the generation time and the confidentiality footer.
type enrollmentExportHead struct {
	Title    string
	Subtitle string
	Filters  []string
}

// blocks builds the PDF/DOCX document: one block per record, optionally in
// titled groups.
func (h enrollmentExportHead) blocks(records []lists.Record, groups []lists.RecordGroup) lists.RecordDocument {
	return lists.RecordDocument{
		Title:       h.Title,
		Subtitle:    h.Subtitle,
		GeneratedAt: time.Now(),
		Footer:      exportConfidentialityNote,
		Filters:     h.Filters,
		Records:     records,
		Groups:      groups,
	}
}

// table builds the tabular document (XLSX, and the compact PDF/DOCX).
func (h enrollmentExportHead) table(cols []lists.Column, rows []lists.Row) lists.Document {
	return lists.Document{
		Title:       h.Title,
		Subtitle:    h.Subtitle,
		GeneratedAt: time.Now(),
		Filters:     h.Filters,
		Columns:     cols,
		Rows:        rows,
		Footer:      exportConfidentialityNote,
	}
}

// renderEnrollmentExport renders an enrollment export in its detailed form:
// PDF and DOCX as blocks, XLSX as one table row per child. Only the chosen
// document is built.
func renderEnrollmentExport(svc lists.DocumentRenderer, format lists.Format, filename string, blocks func() lists.RecordDocument, table func() lists.Document) (lists.File, error) {
	switch format {
	case lists.FormatDOCX:
		return svc.RenderRecordsDOCX(blocks(), filename)
	case lists.FormatPDF:
		return svc.RenderRecords(blocks(), filename)
	case lists.FormatXLSX:
		return svc.Render(table(), lists.FormatXLSX, filename)
	default:
		return lists.File{}, fmt.Errorf("unsupported export format %q", format)
	}
}

// ---------------------------------------------------------------------
// PDF/DOCX: one child with its enrollment details.
// ---------------------------------------------------------------------

// enrollmentChildBlock is one child with its enrollment details, the block
// all three enrollment exports print. The parts follow in a fixed order: the
// child's own fields, the submitting guardian's contact, the submission
// (dates, consents, answers), then the co-guardians. An export leaves out the
// parts it does not carry.
type enrollmentChildBlock struct {
	Name        string
	Child       []lists.Field
	Guardian    []lists.Field
	Submission  []lists.Field
	CoGuardians []*capability.RequestGuardian
}

func (b enrollmentChildBlock) record() lists.Record {
	fields := make([]lists.Field, 0, len(b.Child)+len(b.Guardian)+len(b.Submission)+len(b.CoGuardians))
	fields = append(fields, b.Child...)
	fields = append(fields, b.Guardian...)
	fields = append(fields, b.Submission...)
	fields = append(fields, additionalGuardianFields(b.CoGuardians)...)
	return lists.Record{Title: b.Name, Fields: fields}
}

// guardianContactFields is the submitting guardian's contact led by the
// name ("Eltern").
func guardianContactFields(name, email, phone string) []lists.Field {
	return append([]lists.Field{{Label: "Eltern", Value: name}}, guardianReachFields(email, phone)...)
}

// guardianReachFields is how the submitting guardian is reached.
func guardianReachFields(email, phone string) []lists.Field {
	return []lists.Field{
		{Label: "E-Mail", Value: email},
		{Label: "Telefon", Value: phone},
	}
}

// submissionFields is one submission: when it came in or was withdrawn, the
// consents, and the guardian-level answers of the form.
func submissionFields(req *Request, guardianCustoms []capability.FormField) []lists.Field {
	fields := []lists.Field{{Label: "Eingereicht am", Value: submittedAtLabel(req)}}
	if req.WithdrawnAt != nil {
		fields = append(fields, lists.Field{Label: "Zurückgezogen am", Value: timeOrEmpty(req.WithdrawnAt)})
	}
	fields = append(fields, lists.Field{Label: "Zustimmungen", Value: consentSummary(req.ConsentFlags)})
	for _, cf := range guardianCustoms {
		if v, ok := req.CustomData[cf.Key]; ok {
			fields = append(fields, lists.Field{Label: cf.Label, Value: formatCustomValue(cf, v)})
		}
	}
	return fields
}

func submittedAtLabel(req *Request) string {
	return req.SubmittedAt.Format("02.01.2006 15:04")
}

// ---------------------------------------------------------------------
// XLSX: one row per child, columns from one catalog.
// ---------------------------------------------------------------------

// enrollmentColumnLabels is the column catalog of the enrollment tables, so
// a value keeps its heading in every export that prints it.
var enrollmentColumnLabels = map[lists.ColumnID]string{
	"phase":                   "Anmeldung",
	"child_last_name":         "Kind Nachname",
	"child_first_name":        "Kind Vorname",
	"child_dob":               "Geburtsdatum",
	"child_grade":             "Zielklasse",
	"child_status":            "Status",
	"child_status_reason":     "Status-Grund",
	"child_activation":        "Aktivierung",
	"child_offerings":         "Betreuungsangebote",
	"child_companion_note":    departureCompanionLabelDE,
	"offerings":               "Betreuungsangebote",
	"offering_days":           "Tage je Angebot",
	"effective_days":          "Effektive Betreuungstage",
	"day_count":               "Anzahl Tage",
	"pickup_mon":              "Mo Gehzeit",
	"pickup_tue":              "Di Gehzeit",
	"pickup_wed":              "Mi Gehzeit",
	"pickup_thu":              "Do Gehzeit",
	"pickup_fri":              "Fr Gehzeit",
	"guardian_name":           "Eltern",
	"guardian_last_name":      "Eltern Nachname",
	"guardian_first_name":     "Eltern Vorname",
	"guardian_email":          "E-Mail",
	"guardian_phone":          "Telefon",
	"submitted_at":            "Eingereicht am",
	"withdrawn_at":            "Zurückgezogen am",
	"consent_agb":             "Zustimmung AGB",
	"consent_data_processing": "Zustimmung Datenverarbeitung",
	"consent_email_contact":   "Zustimmung E-Mail-Kontakt",
	"consent_photo":           "Zustimmung Foto",
	"additional_guardians":    "Weitere Erziehungsberechtigte",
}

// enrollmentColumns resolves catalog columns in the given order. An ID
// outside the catalog is a programming error; it panics so that every test
// of the export fails on it.
func enrollmentColumns(ids ...lists.ColumnID) []lists.Column {
	cols := make([]lists.Column, 0, len(ids))
	for _, id := range ids {
		label, ok := enrollmentColumnLabels[id]
		if !ok {
			panic(fmt.Sprintf("enrollment export column %q is not in the catalog", id))
		}
		cols = append(cols, lists.Column{ID: id, Label: label})
	}
	return cols
}

// childColumns are the child's columns of a table row, matching
// childRowValues: identity, target class, status, activation, offerings, the
// child-level answers and the coupled "mit wem" note (#1694, a reserved key
// rather than a schema field, so it has a column of its own).
func childColumns(childCustoms []capability.FormField) []lists.Column {
	cols := enrollmentColumns("child_last_name", "child_first_name", "child_dob", "child_grade",
		"child_status", "child_status_reason", "child_activation", "child_offerings")
	cols = append(cols, customColumns("custom_c_", childCustoms)...)
	return append(cols, enrollmentColumns("child_companion_note")...)
}

// answerColumns are the guardian-level answers and the consents, matching
// submissionRowValues.
func answerColumns(guardianCustoms []capability.FormField) []lists.Column {
	cols := customColumns("custom_g_", guardianCustoms)
	return append(cols, enrollmentColumns("consent_agb", "consent_data_processing",
		"consent_email_contact", "consent_photo")...)
}

func customColumns(prefix string, fields []capability.FormField) []lists.Column {
	cols := make([]lists.Column, 0, len(fields))
	for _, cf := range fields {
		cols = append(cols, lists.Column{ID: lists.ColumnID(prefix + cf.Key), Label: cf.Label})
	}
	return cols
}

// submissionRowValues is one submission in a table row, the counterpart of
// submissionFields.
func submissionRowValues(req *Request, guardianCustoms []capability.FormField) map[lists.ColumnID]string {
	values := map[lists.ColumnID]string{
		"submitted_at":            submittedAtLabel(req),
		"withdrawn_at":            timeOrEmpty(req.WithdrawnAt),
		"consent_agb":             consentLabel(req.ConsentFlags, capability.ConsentKeyAGB),
		"consent_data_processing": consentLabel(req.ConsentFlags, capability.ConsentKeyDataProcessing),
		"consent_email_contact":   consentLabel(req.ConsentFlags, capability.ConsentKeyEmailContact),
		"consent_photo":           consentLabel(req.ConsentFlags, capability.ConsentKeyPhoto),
	}
	for _, cf := range guardianCustoms {
		if v, ok := req.CustomData[cf.Key]; ok {
			values[lists.ColumnID("custom_g_"+cf.Key)] = formatCustomValue(cf, v)
		}
	}
	return values
}

// ---------------------------------------------------------------------
// Compact: the class-list table of Document Rendering (#4749).
// ---------------------------------------------------------------------

// enrollmentClassRosterDocument renders class-sorted children in the shared
// class-list table followed by the Erziehungsberechtigte column. classLabel
// heads the class column; grouped opens a section per class.
func enrollmentClassRosterDocument(head enrollmentExportHead, classLabel string, children []lists.ClassRosterChild, grouped bool) lists.Document {
	cols := lists.ResolveColumns(append(lists.ClassRosterColumnIDs(), lists.ColumnGuardianContacts), lists.PresetClassRoster)
	for i := range cols {
		if cols[i].ID == lists.ColumnSchoolClass {
			cols[i].Label = classLabel
		}
	}
	return head.table(cols, lists.ClassRosterRows(children, grouped))
}
