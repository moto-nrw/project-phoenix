package listexport

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	listrecords "github.com/moto-nrw/project-phoenix/modules/documentrendering/records"

	"github.com/xuri/excelize/v2"
)

func TestRenderFormats(t *testing.T) {
	t.Parallel()

	doc := sampleDocument()
	service := NewService()

	tests := []struct {
		name        string
		format      listrecords.Format
		contentType string
		extension   string
	}{
		{name: "pdf", format: listrecords.FormatPDF, contentType: "application/pdf", extension: ".pdf"},
		{name: "docx", format: listrecords.FormatDOCX, contentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", extension: ".docx"},
		{name: "xlsx", format: listrecords.FormatXLSX, contentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", extension: ".xlsx"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file, err := service.Render(doc, tt.format, "OGS Export Liste")
			if err != nil {
				t.Fatalf("Render() error = %v", err)
			}
			if file.ContentType != tt.contentType {
				t.Fatalf("content type = %q, want %q", file.ContentType, tt.contentType)
			}
			if !strings.HasSuffix(file.Filename, tt.extension) {
				t.Fatalf("filename = %q, want suffix %q", file.Filename, tt.extension)
			}
			if len(file.Data) == 0 {
				t.Fatal("expected non-empty export data")
			}
		})
	}
}

func TestRenderPDFWritesDocumentHeader(t *testing.T) {
	t.Parallel()

	file, err := NewService().Render(sampleDocument(), listrecords.FormatPDF, "liste")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if !bytes.HasPrefix(file.Data, []byte("%PDF-1.")) {
		t.Fatalf("PDF header = %q", file.Data[:8])
	}
}

func TestRenderDOCXWritesWordDocument(t *testing.T) {
	t.Parallel()

	file, err := NewService().Render(sampleDocument(), listrecords.FormatDOCX, "liste")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(file.Data), int64(len(file.Data)))
	if err != nil {
		t.Fatalf("zip reader error = %v", err)
	}
	for _, entry := range reader.File {
		if entry.Name != "word/document.xml" {
			continue
		}
		rc, err := entry.Open()
		if err != nil {
			t.Fatalf("open document.xml error = %v", err)
		}
		defer func() {
			if err := rc.Close(); err != nil {
				t.Errorf("close document.xml error = %v", err)
			}
		}()
		content, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("read document.xml error = %v", err)
		}
		if !strings.Contains(string(content), "OGS Wochenliste") {
			t.Fatal("expected DOCX document to contain title")
		}
		return
	}
	t.Fatal("word/document.xml not found")
}

func TestRenderXLSXWritesTable(t *testing.T) {
	t.Parallel()

	file, err := NewService().Render(sampleDocument(), listrecords.FormatXLSX, "liste")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	workbook, err := excelize.OpenReader(bytes.NewReader(file.Data))
	if err != nil {
		t.Fatalf("open xlsx error = %v", err)
	}
	defer func() {
		if err := workbook.Close(); err != nil {
			t.Errorf("close xlsx error = %v", err)
		}
	}()

	title, err := workbook.GetCellValue("Export", "A1")
	if err != nil {
		t.Fatalf("read title error = %v", err)
	}
	if title != "OGS Wochenliste" {
		t.Fatalf("title = %q, want OGS Wochenliste", title)
	}
	cell, err := workbook.GetCellValue("Export", "A7")
	if err != nil {
		t.Fatalf("read row error = %v", err)
	}
	if cell != "Mila Muster" {
		t.Fatalf("first row name = %q, want Mila Muster", cell)
	}
}

func TestRenderXLSXWritesGroupHeadingRows(t *testing.T) {
	t.Parallel()

	doc := sampleDocument()
	doc.Rows = []listrecords.Row{
		{GroupTitle: "Bestätigte Anmeldungen"},
		{Values: map[listrecords.ColumnID]string{listrecords.ColumnName: "Mila Muster", listrecords.ColumnSchoolClass: "1a", listrecords.ColumnWeeklyMonday: "08:00 bis 15:00"}},
	}
	file, err := NewService().Render(doc, listrecords.FormatXLSX, "liste")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	workbook, err := excelize.OpenReader(bytes.NewReader(file.Data))
	if err != nil {
		t.Fatalf("open xlsx error = %v", err)
	}
	defer func() { _ = workbook.Close() }()

	group, err := workbook.GetCellValue("Export", "A7")
	if err != nil {
		t.Fatalf("read group row error = %v", err)
	}
	if group != "Bestätigte Anmeldungen" {
		t.Fatalf("group row = %q, want Bestätigte Anmeldungen", group)
	}
	firstName, err := workbook.GetCellValue("Export", "A8")
	if err != nil {
		t.Fatalf("read first data row error = %v", err)
	}
	if firstName != "Mila Muster" {
		t.Fatalf("first data row = %q, want Mila Muster", firstName)
	}
}

