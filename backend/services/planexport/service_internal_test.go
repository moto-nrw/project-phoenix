package planexport

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// Pure unit tests against in-memory projections: the IDs below are struct
// field values, never database rows. The renderer is captured so the tests
// assert what lands on the page, which is the thing that can silently go
// wrong (a missing cancellation, a reason leaking onto the wall sheet).

// monday is 2026-07-27, a Monday, so days[0..4] are Mon–Fri.
var (
	monday    = timezone.NewDate(2026, time.July, 27)
	tuesday   = monday.AddDays(1)
	wednesday = monday.AddDays(2)
)

type captureRenderer struct {
	doc      listexport.Document
	format   listexport.Format
	filename string
}

func (c *captureRenderer) Render(doc listexport.Document, format listexport.Format, filenameBase string) (listexport.File, error) {
	c.doc = doc
	c.format = format
	c.filename = filenameBase
	return listexport.File{Data: []byte("rendered"), ContentType: "application/pdf", Filename: filenameBase + ".pdf"}, nil
}

type stubOverview struct {
	overview *scheduleSvc.StaffScheduleOverview
}

func (s stubOverview) GetOverview(_ context.Context, from, to timezone.Date) (*scheduleSvc.StaffScheduleOverview, error) {
	out := *s.overview
	out.From, out.To = from, to
	return &out, nil
}

type stubShiftTypes struct{ types []*scheduleModel.ShiftType }

func (s stubShiftTypes) ListAll(context.Context) ([]*scheduleModel.ShiftType, error) {
	return s.types, nil
}

func staffMember(id int64, first, last string) *usersModel.Staff {
	member := &usersModel.Staff{Person: &usersModel.Person{FirstName: first, LastName: last}}
	member.ID = id
	return member
}

func clock(hour, minute int) time.Time {
	return time.Date(2000, time.January, 1, hour, minute, 0, 0, time.UTC)
}

func shift(id, staffID int64, date timezone.Date, from, to time.Time) *scheduleModel.StaffShift {
	s := &scheduleModel.StaffShift{StaffID: staffID, Date: date, StartTime: from, EndTime: to}
	s.ID = id
	return s
}

func shiftType(id int64, name string) *scheduleModel.ShiftType {
	t := &scheduleModel.ShiftType{Name: name}
	t.ID = id
	return t
}

func ptr[T any](v T) *T { return &v }

// newDienstplanService builds the service with only the Dienstplan side
// wired; the care-plan readers stay nil, which the Dienstplan path never
// touches.
func newDienstplanService(overview *scheduleSvc.StaffScheduleOverview, types []*scheduleModel.ShiftType) (Service, *captureRenderer) {
	renderer := &captureRenderer{}
	return NewService(Dependencies{
		Overview:   stubOverview{overview: overview},
		ShiftTypes: stubShiftTypes{types: types},
		Renderer:   renderer,
	}, nil), renderer
}

func defaultParams() Params {
	return Params{
		From:     monday,
		To:       monday.AddDays(4),
		Template: TemplateByPerson,
		Variant:  VariantNotice,
		Format:   listexport.FormatPDF,
	}
}

// cellFor returns the rendered cell of the row with the given label, with
// the style markers stripped — most assertions are about the text.
func cellFor(t *testing.T, doc listexport.Document, label string, column listexport.ColumnID) string {
	t.Helper()
	return listexport.StripStyleMarkers(rawCellFor(t, doc, label, column))
}

// cellLines returns the cell's lines with their styles, for the assertions
// that are about emphasis rather than wording.
func cellLines(t *testing.T, doc listexport.Document, label string, column listexport.ColumnID) []listexport.Line {
	t.Helper()
	raw := rawCellFor(t, doc, label, column)
	lines := make([]listexport.Line, 0, 4)
	for _, encoded := range strings.Split(raw, "\n") {
		decoded, _ := listexport.DecodeLine(encoded)
		lines = append(lines, decoded)
	}
	return lines
}

func rawCellFor(t *testing.T, doc listexport.Document, label string, column listexport.ColumnID) string {
	t.Helper()
	for _, row := range doc.Rows {
		if listexport.StripStyleMarkers(row.Values[listexport.ColumnPlanRowLabel]) == label {
			return row.Values[column]
		}
	}
	t.Fatalf("no row labelled %q in %v", label, rowLabels(doc))
	return ""
}

