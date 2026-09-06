package planexport

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// The plan exports read half a dozen optional sources. The rule this file
// pins down is that only the two loads a plan cannot exist without — the
// overview for the Dienstplan, the instances for the Betreuungsplan — may
// fail the export. Everything else (names, rooms, colours, head counts,
// closing days) costs a detail on the sheet and never the sheet itself. A
// silent regression here turns a failed colour lookup into a 500 on a
// printout somebody is waiting for.

var errBoom = errors.New("boom")

// Failing readers, one per optional source.

type failingOverview struct{}

func (failingOverview) GetOverview(context.Context, timezone.Date, timezone.Date) (*scheduleSvc.StaffScheduleOverview, error) {
	return nil, errBoom
}

type failingShiftTypes struct{}

func (failingShiftTypes) ListAll(context.Context) ([]*scheduleModel.ShiftType, error) {
	return nil, errBoom
}

type failingInstances struct{}

func (failingInstances) FindByTenantAndDateRange(context.Context, scheduleModel.Date, scheduleModel.Date) ([]*scheduleModel.ActivityInstance, error) {
	return nil, errBoom
}

type failingInstanceStaff struct{}

func (failingInstanceStaff) FindByInstanceIDs(context.Context, []int64) ([]*scheduleModel.InstanceStaff, error) {
	return nil, errBoom
}

type failingRooms struct{}

func (failingRooms) FindByIDs(context.Context, []int64) ([]*facilitiesModel.Room, error) {
	return nil, errBoom
}

type failingStaffDirectory struct{}

func (failingStaffDirectory) ListAllWithPerson(context.Context) ([]*usersModel.Staff, error) {
	return nil, errBoom
}

func (failingStaffDirectory) FindWithPersonByIDs(context.Context, []int64) (map[int64]*usersModel.Staff, error) {
	return nil, errBoom
}

type failingStudentCounts struct{}

func (failingStudentCounts) CountNonAbsentByInstanceIDs(context.Context, []int64) (map[int64]int, error) {
	return nil, errBoom
}

type stubActivityGroups struct {
	groups []*activitiesModel.Group
	err    error
}

func (s stubActivityGroups) FindByIDs(context.Context, []int64) ([]*activitiesModel.Group, error) {
	return s.groups, s.err
}

type stubPlanningTracks struct {
	tracks []*scheduleModel.PlanningTrack
	err    error
}

func (s stubPlanningTracks) ListAll(context.Context) ([]*scheduleModel.PlanningTrack, error) {
	return s.tracks, s.err
}

type stubHolidays struct {
	scheduleSvc.HolidayService
	days []scheduleSvc.Holiday
	err  error
}

func (s stubHolidays) HolidaysInRange(context.Context, timezone.Date, timezone.Date) ([]scheduleSvc.Holiday, error) {
	return s.days, s.err
}

type failingClosingDays struct {
	scheduleSvc.ClosingDayService
}

func (failingClosingDays) ClosingDaysInRange(context.Context, timezone.Date, timezone.Date) ([]*scheduleModel.ClosingDay, error) {
	return nil, errBoom
}

func activityGroup(id, planningTrackID int64) *activitiesModel.Group {
	g := &activitiesModel.Group{PlanningTrackID: &planningTrackID}
	g.ID = id
	return g
}

func planningTrack(id int64, color string) *scheduleModel.PlanningTrack {
	track := &scheduleModel.PlanningTrack{Color: color}
	track.ID = id
	return track
}

func withGroup(i *scheduleModel.ActivityInstance, groupID int64) *scheduleModel.ActivityInstance {
	i.ActivityGroupID = &groupID
	return i
}

// A plan needs its own data. Without the projection it prints from there is
// nothing to print, and the caller has to hear about it rather than receive
// an empty sheet that looks like an empty week.
func TestExportsFailOnTheirOwnDataSource(t *testing.T) {
	t.Parallel()

	renderer := &captureRenderer{}

	dienstplan := NewService(Dependencies{Overview: failingOverview{}, Renderer: renderer}, nil)
	if _, err := dienstplan.ExportDienstplan(context.Background(), defaultParams()); err == nil {
		t.Fatal("a failing overview must fail the export")
	}

	betreuungsplan := NewService(Dependencies{
		Instances:     failingInstances{},
		InstanceStaff: stubInstanceStaff{},
		Renderer:      renderer,
	}, nil)
	if _, err := betreuungsplan.ExportBetreuungsplan(context.Background(), careParams()); err == nil {
		t.Fatal("a failing instance read must fail the export")
	}

	betreuungsplan = NewService(Dependencies{
		Instances:     stubInstances{instances: []*scheduleModel.ActivityInstance{instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3)}},
		InstanceStaff: failingInstanceStaff{},
		Renderer:      renderer,
	}, nil)
	if _, err := betreuungsplan.ExportBetreuungsplan(context.Background(), careParams()); err == nil {
		t.Fatal("a failing staff-assignment read must fail the export")
	}
}

// A half-wired service is a programming error, not a user error: it must say
// so instead of rendering a plausible but empty document.
func TestExportsRefuseAnUnwiredService(t *testing.T) {
	t.Parallel()

	unwired := NewService(Dependencies{}, nil)
	if _, err := unwired.ExportDienstplan(context.Background(), defaultParams()); err == nil {
		t.Fatal("expected the Dienstplan to refuse an unwired service")
	}
	if _, err := unwired.ExportBetreuungsplan(context.Background(), careParams()); err == nil {
		t.Fatal("expected the Betreuungsplan to refuse an unwired service")
	}
}

