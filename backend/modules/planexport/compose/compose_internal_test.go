package compose

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/services/listexport"
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
	monday  = calendar.NewDate(2026, time.July, 27)
	errBoom = errors.New("boom")
)

func clock(hour, minute int) time.Time {
	return time.Date(2000, time.January, 1, hour, minute, 0, 0, time.UTC)
}

// clockString is the public ClockLayout form of a fixture clock;
// wall restores the normalized wall clock the binding hands the
// renderer.
func clockString(hour, minute int) string {
	return clock(hour, minute).Format(workforce.ClockLayout)
}

func wall(hour, minute int) time.Time {
	return calendar.NormalizeWallClock(clock(hour, minute))
}

func day(date calendar.Date) planexport.Date { return planexport.Date(date.String()) }

func ptr[T any](v T) *T { return &v }

type fakeOverview struct {
	overview workforce.StaffScheduleOverview
	err      error
	from, to string
}

func (f *fakeOverview) Overview(_ context.Context, from, to string) (workforce.StaffScheduleOverview, error) {
	f.from, f.to = from, to
	return f.overview, f.err
}

type fakeShiftTypes struct {
	types []workforce.ShiftType
	err   error
}

func (f fakeShiftTypes) ListShiftTypes(context.Context) ([]workforce.ShiftType, error) {
	return f.types, f.err
}

type fakeInstances struct {
	instances []*timetable.ScheduledInstance
	err       error
	from, to  calendar.Date
}

func (f *fakeInstances) FindByTenantAndDateRange(_ context.Context, from, to calendar.Date) ([]*timetable.ScheduledInstance, error) {
	f.from, f.to = from, to
	return f.instances, f.err
}

type fakeInstanceStaff struct {
	rows []*timetable.InstanceStaff
	err  error
}

func (f fakeInstanceStaff) FindByInstanceIDs(context.Context, []int64) ([]*timetable.InstanceStaff, error) {
	return f.rows, f.err
}

type fakeRooms struct {
	rooms []facilities.Room
	err   error
}

func (f fakeRooms) ListRoomsByID(context.Context, []int64) ([]facilities.Room, error) {
	return f.rooms, f.err
}

type fakeGroups struct {
	groups []*timetable.Group
	err    error
}

func (f fakeGroups) FindByIDs(context.Context, []int64) ([]*timetable.Group, error) {
	return f.groups, f.err
}

type fakeTracks struct {
	tracks []timetable.PlanningTrack
	err    error
	filter timetable.PlanningTrackFilter
}

func (f *fakeTracks) ListPlanningTracks(_ context.Context, filter timetable.PlanningTrackFilter) ([]timetable.PlanningTrack, error) {
	f.filter = filter
	return f.tracks, f.err
}

type fakeStaff struct {
	members map[int64]*planexport.StaffMember
}

func (f fakeStaff) StaffByIDs(context.Context, []int64) (map[int64]*planexport.StaffMember, error) {
	return f.members, nil
}

type fakeStudentCounts struct {
	counts map[int64]int
}

func (f fakeStudentCounts) CountNonAbsentByInstanceIDs(context.Context, []int64) (map[int64]int, error) {
	return f.counts, nil
}

type fakeClosingDays struct {
	days []*planexport.ClosingPeriod
}

func (f fakeClosingDays) ClosingDaysInRange(context.Context, planexport.Date, planexport.Date) ([]*planexport.ClosingPeriod, error) {
	return f.days, nil
}

type fakeHolidays struct {
	days []planexport.Holiday
}

func (f fakeHolidays) HolidaysInRange(context.Context, planexport.Date, planexport.Date) ([]planexport.Holiday, error) {
	return f.days, nil
}

type captureRenderer struct {
	doc listexport.Document
}

func (c *captureRenderer) Render(doc listexport.Document, _ listexport.Format, filenameBase string) (listexport.File, error) {
	c.doc = doc
	return listexport.File{Data: []byte("rendered"), ContentType: "application/pdf", Filename: filenameBase + ".pdf"}, nil
}

func plannedShift(id, staffID int64, date calendar.Date) workforce.PlannedShift {
	return workforce.PlannedShift{StaffShift: workforce.StaffShift{
		ID: id, StaffID: staffID, Date: date.String(), StartTime: clockString(7, 30), EndTime: clockString(14, 0),
	}}
}