func rowLabels(doc listexport.Document) []string {
	labels := make([]string, 0, len(doc.Rows))
	for _, row := range doc.Rows {
		if row.GroupTitle != "" {
			labels = append(labels, "#"+row.GroupTitle)
			continue
		}
		labels = append(labels, listexport.StripStyleMarkers(row.Values[listexport.ColumnPlanRowLabel]))
	}
	return labels
}

func TestDienstplanByPersonRendersShiftAndTask(t *testing.T) {
	overview := &scheduleSvc.StaffScheduleOverview{
		Staff:  []*usersModel.Staff{staffMember(7, "Franziska", "Kessener")},
		Shifts: []*scheduleModel.StaffShift{withType(shift(1, 7, monday, clock(7, 30), clock(14, 0)), 4)},
		Assignments: []scheduleSvc.StaffScheduleAssignment{{
			StaffID: 7, Date: monday, StartTime: clock(12, 0), EndTime: clock(13, 0),
			ActivityTitle: "Mensa", RoomName: "Speisesaal",
		}},
	}
	service, renderer := newDienstplanService(overview, []*scheduleModel.ShiftType{shiftType(4, "Frühdienst")})

	if _, err := service.ExportDienstplan(context.Background(), defaultParams()); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}

	cell := cellFor(t, renderer.doc, "Kessener, Franziska", listexport.ColumnPlanMonday)
	lines := strings.Split(cell, "\n")
	if len(lines) != 2 {
		t.Fatalf("cell = %q, want a shift line and a task line", cell)
	}
	if lines[0] != "07:30–14:00 · Frühdienst" {
		t.Fatalf("shift line = %q", lines[0])
	}
	if lines[1] != "12:00–13:00 Mensa · Speisesaal" {
		t.Fatalf("task line = %q", lines[1])
	}
	// A day without anything planned must say so, not go blank.
	if got := cellFor(t, renderer.doc, "Kessener, Franziska", listexport.ColumnPlanTuesday); got != "—" {
		t.Fatalf("empty day = %q, want an em dash", got)
	}
}

func withType(s *scheduleModel.StaffShift, typeID int64) *scheduleModel.StaffShift {
	s.ShiftTypeID = &typeID
	return s
}

// The wall sheet names the cancellation and its cover, but never the reason:
// "krank" beside a name is a health datum.
func TestDienstplanNoticeVariantHidesCancellationReason(t *testing.T) {
	cancelled := shift(1, 7, monday, clock(7, 30), clock(14, 0))
	cancelled.Cancelled = true
	cancelled.ChangeReason = ptr("krank")
	cover := shift(2, 8, monday, clock(7, 30), clock(14, 0))
	cover.OriginShiftID = ptr(int64(1))

	overview := &scheduleSvc.StaffScheduleOverview{
		Staff: []*usersModel.Staff{
			staffMember(7, "Franziska", "Kessener"),
			staffMember(8, "Anna", "Müller"),
		},
		Shifts: []*scheduleModel.StaffShift{cancelled, cover},
	}

	for _, tc := range []struct {
		variant    Variant
		wantReason bool
	}{
		{VariantNotice, false},
		{VariantInternal, true},
	} {
		t.Run(string(tc.variant), func(t *testing.T) {
			service, renderer := newDienstplanService(overview, nil)
			params := defaultParams()
			params.Variant = tc.variant
			if _, err := service.ExportDienstplan(context.Background(), params); err != nil {
				t.Fatalf("ExportDienstplan: %v", err)
			}

			cell := cellFor(t, renderer.doc, "Kessener, Franziska", listexport.ColumnPlanMonday)
			if !strings.Contains(cell, "entfällt") {
				t.Fatalf("cell = %q, want the cancellation marked", cell)
			}
			if !strings.Contains(cell, "Vertretung: Müller, A.") {
				t.Fatalf("cell = %q, want the cover named", cell)
			}
			if got := strings.Contains(cell, "krank"); got != tc.wantReason {
				t.Fatalf("cell = %q, reason present = %v, want %v", cell, got, tc.wantReason)
			}
		})
	}
}