// Every optional lookup failing at once still produces the plan: the shift is
// on the sheet, it has just lost its Schichtart label.
func TestDienstplanSurvivesFailingOptionalLookups(t *testing.T) {
	t.Parallel()

	renderer := &captureRenderer{}
	service := NewService(Dependencies{
		Overview: stubOverview{overview: &scheduleSvc.StaffScheduleOverview{
			Staff:  []*usersModel.Staff{staffMember(7, "Franziska", "Kessener")},
			Shifts: []*scheduleModel.StaffShift{withType(shift(1, 7, monday, clock(7, 30), clock(14, 0)), 4)},
		}},
		ShiftTypes:  failingShiftTypes{},
		ClosingDays: failingClosingDays{},
		Holidays:    stubHolidays{err: errBoom},
		Renderer:    renderer,
	}, nil)

	if _, err := service.ExportDienstplan(context.Background(), defaultParams()); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}
	if got := cellFor(t, renderer.doc, "Kessener, Franziska", listexport.ColumnPlanMonday); got != "07:30–14:00" {
		t.Fatalf("cell = %q, want the shift without its Schichtart label", got)
	}
}

// The same for the care plan: no room name, no staff name, no head count —
// but the block is still on the sheet with its window.
func TestBetreuungsplanSurvivesFailingOptionalLookups(t *testing.T) {
	t.Parallel()

	renderer := &captureRenderer{}
	service := NewService(Dependencies{
		Instances:     stubInstances{instances: []*scheduleModel.ActivityInstance{instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3)}},
		InstanceStaff: stubInstanceStaff{rows: []*scheduleModel.InstanceStaff{instanceStaff(11, 7)}},
		Rooms:         failingRooms{},
		Staff:         failingStaffDirectory{},
		Students:      failingStudentCounts{},
		Renderer:      renderer,
	}, nil)

	if _, err := service.ExportBetreuungsplan(context.Background(), careParams()); err != nil {
		t.Fatalf("ExportBetreuungsplan: %v", err)
	}
	cell := cellFor(t, renderer.doc, "Mensa", listexport.ColumnPlanMonday)
	if !strings.HasPrefix(cell, "12:00–13:00") {
		t.Fatalf("cell = %q, want the block window", cell)
	}
	if !strings.Contains(cell, "Unbekannt") {
		t.Fatalf("cell = %q, want the unresolvable staff member named as unknown", cell)
	}
	if strings.Contains(cell, "Kinder") {
		t.Fatalf("cell = %q, want no head count when the count lookup failed", cell)
	}
}

// The colour bar carries the Planungsspur colour from the planner onto paper.
// Each link in the chain (template, track, stored colour) is optional and
// only ever costs the bar.
func TestBetreuungsplanBlockColour(t *testing.T) {
	t.Parallel()

	build := func(groups ActivityGroupBatchReader, tracks PlanningTrackReader, grouped bool) *captureRenderer {
		block := instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3)
		if grouped {
			block = withGroup(block, 5)
		}
		renderer := &captureRenderer{}
		service := NewService(Dependencies{
			Instances:      stubInstances{instances: []*scheduleModel.ActivityInstance{block}},
			InstanceStaff:  stubInstanceStaff{},
			ActivityGroups: groups,
			PlanningTracks: tracks,
			Renderer:       renderer,
		}, nil)
		if _, err := service.ExportBetreuungsplan(context.Background(), careParams()); err != nil {
			t.Fatalf("ExportBetreuungsplan: %v", err)
		}
		return renderer
	}

	groups := stubActivityGroups{groups: []*activitiesModel.Group{activityGroup(5, 9)}}
	tracks := stubPlanningTracks{tracks: []*scheduleModel.PlanningTrack{planningTrack(9, "#5080D8")}}

	renderer := build(groups, tracks, true)
	if got := cellLines(t, renderer.doc, "Mensa", listexport.ColumnPlanMonday)[0].Accent; got != "#5080D8" {
		t.Fatalf("accent = %q, want the Planungsspur colour", got)
	}

	for name, tc := range map[string]struct {
		groups  ActivityGroupBatchReader
		tracks  PlanningTrackReader
		grouped bool
	}{
		"readers unwired":       {nil, nil, true},
		"block has no template": {groups, tracks, false},
		"template read fails":   {stubActivityGroups{err: errBoom}, tracks, true},
		"track read fails":      {groups, stubPlanningTracks{err: errBoom}, true},
		"track has no colour":   {groups, stubPlanningTracks{tracks: []*scheduleModel.PlanningTrack{planningTrack(9, "")}}, true},
	} {
		t.Run(name, func(t *testing.T) {
			renderer := build(tc.groups, tc.tracks, tc.grouped)
			lines := cellLines(t, renderer.doc, "Mensa", listexport.ColumnPlanMonday)
			if lines[0].Accent != "" {
				t.Fatalf("accent = %q, want none", lines[0].Accent)
			}
			if lines[0].Style != listexport.LineStrong {
				t.Fatalf("style = %v, want the anchor to stay strong without its bar", lines[0].Style)
			}
		})
	}
}

