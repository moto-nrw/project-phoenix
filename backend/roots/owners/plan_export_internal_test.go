package owners

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The bindings translate the owners' public records into the plan
// export's plain records and decide nothing themselves. These tests pin that
// translation field by field: a date that arrives one day off, a
// cancellation read from the wrong column or a dropped nil row would print a
// wrong plan without any renderer test noticing, because the renderer only
// ever sees the records.

var (
	planExportMonday  = calendar.NewDate(2026, time.July, 27)
	errPlanExportBoom = errors.New("boom")
)

func planExportClock(hour, minute int) time.Time {
	return time.Date(2000, time.January, 1, hour, minute, 0, 0, time.UTC)
}

// planExportClockString is the public ClockLayout form of a fixture clock;
// planExportWall restores the normalized wall clock the binding hands the
// renderer.
func planExportClockString(hour, minute int) string {
	return planExportClock(hour, minute).Format(workforce.ClockLayout)
}

func planExportWall(hour, minute int) time.Time {
	return calendar.NormalizeWallClock(planExportClock(hour, minute))
}

func planExportDay(date calendar.Date) planexport.Date { return planexport.Date(date.String()) }

func planExportPtr[T any](v T) *T { return &v }

type planExportFakeOverview struct {
	overview workforce.StaffScheduleOverview
	err      error
	from, to string
}

func (f *planExportFakeOverview) Overview(_ context.Context, from, to string) (workforce.StaffScheduleOverview, error) {
	f.from, f.to = from, to
	return f.overview, f.err
}

type planExportFakeShiftTypes struct {
	types []workforce.ShiftType
	err   error
}

func (f planExportFakeShiftTypes) ListShiftTypes(context.Context) ([]workforce.ShiftType, error) {
	return f.types, f.err
}

type planExportFakeInstances struct {
	instances []*timetable.ScheduledInstance
	err       error
	from, to  calendar.Date
}

func (f *planExportFakeInstances) FindByTenantAndDateRange(_ context.Context, from, to calendar.Date) ([]*timetable.ScheduledInstance, error) {
	f.from, f.to = from, to
	return f.instances, f.err
}

type planExportFakeInstanceStaff struct {
	rows []*timetable.InstanceStaff
	err  error
}

func (f planExportFakeInstanceStaff) FindByInstanceIDs(context.Context, []int64) ([]*timetable.InstanceStaff, error) {
	return f.rows, f.err
}

type planExportFakeRooms struct {
	rooms []facilities.Room
	err   error
}

func (f planExportFakeRooms) ListRoomsByID(context.Context, []int64) ([]facilities.Room, error) {
	return f.rooms, f.err
}

type planExportFakeGroups struct {
	groups []*timetable.Group
	err    error
}

func (f planExportFakeGroups) FindByIDs(context.Context, []int64) ([]*timetable.Group, error) {
	return f.groups, f.err
}

type planExportFakeTracks struct {
	tracks []timetable.PlanningTrack
	err    error
	filter timetable.PlanningTrackFilter
}

func (f *planExportFakeTracks) ListPlanningTracks(_ context.Context, filter timetable.PlanningTrackFilter) ([]timetable.PlanningTrack, error) {
	f.filter = filter
	return f.tracks, f.err
}

type planExportFakeStaff struct {
	members map[int64]*planexport.StaffMember
}

func (f planExportFakeStaff) StaffByIDs(context.Context, []int64) (map[int64]*planexport.StaffMember, error) {
	return f.members, nil
}

type planExportFakeStudentCounts struct {
	counts map[int64]int
}

func (f planExportFakeStudentCounts) CountNonAbsentByInstanceIDs(context.Context, []int64) (map[int64]int, error) {
	return f.counts, nil
}

type planExportFakeClosingDays struct {
	days []*planexport.ClosingPeriod
}

func (f planExportFakeClosingDays) ClosingDaysInRange(context.Context, planexport.Date, planexport.Date) ([]*planexport.ClosingPeriod, error) {
	return f.days, nil
}

type planExportFakeHolidays struct {
	days []planexport.Holiday
}

func (f planExportFakeHolidays) HolidaysInRange(context.Context, planexport.Date, planexport.Date) ([]planexport.Holiday, error) {
	return f.days, nil
}

type planExportCaptureRenderer struct {
	doc lists.Document
}

func (c *planExportCaptureRenderer) Render(doc lists.Document, _ lists.Format, filenameBase string) (lists.File, error) {
	c.doc = doc
	return lists.File{Data: []byte("rendered"), ContentType: "application/pdf", Filename: filenameBase + ".pdf"}, nil
}

func planExportPlannedShift(id, staffID int64, date calendar.Date) workforce.PlannedShift {
	return workforce.PlannedShift{StaffShift: workforce.StaffShift{
		ID: id, StaffID: staffID, Date: date.String(), StartTime: planExportClockString(7, 30), EndTime: planExportClockString(14, 0),
	}}
}