// Uncovered intervals are an office concern, not a corridor one.
func TestDienstplanUncoveredIntervalsOnlyInternal(t *testing.T) {
	overview := &scheduleSvc.StaffScheduleOverview{
		Staff: []*usersModel.Staff{staffMember(7, "Franziska", "Kessener")},
		Assignments: []scheduleSvc.StaffScheduleAssignment{{
			StaffID: 7, Date: monday, StartTime: clock(12, 0), EndTime: clock(13, 0),
			ActivityTitle: "Mensa",
			UncoveredIntervals: []scheduleSvc.ShiftCoverageInterval{
				{StartTime: clock(12, 30), EndTime: clock(13, 0)},
			},
		}},
	}

	service, renderer := newDienstplanService(overview, nil)
	if _, err := service.ExportDienstplan(context.Background(), defaultParams()); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}
	if cell := cellFor(t, renderer.doc, "Kessener, Franziska", listexport.ColumnPlanMonday); strings.Contains(cell, "ohne Schicht") {
		t.Fatalf("notice cell leaked a coverage gap: %q", cell)
	}

	service, renderer = newDienstplanService(overview, nil)
	params := defaultParams()
	params.Variant = VariantInternal
	if _, err := service.ExportDienstplan(context.Background(), params); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}
	if cell := cellFor(t, renderer.doc, "Kessener, Franziska", listexport.ColumnPlanMonday); !strings.Contains(cell, "ohne Schicht 12:30–13:00") {
		t.Fatalf("internal cell = %q, want the coverage gap", cell)
	}
}

// A wall plan listing every staff member with five empty days wastes the
// page the reader needs for the rest.
func TestDienstplanSkipsStaffWithoutEntries(t *testing.T) {
	overview := &scheduleSvc.StaffScheduleOverview{
		Staff: []*usersModel.Staff{
			staffMember(7, "Franziska", "Kessener"),
			staffMember(9, "Ohne", "Schicht"),
		},
		Shifts: []*scheduleModel.StaffShift{shift(1, 7, monday, clock(7, 30), clock(14, 0))},
	}
	service, renderer := newDienstplanService(overview, nil)
	if _, err := service.ExportDienstplan(context.Background(), defaultParams()); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}

	labels := rowLabels(renderer.doc)
	if len(labels) != 1 || labels[0] != "Kessener, Franziska" {
		t.Fatalf("rows = %v, want only the staff member with entries", labels)
	}
}

// The Einsatzbereich view has a row per deployment: shifts fill their
// Schichtart row, blocks fill their own, and someone doing both appears in
// both with different times — a deployment sheet whose Ganztag row goes
// blank on the days someone also runs an Angebot answers nothing.
func TestDienstplanByAreaGroupsNamesUnderTasks(t *testing.T) {
	overview := &scheduleSvc.StaffScheduleOverview{
		Staff: []*usersModel.Staff{
			staffMember(7, "Franziska", "Kessener"),
			staffMember(8, "Anna", "Müller"),
		},
		Shifts: []*scheduleModel.StaffShift{
			// Monday: both have a Mensa block, so neither forms a Schichtart row.
			withType(shift(1, 7, monday, clock(11, 0), clock(16, 0)), 4),
			withType(shift(2, 8, monday, clock(11, 0), clock(16, 0)), 4),
			// Tuesday: a shift with no block at all — this is the Randstunde case.
			withType(shift(3, 7, tuesday, clock(7, 30), clock(8, 0)), 5),
		},
		Assignments: []scheduleSvc.StaffScheduleAssignment{
			{StaffID: 7, Date: monday, StartTime: clock(12, 0), EndTime: clock(13, 0), ActivityTitle: "Mensa", RoomName: "Speisesaal"},
			{StaffID: 8, Date: monday, StartTime: clock(12, 0), EndTime: clock(13, 0), ActivityTitle: "Mensa", RoomName: "Speisesaal"},
		},
	}
	service, renderer := newDienstplanService(overview, []*scheduleModel.ShiftType{
		shiftType(4, "Ganztag"), shiftType(5, "Randstunde"),
	})

	params := defaultParams()
	params.Template = TemplateByArea
	if _, err := service.ExportDienstplan(context.Background(), params); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}

	cell := cellFor(t, renderer.doc, "Mensa", listexport.ColumnPlanMonday)
	want := "12:00–13:00 · Speisesaal\nKessener, F.\nMüller, A."
	if cell != want {
		t.Fatalf("Mensa cell = %q, want %q", cell, want)
	}

	// The same two people are also on general Ganztag duty that day, and the
	// Ganztag row says so — with its own window, so nobody reads it as two
	// separate people.
	if cell := cellFor(t, renderer.doc, "Ganztag", listexport.ColumnPlanMonday); cell != "11:00–16:00\nKessener, F.\nMüller, A." {
		t.Fatalf("Ganztag cell = %q", cell)
	}

	if cell := cellFor(t, renderer.doc, "Randstunde", listexport.ColumnPlanTuesday); cell != "07:30–08:00\nKessener, F." {
		t.Fatalf("Randstunde cell = %q", cell)
	}

	// Rows read down the day: the earliest window first.
	labels := rowLabels(renderer.doc)
	if labels[0] != "Randstunde" {
		t.Fatalf("row order = %v, want the earliest window first", labels)
	}
}