// A closing day is the tenant's own decision and carries the more specific
// wording, so it wins over the holiday that falls on the same date.
func TestNonWorkingDaysPrefersClosingDayOverHoliday(t *testing.T) {
	t.Parallel()

	renderer := &captureRenderer{}
	service := NewService(Dependencies{
		Instances:     stubInstances{instances: []*scheduleModel.ActivityInstance{instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3)}},
		InstanceStaff: stubInstanceStaff{},
		Holidays: stubHolidays{days: []scheduleSvc.Holiday{
			{Date: tuesday, Name: "Christi Himmelfahrt"},
			{Date: wednesday, Name: "Fronleichnam"},
		}},
		ClosingDays: stubClosingDays{days: []*scheduleModel.ClosingDay{
			// Deliberately overruns the printed week on both ends: only the
			// days inside it may be labelled.
			closingRange(monday.AddDays(-3), tuesday, "Betriebsferien"),
		}},
		Renderer: renderer,
	}, nil)

	if _, err := service.ExportBetreuungsplan(context.Background(), careParams()); err != nil {
		t.Fatalf("ExportBetreuungsplan: %v", err)
	}
	if got := cellFor(t, renderer.doc, "Mensa", listexport.ColumnPlanTuesday); got != "Schließtag: Betriebsferien" {
		t.Fatalf("Tuesday = %q, want the closing day to win over the holiday", got)
	}
	if got := cellFor(t, renderer.doc, "Mensa", listexport.ColumnPlanWednesday); got != "Feiertag: Fronleichnam" {
		t.Fatalf("Wednesday = %q, want the holiday labelled", got)
	}
	// The closure label leads its day; the block planned on it still follows,
	// because a closed day with something on it is a fact worth printing.
	if got := cellFor(t, renderer.doc, "Mensa", listexport.ColumnPlanMonday); !strings.HasPrefix(got, "Schließtag: Betriebsferien\n") {
		t.Fatalf("Monday = %q, want the closure to lead the cell", got)
	}
	if got := cellFor(t, renderer.doc, "Mensa", listexport.ColumnPlanThursday); got != "—" {
		t.Fatalf("Thursday = %q, want an ordinary empty day", got)
	}
}

// A block with no title still needs a row: dropping it would silently shrink
// the plan.
func TestBetreuungsplanUntitledBlocksAndSameDayOrder(t *testing.T) {
	t.Parallel()

	service, renderer := newBetreuungsplanService(
		[]*scheduleModel.ActivityInstance{
			instance(12, monday, clock(15, 0), clock(16, 0), "", 4),
			instance(11, monday, clock(12, 0), clock(13, 0), "", 3),
		},
		nil,
		nil,
	)
	if _, err := service.ExportBetreuungsplan(context.Background(), careParams()); err != nil {
		t.Fatalf("ExportBetreuungsplan: %v", err)
	}
	cell := cellFor(t, renderer.doc, "Ohne Titel", listexport.ColumnPlanMonday)
	lines := strings.Split(cell, "\n")
	if len(lines) < 3 || !strings.HasPrefix(lines[0], "12:00") {
		t.Fatalf("cell = %q, want both blocks in start order", cell)
	}
}

// Substitutes are named on the wall sheet — that is who is actually there —
// and an unresolvable staff member is stated rather than dropped.
func TestBetreuungsplanStaffLineMarksSubstitutes(t *testing.T) {
	t.Parallel()

	substitute := instanceStaff(11, 8)
	substitute.IsSubstitute = true
	unknown := instanceStaff(11, 99)

	service, renderer := newBetreuungsplanService(
		[]*scheduleModel.ActivityInstance{instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3)},
		[]*scheduleModel.InstanceStaff{substitute, unknown},
		nil,
	)
	if _, err := service.ExportBetreuungsplan(context.Background(), careParams()); err != nil {
		t.Fatalf("ExportBetreuungsplan: %v", err)
	}
	cell := cellFor(t, renderer.doc, "Mensa", listexport.ColumnPlanMonday)
	if !strings.Contains(cell, "Müller, A. (Vertretung)") {
		t.Fatalf("cell = %q, want the substitute marked", cell)
	}
	if !strings.Contains(cell, "Unbekannt") {
		t.Fatalf("cell = %q, want the unresolvable staff member stated", cell)
	}
}

// The staffing note is an office concern: it names why a block is thin.
func TestBetreuungsplanUnderstaffedNoteOnlyInternal(t *testing.T) {
	t.Parallel()

	block := instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3)
	block.UnderstaffedNote = ptr("eine Kraft fehlt")

	for _, tc := range []struct {
		variant  Variant
		wantNote bool
	}{
		{VariantNotice, false},
		{VariantInternal, true},
	} {
		t.Run(string(tc.variant), func(t *testing.T) {
			service, renderer := newBetreuungsplanService(
				[]*scheduleModel.ActivityInstance{block}, nil, nil)
			params := careParams()
			params.Variant = tc.variant
			if _, err := service.ExportBetreuungsplan(context.Background(), params); err != nil {
				t.Fatalf("ExportBetreuungsplan: %v", err)
			}
			cell := cellFor(t, renderer.doc, "Mensa", listexport.ColumnPlanMonday)
			if got := strings.Contains(cell, "eine Kraft fehlt"); got != tc.wantNote {
				t.Fatalf("cell = %q, note present = %v, want %v", cell, got, tc.wantNote)
			}
		})
	}
}

// A weekend block gets its own column instead of being dropped or folded
// into a weekday.
func TestBetreuungsplanPrintsWeekendBlocksInTheirOwnColumn(t *testing.T) {
	t.Parallel()

	saturday := monday.AddDays(5)
	service, renderer := newBetreuungsplanService(
		[]*scheduleModel.ActivityInstance{
			instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3),
			instance(12, saturday, clock(9, 0), clock(11, 0), "Mensa", 3),
		},
		nil,
		nil,
	)
	params := careParams()
	params.To = monday.AddDays(11) // two weeks, so Saturday is inside the range
	if _, err := service.ExportBetreuungsplan(context.Background(), params); err != nil {
		t.Fatalf("ExportBetreuungsplan: %v", err)
	}

	if got := columnIDs(renderer.doc); len(got) != fullWeekDays+1 {
		t.Fatalf("columns = %v, want the row label plus the whole week", got)
	}
	if got := cellFor(t, renderer.doc, "Mensa", listexport.ColumnPlanSaturday); !strings.Contains(got, "09:00–11:00") {
		t.Fatalf("Saturday cell = %q, want the Saturday block", got)
	}
	for _, column := range listexport.PlanDayColumns[:workweekDays] {
		cell := cellFor(t, renderer.doc, "Mensa", column)
		if strings.Contains(cell, "09:00–11:00") {
			t.Fatalf("column %s = %q, the Saturday block leaked into a weekday", column, cell)
		}
	}
}