func scheduledBlock(id int64, date calendar.Date, title, status string) *timetable.ScheduledInstance {
	return &timetable.ScheduledInstance{
		ID: id, Date: date, Title: title, StartTime: clock(12, 0), EndTime: clock(13, 0), RoomID: 3, Status: status,
	}
}

func TestPlanExportOverviewMapsEveryPrintedField(t *testing.T) {
	t.Parallel()

	cancelled := plannedShift(1, 7, monday)
	cancelled.Cancelled = true
	cancelled.ChangeReason = ptr("krank")
	cancelled.ShiftTypeID = ptr[int64](4)
	cancelled.Notes = "Frühdienst"
	cover := plannedShift(2, 8, monday)
	cover.OriginShiftID = ptr[int64](1)
	source := &fakeOverview{overview: workforce.StaffScheduleOverview{
		Staff:  []workforce.OverviewStaff{{ID: 7, FirstName: "Franziska", LastName: "Kessener"}, {ID: 9}},
		Shifts: []workforce.PlannedShift{cancelled, cover},
		Assignments: []workforce.OverviewAssignment{{
			StaffID: 7, Date: monday.String(), StartTime: clockString(12, 0), EndTime: clockString(13, 0),
			ActivityTitle: "Mensa", ActivityGroupID: ptr[int64](21), RoomName: "Speisesaal",
			IsSubstitute: true, IsAbsent: true,
			UncoveredIntervals: []workforce.CoverageInterval{{StartTime: clockString(12, 30), EndTime: clockString(13, 0)}},
		}},
	}}

	overview, err := (overviewBinding{source: source}).StaffScheduleOverview(context.Background(), day(monday), day(monday.AddDays(6)))
	require.NoError(t, err)
	assert.Equal(t, monday.String(), source.from)
	assert.Equal(t, monday.AddDays(6).String(), source.to)

	require.Len(t, overview.Staff, 2, "a staff row without a person record is kept")
	assert.Equal(t, &planexport.StaffMember{ID: 7, FirstName: "Franziska", LastName: "Kessener"}, overview.Staff[0])
	assert.Equal(t, &planexport.StaffMember{ID: 9}, overview.Staff[1])

	require.Len(t, overview.Shifts, 2)
	assert.Equal(t, &planexport.Shift{
		ID: 1, StaffID: 7, Date: "2026-07-27", StartTime: wall(7, 30), EndTime: wall(14, 0),
		ShiftTypeID: ptr[int64](4), Cancelled: true, ChangeReason: ptr("krank"), Notes: "Frühdienst",
	}, overview.Shifts[0])
	assert.Equal(t, ptr[int64](1), overview.Shifts[1].OriginShiftID)

	require.Len(t, overview.Assignments, 1)
	assert.Equal(t, planexport.Assignment{
		StaffID: 7, Date: "2026-07-27", StartTime: wall(12, 0), EndTime: wall(13, 0),
		ActivityTitle: "Mensa", ActivityGroupID: ptr[int64](21), RoomName: "Speisesaal",
		IsSubstitute: true, IsAbsent: true,
		UncoveredIntervals: []planexport.Interval{{StartTime: wall(12, 30), EndTime: wall(13, 0)}},
	}, overview.Assignments[0])
}

func TestPlanExportOverviewToleratesAnEmptyProjection(t *testing.T) {
	t.Parallel()

	overview, err := (overviewBinding{source: &fakeOverview{}}).StaffScheduleOverview(context.Background(), day(monday), day(monday))
	require.NoError(t, err)
	assert.Equal(t, &planexport.StaffScheduleOverview{}, overview)
}