func TestRenderXLSXStartsEachGroupOnANewPrintedPage(t *testing.T) {
	t.Parallel()

	doc := sampleDocument()
	doc.Rows = []listrecords.Row{
		{GroupTitle: "Woche 1"},
		{Values: map[listrecords.ColumnID]string{listrecords.ColumnName: "Mila Muster"}},
		{GroupTitle: "Woche 2"},
		{Values: map[listrecords.ColumnID]string{listrecords.ColumnName: "Finn Beispiel"}},
	}
	file, err := NewService().Render(doc, listrecords.FormatXLSX, "liste")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	reader, err := zip.NewReader(bytes.NewReader(file.Data), int64(len(file.Data)))
	if err != nil {
		t.Fatalf("open xlsx: %v", err)
	}
	for _, entry := range reader.File {
		if !strings.HasPrefix(entry.Name, "xl/worksheets/") {
			continue
		}
		rc, err := entry.Open()
		if err != nil {
			t.Fatalf("open %s: %v", entry.Name, err)
		}
		content, err := io.ReadAll(rc)
		closeErr := rc.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("read %s: %v / %v", entry.Name, err, closeErr)
		}
		if strings.Contains(string(content), "<rowBreaks") && strings.Contains(string(content), `id="8"`) {
			return
		}
	}
	t.Fatal("expected a manual page break before the second group")
}

func TestRenderXLSXAppliesPrintSetup(t *testing.T) {
	t.Parallel()

	file, err := NewService().Render(sampleDocument(), listrecords.FormatXLSX, "liste")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	workbook, err := excelize.OpenReader(bytes.NewReader(file.Data))
	if err != nil {
		t.Fatalf("open xlsx error = %v", err)
	}
	defer func() { _ = workbook.Close() }()

	layout, err := workbook.GetPageLayout("Export")
	if err != nil {
		t.Fatalf("get page layout error = %v", err)
	}
	if layout.Orientation == nil || *layout.Orientation != "landscape" {
		t.Errorf("orientation = %v, want landscape", layout.Orientation)
	}
	if layout.FitToWidth == nil || *layout.FitToWidth != 1 {
		t.Errorf("fit to width = %v, want 1", layout.FitToWidth)
	}

	// Header row must be registered as a repeating print title.
	var sawPrintTitles bool
	for _, dn := range workbook.GetDefinedName() {
		if dn.Name == "_xlnm.Print_Titles" {
			sawPrintTitles = true
		}
	}
	if !sawPrintTitles {
		t.Error("expected _xlnm.Print_Titles defined name for repeating header row")
	}
}

func TestRenderXLSXStampsConfidentialityFooter(t *testing.T) {
	t.Parallel()

	doc := sampleDocument()
	doc.Footer = "Vertraulich – nach Gebrauch sicher vernichten."
	file, err := NewService().Render(doc, listrecords.FormatXLSX, "liste")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	workbook, err := excelize.OpenReader(bytes.NewReader(file.Data))
	if err != nil {
		t.Fatalf("open xlsx error = %v", err)
	}
	defer func() { _ = workbook.Close() }()

	hf, err := workbook.GetHeaderFooter("Export")
	if err != nil {
		t.Fatalf("get header/footer error = %v", err)
	}
	// The same full PII leaves the system as in the PDF, so the printed
	// XLSX must carry the same handling instruction on every page.
	if hf == nil || !strings.Contains(hf.OddFooter, "Vertraulich") {
		t.Errorf("odd footer = %+v, want it to contain the confidentiality note", hf)
	}
}

func TestRenderXLSXNoFooterWhenUnset(t *testing.T) {
	t.Parallel()

	file, err := NewService().Render(sampleDocument(), listrecords.FormatXLSX, "liste")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	workbook, err := excelize.OpenReader(bytes.NewReader(file.Data))
	if err != nil {
		t.Fatalf("open xlsx error = %v", err)
	}
	defer func() { _ = workbook.Close() }()

	hf, err := workbook.GetHeaderFooter("Export")
	if err != nil {
		t.Fatalf("get header/footer error = %v", err)
	}
	// sampleDocument() carries no footer — no footer must be stamped.
	if hf != nil && hf.OddFooter != "" {
		t.Errorf("odd footer = %q, want empty when listrecords.Document.Footer is unset", hf.OddFooter)
	}
}