// A single week dates its day headers and needs no group heading; several
// weeks keep generic headers and get one titled sheet each.
func TestDienstplanWeekHeadingsAndColumns(t *testing.T) {
	overview := &scheduleSvc.StaffScheduleOverview{
		Staff:  []*usersModel.Staff{staffMember(7, "Franziska", "Kessener")},
		Shifts: []*scheduleModel.StaffShift{shift(1, 7, monday, clock(7, 30), clock(14, 0))},
	}

	service, renderer := newDienstplanService(overview, nil)
	if _, err := service.ExportDienstplan(context.Background(), defaultParams()); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}
	if got := renderer.doc.Columns[1].Label; got != "Montag, 27.07." {
		t.Fatalf("single-week column = %q, want the date in the header", got)
	}
	for _, row := range renderer.doc.Rows {
		if row.GroupTitle != "" {
			t.Fatalf("single week should not emit a group heading, got %q", row.GroupTitle)
		}
	}

	service, renderer = newDienstplanService(overview, nil)
	params := defaultParams()
	params.To = monday.AddDays(11) // spans two calendar weeks
	if _, err := service.ExportDienstplan(context.Background(), params); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}
	if got := renderer.doc.Columns[1].Label; got != "Montag" {
		t.Fatalf("multi-week column = %q, want a generic header", got)
	}
	headings := 0
	for _, row := range renderer.doc.Rows {
		if row.GroupTitle != "" {
			headings++
		}
	}
	if headings != 2 {
		t.Fatalf("group headings = %d, want one per week", headings)
	}
	// The second week is empty and must say so rather than vanish.
	if !strings.Contains(strings.Join(rowLabels(renderer.doc), "|"), "Keine Einträge in dieser Woche") {
		t.Fatalf("rows = %v, want the empty week stated", rowLabels(renderer.doc))
	}
}

// A request for any weekday prints that whole week: a wall plan is a week.
func TestExpandWeeksWidensToFullWeeks(t *testing.T) {
	weeks, err := expandWeeks(wednesday, wednesday)
	if err != nil {
		t.Fatalf("expandWeeks: %v", err)
	}
	if len(weeks) != 1 {
		t.Fatalf("weeks = %d, want 1", len(weeks))
	}
	if weeks[0].days[0] != monday || weeks[0].days[4] != monday.AddDays(4) {
		t.Fatalf("week = %v–%v, want Monday–Friday", weeks[0].days[0], weeks[0].days[4])
	}
	if got := weeks[0].label(); got != "KW 31 · 27.07.–31.07.2026" {
		t.Fatalf("label = %q", got)
	}
}

func TestExpandWeeksRejectsOversizedRange(t *testing.T) {
	if _, err := expandWeeks(monday, monday.AddDays(7*maxExportWeeks)); err == nil {
		t.Fatal("expected a range error beyond the week cap")
	}
}

