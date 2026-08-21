package enrollment

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/listexport"
)

func TestParsePhaseExportRequest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		body       string
		query      string
		want       listexport.Format
		wantStatus string
		wantErr    bool
	}{
		{name: "empty body defaults to pdf, no filter", body: "", want: listexport.FormatPDF},
		{name: "lowercase body xlsx", body: `{"format":"xlsx"}`, want: listexport.FormatXLSX},
		{name: "lowercase body docx", body: `{"format":"docx"}`, want: listexport.FormatDOCX},
		{name: "uppercase body normalised", body: `{"format":"PDF"}`, want: listexport.FormatPDF},
		{name: "uppercase query normalised", query: "format=XLSX", want: listexport.FormatXLSX},
		{name: "body wins over query", body: `{"format":"pdf"}`, query: "format=xlsx", want: listexport.FormatPDF},
		{name: "unsupported format rejected", body: `{"format":"csv"}`, wantErr: true},
		{name: "malformed body rejected", body: `{`, wantErr: true},
		// child_status filter
		{name: "valid child_status kept", body: `{"format":"xlsx","child_status":"approved"}`, want: listexport.FormatXLSX, wantStatus: "approved"},
		{name: "child_status uppercased normalised", body: `{"child_status":"WITHDRAWN"}`, want: listexport.FormatPDF, wantStatus: "withdrawn"},
		{name: "child_status all means no filter", body: `{"child_status":"all"}`, want: listexport.FormatPDF, wantStatus: ""},
		{name: "child_status from query", query: "child_status=waitlisted", want: listexport.FormatPDF, wantStatus: "waitlisted"},
		{name: "unknown child_status rejected", body: `{"child_status":"bogus"}`, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			url := "/export"
			if tc.query != "" {
				url += "?" + tc.query
			}
			req := httptest.NewRequest("POST", url, strings.NewReader(tc.body))
			got, status, err := parsePhaseExportRequest(req)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got format %q status %q", got, status)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("format = %q, want %q", got, tc.want)
			}
			if status != tc.wantStatus {
				t.Errorf("child_status = %q, want %q", status, tc.wantStatus)
			}
		})
	}
}

// sampleExport builds an in-memory PhaseExport (no DB) covering the
// shapes the renderers must handle: guardian + child custom fields,
// a select with an option label, consents, and an offering.
func sampleExport() *enrollmentService.PhaseExport {
	schema := &enrollmentModels.FormSchema{
		Fields: []enrollmentModels.FormField{
			{Key: "notes", Label: "Hinweise", Type: enrollmentModels.FormFieldText, SortOrder: 1},
			{Key: "bus", Label: "Buskind", Type: enrollmentModels.FormFieldBoolean, AppliesToCh: true, SortOrder: 1},
			{Key: "meal", Label: "Essen", Type: enrollmentModels.FormFieldSelect, AppliesToCh: true, SortOrder: 2,
				Options: []enrollmentModels.FormFieldOption{{Label: "Vegetarisch", Value: "veg"}}},
		},
	}
	schema.ID = 50
	sid := schema.ID

	phone := "0151-123"
	req := &enrollmentModels.Request{
		SchemaID:          &sid,
		GuardianFirstName: "Anna",
		GuardianLastName:  "Muster",
		GuardianEmail:     "anna@example.test",
		GuardianPhone:     &phone,
		ConsentFlags: map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": false,
		},
		CustomData:  map[string]any{"notes": "Bitte Hintereingang"},
		SubmittedAt: time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC),
	}

	grade := int16(1)
	child := &enrollmentModels.RequestChild{
		FirstName:        "Lina",
		LastName:         "Muster",
		DateOfBirth:      timezone.NewDate(2018, 5, 12),
		TargetGradeLevel: &grade,
		Status:           enrollmentModels.ChildStatusApproved,
		ActivationMode:   enrollmentModels.ChildActivationScheduled,
		CustomData:       map[string]any{"bus": true, "meal": "veg"},
	}

	offerings := []enrollmentService.ChildOfferingRow{
		{OfferingName: "Kernzeit", AvailableDays: []string{"mon", "tue"}},
	}

	return &enrollmentService.PhaseExport{
		Phase:   &enrollmentModels.Phase{Name: "Schuljahr 2026/27"},
		Schemas: map[int64]*enrollmentModels.FormSchema{schema.ID: schema},
		Rows: []enrollmentService.ExportRequestRow{
			{Request: req, Children: []enrollmentService.ExportChildRow{{Child: child, Offerings: offerings}}},
		},
	}
}

func sampleStudentEnrollmentExport() *enrollmentService.StudentEnrollmentExport {
	data := sampleExport()
	phase := data.Phase
	phase.ID = 7801
	data.Rows[0].Request.PhaseID = phase.ID
	withdrawnAt := time.Date(2026, 6, 2, 9, 30, 0, 0, time.UTC)
	data.Rows[0].Request.WithdrawnAt = &withdrawnAt

	return &enrollmentService.StudentEnrollmentExport{
		StudentID: 8801,
		Schemas:   data.Schemas,
		Phases:    map[int64]*enrollmentModels.Phase{phase.ID: phase},
		Rows:      data.Rows,
	}
}

func columnLabels(doc listexport.Document) map[listexport.ColumnID]string {
	out := make(map[listexport.ColumnID]string, len(doc.Columns))
	for _, c := range doc.Columns {
		out[c.ID] = c.Label
	}
	return out
}

func tableDataRows(rows []listexport.Row) []listexport.Row {
	out := make([]listexport.Row, 0, len(rows))
	for _, row := range rows {
		if row.GroupTitle == "" {
			out = append(out, row)
		}
	}
	return out
}

func TestBuildPhaseExportTable_FullRow(t *testing.T) {
	t.Parallel()

	doc := buildPhaseExportTable(sampleExport(), "Anmeldungen – Test", "")

	labels := columnLabels(doc)
	for id, wantLabel := range map[listexport.ColumnID]string{
		"custom_g_notes": "Hinweise",
		"custom_c_bus":   "Buskind",
		"custom_c_meal":  "Essen",
		"consent_photo":  "Zustimmung Foto",
	} {
		if labels[id] != wantLabel {
			t.Errorf("column %q label = %q, want %q", id, labels[id], wantLabel)
		}
	}

	rows := tableDataRows(doc.Rows)
	if len(rows) != 1 {
		t.Fatalf("data rows = %d, want 1 (one child)", len(rows))
	}
	v := rows[0].Values
	checks := map[listexport.ColumnID]string{
		"guardian_first_name": "Anna",
		"guardian_phone":      "0151-123",
		"consent_agb":         "Ja",
		"consent_photo":       "Nein",
		"custom_g_notes":      "Bitte Hintereingang",
		"child_first_name":    "Lina",
		"child_grade":         "1. Klasse",
		"child_status":        "Angenommen",
		"custom_c_bus":        "Ja",
		"custom_c_meal":       "Vegetarisch", // select value -> option label
	}
	for id, want := range checks {
		if v[id] != want {
			t.Errorf("value %q = %q, want %q", id, v[id], want)
		}
	}
	if !strings.Contains(v["child_offerings"], "Kernzeit") {
		t.Errorf("child_offerings = %q, want it to contain Kernzeit", v["child_offerings"])
	}
}