func planExportScheduledBlock(id int64, date calendar.Date, title, status string) *timetable.ScheduledInstance {
	return &timetable.ScheduledInstance{
		ID: id, Date: date, Title: title, StartTime: planExportClock(12, 0), EndTime: planExportClock(13, 0), RoomID: 3, Status: status,
	}
}

func TestPlanExportOverviewMapsEveryPrintedField(t *testing.T) {
	t.Parallel()

	cancelled := planExportPlannedShift(1, 7, planExportMonday)
	cancelled.Cancelled = true
	cancelled.ChangeReason = planExportPtr("krank")
	cancelled.ShiftTypeID = planExportPtr[int64](4)
	cancelled.Notes = "Frühdienst"
	cover := planExportPlannedShift(2, 8, planExportMonday)
	cover.OriginShiftID = planExportPtr[int64](1)
	source := &planExportFakeOverview{overview: workforce.StaffScheduleOverview{
		Staff:  []workforce.OverviewStaff{{ID: 7, FirstName: "Franziska", LastName: "Kessener"}, {ID: 9}},
		Shifts: []workforce.PlannedShift{cancelled, cover},
		Assignments: []workforce.OverviewAssignment{{
			StaffID: 7, Date: planExportMonday.String(), StartTime: planExportClockString(12, 0), EndTime: planExportClockString(13, 0),
			ActivityTitle: "Mensa", ActivityGroupID: planExportPtr[int64](21), RoomName: "Speisesaal",
			IsSubstitute: true, IsAbsent: true,
			UncoveredIntervals: []workforce.CoverageInterval{{StartTime: planExportClockString(12, 30), EndTime: planExportClockString(13, 0)}},
		}},
	}}

	overview, err := (planExportOverview{source: source}).StaffScheduleOverview(context.Background(), planExportDay(planExportMonday), planExportDay(planExportMonday.AddDays(6)))
	require.NoError(t, err)
	assert.Equal(t, planExportMonday.String(), source.from)
	assert.Equal(t, planExportMonday.AddDays(6).String(), source.to)

	require.Len(t, overview.Staff, 2, "a staff row without a person record is kept")
	assert.Equal(t, &planexport.StaffMember{ID: 7, FirstName: "Franziska", LastName: "Kessener"}, overview.Staff[0])
	assert.Equal(t, &planexport.StaffMember{ID: 9}, overview.Staff[1])

	require.Len(t, overview.Shifts, 2)
	assert.Equal(t, &planexport.Shift{
		ID: 1, StaffID: 7, Date: "2026-07-27", StartTime: planExportWall(7, 30), EndTime: planExportWall(14, 0),
		ShiftTypeID: planExportPtr[int64](4), Cancelled: true, ChangeReason: planExportPtr("krank"), Notes: "Frühdienst",
	}, overview.Shifts[0])
	assert.Equal(t, planExportPtr[int64](1), overview.Shifts[1].OriginShiftID)

	require.Len(t, overview.Assignments, 1)
	assert.Equal(t, planexport.Assignment{
		StaffID: 7, Date: "2026-07-27", StartTime: planExportWall(12, 0), EndTime: planExportWall(13, 0),
		ActivityTitle: "Mensa", ActivityGroupID: planExportPtr[int64](21), RoomName: "Speisesaal",
		IsSubstitute: true, IsAbsent: true,
		UncoveredIntervals: []planexport.Interval{{StartTime: planExportWall(12, 30), EndTime: planExportWall(13, 0)}},
	}, overview.Assignments[0])
}

func TestPlanExportOverviewToleratesAnEmptyProjection(t *testing.T) {
	t.Parallel()

	overview, err := (planExportOverview{source: &planExportFakeOverview{}}).StaffScheduleOverview(context.Background(), planExportDay(planExportMonday), planExportDay(planExportMonday))
	require.NoError(t, err)
	assert.Equal(t, &planexport.StaffScheduleOverview{}, overview)
}

