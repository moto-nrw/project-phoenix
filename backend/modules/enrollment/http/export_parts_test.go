package enrollmenthttp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func fieldLabels(fields []lists.Field) []string {
	labels := make([]string, 0, len(fields))
	for _, f := range fields {
		labels = append(labels, f.Label)
	}
	return labels
}

func columnIDs(cols []lists.Column) []lists.ColumnID {
	ids := make([]lists.ColumnID, 0, len(cols))
	for _, c := range cols {
		ids = append(ids, c.ID)
	}
	return ids
}

func TestEnrollmentChildBlockOrdersItsParts(t *testing.T) {
	t.Parallel()

	email := "oma@example.test"
	record := enrollmentChildBlock{
		Name:        "Lina Muster",
		Child:       []lists.Field{{Label: "Zielklasse", Value: "1a"}},
		Guardian:    guardianContactFields("Anna Muster", "anna@example.test", "0151-123"),
		Submission:  []lists.Field{{Label: "Eingereicht am", Value: "01.06.2026 08:00"}},
		CoGuardians: []*capability.RequestGuardian{{FirstName: "Oma", LastName: "Muster", Email: &email}},
	}.record()

	assert.Equal(t, "Lina Muster", record.Title)
	assert.Equal(t, []string{"Zielklasse", "Eltern", "E-Mail", "Telefon", "Eingereicht am", "Weitere Erziehungsberechtigte"}, fieldLabels(record.Fields))
	assert.Equal(t, "Oma Muster (oma@example.test)", record.Fields[5].Value)
}

func TestEnrollmentChildBlockLeavesOutMissingParts(t *testing.T) {
	t.Parallel()

	record := enrollmentChildBlock{Name: "Kind", Child: []lists.Field{{Label: "Status", Value: "Angenommen"}}}.record()

	assert.Equal(t, []string{"Status"}, fieldLabels(record.Fields))
}

func TestSubmissionFieldsListDatesConsentsAndAnswers(t *testing.T) {
	t.Parallel()

	data := sampleStudentEnrollmentExport()
	guardianCustoms, _ := collectCustomFields(data.Schemas)

	fields := submissionFields(data.Rows[0].Request, guardianCustoms)

	assert.Equal(t, []string{"Eingereicht am", "Zurückgezogen am", "Zustimmungen", "Hinweise"}, fieldLabels(fields))
	assert.Equal(t, "01.06.2026 08:00", fields[0].Value)
	assert.Equal(t, "02.06.2026 09:30", fields[1].Value)
	assert.Equal(t, "Bitte Hintereingang", fields[3].Value)
}

// All three exports print a child through the shared block: the child's
// fields lead, then each export's guardian and submission parts.
func TestEnrollmentExportsPrintChildrenThroughTheSharedBlock(t *testing.T) {
	t.Parallel()

	data := sampleExport()
	guardianCustoms, childCustoms := collectCustomFields(data.Schemas)
	child := data.Rows[0].Children[0]
	childLabels := fieldLabels(childFields(child, childCustoms))
	submission := fieldLabels(submissionFields(data.Rows[0].Request, guardianCustoms))

	t.Run("Anmeldungen der Phase", func(t *testing.T) {
		doc := buildPhaseExportRecords(data, "P", "")
		require.Len(t, doc.Records, 1)
		want := append(append(append([]string{}, childLabels...), "Eltern", "E-Mail", "Telefon"), submission...)
		assert.Equal(t, want, fieldLabels(doc.Records[0].Fields))
	})

	t.Run("Anmeldungen eines Kindes", func(t *testing.T) {
		student := sampleStudentEnrollmentExport()
		doc := buildStudentEnrollmentExportRecords(student, "S")
		require.Len(t, doc.Records, 1)
		studentSubmission := fieldLabels(submissionFields(student.Rows[0].Request, guardianCustoms))
		want := append(append(append([]string{}, childLabels...), "Anmeldung"), studentSubmission...)
		assert.Equal(t, want, fieldLabels(doc.Records[0].Fields))
	})

	t.Run("Anmelde-Auswertung", func(t *testing.T) {
		report := compactLayoutTestReport()
		doc := buildCareUsageRecordDocument(report)
		require.Len(t, doc.Records, len(report.Rows)+1)
		want := append(fieldLabels(careUsageChildFields(report.Rows[0])), "Eltern", "E-Mail", "Telefon")
		assert.Equal(t, want, fieldLabels(doc.Records[1].Fields))
	})
}

func TestEnrollmentExportHeadStampsEveryLayout(t *testing.T) {
	t.Parallel()

	head := enrollmentExportHead{Title: "T", Subtitle: "S", Filters: []string{"Status: Angenommen"}}

	blocks := head.blocks(nil, nil)
	table := head.table(nil, nil)

	assert.Equal(t, exportConfidentialityNote, blocks.Footer)
	assert.Equal(t, exportConfidentialityNote, table.Footer)
	assert.Equal(t, head.Filters, blocks.Filters)
	assert.Equal(t, head.Filters, table.Filters)
	assert.False(t, blocks.GeneratedAt.IsZero())
	assert.False(t, table.GeneratedAt.IsZero())
}