// #1694: the accompanied-mode "mit wem" note rides on a reserved custom-data
// key (not a schema field), so the field-iterating export loops never emit it.
// It must surface in a dedicated column/field, because the backend persists it
// onto the student on approval and staff need it for offline supervision.
func TestBuildPhaseExportTable_IncludesCompanionNote(t *testing.T) {
	t.Parallel()

	data := sampleExport()
	data.Rows[0].Children[0].Child.CustomData[enrollmentModels.TargetStudentDepartureCompanionNote] = "Geschwisterkind Mia"

	doc := buildPhaseExportTable(data, "Anmeldungen – Test", "")

	if got := columnLabels(doc)["child_companion_note"]; got != "Mit welchem Kind?" {
		t.Fatalf("child_companion_note column label = %q, want Mit welchem Kind?", got)
	}
	rows := tableDataRows(doc.Rows)
	if len(rows) != 1 {
		t.Fatalf("data rows = %d, want 1", len(rows))
	}
	if got := rows[0].Values["child_companion_note"]; got != "Geschwisterkind Mia" {
		t.Errorf("child_companion_note value = %q, want Geschwisterkind Mia", got)
	}
}

// Without a note the dedicated column still exists (stable schema) but the cell
// stays empty — no stray reserved-key leakage.
func TestBuildPhaseExportTable_CompanionNoteEmptyWhenAbsent(t *testing.T) {
	t.Parallel()

	doc := buildPhaseExportTable(sampleExport(), "Anmeldungen – Test", "")
	rows := tableDataRows(doc.Rows)
	if len(rows) != 1 {
		t.Fatalf("data rows = %d, want 1", len(rows))
	}
	if got := rows[0].Values["child_companion_note"]; got != "" {
		t.Errorf("child_companion_note value = %q, want empty", got)
	}
}

func TestBuildStudentEnrollmentExportTable_IncludesWithdrawnAt(t *testing.T) {
	t.Parallel()

	doc := buildStudentEnrollmentExportTable(sampleStudentEnrollmentExport(), "Anmeldungen Kind")

	labels := columnLabels(doc)
	if labels["withdrawn_at"] != "Zurückgezogen am" {
		t.Fatalf("withdrawn_at column label = %q, want Zurückgezogen am", labels["withdrawn_at"])
	}

	rows := tableDataRows(doc.Rows)
	if len(rows) != 1 {
		t.Fatalf("data rows = %d, want 1", len(rows))
	}
	if got := rows[0].Values["withdrawn_at"]; got != "02.06.2026 09:30" {
		t.Errorf("withdrawn_at = %q, want 02.06.2026 09:30", got)
	}
}

// The XLSX columns must lead with the child (every child column, incl.
// child custom fields), then the Eltern/contact block — matching the
// child-first PDF.
func TestBuildPhaseExportTable_ChildColumnsBeforeGuardian(t *testing.T) {
	t.Parallel()

	doc := buildPhaseExportTable(sampleExport(), "Anmeldungen – Test", "")
	idx := make(map[listexport.ColumnID]int, len(doc.Columns))
	for i, c := range doc.Columns {
		idx[c.ID] = i
	}

	// First column is the child surname (the sort key); the guardian
	// columns and consents all come after every child column.
	if doc.Columns[0].ID != "child_last_name" {
		t.Errorf("first column = %q, want child_last_name", doc.Columns[0].ID)
	}
	lastChild := idx["child_offerings"]
	if c := idx["custom_c_bus"]; c > lastChild {
		lastChild = c // child custom fields belong to the child block
	}
	for _, after := range []listexport.ColumnID{
		"guardian_last_name", "guardian_email", "custom_g_notes", "consent_agb",
	} {
		if idx[after] < lastChild {
			t.Errorf("column %q (index %d) must come after the child block (ends %d)", after, idx[after], lastChild)
		}
	}
}

// The applied child-status filter must be named in both documents'
// header (Filters), mirroring the students export. No filter → no label.
func TestPhaseExport_FilterLabelInHeader(t *testing.T) {
	t.Parallel()

	// PDF (RecordDocument)
	pdf := buildPhaseExportRecords(sampleExport(), "P", "approved")
	if len(pdf.Filters) != 1 || pdf.Filters[0] != "Status: Angenommen" {
		t.Errorf("pdf filters = %v, want [\"Status: Angenommen\"]", pdf.Filters)
	}
	// XLSX (Document)
	xlsx := buildPhaseExportTable(sampleExport(), "P", "withdrawn")
	if len(xlsx.Filters) != 1 || xlsx.Filters[0] != "Status: Zurückgezogen" {
		t.Errorf("xlsx filters = %v, want [\"Status: Zurückgezogen\"]", xlsx.Filters)
	}
	// No filter → no header label in either format.
	if got := buildPhaseExportRecords(sampleExport(), "P", "").Filters; len(got) != 0 {
		t.Errorf("unfiltered pdf filters = %v, want empty", got)
	}
	if got := buildPhaseExportTable(sampleExport(), "P", "").Filters; len(got) != 0 {
		t.Errorf("unfiltered xlsx filters = %v, want empty", got)
	}
}

func fieldValue(fields []listexport.Field, label string) (string, bool) {
	for _, f := range fields {
		if f.Label == label {
			return f.Value, true
		}
	}
	return "", false
}