func TestPlanExportInstancesMapCancellationFromStatus(t *testing.T) {
	t.Parallel()

	cancelled := planExportScheduledBlock(11, planExportMonday, "Mensa", timetable.InstanceStatusCancelled)
	cancelled.CancelReason = planExportPtr("Personalmangel")
	cancelled.Notes = planExportPtr("Ersatz")
	cancelled.UnderstaffedNote = planExportPtr("eine Kraft fehlt")
	cancelled.ActivityGroupID = planExportPtr[int64](41)
	// A block whose session ran keeps its session state in the status; it
	// still prints as an ordinary block.
	completed := planExportScheduledBlock(12, planExportMonday.AddDays(1), "Lernzeit", "completed")
	completed.IsDuty = true
	source := &planExportFakeInstances{instances: []*timetable.ScheduledInstance{cancelled, nil, completed}}

	instances, err := (planExportInstances{source: source}).InstancesInRange(context.Background(), planExportDay(planExportMonday), planExportDay(planExportMonday.AddDays(6)))
	require.NoError(t, err)
	assert.Equal(t, planExportMonday, source.from)
	assert.Equal(t, planExportMonday.AddDays(6), source.to)
	require.Len(t, instances, 2)
	assert.Equal(t, &planexport.Instance{
		ID: 11, Date: "2026-07-27", StartTime: planExportClock(12, 0), EndTime: planExportClock(13, 0), Title: "Mensa",
		ActivityGroupID: planExportPtr[int64](41), RoomID: 3, Cancelled: true,
		CancelReason: planExportPtr("Personalmangel"), Notes: planExportPtr("Ersatz"), UnderstaffedNote: planExportPtr("eine Kraft fehlt"),
	}, instances[0])
	assert.False(t, instances[1].Cancelled)
	assert.True(t, instances[1].IsDuty, "a duty prints without a head count")
	assert.False(t, instances[0].IsDuty)
	assert.Equal(t, planexport.Date("2026-07-28"), instances[1].Date)
}

func TestPlanExportRowBindingsMapPlainRecordsAndSkipNilRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	staffRows, err := (planExportInstanceStaff{source: planExportFakeInstanceStaff{rows: []*timetable.InstanceStaff{
		{InstanceID: 11, StaffID: 7, RoomID: planExportPtr[int64](4), IsSubstitute: true, IsAbsent: true}, nil,
	}}}).InstanceStaffByInstanceIDs(ctx, []int64{11})
	require.NoError(t, err)
	assert.Equal(t, []*planexport.InstanceStaff{{InstanceID: 11, StaffID: 7, RoomID: planExportPtr[int64](4), IsSubstitute: true, IsAbsent: true}}, staffRows)

	rooms, err := (planExportRooms{source: planExportFakeRooms{rooms: []facilities.Room{{ID: 3, Name: "Speisesaal"}}}}).RoomsByIDs(ctx, []int64{3})
	require.NoError(t, err)
	assert.Equal(t, []*planexport.Room{{ID: 3, Name: "Speisesaal"}}, rooms)

	types, err := (planExportShiftTypes{source: planExportFakeShiftTypes{types: []workforce.ShiftType{{ID: 4, Name: "Ganztag", Color: "#83CD2D"}}}}).ListShiftTypes(ctx)
	require.NoError(t, err)
	assert.Equal(t, []*planexport.ShiftType{{ID: 4, Name: "Ganztag", Color: "#83CD2D"}}, types)

	groups, err := (planExportActivityGroups{source: planExportFakeGroups{groups: []*timetable.Group{{ID: 5, PlanningTrackID: planExportPtr[int64](9)}, nil}}}).ActivityGroupsByIDs(ctx, []int64{5})
	require.NoError(t, err)
	assert.Equal(t, []*planexport.ActivityGroup{{ID: 5, PlanningTrackID: planExportPtr[int64](9)}}, groups)

	tracks := &planExportFakeTracks{tracks: []timetable.PlanningTrack{{ID: 9, Color: "#5080D8"}}}
	listed, err := (planExportPlanningTracks{source: tracks}).ListPlanningTracks(ctx)
	require.NoError(t, err)
	assert.Equal(t, []*planexport.PlanningTrack{{ID: 9, Color: "#5080D8"}}, listed)
	assert.Equal(t, timetable.PlanningTrackFilter{Ordered: true}, tracks.filter, "every Planungsspur, archived ones included, in the planner's order")
}

// Source failures surface unchanged, so the capability keeps deciding which
// of them fail the export and which only cost a detail on the sheet.
func TestPlanExportBindingsSurfaceSourceErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	_, err := (planExportOverview{source: &planExportFakeOverview{err: errPlanExportBoom}}).StaffScheduleOverview(ctx, planExportDay(planExportMonday), planExportDay(planExportMonday))
	require.ErrorIs(t, err, errPlanExportBoom)
	_, err = (planExportInstances{source: &planExportFakeInstances{err: errPlanExportBoom}}).InstancesInRange(ctx, planExportDay(planExportMonday), planExportDay(planExportMonday))
	require.ErrorIs(t, err, errPlanExportBoom)
	_, err = (planExportInstanceStaff{source: planExportFakeInstanceStaff{err: errPlanExportBoom}}).InstanceStaffByInstanceIDs(ctx, nil)
	require.ErrorIs(t, err, errPlanExportBoom)
	_, err = (planExportRooms{source: planExportFakeRooms{err: errPlanExportBoom}}).RoomsByIDs(ctx, nil)
	require.ErrorIs(t, err, errPlanExportBoom)
	_, err = (planExportShiftTypes{source: planExportFakeShiftTypes{err: errPlanExportBoom}}).ListShiftTypes(ctx)
	require.ErrorIs(t, err, errPlanExportBoom)
	_, err = (planExportActivityGroups{source: planExportFakeGroups{err: errPlanExportBoom}}).ActivityGroupsByIDs(ctx, nil)
	require.ErrorIs(t, err, errPlanExportBoom)
	_, err = (planExportPlanningTracks{source: &planExportFakeTracks{err: errPlanExportBoom}}).ListPlanningTracks(ctx)
	require.ErrorIs(t, err, errPlanExportBoom)
}