func TestColumnCatalogExcludesRoomAndInternalIdentifier(t *testing.T) {
	t.Parallel()

	catalog := listrecords.ColumnCatalog()
	blocked := []listrecords.ColumnID{"room", "homeroom", "identifier", "internal_identifier"}
	for _, columnID := range blocked {
		if _, ok := catalog[columnID]; ok {
			t.Fatalf("column catalog contains blocked column %q", columnID)
		}
	}
}

func TestColumnCatalogIncludesDailyStatus(t *testing.T) {
	t.Parallel()

	catalog := listrecords.ColumnCatalog()
	column, ok := catalog[listrecords.ColumnDailyStatus]
	if !ok {
		t.Fatal("column catalog missing daily_status")
	}
	if column.Label != "Tagesstatus" {
		t.Fatalf("daily_status label = %q, want Tagesstatus", column.Label)
	}
}

// listrecords.ResolveColumns drops unknown column IDs silently, so a preset whose columns
// are missing from the catalog would render as a shorter list rather than an
// error. Pin both entries so that failure mode cannot reappear unnoticed.
func TestColumnCatalogIncludesBirthdayColumns(t *testing.T) {
	t.Parallel()

	catalog := listrecords.ColumnCatalog()
	for columnID, wantLabel := range map[listrecords.ColumnID]string{
		listrecords.ColumnBirthday: "Geburtstag",
		listrecords.ColumnAge:      "Alter",
	} {
		column, ok := catalog[columnID]
		if !ok {
			t.Fatalf("column catalog missing %q", columnID)
		}
		if column.Label != wantLabel {
			t.Fatalf("%q label = %q, want %q", columnID, column.Label, wantLabel)
		}
	}
}

func TestResolveColumnsKeepsBirthdayPresetColumns(t *testing.T) {
	t.Parallel()

	got := listrecords.ResolveColumns(nil, listrecords.PresetBirthdayList)

	want := listrecords.DefaultColumnsForPreset(listrecords.PresetBirthdayList)
	if len(got) != len(want) {
		t.Fatalf("listrecords.ResolveColumns(nil, birthday_list) len = %d, want %d — a column was dropped by the catalog", len(got), len(want))
	}
	for i, column := range got {
		if column.ID != want[i] {
			t.Fatalf("column %d = %q, want %q", i, column.ID, want[i])
		}
	}
}

func TestDefaultColumnsForPreset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		preset listrecords.Preset
		want   []listrecords.ColumnID
	}{
		{listrecords.PresetOGSCompact, []listrecords.ColumnID{listrecords.ColumnName, listrecords.ColumnSchoolClass, listrecords.ColumnGroup, listrecords.ColumnCareDays, listrecords.ColumnDeparture, listrecords.ColumnPlannedPickup}},
		{listrecords.PresetClassRoster, []listrecords.ColumnID{listrecords.ColumnName, listrecords.ColumnSchoolClass, listrecords.ColumnGroup, listrecords.ColumnEnrollmentSummary, listrecords.ColumnCareDays, listrecords.ColumnWeeklyMonday, listrecords.ColumnWeeklyTuesday, listrecords.ColumnWeeklyWednesday, listrecords.ColumnWeeklyThursday, listrecords.ColumnWeeklyFriday, listrecords.ColumnDeparture}},
		{listrecords.PresetDailyPlanning, []listrecords.ColumnID{listrecords.ColumnName, listrecords.ColumnSchoolClass, listrecords.ColumnGroup, listrecords.ColumnDailyStatus, listrecords.ColumnPlannedArrival, listrecords.ColumnPlannedPickup, listrecords.ColumnDailyNotes}},
		{listrecords.PresetAttendanceSnapshot, []listrecords.ColumnID{listrecords.ColumnName, listrecords.ColumnSchoolClass, listrecords.ColumnGroup, listrecords.ColumnCurrentLocation, listrecords.ColumnPlannedPickup}},
		{listrecords.PresetPickupList, []listrecords.ColumnID{listrecords.ColumnName, listrecords.ColumnSchoolClass, listrecords.ColumnGroup, listrecords.ColumnDeparture, listrecords.ColumnPlannedPickup, listrecords.ColumnDailyNotes}},
		{listrecords.PresetBlankChecklist, []listrecords.ColumnID{listrecords.ColumnName, listrecords.ColumnSchoolClass, listrecords.ColumnGroup}},
		{listrecords.PresetBirthdayList, []listrecords.ColumnID{listrecords.ColumnName, listrecords.ColumnSchoolClass, listrecords.ColumnGroup, listrecords.ColumnBirthday, listrecords.ColumnAge}},
		{listrecords.Preset("unknown"), []listrecords.ColumnID{listrecords.ColumnName, listrecords.ColumnSchoolClass, listrecords.ColumnGroup, listrecords.ColumnWeeklyMonday, listrecords.ColumnWeeklyTuesday, listrecords.ColumnWeeklyWednesday, listrecords.ColumnWeeklyThursday, listrecords.ColumnWeeklyFriday}},
	}

	for _, tt := range tests {
		t.Run(string(tt.preset), func(t *testing.T) {
			got := listrecords.DefaultColumnsForPreset(tt.preset)
			if strings.Join(columnIDsToStrings(got), ",") != strings.Join(columnIDsToStrings(tt.want), ",") {
				t.Fatalf("listrecords.DefaultColumnsForPreset(%q) = %v, want %v", tt.preset, got, tt.want)
			}
		})
	}
}