// The PDF is child-first: one standalone block per child (child name as
// the heading, no sub-records), with the guardian/contact details
// repeated inside each block so it is self-contained for the offline
// fallback.
func TestBuildPhaseExportRecords_ChildIsPrimaryWithGuardianRepeated(t *testing.T) {
	t.Parallel()

	doc := buildPhaseExportRecords(sampleExport(), "Anmeldungen – Test", "")

	if len(doc.Records) != 1 {
		t.Fatalf("records = %d, want 1 (one child)", len(doc.Records))
	}
	rec := doc.Records[0]
	if rec.Title != "Lina Muster" {
		t.Errorf("record title = %q, want the child name 'Lina Muster'", rec.Title)
	}
	if len(rec.Subs) != 0 {
		t.Errorf("subs = %d, want 0 (child blocks are flat, not nested)", len(rec.Subs))
	}
	if !strings.Contains(doc.Footer, "personenbezogene Daten") {
		t.Errorf("footer = %q, want confidentiality handling note", doc.Footer)
	}
	if !strings.Contains(doc.Subtitle, "1 Anmeldungen") {
		t.Errorf("subtitle = %q, want it to report the counts", doc.Subtitle)
	}

	// Child's own data is present...
	if v, _ := fieldValue(rec.Fields, "Geburtsdatum"); v != "12.05.2018" {
		t.Errorf("Geburtsdatum = %q, want 12.05.2018", v)
	}
	if v, _ := fieldValue(rec.Fields, "Essen"); v != "Vegetarisch" {
		t.Errorf("child custom 'Essen' = %q, want resolved label 'Vegetarisch'", v)
	}
	// ...and the guardian/contact block is repeated inside the child block.
	if v, _ := fieldValue(rec.Fields, "Eltern"); v != "Anna Muster" {
		t.Errorf("Eltern = %q, want 'Anna Muster' repeated on the child block", v)
	}
	if v, _ := fieldValue(rec.Fields, "E-Mail"); v != "anna@example.test" {
		t.Errorf("guardian E-Mail = %q, want it repeated on the child block", v)
	}
	if v, ok := fieldValue(rec.Fields, "Zustimmungen"); !ok || v == "" {
		t.Error("consent summary must appear on the child block")
	}
}

// Co-guardians submitted alongside the primary contact must appear in both
// export formats — names, emails and phones — matching the admin detail and
// the public status page. A phone-only co-guardian (no email) must still
// carry the phone.
func TestPhaseExport_IncludesAdditionalGuardians(t *testing.T) {
	t.Parallel()

	data := sampleExport()
	omaEmail := "oma@example.test"
	omaPhone := "0151-555"
	opaPhone := "0151-777"
	data.Rows[0].Guardians = []*enrollmentModels.RequestGuardian{
		{FirstName: "Oma", LastName: "Muster", Email: &omaEmail, Phone: &omaPhone},
		{FirstName: "Opa", LastName: "Muster", Phone: &opaPhone}, // phone-only, no email
	}

	// PDF: one "Weitere Erziehungsberechtigte" field per co-guardian.
	pdf := buildPhaseExportRecords(data, "P", "")
	if len(pdf.Records) != 1 {
		t.Fatalf("records = %d, want 1", len(pdf.Records))
	}
	var coGuardianValues []string
	for _, f := range pdf.Records[0].Fields {
		if f.Label == "Weitere Erziehungsberechtigte" {
			coGuardianValues = append(coGuardianValues, f.Value)
		}
	}
	if len(coGuardianValues) != 2 {
		t.Fatalf("co-guardian fields = %d, want 2 (%v)", len(coGuardianValues), coGuardianValues)
	}
	joined := strings.Join(coGuardianValues, " | ")
	for _, want := range []string{"Oma Muster", "oma@example.test", "0151-555", "Opa Muster", "0151-777"} {
		if !strings.Contains(joined, want) {
			t.Errorf("PDF co-guardian fields %q missing %q", joined, want)
		}
	}

	// XLSX: a single "Weitere Erziehungsberechtigte" cell joining both.
	xlsx := buildPhaseExportTable(data, "P", "")
	rows := tableDataRows(xlsx.Rows)
	if len(rows) != 1 {
		t.Fatalf("xlsx data rows = %d, want 1", len(rows))
	}
	cell := rows[0].Values["additional_guardians"]
	for _, want := range []string{"Oma Muster", "oma@example.test", "0151-555", "Opa Muster", "0151-777"} {
		if !strings.Contains(cell, want) {
			t.Errorf("XLSX additional_guardians cell %q missing %q", cell, want)
		}
	}
}

func statusExportSample() *enrollmentService.PhaseExport {
	mkReq := func(first, last string) *enrollmentModels.Request {
		return &enrollmentModels.Request{
			GuardianFirstName: first,
			GuardianLastName:  last,
			GuardianEmail:     strings.ToLower(first) + "." + strings.ToLower(last) + "@example.test",
			SubmittedAt:       time.Date(2026, 6, 1, 8, 0, 0, 0, time.UTC),
			ConsentFlags:      map[string]any{},
		}
	}
	mkChild := func(first, last, status string) enrollmentService.ExportChildRow {
		return enrollmentService.ExportChildRow{Child: &enrollmentModels.RequestChild{
			FirstName:   first,
			LastName:    last,
			DateOfBirth: timezone.NewDate(2018, 1, 1),
			Status:      status,
		}}
	}
	return &enrollmentService.PhaseExport{
		Phase: &enrollmentModels.Phase{Name: "P"},
		Rows: []enrollmentService.ExportRequestRow{
			{Request: mkReq("Gesa", "Submitted"), Children: []enrollmentService.ExportChildRow{mkChild("Sina", "Ziegler", enrollmentModels.ChildStatusSubmitted)}},
			{Request: mkReq("Gesa", "Approved"), Children: []enrollmentService.ExportChildRow{
				mkChild("Aaron", "Meyer", enrollmentModels.ChildStatusApproved),
				mkChild("Lena", "Alt", enrollmentModels.ChildStatusApproved),
			}},
			{Request: mkReq("Gesa", "Rejected"), Children: []enrollmentService.ExportChildRow{mkChild("Ben", "Rose", enrollmentModels.ChildStatusRejected)}},
			{Request: mkReq("Gesa", "Waitlist"), Children: []enrollmentService.ExportChildRow{mkChild("Mia", "Klein", enrollmentModels.ChildStatusWaitlisted)}},
		},
	}
}

func TestBuildPhaseExportRecords_GroupsByChildStatus(t *testing.T) {
	t.Parallel()

	doc := buildPhaseExportRecords(statusExportSample(), "P", "")

	gotTitles := make([]string, 0, len(doc.Groups))
	for _, group := range doc.Groups {
		gotTitles = append(gotTitles, group.Title)
	}
	wantTitles := []string{
		"Bestätigte Anmeldungen",
		"Abgelehnte Anmeldungen",
		"Warteliste",
		"Eingegangene Anmeldungen",
	}
	if strings.Join(gotTitles, "|") != strings.Join(wantTitles, "|") {
		t.Fatalf("group titles = %v, want %v", gotTitles, wantTitles)
	}

	gotApproved := []string{doc.Groups[0].Records[0].Title, doc.Groups[0].Records[1].Title}
	wantApproved := []string{"Lena Alt", "Aaron Meyer"}
	if strings.Join(gotApproved, "|") != strings.Join(wantApproved, "|") {
		t.Errorf("approved group order = %v, want %v", gotApproved, wantApproved)
	}

	gotFlat := make([]string, 0, len(doc.Records))
	for _, record := range doc.Records {
		gotFlat = append(gotFlat, record.Title)
	}
	wantFlat := []string{"Lena Alt", "Aaron Meyer", "Ben Rose", "Mia Klein", "Sina Ziegler"}
	if strings.Join(gotFlat, "|") != strings.Join(wantFlat, "|") {
		t.Errorf("flat record order = %v, want grouped order %v", gotFlat, wantFlat)
	}
}