// The ordinary week has no weekend to print, and two permanently empty
// columns would cost a quarter of the page.
func TestBetreuungsplanStopsAfterFridayWithoutWeekendBlocks(t *testing.T) {
	t.Parallel()

	service, renderer := newBetreuungsplanService(
		[]*scheduleModel.ActivityInstance{instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3)},
		nil,
		nil,
	)
	if _, err := service.ExportBetreuungsplan(context.Background(), careParams()); err != nil {
		t.Fatalf("ExportBetreuungsplan: %v", err)
	}
	if got := columnIDs(renderer.doc); len(got) != workweekDays+1 {
		t.Fatalf("columns = %v, want the row label plus Monday–Friday", got)
	}
}

// A Saturday shift is valid (a series accepts ISO weekdays 1–7) and has to
// reach the printed roster.
func TestDienstplanPrintsWeekendShifts(t *testing.T) {
	t.Parallel()

	saturday := monday.AddDays(5)
	overview := &scheduleSvc.StaffScheduleOverview{
		Staff:  []*usersModel.Staff{staffMember(7, "Franziska", "Kessener")},
		Shifts: []*scheduleModel.StaffShift{shift(1, 7, saturday, clock(9, 0), clock(12, 0))},
	}
	service, renderer := newDienstplanService(overview, nil)

	if _, err := service.ExportDienstplan(context.Background(), defaultParams()); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}
	if got := cellFor(t, renderer.doc, "Kessener, Franziska", listexport.ColumnPlanSaturday); !strings.Contains(got, "09:00–12:00") {
		t.Fatalf("Saturday cell = %q, want the weekend shift", got)
	}
}

func columnIDs(doc listexport.Document) []listexport.ColumnID {
	ids := make([]listexport.ColumnID, 0, len(doc.Columns))
	for _, column := range doc.Columns {
		ids = append(ids, column.ID)
	}
	return ids
}

// A shift without a Schichtart still needs a row on the deployment sheet;
// unnamed, it would otherwise disappear from the plan entirely.
func TestDienstplanByAreaKeepsShiftsWithoutShiftType(t *testing.T) {
	t.Parallel()

	absent := scheduleSvc.StaffScheduleAssignment{
		StaffID: 7, Date: monday, StartTime: clock(12, 0), EndTime: clock(13, 0),
		ActivityTitle: "Mensa", IsAbsent: true,
	}
	overview := &scheduleSvc.StaffScheduleOverview{
		Staff:       []*usersModel.Staff{staffMember(7, "Franziska", "Kessener"), nil},
		Shifts:      []*scheduleModel.StaffShift{shift(1, 7, monday, clock(7, 30), clock(14, 0)), nil},
		Assignments: []scheduleSvc.StaffScheduleAssignment{absent},
	}

	service, renderer := newDienstplanService(overview, nil)
	params := defaultParams()
	params.Template = TemplateByArea
	if _, err := service.ExportDienstplan(context.Background(), params); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}
	if got := cellFor(t, renderer.doc, areaWithoutShiftType, listexport.ColumnPlanMonday); got != "07:30–14:00\nKessener, F." {
		t.Fatalf("cell = %q, want the shift under the fallback area", got)
	}
	labels := strings.Join(rowLabels(renderer.doc), "|")
	if strings.Contains(labels, "Mensa") {
		t.Fatalf("rows = %v, want the absence left off the wall sheet", labels)
	}

	// The internal sheet marks it instead.
	service, renderer = newDienstplanService(overview, nil)
	params.Variant = VariantInternal
	if _, err := service.ExportDienstplan(context.Background(), params); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}
	if got := cellFor(t, renderer.doc, "Mensa", listexport.ColumnPlanMonday); !strings.Contains(got, "(abwesend)") {
		t.Fatalf("internal cell = %q, want the absence marked", got)
	}
}

// A replacement shift is its own row and says so; the shift note is internal.
func TestDienstplanMarksSubstituteShiftAndNote(t *testing.T) {
	t.Parallel()

	cover := shift(2, 7, monday, clock(7, 30), clock(14, 0))
	cover.OriginShiftID = ptr(int64(1))
	cover.Notes = "übernimmt die Randstunde"

	overview := &scheduleSvc.StaffScheduleOverview{
		Staff:  []*usersModel.Staff{staffMember(7, "Franziska", "Kessener")},
		Shifts: []*scheduleModel.StaffShift{cover},
	}

	service, renderer := newDienstplanService(overview, nil)
	if _, err := service.ExportDienstplan(context.Background(), defaultParams()); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}
	cell := cellFor(t, renderer.doc, "Kessener, Franziska", listexport.ColumnPlanMonday)
	if !strings.Contains(cell, "Vertretung") {
		t.Fatalf("cell = %q, want the replacement marked", cell)
	}
	if strings.Contains(cell, "übernimmt") {
		t.Fatalf("cell = %q, want the note kept off the wall sheet", cell)
	}

	service, renderer = newDienstplanService(overview, nil)
	params := defaultParams()
	params.Variant = VariantInternal
	if _, err := service.ExportDienstplan(context.Background(), params); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}
	if cell := cellFor(t, renderer.doc, "Kessener, Franziska", listexport.ColumnPlanMonday); !strings.Contains(cell, "Hinweis: übernimmt die Randstunde") {
		t.Fatalf("internal cell = %q, want the note", cell)
	}
}