func TestParamsValidation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Params)
		allowed []Template
		wantErr bool
	}{
		{name: "valid", mutate: func(*Params) {}, allowed: TemplatesForDienstplan},
		{name: "inverted range", mutate: func(p *Params) { p.To = p.From.AddDays(-1) }, allowed: TemplatesForDienstplan, wantErr: true},
		{name: "unknown variant", mutate: func(p *Params) { p.Variant = "geheim" }, allowed: TemplatesForDienstplan, wantErr: true},
		{name: "docx refused", mutate: func(p *Params) { p.Format = listexport.FormatDOCX }, allowed: TemplatesForDienstplan, wantErr: true},
		{name: "care template on staff plan", mutate: func(p *Params) { p.Template = TemplateByOffering }, allowed: TemplatesForDienstplan, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := defaultParams()
			tt.mutate(&params)
			err := params.validate(tt.allowed)
			if tt.wantErr && err == nil {
				t.Fatal("expected a validation error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestParseParamsDefaults(t *testing.T) {
	params, err := ParseParams("2026-07-27", "2026-07-31", string(TemplateByPerson), "", "")
	if err != nil {
		t.Fatalf("ParseParams: %v", err)
	}
	if params.Variant != VariantNotice {
		t.Fatalf("variant = %q, want the wall sheet by default", params.Variant)
	}
	if params.Format != listexport.FormatPDF {
		t.Fatalf("format = %q, want PDF by default", params.Format)
	}
	if _, err := ParseParams("27.07.2026", "2026-07-31", string(TemplateByPerson), "", ""); err == nil {
		t.Fatal("expected a parse error for a German date")
	}
}

// Closing days and holidays are labelled, so an empty Tuesday reads as a
// closure rather than as a planning mistake.
func TestDienstplanLabelsNonWorkingDays(t *testing.T) {
	renderer := &captureRenderer{}
	service := NewService(Dependencies{
		Overview: stubOverview{overview: &scheduleSvc.StaffScheduleOverview{
			Staff:  []*usersModel.Staff{staffMember(7, "Franziska", "Kessener")},
			Shifts: []*scheduleModel.StaffShift{shift(1, 7, monday, clock(7, 30), clock(14, 0))},
		}},
		ClosingDays: stubClosingDays{days: []*scheduleModel.ClosingDay{closingRange(tuesday, tuesday, "Betriebsferien")}},
		Renderer:    renderer,
	}, nil)

	if _, err := service.ExportDienstplan(context.Background(), defaultParams()); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}
	if got := cellFor(t, renderer.doc, "Kessener, Franziska", listexport.ColumnPlanTuesday); got != "Schließtag: Betriebsferien" {
		t.Fatalf("closed day cell = %q", got)
	}
}

type stubClosingDays struct {
	scheduleSvc.ClosingDayService
	days []*scheduleModel.ClosingDay
}

func (s stubClosingDays) ClosingDaysInRange(context.Context, timezone.Date, timezone.Date) ([]*scheduleModel.ClosingDay, error) {
	return s.days, nil
}

func closingRange(start, end timezone.Date, reason string) *scheduleModel.ClosingDay {
	return &scheduleModel.ClosingDay{
		Model:     base.Model{},
		StartDate: start,
		EndDate:   end,
		Reason:    reason,
	}
}

// Rooms are routinely named after what happens in them; "Mensa · Mensa" is
// noise on a sheet meant to be read from across a room.
func TestDienstplanDropsRoomThatRepeatsTheTitle(t *testing.T) {
	overview := &scheduleSvc.StaffScheduleOverview{
		Staff: []*usersModel.Staff{staffMember(7, "Franziska", "Kessener")},
		Assignments: []scheduleSvc.StaffScheduleAssignment{{
			StaffID: 7, Date: monday, StartTime: clock(12, 0), EndTime: clock(13, 0),
			ActivityTitle: "Mensa", RoomName: "Mensa",
		}},
	}
	service, renderer := newDienstplanService(overview, nil)
	if _, err := service.ExportDienstplan(context.Background(), defaultParams()); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}
	if cell := cellFor(t, renderer.doc, "Kessener, Franziska", listexport.ColumnPlanMonday); cell != "12:00–13:00 Mensa" {
		t.Fatalf("cell = %q, want the repeated room dropped", cell)
	}
}

// The whole legibility of the sheet rides on these three ranks: the shift
// window anchors the cell in bold, the task inside it reads in normal
// weight, and secondary facts recede. Printed in one weight — the first
// version of this export — a Dienst and an Angebot are indistinguishable.
func TestDienstplanCellRanksShiftTaskAndDetail(t *testing.T) {
	cancelled := shift(1, 7, monday, clock(7, 30), clock(14, 0))
	cancelled.Cancelled = true
	cancelled.ChangeReason = ptr("Fortbildung")
	cover := shift(2, 8, monday, clock(7, 30), clock(14, 0))
	cover.OriginShiftID = ptr(int64(1))

	overview := &scheduleSvc.StaffScheduleOverview{
		Staff: []*usersModel.Staff{
			staffMember(7, "Franziska", "Kessener"),
			staffMember(8, "Anna", "Müller"),
		},
		Shifts: []*scheduleModel.StaffShift{cancelled, cover},
		Assignments: []scheduleSvc.StaffScheduleAssignment{{
			StaffID: 7, Date: monday, StartTime: clock(12, 0), EndTime: clock(13, 0),
			ActivityTitle: "Mensa", RoomName: "Speisesaal",
		}},
	}

	service, renderer := newDienstplanService(overview, nil)
	params := defaultParams()
	params.Variant = VariantInternal
	if _, err := service.ExportDienstplan(context.Background(), params); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}

	lines := cellLines(t, renderer.doc, "Kessener, Franziska", listexport.ColumnPlanMonday)
	want := []listexport.Line{
		{Text: "07:30–14:00 · entfällt", Style: listexport.LineStrong},
		{Text: "Grund: Fortbildung", Style: listexport.LineMuted},
		{Text: "Vertretung: Müller, A.", Style: listexport.LineMuted},
		{Text: "12:00–13:00 Mensa · Speisesaal", Style: listexport.LineNormal},
	}
	if len(lines) != len(want) {
		t.Fatalf("lines = %+v, want %+v", lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("line %d = %+v, want %+v", i, lines[i], want[i])
		}
	}

	// A row label is its own anchor, and an empty day recedes rather than
	// competing with the entries around it.
	if got := cellLines(t, renderer.doc, "Kessener, Franziska", listexport.ColumnPlanTuesday); len(got) != 1 ||
		got[0].Style != listexport.LineMuted || got[0].Text != "—" {
		t.Fatalf("empty day = %+v, want a single muted em dash", got)
	}
}