func TestBuildPhaseExportTable_InsertsStatusGroupRows(t *testing.T) {
	t.Parallel()

	doc := buildPhaseExportTable(statusExportSample(), "P", "")

	got := make([]string, 0, len(doc.Rows))
	for _, row := range doc.Rows {
		if row.GroupTitle != "" {
			got = append(got, "group:"+row.GroupTitle)
			continue
		}
		got = append(got, strings.TrimSpace(row.Values["child_first_name"]+" "+row.Values["child_last_name"]))
	}
	want := []string{
		"group:Bestätigte Anmeldungen",
		"Lena Alt",
		"Aaron Meyer",
		"group:Abgelehnte Anmeldungen",
		"Ben Rose",
		"group:Warteliste",
		"Mia Klein",
		"group:Eingegangene Anmeldungen",
		"Sina Ziegler",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("rows = %v, want %v", got, want)
	}
}

// Siblings become separate blocks and the whole document is ordered by
// child surname (case-insensitive), then first name — not by submission.
func TestBuildPhaseExportRecords_OrdersChildrenBySurname(t *testing.T) {
	t.Parallel()

	mk := func(first, last string) enrollmentService.ExportChildRow {
		return enrollmentService.ExportChildRow{Child: &enrollmentModels.RequestChild{
			FirstName: first, LastName: last,
			Status:      enrollmentModels.ChildStatusApproved,
			DateOfBirth: timezone.NewDate(2018, 1, 1),
		}}
	}
	reqA := &enrollmentModels.Request{GuardianFirstName: "G", GuardianLastName: "A", SubmittedAt: time.Now()}
	reqB := &enrollmentModels.Request{GuardianFirstName: "G", GuardianLastName: "B", SubmittedAt: time.Now()}
	data := &enrollmentService.PhaseExport{
		Phase: &enrollmentModels.Phase{Name: "P"},
		Rows: []enrollmentService.ExportRequestRow{
			// Submission order is deliberately NOT alphabetical.
			{Request: reqB, Children: []enrollmentService.ExportChildRow{mk("Max", "Muster"), mk("Lina", "Muster")}},
			{Request: reqA, Children: []enrollmentService.ExportChildRow{mk("Ava", "schmidt"), mk("Tim", "Braun")}},
		},
	}

	doc := buildPhaseExportRecords(data, "P", "")
	got := make([]string, 0, len(doc.Records))
	for _, r := range doc.Records {
		got = append(got, r.Title)
	}
	want := []string{"Tim Braun", "Lina Muster", "Max Muster", "Ava schmidt"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("order = %v, want %v (surname A–Z, case-insensitive, then first name)", got, want)
	}
}

// The XLSX rows must appear in the SAME order as the PDF blocks — both
// derive from orderedExportEntries. This regression test pins that the
// two formats stay in lock-step (child surname A–Z, case-insensitive).
func TestPhaseExport_PDFAndXLSXShareRowOrder(t *testing.T) {
	t.Parallel()

	mk := func(first, last string) enrollmentService.ExportChildRow {
		return enrollmentService.ExportChildRow{Child: &enrollmentModels.RequestChild{
			FirstName: first, LastName: last,
			Status:      enrollmentModels.ChildStatusApproved,
			DateOfBirth: timezone.NewDate(2018, 1, 1),
		}}
	}
	reqA := &enrollmentModels.Request{GuardianFirstName: "G", GuardianLastName: "A", SubmittedAt: time.Now()}
	reqB := &enrollmentModels.Request{GuardianFirstName: "G", GuardianLastName: "B", SubmittedAt: time.Now()}
	data := &enrollmentService.PhaseExport{
		Phase: &enrollmentModels.Phase{Name: "P"},
		Rows: []enrollmentService.ExportRequestRow{
			{Request: reqB, Children: []enrollmentService.ExportChildRow{mk("Max", "Muster"), mk("Lina", "Muster")}},
			{Request: reqA, Children: []enrollmentService.ExportChildRow{mk("Ava", "schmidt"), mk("Tim", "Braun")}},
		},
	}

	pdf := buildPhaseExportRecords(data, "P", "")
	xlsx := buildPhaseExportTable(data, "P", "")
	rows := tableDataRows(xlsx.Rows)
	if len(pdf.Records) != len(rows) {
		t.Fatalf("record/row counts differ: pdf=%d xlsx=%d", len(pdf.Records), len(rows))
	}
	for i := range pdf.Records {
		// PDF heading "First Last" must match the XLSX row's child name cells.
		row := rows[i].Values
		xlsxName := strings.TrimSpace(row["child_first_name"] + " " + row["child_last_name"])
		if pdf.Records[i].Title != xlsxName {
			t.Errorf("row %d order mismatch: pdf=%q xlsx=%q", i, pdf.Records[i].Title, xlsxName)
		}
	}
	// And the order is the alphabetical-by-surname order, not submission order.
	want := []string{"Tim Braun", "Lina Muster", "Max Muster", "Ava schmidt"}
	for i, w := range want {
		if pdf.Records[i].Title != w {
			t.Errorf("position %d = %q, want %q", i, pdf.Records[i].Title, w)
		}
	}
}

// A registration with no child rows must still surface as a guardian-only
// block rather than vanishing from the export.
func TestBuildPhaseExportRecords_ChildlessRegistrationKeepsGuardian(t *testing.T) {
	t.Parallel()

	data := &enrollmentService.PhaseExport{
		Phase: &enrollmentModels.Phase{Name: "P"},
		Rows: []enrollmentService.ExportRequestRow{
			{Request: &enrollmentModels.Request{
				GuardianFirstName: "Solo", GuardianLastName: "Parent",
				GuardianEmail: "solo@example.test", SubmittedAt: time.Now(),
				ConsentFlags: map[string]any{},
			}},
		},
	}
	doc := buildPhaseExportRecords(data, "P", "")
	if len(doc.Records) != 1 {
		t.Fatalf("records = %d, want 1 guardian-only block", len(doc.Records))
	}
	if doc.Records[0].Title != "Solo Parent" {
		t.Errorf("title = %q, want guardian name 'Solo Parent'", doc.Records[0].Title)
	}
	if v, _ := fieldValue(doc.Records[0].Fields, "E-Mail"); v != "solo@example.test" {
		t.Errorf("guardian E-Mail = %q, want it present on the fallback block", v)
	}
}