// Two blocks starting at the same minute read in title order, so the cell is
// stable across exports of the same week.
func TestDienstplanOrdersSimultaneousTasksByTitle(t *testing.T) {
	t.Parallel()

	overview := &scheduleSvc.StaffScheduleOverview{
		Staff: []*usersModel.Staff{staffMember(7, "Franziska", "Kessener")},
		Assignments: []scheduleSvc.StaffScheduleAssignment{
			{StaffID: 7, Date: monday, StartTime: clock(12, 0), EndTime: clock(13, 0), ActivityTitle: "Mensa"},
			{StaffID: 7, Date: monday, StartTime: clock(12, 0), EndTime: clock(13, 0), ActivityTitle: "Lernzeit"},
		},
	}
	service, renderer := newDienstplanService(overview, nil)
	if _, err := service.ExportDienstplan(context.Background(), defaultParams()); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}
	cell := cellFor(t, renderer.doc, "Kessener, Franziska", listexport.ColumnPlanMonday)
	if want := "12:00–13:00 Lernzeit\n12:00–13:00 Mensa"; cell != want {
		t.Fatalf("cell = %q, want %q", cell, want)
	}
}

// Two shifts on one day read in start order, and a block someone covers for
// a colleague says so on the wall sheet: that is who will be standing there.
func TestDienstplanOrdersShiftsAndMarksSubstitutedTasks(t *testing.T) {
	t.Parallel()

	overview := &scheduleSvc.StaffScheduleOverview{
		Staff: []*usersModel.Staff{staffMember(7, "Franziska", "Kessener")},
		Shifts: []*scheduleModel.StaffShift{
			shift(2, 7, monday, clock(14, 0), clock(16, 30)),
			shift(1, 7, monday, clock(7, 30), clock(9, 0)),
		},
		Assignments: []scheduleSvc.StaffScheduleAssignment{{
			StaffID: 7, Date: monday, StartTime: clock(15, 0), EndTime: clock(16, 0),
			ActivityTitle: "Lernzeit", IsSubstitute: true,
		}},
	}
	service, renderer := newDienstplanService(overview, nil)
	if _, err := service.ExportDienstplan(context.Background(), defaultParams()); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}
	cell := cellFor(t, renderer.doc, "Kessener, Franziska", listexport.ColumnPlanMonday)
	want := "07:30–09:00\n14:00–16:30\n15:00–16:00 Lernzeit · Vertretung"
	if cell != want {
		t.Fatalf("cell = %q, want %q", cell, want)
	}
}

// A staff member the overview lists without a person record is a data fault,
// not a reason to omit their row or crash the export.
func TestDienstplanNamesStaffWithoutPersonRecord(t *testing.T) {
	t.Parallel()

	headless := &usersModel.Staff{}
	headless.ID = 7
	overview := &scheduleSvc.StaffScheduleOverview{
		Staff:  []*usersModel.Staff{headless},
		Shifts: []*scheduleModel.StaffShift{shift(1, 7, monday, clock(7, 30), clock(14, 0))},
	}
	service, renderer := newDienstplanService(overview, nil)
	if _, err := service.ExportDienstplan(context.Background(), defaultParams()); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}
	if labels := rowLabels(renderer.doc); len(labels) != 1 || labels[0] != "Unbekannt" {
		t.Fatalf("rows = %v, want the nameless staff member kept", labels)
	}
}

// A bad request must be refused at the export, not just by the validator the
// handlers happen to call first.
func TestExportsRefuseBadRequests(t *testing.T) {
	t.Parallel()

	service, _ := newDienstplanService(&scheduleSvc.StaffScheduleOverview{}, nil)

	params := defaultParams()
	params.Template = TemplateByOffering
	if _, err := service.ExportDienstplan(context.Background(), params); err == nil {
		t.Fatal("expected the care template to be refused by the staff export")
	}

	params = defaultParams()
	params.To = monday.AddDays(7 * maxExportWeeks)
	if _, err := service.ExportDienstplan(context.Background(), params); err == nil {
		t.Fatal("expected an oversized range to be refused")
	}

	care, _ := newBetreuungsplanService(nil, nil, nil)
	careRange := careParams()
	careRange.To = monday.AddDays(7 * maxExportWeeks)
	if _, err := care.ExportBetreuungsplan(context.Background(), careRange); err == nil {
		t.Fatal("expected an oversized range to be refused on the care plan too")
	}
}

// Someone marked absent for a block is not working it: the wall sheet must not
// send anyone looking for them there, and the internal sheet says why the
// block is thin.
func TestDienstplanByPersonHidesAbsentTasks(t *testing.T) {
	t.Parallel()

	overview := &scheduleSvc.StaffScheduleOverview{
		Staff: []*usersModel.Staff{staffMember(7, "Franziska", "Kessener"), nil},
		Assignments: []scheduleSvc.StaffScheduleAssignment{
			{StaffID: 7, Date: monday, StartTime: clock(14, 0), EndTime: clock(15, 0), ActivityTitle: "Lernzeit"},
			{StaffID: 7, Date: monday, StartTime: clock(12, 0), EndTime: clock(13, 0), ActivityTitle: "Mensa", IsAbsent: true},
		},
	}

	service, renderer := newDienstplanService(overview, nil)
	if _, err := service.ExportDienstplan(context.Background(), defaultParams()); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}
	if got := cellFor(t, renderer.doc, "Kessener, Franziska", listexport.ColumnPlanMonday); got != "14:00–15:00 Lernzeit" {
		t.Fatalf("notice cell = %q, want only the block she actually works", got)
	}

	service, renderer = newDienstplanService(overview, nil)
	params := defaultParams()
	params.Variant = VariantInternal
	if _, err := service.ExportDienstplan(context.Background(), params); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}
	// Earliest first, so the internal cell reads down the day.
	want := "12:00–13:00 Mensa · abwesend\n14:00–15:00 Lernzeit"
	if got := cellFor(t, renderer.doc, "Kessener, Franziska", listexport.ColumnPlanMonday); got != want {
		t.Fatalf("internal cell = %q, want %q", got, want)
	}
}

