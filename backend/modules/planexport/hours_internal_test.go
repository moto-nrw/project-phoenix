package planexport

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/services/listexport"
)

// stubHours serves the weekly figures and records the window it was asked for.
type stubHours struct {
	hours    WeeklyHours
	from, to Date
}

func (s *stubHours) WeeklyHours(_ context.Context, from, to Date) (WeeklyHours, error) {
	s.from, s.to = from, to
	return s.hours, nil
}

func exportHours(t *testing.T, hours WeeklyHours, types []*ShiftType, from, to Date) (*captureRenderer, *stubHours) {
	t.Helper()
	service, renderer := newDienstplanService(&StaffScheduleOverview{}, types)
	reader := &stubHours{hours: hours}
	if _, err := service.ExportDienstplanHours(context.Background(), hoursParams(from, to), reader); err != nil {
		t.Fatalf("ExportDienstplanHours: %v", err)
	}
	return renderer, reader
}

func hoursParams(from, to Date) Params {
	return Params{From: from, To: to, Template: TemplateByHours, Variant: VariantNotice, Format: listexport.FormatXLSX}
}

func columnLabels(doc listexport.Document) []string {
	labels := make([]string, 0, len(doc.Columns))
	for _, column := range doc.Columns {
		labels = append(labels, column.Label)
	}
	return labels
}

// The hours sheet (#3819) is one row per person: each Schichtart in its own
// column, then the weekly total against the target. Columns are sorted by
// name, the shifts without a Schichtart last, and a Schichtart the person did
// not work that week prints a dash, not "0 h".
func TestDienstplanHoursSheetSplitsTheWeekBySchichtart(t *testing.T) {
	t.Parallel()

	target, delta := 1320, -30
	overTarget, overDelta := 1500, 120
	hours := WeeklyHours{
		Staff: []*StaffMember{
			staffMember(8, "Deniz", "Kaya"),
			staffMember(7, "Anna", "Müller"),
			staffMember(9, "Ohne", "Dienst"),
		},
		Summaries: []WeeklySummary{
			{
				StaffID: 7, WeekStart: dayKey(monday), PlannedMinutes: 1290, TargetMinutes: &target, DeltaMinutes: &delta,
				ByShiftType: []ShiftTypeMinutes{{ShiftTypeID: ptr(int64(2)), Minutes: 1080}, {ShiftTypeID: ptr(int64(5)), Minutes: 120}, {Minutes: 90}},
			},
			{
				StaffID: 8, WeekStart: dayKey(monday), PlannedMinutes: 1620, TargetMinutes: &overTarget, DeltaMinutes: &overDelta,
				ByShiftType: []ShiftTypeMinutes{{ShiftTypeID: ptr(int64(2)), Minutes: 1500}, {ShiftTypeID: ptr(int64(4)), Minutes: 120}},
			},
		},
	}
	types := []*ShiftType{shiftType(2, "Wochenstunden Ganztag"), shiftType(4, "Verfügungsstunden"), shiftType(5, "Vertretungsunterricht")}
	renderer, reader := exportHours(t, hours, types, dayKey(monday.AddDays(2)), dayKey(monday.AddDays(2)))
	doc := renderer.doc

	// A Wednesday request reads the whole Monday–Sunday week: the weekly
	// figures count weekend shifts too.
	if reader.from != dayKey(monday) || reader.to != dayKey(monday.AddDays(6)) {
		t.Fatalf("reader window = %s..%s, want the whole week", reader.from, reader.to)
	}

	if doc.Title != "Stundenübersicht" || renderer.filename != "Stundenuebersicht 2026-07-27" {
		t.Fatalf("title = %q, filename = %q", doc.Title, renderer.filename)
	}
	wantColumns := []string{"Mitarbeitende", "Verfügungsstunden", "Vertretungsunterricht", "Wochenstunden Ganztag", "Ohne Schichtart", "Gesamt", "Soll", "Differenz"}
	if got := columnLabels(doc); !equalStrings(got, wantColumns) {
		t.Fatalf("columns = %v, want %v", got, wantColumns)
	}
	// Staff keep the overview order; a person without a summary has no row.
	if got := rowLabels(doc); !equalStrings(got, []string{"Kaya, Deniz", "Müller, Anna"}) {
		t.Fatalf("rows = %v", got)
	}

	want := map[listexport.ColumnID]string{
		hoursTypeColumn(2): "18 h",
		hoursTypeColumn(4): "—",
		hoursTypeColumn(5): "2 h",
		hoursColumnUntyped: "1,5 h",
		hoursColumnTotal:   "21,5 h",
		hoursColumnTarget:  "22 h",
		hoursColumnDelta:   "−0,5 h",
	}
	for column, value := range want {
		if got := cellFor(t, doc, "Müller, Anna", column); got != value {
			t.Errorf("Müller %s = %q, want %q", column, got, value)
		}
	}
	if got := cellFor(t, doc, "Kaya, Deniz", hoursColumnDelta); got != "+2 h" {
		t.Errorf("Kaya delta = %q, want +2 h", got)
	}
	if got := cellFor(t, doc, "Kaya, Deniz", hoursColumnUntyped); got != "—" {
		t.Errorf("Kaya without Schichtart = %q, want a dash", got)
	}
}