// A phase whose form was edited has its requests pinned to different
// schema versions. When the same custom-field key exists in more than
// one version, the export must use the CURRENT (newest) version's label,
// not a stale one — an admin who renames a field expects the new name.
func TestCollectCustomFields_NewestSchemaLabelWins(t *testing.T) {
	t.Parallel()

	oldSchema := &enrollmentModels.FormSchema{
		Fields: []enrollmentModels.FormField{
			{Key: "notes", Label: "Alter Name", Type: enrollmentModels.FormFieldText, SortOrder: 1},
		},
	}
	oldSchema.ID = 10
	newSchema := &enrollmentModels.FormSchema{
		Fields: []enrollmentModels.FormField{
			{Key: "notes", Label: "Neuer Name", Type: enrollmentModels.FormFieldText, SortOrder: 1},
		},
	}
	newSchema.ID = 20

	guardian, _ := collectCustomFields(map[int64]*enrollmentModels.FormSchema{
		oldSchema.ID: oldSchema,
		newSchema.ID: newSchema,
	})
	if len(guardian) != 1 {
		t.Fatalf("guardian custom fields = %d, want 1 (deduped by key)", len(guardian))
	}
	if guardian[0].Label != "Neuer Name" {
		t.Errorf("label = %q, want %q (newest schema version wins)", guardian[0].Label, "Neuer Name")
	}
}

func TestBuildPhaseExportFile_RejectsUnsupportedFormat(t *testing.T) {
	t.Parallel()

	if _, err := buildPhaseExportFile(nil, sampleExport(), "csv", ""); err == nil {
		t.Error("csv must be rejected on the phase export endpoint")
	}
}

func TestParseCareUsageExportRequestSupportsDOCX(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "/care-usage/export", strings.NewReader(`{
		"format": "docx",
		"filters": {"phase_id": "42", "status": "all", "care_offering_id": "9007199254740993"}
	}`))

	format, filters, err := parseCareUsageExportRequest(req)
	if err != nil {
		t.Fatalf("parseCareUsageExportRequest: %v", err)
	}
	if format != listexport.FormatDOCX {
		t.Fatalf("format = %q, want %q", format, listexport.FormatDOCX)
	}
	if filters.PhaseID != 42 {
		t.Fatalf("phase_id = %d, want 42", filters.PhaseID)
	}
	if len(filters.CareOfferingIDs) != 1 || filters.CareOfferingIDs[0] != 9007199254740993 {
		t.Fatalf("care_offering_ids = %#v, want [9007199254740993]", filters.CareOfferingIDs)
	}
}

func TestParseCareUsageFiltersFromQueryAllowsZeroDayCount(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/care-usage?phase_id=42&day_count=0&weekday=mon&pickup_time=14:30", nil)

	filters, err := parseCareUsageFiltersFromQuery(req)
	if err != nil {
		t.Fatalf("parseCareUsageFiltersFromQuery: %v", err)
	}
	if filters.DayCount == nil || *filters.DayCount != 0 {
		t.Fatalf("day_count = %#v, want pointer to 0", filters.DayCount)
	}
	if filters.Weekday != "mon" || filters.PickupTime != "14:30" {
		t.Fatalf("weekday/pickup_time = %q/%q, want mon/14:30", filters.Weekday, filters.PickupTime)
	}
}

func TestParseCareUsageFiltersFromQueryTreatsEmptyCareOfferingIDsAsExplicit(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("GET", "/care-usage?phase_id=42&care_offering_ids=", nil)

	filters, err := parseCareUsageFiltersFromQuery(req)
	if err != nil {
		t.Fatalf("parseCareUsageFiltersFromQuery: %v", err)
	}
	if !filters.CareOfferingIDsSet {
		t.Fatal("CareOfferingIDsSet = false, want true")
	}
	if len(filters.CareOfferingIDs) != 0 {
		t.Fatalf("care_offering_ids = %#v, want empty", filters.CareOfferingIDs)
	}
}

func TestParseCareUsageExportRequestAllowsZeroDayCount(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "/care-usage/export", strings.NewReader(`{
		"format": "xlsx",
		"filters": {"phase_id": "42", "status": "all", "day_count": 0, "weekday": "fri", "pickup_time": "16:00"}
	}`))

	_, filters, err := parseCareUsageExportRequest(req)
	if err != nil {
		t.Fatalf("parseCareUsageExportRequest: %v", err)
	}
	if filters.DayCount == nil || *filters.DayCount != 0 {
		t.Fatalf("day_count = %#v, want pointer to 0", filters.DayCount)
	}
	if filters.Weekday != "fri" || filters.PickupTime != "16:00" {
		t.Fatalf("weekday/pickup_time = %q/%q, want fri/16:00", filters.Weekday, filters.PickupTime)
	}
}

func TestParseCareUsageExportRequestTreatsEmptyCareOfferingIDsAsExplicit(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "/care-usage/export", strings.NewReader(`{
		"format": "xlsx",
		"filters": {"phase_id": "42", "care_offering_ids": []}
	}`))

	_, filters, err := parseCareUsageExportRequest(req)
	if err != nil {
		t.Fatalf("parseCareUsageExportRequest: %v", err)
	}
	if !filters.CareOfferingIDsSet {
		t.Fatal("CareOfferingIDsSet = false, want true")
	}
	if len(filters.CareOfferingIDs) != 0 {
		t.Fatalf("care_offering_ids = %#v, want empty", filters.CareOfferingIDs)
	}
}