// The Einsatzbereich sheet is read to answer "who is standing here": a cover,
// a cancellation, and a substituted block each have to be visible in the cell,
// and a block with no title still needs a row.
func TestDienstplanByAreaLabelsCoverCancellationAndUntitledBlocks(t *testing.T) {
	t.Parallel()

	cancelled := withType(shift(1, 7, monday, clock(7, 30), clock(9, 0)), 4)
	cancelled.Cancelled = true
	cover := withType(shift(2, 8, monday, clock(7, 30), clock(9, 0)), 4)
	cover.OriginShiftID = ptr(int64(1))

	overview := &scheduleSvc.StaffScheduleOverview{
		Staff: []*usersModel.Staff{
			staffMember(7, "Franziska", "Kessener"),
			staffMember(8, "Anna", "Müller"),
		},
		Shifts: []*scheduleModel.StaffShift{cancelled, cover},
		Assignments: []scheduleSvc.StaffScheduleAssignment{
			// Same window as the shift above, so the row order falls to the title.
			{StaffID: 7, Date: monday, StartTime: clock(7, 30), EndTime: clock(9, 0), ActivityTitle: "Frühbetreuung", IsSubstitute: true},
			{StaffID: 8, Date: monday, StartTime: clock(7, 30), EndTime: clock(9, 0), ActivityTitle: "  "},
		},
	}

	service, renderer := newDienstplanService(overview, []*scheduleModel.ShiftType{shiftType(4, "Randstunde")})
	params := defaultParams()
	params.Template = TemplateByArea
	if _, err := service.ExportDienstplan(context.Background(), params); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}

	if got := cellFor(t, renderer.doc, "Randstunde", listexport.ColumnPlanMonday); got != "07:30–09:00\nKessener, F. (entfällt)\nMüller, A. (Vertretung)" {
		t.Fatalf("Randstunde cell = %q", got)
	}
	if got := cellFor(t, renderer.doc, "Frühbetreuung", listexport.ColumnPlanMonday); got != "07:30–09:00\nKessener, F. (Vertretung)" {
		t.Fatalf("Frühbetreuung cell = %q", got)
	}
	if got := cellFor(t, renderer.doc, areaWithoutShiftType, listexport.ColumnPlanMonday); got != "07:30–09:00\nMüller, A." {
		t.Fatalf("untitled block cell = %q, want it under the fallback area", got)
	}

	// Every window starts at 07:30, so the rows fall back to title order.
	labels := rowLabels(renderer.doc)
	want := []string{areaWithoutShiftType, "Frühbetreuung", "Randstunde"}
	for i, label := range want {
		if i >= len(labels) || labels[i] != label {
			t.Fatalf("row order = %v, want %v", labels, want)
		}
	}
}

// A cover shift whose replacement is not in the overview's staff list still
// has to name someone rather than print an empty "Vertretung:".
func TestDienstplanNamesAnUnknownCover(t *testing.T) {
	t.Parallel()

	cancelled := shift(1, 7, monday, clock(7, 30), clock(14, 0))
	cancelled.Cancelled = true
	cover := shift(2, 99, monday, clock(7, 30), clock(14, 0))
	cover.OriginShiftID = ptr(int64(1))

	overview := &scheduleSvc.StaffScheduleOverview{
		Staff:  []*usersModel.Staff{staffMember(7, "Franziska", "Kessener")},
		Shifts: []*scheduleModel.StaffShift{cancelled, cover},
	}
	service, renderer := newDienstplanService(overview, nil)
	if _, err := service.ExportDienstplan(context.Background(), defaultParams()); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}
	if got := cellFor(t, renderer.doc, "Kessener, Franziska", listexport.ColumnPlanMonday); !strings.Contains(got, "Vertretung: Unbekannt") {
		t.Fatalf("cell = %q, want the unresolvable cover named", got)
	}
}

// Two offerings starting at the same minute read in title order, so the same
// week exports to the same sheet every time.
func TestBetreuungsplanOrdersSimultaneousOfferingsByTitle(t *testing.T) {
	t.Parallel()

	service, renderer := newBetreuungsplanService(
		[]*scheduleModel.ActivityInstance{
			instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3),
			instance(12, monday, clock(12, 0), clock(13, 0), "Lernzeit", 4),
		},
		[]*scheduleModel.InstanceStaff{instanceStaff(11, 7)},
		nil,
	)
	if _, err := service.ExportBetreuungsplan(context.Background(), careParams()); err != nil {
		t.Fatalf("ExportBetreuungsplan: %v", err)
	}
	labels := rowLabels(renderer.doc)
	if len(labels) != 2 || labels[0] != "Lernzeit" || labels[1] != "Mensa" {
		t.Fatalf("rows = %v, want title order on equal start times", labels)
	}
}