// The capability validates its own request days, so a malformed day reaching
// a binding is a programming error and is reported instead of being widened
// to the zero date's week.
func TestPlanExportBindingsRefuseMalformedDays(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	_, err := (planExportOverview{source: &planExportFakeOverview{}}).StaffScheduleOverview(ctx, "27.07.2026", planExportDay(planExportMonday))
	require.Error(t, err)
	_, err = (planExportInstances{source: &planExportFakeInstances{}}).InstancesInRange(ctx, planExportDay(planExportMonday), "")
	require.Error(t, err)
}

// The full binding renders the owners' records through the real layout: the
// same block, room, staff and colour end up on the sheet.
func TestNewRendersTheBetreuungsplanOverOwnerRecords(t *testing.T) {
	t.Parallel()

	block := planExportScheduledBlock(11, planExportMonday, "Mensa", timetable.InstanceStatusPlanned)
	block.ActivityGroupID = planExportPtr[int64](5)
	renderer := &planExportCaptureRenderer{}
	service := newPlanExport(planExportSources{
		Overview:       &planExportFakeOverview{},
		ShiftTypes:     planExportFakeShiftTypes{},
		Instances:      &planExportFakeInstances{instances: []*timetable.ScheduledInstance{block}},
		InstanceStaff:  planExportFakeInstanceStaff{rows: []*timetable.InstanceStaff{{InstanceID: 11, StaffID: 7}}},
		Students:       planExportFakeStudentCounts{counts: map[int64]int{11: 24}},
		Rooms:          planExportFakeRooms{rooms: []facilities.Room{{ID: 3, Name: "Speisesaal"}}},
		Staff:          planExportFakeStaff{members: map[int64]*planexport.StaffMember{7: {ID: 7, FirstName: "Franziska", LastName: "Kessener"}}},
		ActivityGroups: planExportFakeGroups{groups: []*timetable.Group{{ID: 5, PlanningTrackID: planExportPtr[int64](9)}}},
		PlanningTracks: &planExportFakeTracks{tracks: []timetable.PlanningTrack{{ID: 9, Color: "#5080D8"}}},
		ClosingDays: planExportFakeClosingDays{days: []*planexport.ClosingPeriod{{
			StartDate: planExportDay(planExportMonday.AddDays(1)), EndDate: planExportDay(planExportMonday.AddDays(1)), Reason: "Betriebsferien",
		}}},
		Holidays: planExportFakeHolidays{days: []planexport.Holiday{{Date: planExportDay(planExportMonday.AddDays(2)), Name: "Fronleichnam"}}},
		Renderer: renderer,
	})
	params, err := planexport.ParseParams(planExportMonday.String(), planExportMonday.AddDays(4).String(), string(planexport.TemplateByOffering), "", "")
	require.NoError(t, err)
	_, err = service.ExportBetreuungsplan(context.Background(), params)
	require.NoError(t, err)

	require.Len(t, renderer.doc.Rows, 1)
	row := renderer.doc.Rows[0]
	assertPlanCell(t, row.Values[lists.ColumnPlanRowLabel], "Mensa")
	assertPlanCell(t, row.Values[lists.ColumnPlanMonday], "12:00–13:00 · Speisesaal", "Kessener, F.", "24 Kinder")
	anchor := strings.SplitN(row.Values[lists.ColumnPlanMonday], "\n", 2)[0]
	assert.Contains(t, anchor, "#5080D8", "the Planungsspur colour reaches the sheet through the two Timetable reads")
	assertPlanCell(t, row.Values[lists.ColumnPlanTuesday], "Schließtag: Betriebsferien")
	assertPlanCell(t, row.Values[lists.ColumnPlanWednesday], "Feiertag: Fronleichnam")
}

// assertPlanCell asserts the printed lines of one sheet cell. A line's style
// and accent are prefixes Document Rendering encodes and owns; the text each
// line ends with is what the binding delivered.
func assertPlanCell(t *testing.T, cell string, want ...string) {
	t.Helper()
	lines := strings.Split(cell, "\n")
	require.Len(t, lines, len(want), "cell %q", cell)
	for i, text := range want {
		assert.True(t, strings.HasSuffix(lines[i], text), "line %d of %q should print %q", i, cell, text)
	}
}