func TestCareUsageReportResponseStringifiesIDs(t *testing.T) {
	t.Parallel()

	report := &enrollmentService.CareUsageReport{
		Phase: enrollmentService.CareUsagePhase{ID: 9007199254740993, Name: "Demo"},
		Filters: enrollmentService.CareUsageAppliedFilters{
			PhaseID:         9007199254740993,
			Status:          "all",
			CareOfferingIDs: []int64{9007199254740995},
			Weekday:         "mon",
			PickupTime:      "14:30",
		},
		Totals: enrollmentService.CareUsageTotals{
			Children:            1,
			ByDayCount:          map[string]int{"1": 1},
			ByWeekdayPickupTime: map[string]map[string]int{"mon": {"14:30": 1}},
		},
		ByOffering: []enrollmentService.CareUsageOfferingStat{
			{OfferingID: 9007199254740995, OfferingName: "OGS", Children: 1, ByDayCount: map[string]int{"1": 1}},
		},
		FilterOptions: enrollmentService.CareUsageFilterOptions{
			Offerings: []enrollmentService.CareUsageOfferingOption{
				{ID: 9007199254740995, Name: "OGS"},
			},
		},
		Rows: []enrollmentService.CareUsageRow{
			{
				RequestID:         9007199254740997,
				ChildID:           9007199254740999,
				ChildFirstName:    "Lina",
				ChildLastName:     "Muster",
				DateOfBirth:       "2019-01-01",
				Status:            enrollmentModels.ChildStatusApproved,
				EffectiveDays:     []string{"mon"},
				DayCount:          1,
				PickupByDay:       map[string]string{"mon": "14:30"},
				GuardianFirstName: "Eva",
				GuardianLastName:  "Muster",
				GuardianEmail:     "eva@example.test",
				SubmittedAt:       time.Date(2026, 6, 18, 11, 15, 0, 0, time.UTC),
				Offerings: []enrollmentService.CareUsageRowOffering{
					{ID: 9007199254740995, Name: "OGS", Days: []string{"mon"}, DaysSource: "selected", DaysOfWeekMode: "parent_choice"},
				},
			},
		},
	}

	raw, err := json.Marshal(toCareUsageReportResponse(report))
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	payload := string(raw)
	for _, want := range []string{
		`"id":"9007199254740993"`,
		`"phase_id":"9007199254740993"`,
		`"care_offering_ids":["9007199254740995"]`,
		`"weekday":"mon"`,
		`"pickup_time":"14:30"`,
		`"pickup_by_day":{"mon":"14:30"}`,
		`"offering_id":"9007199254740995"`,
		`"request_id":"9007199254740997"`,
		`"child_id":"9007199254740999"`,
	} {
		if !strings.Contains(payload, want) {
			t.Fatalf("response JSON %s missing %s", payload, want)
		}
	}
}

func TestCareUsageReportResponseSerializesExplicitEmptyCareOfferingIDs(t *testing.T) {
	t.Parallel()

	report := &enrollmentService.CareUsageReport{
		Phase:   enrollmentService.CareUsagePhase{ID: 42, Name: "Demo"},
		Filters: enrollmentService.CareUsageAppliedFilters{PhaseID: 42, Status: "all", CareOfferingIDs: []int64{}},
		Totals:  enrollmentService.CareUsageTotals{ByDayCount: map[string]int{"0": 1}},
	}

	raw, err := json.Marshal(toCareUsageReportResponse(report))
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if !strings.Contains(string(raw), `"care_offering_ids":[]`) {
		t.Fatalf("response JSON %s missing empty care_offering_ids", raw)
	}
}

func TestCareUsageReportResponseSerializesEmptyDaySlicesAsArrays(t *testing.T) {
	t.Parallel()

	report := &enrollmentService.CareUsageReport{
		Phase: enrollmentService.CareUsagePhase{ID: 42, Name: "Demo"},
		Filters: enrollmentService.CareUsageAppliedFilters{
			PhaseID: 42,
			Status:  "all",
		},
		Totals: enrollmentService.CareUsageTotals{ByDayCount: map[string]int{"0": 1}},
		Rows: []enrollmentService.CareUsageRow{
			{
				RequestID:         10,
				ChildID:           20,
				ChildFirstName:    "Lina",
				ChildLastName:     "Muster",
				DateOfBirth:       "2019-01-01",
				Status:            enrollmentModels.ChildStatusApproved,
				EffectiveDays:     nil,
				DayCount:          0,
				GuardianFirstName: "Eva",
				GuardianLastName:  "Muster",
				GuardianEmail:     "eva@example.test",
				SubmittedAt:       time.Date(2026, 6, 18, 11, 15, 0, 0, time.UTC),
				Offerings: []enrollmentService.CareUsageRowOffering{
					{ID: 1, Name: "OGS", Days: nil, DaysSource: "selected", DaysOfWeekMode: "parent_choice"},
				},
			},
		},
	}

	raw, err := json.Marshal(toCareUsageReportResponse(report))
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	payload := string(raw)
	for _, want := range []string{`"effective_days":[]`, `"days":[]`} {
		if !strings.Contains(payload, want) {
			t.Fatalf("response JSON %s missing %s", payload, want)
		}
	}
}

func TestCareUsageReportResponseSerializesDayProvenance(t *testing.T) {
	t.Parallel()

	report := &enrollmentService.CareUsageReport{
		Phase:   enrollmentService.CareUsagePhase{ID: 42, Name: "Demo"},
		Filters: enrollmentService.CareUsageAppliedFilters{PhaseID: 42, Status: "all"},
		Totals:  enrollmentService.CareUsageTotals{ByDayCount: map[string]int{"5": 1}},
		Rows: []enrollmentService.CareUsageRow{
			{
				RequestID:         10,
				ChildID:           20,
				ChildFirstName:    "Lina",
				ChildLastName:     "Muster",
				DateOfBirth:       "2019-01-01",
				Status:            enrollmentModels.ChildStatusApproved,
				EffectiveDays:     []string{"mon", "tue", "wed", "thu", "fri"},
				DayCount:          5,
				GuardianFirstName: "Eva",
				GuardianLastName:  "Muster",
				GuardianEmail:     "eva@example.test",
				SubmittedAt:       time.Date(2026, 6, 18, 11, 15, 0, 0, time.UTC),
				Offerings: []enrollmentService.CareUsageRowOffering{
					{
						ID:                    1,
						Name:                  "Randstunde",
						Days:                  []string{"mon", "tue", "wed", "thu", "fri"},
						DaysSource:            "selected",
						DaysOfWeekMode:        "parent_choice",
						ManualSelectedDays:    []string{"fri"},
						AutomaticSelectedDays: []string{"mon", "tue", "wed", "thu"},
					},
				},
			},
		},
	}

	raw, err := json.Marshal(toCareUsageReportResponse(report))
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	payload := string(raw)
	for _, want := range []string{`"manual_selected_days":["fri"]`, `"automatic_selected_days":["mon","tue","wed","thu"]`} {
		if !strings.Contains(payload, want) {
			t.Fatalf("response JSON %s missing %s", payload, want)
		}
	}
}