// The care plan uses the same three ranks.
func TestBetreuungsplanCellRanks(t *testing.T) {
	service, renderer := newBetreuungsplanService(
		[]*scheduleModel.ActivityInstance{instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3)},
		[]*scheduleModel.InstanceStaff{instanceStaff(11, 7)},
		map[int64]int{11: 24},
	)
	if _, err := service.ExportBetreuungsplan(context.Background(), careParams()); err != nil {
		t.Fatalf("ExportBetreuungsplan: %v", err)
	}

	lines := cellLines(t, renderer.doc, "Mensa", listexport.ColumnPlanMonday)
	want := []listexport.Line{
		{Text: "12:00–13:00 · Speisesaal", Style: listexport.LineStrong},
		{Text: "Kessener, F.", Style: listexport.LineNormal},
		{Text: "24 Kinder", Style: listexport.LineMuted},
	}
	for i := range want {
		if i >= len(lines) || lines[i] != want[i] {
			t.Fatalf("lines = %+v, want %+v", lines, want)
		}
	}
}

// The colour bar carries the Schichtart colour from the screen onto the
// paper. It sits on the shift line only: on the staff plan the bar means
// "this is a Dienst", which is the distinction the sheet exists to make.
func TestDienstplanShiftLineCarriesShiftTypeColour(t *testing.T) {
	overview := &scheduleSvc.StaffScheduleOverview{
		Staff:  []*usersModel.Staff{staffMember(7, "Franziska", "Kessener")},
		Shifts: []*scheduleModel.StaffShift{withType(shift(1, 7, monday, clock(7, 30), clock(14, 0)), 4)},
		Assignments: []scheduleSvc.StaffScheduleAssignment{{
			StaffID: 7, Date: monday, StartTime: clock(12, 0), EndTime: clock(13, 0),
			ActivityTitle: "Mensa",
		}},
	}
	coloured := shiftType(4, "Ganztag")
	coloured.Color = "#83CD2D"

	service, renderer := newDienstplanService(overview, []*scheduleModel.ShiftType{coloured})
	if _, err := service.ExportDienstplan(context.Background(), defaultParams()); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}

	lines := cellLines(t, renderer.doc, "Kessener, Franziska", listexport.ColumnPlanMonday)
	if lines[0].Accent != "#83CD2D" {
		t.Fatalf("shift line accent = %q, want the Schichtart colour", lines[0].Accent)
	}
	if lines[1].Accent != "" {
		t.Fatalf("task line accent = %q, want none — the bar marks the Dienst", lines[1].Accent)
	}
}

// A Schichtart without a stored colour costs the bar and nothing else.
func TestDienstplanWithoutColourStillRendersStrongLine(t *testing.T) {
	overview := &scheduleSvc.StaffScheduleOverview{
		Staff:  []*usersModel.Staff{staffMember(7, "Franziska", "Kessener")},
		Shifts: []*scheduleModel.StaffShift{shift(1, 7, monday, clock(7, 30), clock(14, 0))},
	}
	service, renderer := newDienstplanService(overview, nil)
	if _, err := service.ExportDienstplan(context.Background(), defaultParams()); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}
	lines := cellLines(t, renderer.doc, "Kessener, Franziska", listexport.ColumnPlanMonday)
	if lines[0].Style != listexport.LineStrong || lines[0].Accent != "" {
		t.Fatalf("line = %+v, want a strong line without an accent", lines[0])
	}
}