// One block on a template, one without: the colour bar reaches the first and
// the second prints unchanged. A single uncoloured block must not cost the
// colours of the rest of the sheet.
func TestBetreuungsplanColoursOnlyTheBlocksItCan(t *testing.T) {
	t.Parallel()

	renderer := &captureRenderer{}
	service := NewService(Dependencies{
		Instances: stubInstances{instances: []*scheduleModel.ActivityInstance{
			withGroup(instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3), 5),
			instance(12, monday, clock(14, 0), clock(15, 0), "Lernzeit", 4),
		}},
		InstanceStaff:  stubInstanceStaff{},
		ActivityGroups: stubActivityGroups{groups: []*activitiesModel.Group{activityGroup(5, 9), nil}},
		PlanningTracks: stubPlanningTracks{tracks: []*scheduleModel.PlanningTrack{planningTrack(9, "#5080D8"), nil}},
		Renderer:       renderer,
	}, nil)

	if _, err := service.ExportBetreuungsplan(context.Background(), careParams()); err != nil {
		t.Fatalf("ExportBetreuungsplan: %v", err)
	}
	if got := cellLines(t, renderer.doc, "Mensa", listexport.ColumnPlanMonday)[0].Accent; got != "#5080D8" {
		t.Fatalf("Mensa accent = %q, want the Planungsspur colour", got)
	}
	if got := cellLines(t, renderer.doc, "Lernzeit", listexport.ColumnPlanMonday)[0].Accent; got != "" {
		t.Fatalf("Lernzeit accent = %q, want none for a block without a template", got)
	}
}

// Names are the one thing on the sheet nobody can look up elsewhere. A
// half-empty record must still print something a reader can act on.
func TestNameFormatting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		first, last string
		short, full string
	}{
		{"Franziska", "Kessener", "Kessener, F.", "Kessener, Franziska"},
		{"", "Kessener", "Kessener", "Kessener"},
		{"Franziska", "", "Franziska", "Franziska"},
		{" ", " ", "Unbekannt", "Unbekannt"},
	}
	for _, tt := range tests {
		if got := shortName(tt.first, tt.last); got != tt.short {
			t.Fatalf("shortName(%q, %q) = %q, want %q", tt.first, tt.last, got, tt.short)
		}
		if got := fullName(tt.first, tt.last); got != tt.full {
			t.Fatalf("fullName(%q, %q) = %q, want %q", tt.first, tt.last, got, tt.full)
		}
	}
	if got := staffShortName(nil); got != "Unbekannt" {
		t.Fatalf("staffShortName(nil) = %q", got)
	}
}

// Defensive: the exports always pass at least one week, but the document
// helpers must not index into an empty slice if that ever stops being true.
func TestDocumentHelpersTolerateAnEmptyRange(t *testing.T) {
	t.Parallel()

	if got := rangeSubtitle(nil); got != "" {
		t.Fatalf("rangeSubtitle(nil) = %q, want empty", got)
	}
	if got := filename("Dienstplan", nil); got != "Dienstplan" {
		t.Fatalf("filename with no weeks = %q", got)
	}
	// The helpers see the printed weeks, so they name the last printed day.
	weeks, err := expandWeeks(monday, monday.AddDays(7))
	if err != nil {
		t.Fatalf("expandWeeks: %v", err)
	}
	weeks = narrowWeeks(weeks, nil)
	if got := filename("Dienstplan", weeks); got != "Dienstplan 2026-07-27 bis 2026-08-07" {
		t.Fatalf("multi-week filename = %q", got)
	}
	if got := rangeSubtitle(weeks); got != "27.07.2026 – 07.08.2026 (2 Wochen)" {
		t.Fatalf("multi-week subtitle = %q", got)
	}
}

// The variant is stamped in the header so nobody has to guess which of the
// two sheets they are holding.
func TestDocumentNamesTheVariant(t *testing.T) {
	t.Parallel()

	overview := &scheduleSvc.StaffScheduleOverview{
		Staff:  []*usersModel.Staff{staffMember(7, "Franziska", "Kessener")},
		Shifts: []*scheduleModel.StaffShift{shift(1, 7, monday, clock(7, 30), clock(14, 0))},
	}
	service, renderer := newDienstplanService(overview, nil)
	params := defaultParams()
	params.Variant = VariantInternal
	if _, err := service.ExportDienstplan(context.Background(), params); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}
	if got := renderer.doc.Filters[0]; !strings.Contains(got, "Interne Fassung") {
		t.Fatalf("header pill = %q, want the internal variant named", got)
	}
	if renderer.filename != "Dienstplan 2026-07-27" {
		t.Fatalf("filename = %q", renderer.filename)
	}
}

// The injected logger is the one that must receive the export record — an
// export nobody can trace back is not auditable.
func TestExportLogsThroughTheInjectedLogger(t *testing.T) {
	t.Parallel()

	var sink bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&sink, nil))
	renderer := &captureRenderer{}
	service := NewService(Dependencies{
		Overview: stubOverview{overview: &scheduleSvc.StaffScheduleOverview{
			Staff:  []*usersModel.Staff{staffMember(7, "Franziska", "Kessener")},
			Shifts: []*scheduleModel.StaffShift{shift(1, 7, monday, clock(7, 30), clock(14, 0))},
		}},
		Renderer: renderer,
	}, logger)

	if _, err := service.ExportDienstplan(context.Background(), defaultParams()); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}
	if !strings.Contains(sink.String(), "dienstplan export rendered") {
		t.Fatalf("log = %q, want the export recorded", sink.String())
	}
}