func TestCareUsageOfferingDayDetailsIncludesDayProvenance(t *testing.T) {
	t.Parallel()

	got := careUsageOfferingDayDetails([]enrollmentService.CareUsageRowOffering{
		{
			Name:                  "Randstunde",
			Days:                  []string{"mon", "tue", "wed", "thu", "fri"},
			DaysSource:            "selected",
			ManualSelectedDays:    []string{"fri"},
			AutomaticSelectedDays: []string{"mon", "tue", "wed", "thu"},
		},
	})

	if got != "Randstunde (Mo, Di, Mi, Do automatisch; Fr manuell)" {
		t.Fatalf("day details = %q", got)
	}
}

func TestBuildCareUsageRecordDocumentUsesPickupPlanningBuckets(t *testing.T) {
	t.Parallel()

	report := &enrollmentService.CareUsageReport{
		Phase:   enrollmentService.CareUsagePhase{ID: 42, Name: "Demo"},
		Filters: enrollmentService.CareUsageAppliedFilters{PhaseID: 42, Status: "all"},
		Totals: enrollmentService.CareUsageTotals{
			Children: 3,
			ByWeekdayPickupTime: map[string]map[string]int{
				"mon": {"15:30": 2, "16:00": 1},
				"tue": {"14:45": 1},
				"fri": {"16:00": 3},
			},
		},
		FilterOptions: enrollmentService.CareUsageFilterOptions{
			PickupTimes: []string{"14:45", "15:30"},
		},
	}

	doc := buildCareUsageRecordDocument(report)
	require.NotEmpty(t, doc.Records)
	assert.Equal(t, "Einsatzplanung nach Gehzeit", doc.Records[0].Title)
	fields := doc.Records[0].Fields
	got := make([]string, 0, len(fields))
	for _, field := range fields {
		got = append(got, field.Label+"="+field.Value)
	}
	for _, want := range []string{"Mo bis 14:45=0", "Mo bis 15:30=2", "Mo bis 16:00=1", "Di bis 14:45=1", "Fr bis 16:00=3"} {
		if !strings.Contains(strings.Join(got, ";"), want) {
			t.Fatalf("pickup planning fields = %#v, missing %s", got, want)
		}
	}
}

func TestBuildCareUsageExportFile_DOCX(t *testing.T) {
	t.Parallel()

	report := &enrollmentService.CareUsageReport{
		Phase:   enrollmentService.CareUsagePhase{ID: 42, Name: "Demo"},
		Filters: enrollmentService.CareUsageAppliedFilters{PhaseID: 42, Status: "all"},
		Totals: enrollmentService.CareUsageTotals{
			Children:   0,
			ByDayCount: map[string]int{"1": 0, "2": 0, "3": 0, "4": 0, "5": 0},
		},
	}

	file, err := buildCareUsageExportFile(listexport.NewService(), report, listexport.FormatDOCX)
	if err != nil {
		t.Fatalf("buildCareUsageExportFile: %v", err)
	}
	if !strings.HasSuffix(file.Filename, ".docx") {
		t.Fatalf("filename = %q, want .docx suffix", file.Filename)
	}
	if len(file.Data) == 0 {
		t.Fatal("expected non-empty docx data")
	}
}

// --- Composite reserved field rendering --------------------------------
//
// These fields arrive as jsonb (map[string]any / []any) in custom_data.
// The export must render them legibly in German — the generic stringify
// fallback produces English keys in random order, which is unusable on a
// printed offline copy.

func TestFormatCustomValue_WeekdaySchedule_GermanWeekOrder(t *testing.T) {
	t.Parallel()

	field := enrollmentModels.FormField{Type: enrollmentModels.FormFieldWeekdaySchedule}
	// Keys deliberately out of order + one empty day, to prove fixed
	// Mo–Fr ordering and that unset days are skipped.
	raw := map[string]any{"thu": "08:00", "mon": "07:30", "tue": ""}
	got := formatCustomValue(field, raw)
	if got != "Mo 07:30, Do 08:00" {
		t.Errorf("weekday schedule = %q, want %q", got, "Mo 07:30, Do 08:00")
	}
}

func TestFormatCustomValue_PhoneList_LabelsAndPrimary(t *testing.T) {
	t.Parallel()

	field := enrollmentModels.FormField{Type: enrollmentModels.FormFieldPhoneList}
	raw := []any{
		map[string]any{"phone_number": "0151-1", "phone_type": "mobile", "is_primary": true},
		map[string]any{"phone_number": "0541-2", "phone_type": "home", "label": "Oma"},
		map[string]any{"phone_number": "", "phone_type": "work"}, // empty → skipped
	}
	got := formatCustomValue(field, raw)
	if got != "Mobil: 0151-1 (primär), Oma: 0541-2" {
		t.Errorf("phone list = %q, want %q", got, "Mobil: 0151-1 (primär), Oma: 0541-2")
	}
}

func TestFormatCustomValue_ContactList_FlagsAreVisible(t *testing.T) {
	t.Parallel()

	field := enrollmentModels.FormField{Type: enrollmentModels.FormFieldContactList}
	raw := []any{
		map[string]any{
			"first_name":           "Otto",
			"last_name":            "Oma",
			"relationship_type":    "Großvater",
			"can_pickup":           true,
			"is_emergency_contact": true,
			"phone_numbers": []any{
				map[string]any{"phone_number": "0170", "phone_type": "mobile"},
			},
		},
	}
	got := formatCustomValue(field, raw)
	for _, want := range []string{"Otto Oma", "Großvater", "Tel: Mobil: 0170", "abholberechtigt", "Notfallkontakt"} {
		if !strings.Contains(got, want) {
			t.Errorf("contact list = %q, want it to contain %q", got, want)
		}
	}
}

func TestFormatCustomValue_Composite_FallsBackOnGarbage(t *testing.T) {
	t.Parallel()

	// A value that doesn't match the expected shape must not be dropped —
	// it falls back to the generic stringifier rather than rendering "".
	field := enrollmentModels.FormField{Type: enrollmentModels.FormFieldWeekdaySchedule}
	if got := formatCustomValue(field, "ganztags"); got != "ganztags" {
		t.Errorf("garbage fallback = %q, want %q", got, "ganztags")
	}
}

// --- Handler wiring (exportPhaseRegistrations) -------------------------
//
// Exercises the full handler path — claims → ExportPhase → renderer →
// streamed file + headers — with db nil (runInTenantTx short-circuits to
// the closure) so no real DB or auth middleware is needed. The unit
// tests above cover the builders in isolation; this covers the wiring.

const exportTestActorID = 4242

// buildExportRouter mounts the export handler behind a middleware that
// injects JWT claims (the production route sits behind RequiresPermission,
// which guarantees claims are present) and lets chi populate {id}.
func buildExportRouter(rs *Resource, claims jwt.AppClaims) chi.Router {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Post("/enrollment/phases/{id}/export", rs.exportPhaseRegistrations)
	r.Post("/enrollment/admin/students/{studentId}/requests/export", rs.exportStudentEnrollmentRequests)
	return r
}