func TestResolveColumnsDedupesSkipsUnknownAndFallsBack(t *testing.T) {
	t.Parallel()

	got := listrecords.ResolveColumns([]listrecords.ColumnID{listrecords.ColumnName, "nope", listrecords.ColumnName, listrecords.ColumnStudentGroup}, listrecords.PresetOGSWeekly)
	if len(got) != 2 {
		t.Fatalf("listrecords.ResolveColumns len = %d, want 2", len(got))
	}
	if got[0].ID != listrecords.ColumnName || got[1].ID != listrecords.ColumnStudentGroup {
		t.Fatalf("listrecords.ResolveColumns = %v", got)
	}

	fallback := listrecords.ResolveColumns([]listrecords.ColumnID{"nope"}, listrecords.PresetOGSWeekly)
	if len(fallback) != len(listrecords.DefaultColumnsForPreset(listrecords.PresetOGSWeekly)) {
		t.Fatalf("fallback len = %d", len(fallback))
	}
}

func TestGeneratedAtLabelDefaultsZeroTime(t *testing.T) {
	t.Parallel()

	if listrecords.GeneratedAtLabel(time.Time{}) == "" {
		t.Fatal("listrecords.GeneratedAtLabel returned empty label for zero time")
	}

	label := listrecords.GeneratedAtLabel(time.Date(2026, time.May, 27, 16, 45, 0, 0, time.UTC))
	if label != "27.05.2026 16:45" {
		t.Fatalf("listrecords.GeneratedAtLabel = %q", label)
	}
}

// A large document must spill onto multiple pages (page count parsed from
// the rendered page objects — see countRenderedPDFPages).
func TestRenderPDFMultiPage(t *testing.T) {
	t.Parallel()

	doc := sampleDocument()
	for i := 0; i < 80; i++ {
		doc.Rows = append(doc.Rows, listrecords.Row{Values: map[listrecords.ColumnID]string{
			listrecords.ColumnName:         "Kind mit sehr langem Namen",
			listrecords.ColumnSchoolClass:  "Klasse 4a",
			listrecords.ColumnWeeklyMonday: "08:00 bis 16:00",
		}})
	}
	file, err := NewService().Render(doc, listrecords.FormatPDF, "multi-page")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if got := countRenderedPDFPages(file.Data); got < 2 {
		t.Fatalf("page count = %d, want multiple pages", got)
	}
}

func TestRenderUnsupportedFormat(t *testing.T) {
	t.Parallel()

	_, err := NewService().Render(sampleDocument(), listrecords.Format("csv"), "liste")
	if err == nil {
		t.Fatal("Render() error = nil, want unsupported format error")
	}
}

func columnIDsToStrings(ids []listrecords.ColumnID) []string {
	values := make([]string, len(ids))
	for i, id := range ids {
		values[i] = string(id)
	}
	return values
}

func sampleDocument() listrecords.Document {
	columns := listrecords.ResolveColumns([]listrecords.ColumnID{listrecords.ColumnName, listrecords.ColumnSchoolClass, listrecords.ColumnWeeklyMonday}, listrecords.PresetOGSWeekly)
	return listrecords.Document{
		Title:       "OGS Wochenliste",
		Subtitle:    "2 Kinder",
		GeneratedAt: time.Date(2026, time.May, 27, 14, 30, 0, 0, time.UTC),
		Filters:     []string{"Suche: Klasse 1a"},
		Columns:     columns,
		Rows: []listrecords.Row{
			{Values: map[listrecords.ColumnID]string{listrecords.ColumnName: "Mila Muster", listrecords.ColumnSchoolClass: "1a", listrecords.ColumnWeeklyMonday: "08:00 bis 15:00"}},
			{Values: map[listrecords.ColumnID]string{listrecords.ColumnName: "Noah Beispiel", listrecords.ColumnSchoolClass: "1a", listrecords.ColumnWeeklyMonday: "08:15 bis 16:00"}},
		},
	}
}