func TestPlanExportInstancesMapCancellationFromStatus(t *testing.T) {
	t.Parallel()

	cancelled := scheduledBlock(11, monday, "Mensa", timetable.InstanceStatusCancelled)
	cancelled.CancelReason = ptr("Personalmangel")
	cancelled.Notes = ptr("Ersatz")
	cancelled.UnderstaffedNote = ptr("eine Kraft fehlt")
	cancelled.ActivityGroupID = ptr[int64](41)
	// A block whose session ran keeps its session state in the status; it
	// still prints as an ordinary block.
	completed := scheduledBlock(12, monday.AddDays(1), "Lernzeit", "completed")
	source := &fakeInstances{instances: []*timetable.ScheduledInstance{cancelled, nil, completed}}

	instances, err := (instanceBinding{source: source}).InstancesInRange(context.Background(), day(monday), day(monday.AddDays(6)))
	require.NoError(t, err)
	assert.Equal(t, monday, source.from)
	assert.Equal(t, monday.AddDays(6), source.to)
	require.Len(t, instances, 2)
	assert.Equal(t, &planexport.Instance{
		ID: 11, Date: "2026-07-27", StartTime: clock(12, 0), EndTime: clock(13, 0), Title: "Mensa",
		ActivityGroupID: ptr[int64](41), RoomID: 3, Cancelled: true,
		CancelReason: ptr("Personalmangel"), Notes: ptr("Ersatz"), UnderstaffedNote: ptr("eine Kraft fehlt"),
	}, instances[0])
	assert.False(t, instances[1].Cancelled)
	assert.Equal(t, planexport.Date("2026-07-28"), instances[1].Date)
}

func TestPlanExportRowBindingsMapPlainRecordsAndSkipNilRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	staffRows, err := (instanceStaffBinding{source: fakeInstanceStaff{rows: []*timetable.InstanceStaff{
		{InstanceID: 11, StaffID: 7, RoomID: ptr[int64](4), IsSubstitute: true, IsAbsent: true}, nil,
	}}}).InstanceStaffByInstanceIDs(ctx, []int64{11})
	require.NoError(t, err)
	assert.Equal(t, []*planexport.InstanceStaff{{InstanceID: 11, StaffID: 7, RoomID: ptr[int64](4), IsSubstitute: true, IsAbsent: true}}, staffRows)

	rooms, err := (roomBinding{source: fakeRooms{rooms: []facilities.Room{{ID: 3, Name: "Speisesaal"}}}}).RoomsByIDs(ctx, []int64{3})
	require.NoError(t, err)
	assert.Equal(t, []*planexport.Room{{ID: 3, Name: "Speisesaal"}}, rooms)

	types, err := (shiftTypeBinding{source: fakeShiftTypes{types: []workforce.ShiftType{{ID: 4, Name: "Ganztag", Color: "#83CD2D"}}}}).ListShiftTypes(ctx)
	require.NoError(t, err)
	assert.Equal(t, []*planexport.ShiftType{{ID: 4, Name: "Ganztag", Color: "#83CD2D"}}, types)

	groups, err := (activityGroupBinding{source: fakeGroups{groups: []*timetable.Group{{ID: 5, PlanningTrackID: ptr[int64](9)}, nil}}}).ActivityGroupsByIDs(ctx, []int64{5})
	require.NoError(t, err)
	assert.Equal(t, []*planexport.ActivityGroup{{ID: 5, PlanningTrackID: ptr[int64](9)}}, groups)

	tracks := &fakeTracks{tracks: []timetable.PlanningTrack{{ID: 9, Color: "#5080D8"}}}
	listed, err := (planningTrackBinding{source: tracks}).ListPlanningTracks(ctx)
	require.NoError(t, err)
	assert.Equal(t, []*planexport.PlanningTrack{{ID: 9, Color: "#5080D8"}}, listed)
	assert.Equal(t, timetable.PlanningTrackFilter{Ordered: true}, tracks.filter, "every Planungsspur, archived ones included, in the planner's order")
}

// Source failures surface unchanged, so the capability keeps deciding which
// of them fail the export and which only cost a detail on the sheet.
func TestPlanExportBindingsSurfaceSourceErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	_, err := (overviewBinding{source: &fakeOverview{err: errBoom}}).StaffScheduleOverview(ctx, day(monday), day(monday))
	require.ErrorIs(t, err, errBoom)
	_, err = (instanceBinding{source: &fakeInstances{err: errBoom}}).InstancesInRange(ctx, day(monday), day(monday))
	require.ErrorIs(t, err, errBoom)
	_, err = (instanceStaffBinding{source: fakeInstanceStaff{err: errBoom}}).InstanceStaffByInstanceIDs(ctx, nil)
	require.ErrorIs(t, err, errBoom)
	_, err = (roomBinding{source: fakeRooms{err: errBoom}}).RoomsByIDs(ctx, nil)
	require.ErrorIs(t, err, errBoom)
	_, err = (shiftTypeBinding{source: fakeShiftTypes{err: errBoom}}).ListShiftTypes(ctx)
	require.ErrorIs(t, err, errBoom)
	_, err = (activityGroupBinding{source: fakeGroups{err: errBoom}}).ActivityGroupsByIDs(ctx, nil)
	require.ErrorIs(t, err, errBoom)
	_, err = (planningTrackBinding{source: &fakeTracks{err: errBoom}}).ListPlanningTracks(ctx)
	require.ErrorIs(t, err, errBoom)
}