func TestExportPhaseRegistrations_StreamsPDFAndForwardsActor(t *testing.T) {
	t.Parallel()

	mock := &mockDecisionService{exportResult: sampleExport()}
	rs := &Resource{
		DecisionService:   mock,
		ListExportService: listexport.NewService(),
		// db nil → runInTenantTx runs the closure directly.
	}
	router := buildExportRouter(rs, jwt.AppClaims{
		ID:    exportTestActorID,
		Roles: []string{"admin", "config_manager"},
	})

	req := httptest.NewRequest("POST", "/enrollment/phases/777/export", strings.NewReader(`{"format":"pdf"}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("content-type = %q, want application/pdf", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, `filename="`) || !strings.HasSuffix(strings.TrimSuffix(cd, `"`), ".pdf") {
		t.Errorf("content-disposition = %q, want an attachment .pdf filename", cd)
	}
	if !bytes.HasPrefix(w.Body.Bytes(), []byte("%PDF-1.")) {
		t.Error("body is not a PDF document")
	}
	if got := w.Header().Get("Content-Length"); got != "" {
		if want := len(w.Body.Bytes()); got != strconv.Itoa(want) {
			t.Errorf("content-length = %s, want %d", got, want)
		}
	}

	// The actor (from claims) and format must reach the service so the
	// GDPR audit row records who exported what.
	if mock.exportCalls != 1 {
		t.Fatalf("ExportPhase calls = %d, want 1", mock.exportCalls)
	}
	if mock.exportPhaseID != 777 {
		t.Errorf("phase id = %d, want 777", mock.exportPhaseID)
	}
	if mock.exportActorID != exportTestActorID {
		t.Errorf("actor id = %d, want %d", mock.exportActorID, exportTestActorID)
	}
	if mock.exportFormat != "pdf" {
		t.Errorf("format = %q, want pdf", mock.exportFormat)
	}
	if !strings.Contains(mock.exportActorRole, "admin") {
		t.Errorf("actor role = %q, want it to carry the claim roles", mock.exportActorRole)
	}
}

func TestExportPhaseRegistrations_StreamsXLSX(t *testing.T) {
	t.Parallel()

	mock := &mockDecisionService{exportResult: sampleExport()}
	rs := &Resource{DecisionService: mock, ListExportService: listexport.NewService()}
	router := buildExportRouter(rs, jwt.AppClaims{
		ID:    exportTestActorID,
		Roles: []string{"admin"},
	})

	req := httptest.NewRequest("POST", "/enrollment/phases/12/export", strings.NewReader(`{"format":"xlsx"}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	// XLSX is a ZIP container — the OOXML magic bytes are "PK\x03\x04".
	if !bytes.HasPrefix(w.Body.Bytes(), []byte("PK\x03\x04")) {
		t.Error("body is not an XLSX (zip) document")
	}
	if mock.exportFormat != "xlsx" {
		t.Errorf("format = %q, want xlsx", mock.exportFormat)
	}
}

func TestExportPhaseRegistrations_StreamsDOCX(t *testing.T) {
	t.Parallel()

	mock := &mockDecisionService{exportResult: sampleExport()}
	rs := &Resource{DecisionService: mock, ListExportService: listexport.NewService()}
	router := buildExportRouter(rs, jwt.AppClaims{
		ID:    exportTestActorID,
		Roles: []string{"admin"},
	})

	req := httptest.NewRequest("POST", "/enrollment/phases/12/export", strings.NewReader(`{"format":"docx"}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		t.Errorf("content-type = %q, want DOCX content type", ct)
	}
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, `filename="`) || !strings.HasSuffix(strings.TrimSuffix(cd, `"`), ".docx") {
		t.Errorf("content-disposition = %q, want an attachment .docx filename", cd)
	}
	if !bytes.HasPrefix(w.Body.Bytes(), []byte("PK\x03\x04")) {
		t.Error("body is not a DOCX zip document")
	}
	xml := testpkg.ReadDocxDocumentXML(t, w.Body.Bytes())
	if strings.Contains(xml, "<w:tbl>") {
		t.Fatal("DOCX export must use record blocks, not the wide table layout")
	}
	if !strings.Contains(xml, "Lina Muster") {
		t.Fatal("DOCX export should contain the child block heading")
	}
	if !strings.Contains(xml, "Betreuungsangebote") {
		t.Fatal("DOCX export should contain the child field labels")
	}
	if mock.exportFormat != "docx" {
		t.Errorf("format = %q, want docx", mock.exportFormat)
	}
}

func TestExportPhaseRegistrations_ServiceErrorIs500(t *testing.T) {
	t.Parallel()

	mock := &mockDecisionService{exportErr: context.DeadlineExceeded}
	rs := &Resource{DecisionService: mock, ListExportService: listexport.NewService()}
	router := buildExportRouter(rs, jwt.AppClaims{
		ID:    exportTestActorID,
		Roles: []string{"admin"},
	})

	req := httptest.NewRequest("POST", "/enrollment/phases/12/export", strings.NewReader(`{"format":"pdf"}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// A failed ExportPhase (e.g. the mandatory audit write failed) must
	// surface as 5xx so no file is served without a recorded disclosure.
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
}

func TestExportStudentEnrollmentRequests_StudentNotFoundIs404(t *testing.T) {
	t.Parallel()

	mock := &mockDecisionService{exportStudentErr: enrollmentService.ErrDecisionStudentNotFound}
	rs := &Resource{DecisionService: mock, ListExportService: listexport.NewService()}
	router := buildExportRouter(rs, jwt.AppClaims{
		ID:    exportTestActorID,
		Roles: []string{"admin"},
	})

	req := httptest.NewRequest("POST", "/enrollment/admin/students/777/requests/export", strings.NewReader(`{"format":"pdf"}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body: %s)", w.Code, w.Body.String())
	}
	if mock.exportStudentID != 777 {
		t.Errorf("student id = %d, want 777", mock.exportStudentID)
	}
}

func TestExportPhaseRegistrations_InvalidPhaseIDIs400(t *testing.T) {
	t.Parallel()

	rs := &Resource{DecisionService: &mockDecisionService{}, ListExportService: listexport.NewService()}
	router := buildExportRouter(rs, jwt.AppClaims{
		ID:    exportTestActorID,
		Roles: []string{"admin"},
	})

	req := httptest.NewRequest("POST", "/enrollment/phases/abc/export", strings.NewReader(`{"format":"pdf"}`))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}