func TestEnrollmentExportsShareTheHead(t *testing.T) {
	t.Parallel()

	report := compactLayoutTestReport()
	data := sampleExport()
	student := sampleStudentEnrollmentExport()
	for name, footer := range map[string]string{
		"phase blocks":     buildPhaseExportRecords(data, "P", "").Footer,
		"phase table":      buildPhaseExportTable(data, "P", "").Footer,
		"student blocks":   buildStudentEnrollmentExportRecords(student, "S").Footer,
		"student table":    buildStudentEnrollmentExportTable(student, "S").Footer,
		"care blocks":      buildCareUsageRecordDocument(report).Footer,
		"care table":       buildCareUsageTableDocument(report).Footer,
		"care compact":     buildCareUsageCompactTableDocument(report).Footer,
		"class roster doc": buildClassRosterTableDocument(&capability.ClassRosterReport{}).Footer,
	} {
		assert.Equal(t, exportConfidentialityNote, footer, name)
	}
}

func TestRenderEnrollmentExportBuildsOnlyTheChosenLayout(t *testing.T) {
	t.Parallel()

	svc := newTestDocumentRenderer()
	for _, tc := range []struct {
		format     lists.Format
		wantBlocks bool
	}{
		{lists.FormatPDF, true},
		{lists.FormatDOCX, true},
		{lists.FormatXLSX, false},
	} {
		var builtBlocks, builtTable bool
		file, err := renderEnrollmentExport(svc, tc.format, "Datei",
			func() lists.RecordDocument { builtBlocks = true; return lists.RecordDocument{Title: "T"} },
			func() lists.Document {
				builtTable = true
				return lists.Document{Title: "T", Columns: enrollmentColumns("child_last_name")}
			},
		)
		require.NoError(t, err, tc.format)
		assert.NotEmpty(t, file.Data, tc.format)
		assert.Equal(t, tc.wantBlocks, builtBlocks, tc.format)
		assert.Equal(t, !tc.wantBlocks, builtTable, tc.format)
	}

	_, err := renderEnrollmentExport(svc, "csv", "Datei",
		func() lists.RecordDocument { t.Fatal("blocks built for csv"); return lists.RecordDocument{} },
		func() lists.Document { t.Fatal("table built for csv"); return lists.Document{} },
	)
	require.Error(t, err)
}

func TestEnrollmentColumnsResolveFromTheCatalog(t *testing.T) {
	t.Parallel()

	cols := enrollmentColumns("guardian_email", "child_grade")
	assert.Equal(t, []lists.Column{{ID: "guardian_email", Label: "E-Mail"}, {ID: "child_grade", Label: "Zielklasse"}}, cols)
	assert.Panics(t, func() { enrollmentColumns("not_a_column") })
}

func TestEnrollmentTablesShareChildAndAnswerColumns(t *testing.T) {
	t.Parallel()

	data := sampleExport()
	guardianCustoms, childCustoms := collectCustomFields(data.Schemas)
	child := columnIDs(childColumns(childCustoms))
	answers := columnIDs(answerColumns(guardianCustoms))
	assert.Equal(t, []lists.ColumnID{
		"child_last_name", "child_first_name", "child_dob", "child_grade", "child_status",
		"child_status_reason", "child_activation", "child_offerings",
		"custom_c_bus", "custom_c_meal", "child_companion_note",
	}, child)
	assert.Equal(t, []lists.ColumnID{
		"custom_g_notes", "consent_agb", "consent_data_processing", "consent_email_contact", "consent_photo",
	}, answers)

	phase := columnIDs(buildPhaseExportTable(data, "P", "").Columns)
	student := columnIDs(buildStudentEnrollmentExportTable(sampleStudentEnrollmentExport(), "S").Columns)
	assert.Equal(t, child, phase[:len(child)])
	assert.Equal(t, answers, phase[len(phase)-1-len(answers):len(phase)-1])
	assert.Equal(t, child, student[3:3+len(child)])
	assert.Equal(t, answers, student[len(student)-len(answers):])
}

func TestSubmissionRowValuesMatchTheBlockFields(t *testing.T) {
	t.Parallel()

	data := sampleStudentEnrollmentExport()
	guardianCustoms, _ := collectCustomFields(data.Schemas)
	req := data.Rows[0].Request

	values := submissionRowValues(req, guardianCustoms)

	assert.Equal(t, "01.06.2026 08:00", values["submitted_at"])
	assert.Equal(t, "02.06.2026 09:30", values["withdrawn_at"])
	assert.Equal(t, "Bitte Hintereingang", values["custom_g_notes"])
	assert.Equal(t, consentLabel(req.ConsentFlags, capability.ConsentKeyPhoto), values["consent_photo"])
}

// The compact Auswertung prints the class-list table of the Klassenliste;
// only the heading of the class column differs.
func TestCompactCareUsageUsesTheClassRosterTable(t *testing.T) {
	t.Parallel()

	compact := buildCareUsageCompactTableDocument(compactLayoutTestReport())
	roster := buildClassRosterTableDocument(&capability.ClassRosterReport{})

	assert.Equal(t, columnIDs(roster.Columns), columnIDs(compact.Columns))
	for i, col := range compact.Columns {
		if col.ID == lists.ColumnSchoolClass {
			assert.Equal(t, "Zielklasse", col.Label)
			assert.Equal(t, "Klasse", roster.Columns[i].Label)
			continue
		}
		assert.Equal(t, roster.Columns[i].Label, col.Label)
	}
}