// #2079: a multi-line cell must wrap and leave its height automatic, so both
// explicit and soft line breaks remain visible in fixed-width plan columns.
func TestRenderXLSXWrapsMultiLineCells(t *testing.T) {
	t.Parallel()

	doc := sampleDocument()
	doc.Rows = []listrecords.Row{
		{Values: map[listrecords.ColumnID]string{listrecords.ColumnName: "07:30–14:00\nKüche\nRaum 2"}},
	}

	file, err := NewService().Render(doc, listrecords.FormatXLSX, "plan")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	workbook, err := excelize.OpenReader(bytes.NewReader(file.Data))
	if err != nil {
		t.Fatalf("open xlsx error = %v", err)
	}
	defer func() {
		if err := workbook.Close(); err != nil {
			t.Errorf("close xlsx error = %v", err)
		}
	}()

	value, err := workbook.GetCellValue("Export", "A7")
	if err != nil {
		t.Fatalf("read cell error = %v", err)
	}
	if !strings.Contains(value, "\n") {
		t.Fatalf("cell lost its line breaks: %q", value)
	}

	styleID, err := workbook.GetCellStyle("Export", "A7")
	if err != nil {
		t.Fatalf("read cell style error = %v", err)
	}
	style, err := workbook.GetStyle(styleID)
	if err != nil {
		t.Fatalf("resolve style error = %v", err)
	}
	if style.Alignment == nil || !style.Alignment.WrapText {
		t.Fatal("expected the body cell to wrap text")
	}

	reader, err := zip.NewReader(bytes.NewReader(file.Data), int64(len(file.Data)))
	if err != nil {
		t.Fatalf("open xlsx archive: %v", err)
	}
	for _, entry := range reader.File {
		if !strings.HasPrefix(entry.Name, "xl/worksheets/") {
			continue
		}
		rc, err := entry.Open()
		if err != nil {
			t.Fatalf("open %s: %v", entry.Name, err)
		}
		content, err := io.ReadAll(rc)
		closeErr := rc.Close()
		if err != nil || closeErr != nil {
			t.Fatalf("read %s: %v / %v", entry.Name, err, closeErr)
		}
		if strings.Contains(string(content), `<row r="7" ht=`) {
			t.Fatal("expected the wrapped row to retain automatic height")
		}
		return
	}
	t.Fatal("worksheet XML not found")
}

// Single-line rows keep the default height so the existing child lists
// look exactly as before.
func TestRenderXLSXKeepsDefaultHeightForSingleLineRows(t *testing.T) {
	t.Parallel()

	file, err := NewService().Render(sampleDocument(), listrecords.FormatXLSX, "liste")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	workbook, err := excelize.OpenReader(bytes.NewReader(file.Data))
	if err != nil {
		t.Fatalf("open xlsx error = %v", err)
	}
	defer func() {
		if err := workbook.Close(); err != nil {
			t.Errorf("close xlsx error = %v", err)
		}
	}()

	height, err := workbook.GetRowHeight("Export", 7)
	if err != nil {
		t.Fatalf("read row height error = %v", err)
	}
	if height > 15 {
		t.Fatalf("row height = %v, want the sheet default", height)
	}
}

// The DOCX renderer honours the same "\n is a line break" contract, so the
// three formats cannot disagree about what a composed cell means.
func TestRenderDOCXWritesLineBreaks(t *testing.T) {
	t.Parallel()

	doc := sampleDocument()
	doc.Rows = []listrecords.Row{
		{Values: map[listrecords.ColumnID]string{listrecords.ColumnName: "07:30–14:00\nKüche"}},
	}

	file, err := NewService().Render(doc, listrecords.FormatDOCX, "plan")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	reader, err := zip.NewReader(bytes.NewReader(file.Data), int64(len(file.Data)))
	if err != nil {
		t.Fatalf("zip reader error = %v", err)
	}
	for _, entry := range reader.File {
		if entry.Name != "word/document.xml" {
			continue
		}
		rc, err := entry.Open()
		if err != nil {
			t.Fatalf("open document.xml error = %v", err)
		}
		defer func() {
			if err := rc.Close(); err != nil {
				t.Errorf("close document.xml error = %v", err)
			}
		}()
		content, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("read document.xml error = %v", err)
		}
		if !strings.Contains(string(content), "<w:br/>") {
			t.Fatal("expected DOCX cell to carry an explicit line break")
		}
		return
	}
	t.Fatal("word/document.xml not found")
}