// Without a resolvable target the sheet still prints the planned hours and
// leaves Soll and Differenz empty instead of inventing a zero target.
func TestDienstplanHoursSheetWithoutTarget(t *testing.T) {
	t.Parallel()

	hours := WeeklyHours{
		Staff: []*StaffMember{staffMember(7, "Anna", "Müller")},
		Summaries: []WeeklySummary{{
			StaffID: 7, WeekStart: dayKey(monday), PlannedMinutes: 1215,
			ByShiftType: []ShiftTypeMinutes{{ShiftTypeID: ptr(int64(2)), Minutes: 1215}},
		}},
	}
	renderer, _ := exportHours(t, hours, nil, dayKey(monday), dayKey(monday))
	doc := renderer.doc
	if got := columnLabels(doc)[1]; got != "Unbekannte Schichtart" {
		t.Fatalf("type column without a resolvable name = %q", got)
	}
	if got := cellFor(t, doc, "Müller, Anna", hoursColumnTotal); got != "20,25 h" {
		t.Fatalf("total = %q, want 20,25 h", got)
	}
	for _, column := range []listexport.ColumnID{hoursColumnTarget, hoursColumnDelta} {
		if got := cellFor(t, doc, "Müller, Anna", column); got != "—" {
			t.Fatalf("%s = %q, want a dash", column, got)
		}
	}
}

// A multi-week sheet groups the rows per week and says so when a week has
// nothing planned, instead of silently dropping it.
func TestDienstplanHoursSheetGroupsWeeks(t *testing.T) {
	t.Parallel()

	nextMonday := monday.AddDays(7)
	hours := WeeklyHours{
		Staff: []*StaffMember{staffMember(7, "Anna", "Müller")},
		Summaries: []WeeklySummary{{
			StaffID: 7, WeekStart: dayKey(nextMonday), PlannedMinutes: 60,
			ByShiftType: []ShiftTypeMinutes{{Minutes: 60}},
		}},
	}
	renderer, _ := exportHours(t, hours, nil, dayKey(monday), dayKey(nextMonday))
	want := []string{"#KW 31 · 27.07.–31.07.2026", "Keine Dienste in dieser Woche", "#KW 32 · 03.08.–07.08.2026", "Müller, Anna"}
	if got := rowLabels(renderer.doc); !equalStrings(got, want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
}

// The hours template belongs to its own entry point: the Dienstplan matrix
// refuses it, and the hours sheet refuses the matrix templates.
func TestDienstplanHoursTemplateHasItsOwnEntryPoint(t *testing.T) {
	t.Parallel()

	service, _ := newDienstplanService(&StaffScheduleOverview{}, nil)
	if _, err := service.ExportDienstplan(context.Background(), hoursParams(dayKey(monday), dayKey(monday))); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("ExportDienstplan(hours) err = %v, want ErrInvalidParams", err)
	}
	params := hoursParams(dayKey(monday), dayKey(monday))
	params.Template = TemplateByPerson
	if _, err := service.ExportDienstplanHours(context.Background(), params, &stubHours{}); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("ExportDienstplanHours(persons) err = %v, want ErrInvalidParams", err)
	}
}

func TestFormatHoursMatchesTheScreen(t *testing.T) {
	t.Parallel()

	cases := map[int]string{0: "0 h", 20: "0,33 h", 90: "1,5 h", 1215: "20,25 h", 2400: "40 h"}
	for minutes, want := range cases {
		if got := formatHours(minutes); got != want {
			t.Errorf("formatHours(%d) = %q, want %q", minutes, got, want)
		}
	}
	if got := formatDeltaHours(0); got != "±0 h" {
		t.Errorf("formatDeltaHours(0) = %q", got)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