// Both dates are required: an absent one would otherwise widen to the zero
// date's week and print a sheet from the year 1.
func TestParamsRejectMissingDates(t *testing.T) {
	t.Parallel()

	params := defaultParams()
	params.From = timezone.Date("")
	if err := params.validate(TemplatesForDienstplan); err == nil {
		t.Fatal("expected a missing from-date to be refused")
	}
	if _, err := ParseParams("2026-07-27", "not-a-date", string(TemplateByPerson), "", ""); err == nil {
		t.Fatal("expected an unparsable to-date to be refused")
	}
}

// Angebot titles are not unique: two separately configured Angebote may both
// be called "Lernzeit". Keying the deployment rows by title would print one
// row that is neither of them, with both teams' names in the same cell.
func TestDienstplanByAreaSeparatesSameNamedOfferings(t *testing.T) {
	t.Parallel()

	overview := &scheduleSvc.StaffScheduleOverview{
		Staff: []*usersModel.Staff{
			staffMember(7, "Franziska", "Kessener"),
			staffMember(8, "Anna", "Müller"),
		},
		Assignments: []scheduleSvc.StaffScheduleAssignment{
			{StaffID: 7, Date: monday, StartTime: clock(14, 0), EndTime: clock(15, 0),
				ActivityTitle: "Lernzeit", ActivityGroupID: ptr(int64(21))},
			{StaffID: 8, Date: monday, StartTime: clock(14, 0), EndTime: clock(15, 0),
				ActivityTitle: "Lernzeit", ActivityGroupID: ptr(int64(22))},
		},
	}

	service, renderer := newDienstplanService(overview, nil)
	params := defaultParams()
	params.Template = TemplateByArea
	if _, err := service.ExportDienstplan(context.Background(), params); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}

	cells := cellsForLabel(renderer.doc, "Lernzeit", listexport.ColumnPlanMonday)
	want := []string{"14:00–15:00\nKessener, F.", "14:00–15:00\nMüller, A."}
	if len(cells) != len(want) {
		t.Fatalf("Lernzeit rows = %v, want one per Angebot", cells)
	}
	for i := range want {
		if cells[i] != want[i] {
			t.Fatalf("Lernzeit rows = %v, want %v", cells, want)
		}
	}
}

// A spontaneous block has no Angebot to be identified by, so it falls back to
// its title — otherwise the same block through the week would print one row
// per occurrence.
func TestDienstplanByAreaMergesSpontaneousBlocksByTitle(t *testing.T) {
	t.Parallel()

	overview := &scheduleSvc.StaffScheduleOverview{
		Staff: []*usersModel.Staff{
			staffMember(7, "Franziska", "Kessener"),
			staffMember(8, "Anna", "Müller"),
		},
		Assignments: []scheduleSvc.StaffScheduleAssignment{
			{StaffID: 7, Date: monday, StartTime: clock(14, 0), EndTime: clock(15, 0), ActivityTitle: "Waldtag"},
			{StaffID: 8, Date: monday, StartTime: clock(14, 0), EndTime: clock(15, 0), ActivityTitle: "Waldtag"},
		},
	}

	service, renderer := newDienstplanService(overview, nil)
	params := defaultParams()
	params.Template = TemplateByArea
	if _, err := service.ExportDienstplan(context.Background(), params); err != nil {
		t.Fatalf("ExportDienstplan: %v", err)
	}

	cells := cellsForLabel(renderer.doc, "Waldtag", listexport.ColumnPlanMonday)
	if len(cells) != 1 || cells[0] != "14:00–15:00\nKessener, F.\nMüller, A." {
		t.Fatalf("Waldtag rows = %v, want one row holding both names", cells)
	}
}

// cellsForLabel returns every row carrying the label, unlike cellFor, which
// stops at the first — the point of these two tests is how many there are.
func cellsForLabel(doc listexport.Document, label string, column listexport.ColumnID) []string {
	cells := make([]string, 0, 2)
	for _, row := range doc.Rows {
		if listexport.StripStyleMarkers(row.Values[listexport.ColumnPlanRowLabel]) == label {
			cells = append(cells, listexport.StripStyleMarkers(row.Values[column]))
		}
	}
	return cells
}

// An Angebot nobody is signed up for states its zero. Printing nothing would
// make it indistinguishable from a count that could not be loaded, which is
// the one case that legitimately leaves the line off
// (TestBetreuungsplanSurvivesFailingOptionalLookups).
func TestBetreuungsplanPrintsZeroAndSingleChildCounts(t *testing.T) {
	t.Parallel()

	service, renderer := newBetreuungsplanService(
		[]*scheduleModel.ActivityInstance{
			instance(11, monday, clock(12, 0), clock(13, 0), "Mensa", 3),
			instance(12, monday, clock(14, 0), clock(15, 0), "Lernzeit", 4),
		},
		nil,
		// Instance 11 is missing from the map, which the repository uses for
		// "no non-absent rows" — a real zero, because the lookup succeeded.
		map[int64]int{12: 1},
	)

	if _, err := service.ExportBetreuungsplan(context.Background(), careParams()); err != nil {
		t.Fatalf("ExportBetreuungsplan: %v", err)
	}
	if got := cellFor(t, renderer.doc, "Mensa", listexport.ColumnPlanMonday); got != "12:00–13:00 · Speisesaal\nKein Personal eingeteilt\n0 Kinder" {
		t.Fatalf("Mensa cell = %q, want the empty group stated", got)
	}
	if got := cellFor(t, renderer.doc, "Lernzeit", listexport.ColumnPlanMonday); got != "14:00–15:00 · Gruppenraum 1\nKein Personal eingeteilt\n1 Kind" {
		t.Fatalf("Lernzeit cell = %q, want a single child in the singular", got)
	}
}