// The capability validates its own request days, so a malformed day reaching
// a binding is a programming error and is reported instead of being widened
// to the zero date's week.
func TestPlanExportBindingsRefuseMalformedDays(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	_, err := (overviewBinding{source: &fakeOverview{}}).StaffScheduleOverview(ctx, "27.07.2026", day(monday))
	require.Error(t, err)
	_, err = (instanceBinding{source: &fakeInstances{}}).InstancesInRange(ctx, day(monday), "")
	require.Error(t, err)
}

// The full binding renders the owners' records through the real layout: the
// same block, room, staff and colour end up on the sheet.
func TestNewRendersTheBetreuungsplanOverOwnerRecords(t *testing.T) {
	t.Parallel()

	block := scheduledBlock(11, monday, "Mensa", timetable.InstanceStatusPlanned)
	block.ActivityGroupID = ptr[int64](5)
	renderer := &captureRenderer{}
	service := New(Sources{
		Overview:       &fakeOverview{},
		ShiftTypes:     fakeShiftTypes{},
		Instances:      &fakeInstances{instances: []*timetable.ScheduledInstance{block}},
		InstanceStaff:  fakeInstanceStaff{rows: []*timetable.InstanceStaff{{InstanceID: 11, StaffID: 7}}},
		Students:       fakeStudentCounts{counts: map[int64]int{11: 24}},
		Rooms:          fakeRooms{rooms: []facilities.Room{{ID: 3, Name: "Speisesaal"}}},
		Staff:          fakeStaff{members: map[int64]*planexport.StaffMember{7: {ID: 7, FirstName: "Franziska", LastName: "Kessener"}}},
		ActivityGroups: fakeGroups{groups: []*timetable.Group{{ID: 5, PlanningTrackID: ptr[int64](9)}}},
		PlanningTracks: &fakeTracks{tracks: []timetable.PlanningTrack{{ID: 9, Color: "#5080D8"}}},
		ClosingDays: fakeClosingDays{days: []*planexport.ClosingPeriod{{
			StartDate: day(monday.AddDays(1)), EndDate: day(monday.AddDays(1)), Reason: "Betriebsferien",
		}}},
		Holidays: fakeHolidays{days: []planexport.Holiday{{Date: day(monday.AddDays(2)), Name: "Fronleichnam"}}},
		Renderer: renderer,
	})
	params, err := planexport.ParseParams(monday.String(), monday.AddDays(4).String(), string(planexport.TemplateByOffering), "", "")
	require.NoError(t, err)
	_, err = service.ExportBetreuungsplan(context.Background(), params)
	require.NoError(t, err)

	require.Len(t, renderer.doc.Rows, 1)
	row := renderer.doc.Rows[0]
	assert.Equal(t, "Mensa", listexport.StripStyleMarkers(row.Values[listexport.ColumnPlanRowLabel]))
	assert.Equal(t, "12:00–13:00 · Speisesaal\nKessener, F.\n24 Kinder", listexport.StripStyleMarkers(row.Values[listexport.ColumnPlanMonday]))
	anchor, _ := listexport.DecodeLine(strings.SplitN(row.Values[listexport.ColumnPlanMonday], "\n", 2)[0])
	assert.Equal(t, "#5080D8", anchor.Accent, "the Planungsspur colour reaches the sheet through the two Timetable reads")
	assert.Equal(t, "Schließtag: Betriebsferien", listexport.StripStyleMarkers(row.Values[listexport.ColumnPlanTuesday]))
	assert.Equal(t, "Feiertag: Fronleichnam", listexport.StripStyleMarkers(row.Values[listexport.ColumnPlanWednesday]))
}
