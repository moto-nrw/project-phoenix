package legacy

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The adapters translate retained rows into the capability's plain records
// and decide nothing themselves. These tests pin that translation field by
// field: a date that arrives one day off, a cancellation read from the wrong
// column or a dropped nil row would print a wrong plan without any renderer
// test noticing, because the renderer only ever sees the records.

var (
	monday  = timezone.NewDate(2026, time.July, 27)
	errBoom = errors.New("boom")
)

func clock(hour, minute int) time.Time {
	return time.Date(2000, time.January, 1, hour, minute, 0, 0, time.UTC)
}

func ptr[T any](v T) *T { return &v }

type fakeOverview struct {
	overview *scheduleSvc.StaffScheduleOverview
	err      error
	from, to timezone.Date
}

func (f *fakeOverview) GetOverview(_ context.Context, from, to timezone.Date) (*scheduleSvc.StaffScheduleOverview, error) {
	f.from, f.to = from, to
	return f.overview, f.err
}

type fakeShiftTypes struct {
	types []*scheduleModel.ShiftType
	err   error
}

func (f fakeShiftTypes) ListAll(context.Context) ([]*scheduleModel.ShiftType, error) {
	return f.types, f.err
}

type fakeInstances struct {
	instances []*scheduleModel.ActivityInstance
	err       error
	from, to  scheduleModel.Date
}

func (f *fakeInstances) FindByTenantAndDateRange(_ context.Context, from, to scheduleModel.Date) ([]*scheduleModel.ActivityInstance, error) {
	f.from, f.to = from, to
	return f.instances, f.err
}

type fakeInstanceStaff struct {
	rows []*scheduleModel.InstanceStaff
	err  error
}

func (f fakeInstanceStaff) FindByInstanceIDs(context.Context, []int64) ([]*scheduleModel.InstanceStaff, error) {
	return f.rows, f.err
}

type fakeRooms struct {
	rooms []*facilitiesModel.Room
	err   error
}

func (f fakeRooms) FindByIDs(context.Context, []int64) ([]*facilitiesModel.Room, error) {
	return f.rooms, f.err
}

type fakeStaff struct {
	members map[int64]*usersModel.Staff
	err     error
}

func (f fakeStaff) ListAllWithPerson(context.Context) ([]*usersModel.Staff, error) {
	return nil, errors.New("not used by the plan export")
}

func (f fakeStaff) FindWithPersonByIDs(context.Context, []int64) (map[int64]*usersModel.Staff, error) {
	return f.members, f.err
}

type fakeActivityGroups struct {
	groups []*activitiesModel.Group
	err    error
}

func (f fakeActivityGroups) FindByIDs(context.Context, []int64) ([]*activitiesModel.Group, error) {
	return f.groups, f.err
}

type fakePlanningTracks struct {
	tracks []*scheduleModel.PlanningTrack
	err    error
}

func (f fakePlanningTracks) ListAll(context.Context) ([]*scheduleModel.PlanningTrack, error) {
	return f.tracks, f.err
}

type fakeClosingDays struct {
	scheduleSvc.ClosingDayService
	days []*scheduleModel.ClosingDay
	err  error
}

func (f fakeClosingDays) ClosingDaysInRange(context.Context, timezone.Date, timezone.Date) ([]*scheduleModel.ClosingDay, error) {
	return f.days, f.err
}

type fakeHolidays struct {
	scheduleSvc.HolidayService
	days []scheduleSvc.Holiday
	err  error
}

func (f fakeHolidays) HolidaysInRange(context.Context, timezone.Date, timezone.Date) ([]scheduleSvc.Holiday, error) {
	return f.days, f.err
}

type captureRenderer struct {
	doc listexport.Document
}

func (c *captureRenderer) Render(doc listexport.Document, _ listexport.Format, filenameBase string) (listexport.File, error) {
	c.doc = doc
	return listexport.File{Data: []byte("rendered"), ContentType: "application/pdf", Filename: filenameBase + ".pdf"}, nil
}

func staffRow(id int64, first, last string) *usersModel.Staff {
	member := &usersModel.Staff{Person: &usersModel.Person{FirstName: first, LastName: last}}
	member.ID = id
	return member
}

func shiftRow(id, staffID int64, date timezone.Date) *scheduleModel.StaffShift {
	shift := &scheduleModel.StaffShift{StaffID: staffID, Date: scheduleModel.Date(date), StartTime: clock(7, 30), EndTime: clock(14, 0)}
	shift.ID = id
	return shift
}

func instanceRow(id int64, date timezone.Date, title string, status string) *scheduleModel.ActivityInstance {
	instance := &scheduleModel.ActivityInstance{Date: scheduleModel.Date(date), Title: title, StartTime: clock(12, 0), EndTime: clock(13, 0), RoomID: 3, Status: status}
	instance.ID = id
	return instance
}

func day(date timezone.Date) planexport.Date {
	return planexport.Date(date.String())
}

func TestOverviewAdapterMapsEveryPrintedField(t *testing.T) {
	t.Parallel()

	cancelled := shiftRow(1, 7, monday)
	cancelled.Cancelled = true
	cancelled.ChangeReason = ptr("krank")
	cancelled.ShiftTypeID = ptr[int64](4)
	cancelled.Notes = "Frühdienst"
	cover := shiftRow(2, 8, monday)
	cover.OriginShiftID = ptr[int64](1)
	headless := &usersModel.Staff{}
	headless.ID = 9
	source := &fakeOverview{overview: &scheduleSvc.StaffScheduleOverview{
		Staff:  []*usersModel.Staff{staffRow(7, "Franziska", "Kessener"), nil, headless},
		Shifts: []*scheduleModel.StaffShift{cancelled, nil, cover},
		Assignments: []scheduleSvc.StaffScheduleAssignment{{
			StaffID: 7, Date: monday, StartTime: clock(12, 0), EndTime: clock(13, 0),
			ActivityTitle: "Mensa", ActivityGroupID: ptr[int64](21), RoomName: "Speisesaal",
			IsSubstitute: true, IsAbsent: true,
			UncoveredIntervals: []scheduleSvc.ShiftCoverageInterval{{StartTime: clock(12, 30), EndTime: clock(13, 0)}},
		}},
	}}

	overview, err := (overviewAdapter{source: source}).StaffScheduleOverview(context.Background(), day(monday), day(monday.AddDays(6)))
	require.NoError(t, err)
	assert.Equal(t, monday, source.from)
	assert.Equal(t, monday.AddDays(6), source.to)

	require.Len(t, overview.Staff, 2, "nil rows are dropped, a staff row without a person record is kept")
	assert.Equal(t, &planexport.StaffMember{ID: 7, FirstName: "Franziska", LastName: "Kessener"}, overview.Staff[0])
	assert.Equal(t, &planexport.StaffMember{ID: 9}, overview.Staff[1])

	require.Len(t, overview.Shifts, 2)
	assert.Equal(t, &planexport.Shift{
		ID: 1, StaffID: 7, Date: "2026-07-27", StartTime: clock(7, 30), EndTime: clock(14, 0),
		ShiftTypeID: ptr[int64](4), Cancelled: true, ChangeReason: ptr("krank"), Notes: "Frühdienst",
	}, overview.Shifts[0])
	assert.Equal(t, ptr[int64](1), overview.Shifts[1].OriginShiftID)

	require.Len(t, overview.Assignments, 1)
	assert.Equal(t, planexport.Assignment{
		StaffID: 7, Date: "2026-07-27", StartTime: clock(12, 0), EndTime: clock(13, 0),
		ActivityTitle: "Mensa", ActivityGroupID: ptr[int64](21), RoomName: "Speisesaal",
		IsSubstitute: true, IsAbsent: true,
		UncoveredIntervals: []planexport.Interval{{StartTime: clock(12, 30), EndTime: clock(13, 0)}},
	}, overview.Assignments[0])
}

func TestOverviewAdapterToleratesAnEmptyProjection(t *testing.T) {
	t.Parallel()

	overview, err := (overviewAdapter{source: &fakeOverview{}}).StaffScheduleOverview(context.Background(), day(monday), day(monday))
	require.NoError(t, err)
	assert.Equal(t, &planexport.StaffScheduleOverview{}, overview)
}

func TestInstanceAdapterMapsCancellationFromStatus(t *testing.T) {
	t.Parallel()

	cancelled := instanceRow(11, monday, "Mensa", scheduleModel.InstanceStatusCancelled)
	cancelled.CancelReason = ptr("Personalmangel")
	cancelled.Notes = ptr("Ersatz")
	cancelled.UnderstaffedNote = ptr("eine Kraft fehlt")
	cancelled.ActivityGroupID = ptr[int64](41)
	planned := instanceRow(12, monday.AddDays(1), "Lernzeit", scheduleModel.InstanceStatusPlanned)
	source := &fakeInstances{instances: []*scheduleModel.ActivityInstance{cancelled, nil, planned}}

	instances, err := (instanceAdapter{source: source}).InstancesInRange(context.Background(), day(monday), day(monday.AddDays(6)))
	require.NoError(t, err)
	assert.Equal(t, scheduleModel.Date(monday), source.from)
	assert.Equal(t, scheduleModel.Date(monday.AddDays(6)), source.to)
	require.Len(t, instances, 2)
	assert.Equal(t, &planexport.Instance{
		ID: 11, Date: "2026-07-27", StartTime: clock(12, 0), EndTime: clock(13, 0), Title: "Mensa",
		ActivityGroupID: ptr[int64](41), RoomID: 3, Cancelled: true,
		CancelReason: ptr("Personalmangel"), Notes: ptr("Ersatz"), UnderstaffedNote: ptr("eine Kraft fehlt"),
	}, instances[0])
	assert.False(t, instances[1].Cancelled)
	assert.Equal(t, planexport.Date("2026-07-28"), instances[1].Date)
}

func TestRowAdaptersMapPlainRecordsAndSkipNilRows(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	staffRows, err := (instanceStaffAdapter{source: fakeInstanceStaff{rows: []*scheduleModel.InstanceStaff{
		{InstanceID: 11, StaffID: 7, RoomID: ptr[int64](4), IsSubstitute: true, IsAbsent: true}, nil,
	}}}).InstanceStaffByInstanceIDs(ctx, []int64{11})
	require.NoError(t, err)
	assert.Equal(t, []*planexport.InstanceStaff{{InstanceID: 11, StaffID: 7, RoomID: ptr[int64](4), IsSubstitute: true, IsAbsent: true}}, staffRows)

	room := &facilitiesModel.Room{Name: "Speisesaal"}
	room.ID = 3
	rooms, err := (roomAdapter{source: fakeRooms{rooms: []*facilitiesModel.Room{room, nil}}}).RoomsByIDs(ctx, []int64{3})
	require.NoError(t, err)
	assert.Equal(t, []*planexport.Room{{ID: 3, Name: "Speisesaal"}}, rooms)

	shiftType := &scheduleModel.ShiftType{Name: "Ganztag", Color: "#83CD2D"}
	shiftType.ID = 4
	types, err := (shiftTypeAdapter{source: fakeShiftTypes{types: []*scheduleModel.ShiftType{shiftType, nil}}}).ListShiftTypes(ctx)
	require.NoError(t, err)
	assert.Equal(t, []*planexport.ShiftType{{ID: 4, Name: "Ganztag", Color: "#83CD2D"}}, types)

	group := &activitiesModel.Group{PlanningTrackID: ptr[int64](9)}
	group.ID = 5
	groups, err := (activityGroupAdapter{source: fakeActivityGroups{groups: []*activitiesModel.Group{group, nil}}}).ActivityGroupsByIDs(ctx, []int64{5})
	require.NoError(t, err)
	assert.Equal(t, []*planexport.ActivityGroup{{ID: 5, PlanningTrackID: ptr[int64](9)}}, groups)

	track := &scheduleModel.PlanningTrack{Color: "#5080D8"}
	track.ID = 9
	tracks, err := (planningTrackAdapter{source: fakePlanningTracks{tracks: []*scheduleModel.PlanningTrack{track, nil}}}).ListPlanningTracks(ctx)
	require.NoError(t, err)
	assert.Equal(t, []*planexport.PlanningTrack{{ID: 9, Color: "#5080D8"}}, tracks)

	headless := &usersModel.Staff{}
	headless.ID = 9
	members, err := (staffAdapter{source: fakeStaff{members: map[int64]*usersModel.Staff{
		7: staffRow(7, "Franziska", "Kessener"), 8: nil, 9: headless,
	}}}).StaffByIDs(ctx, []int64{7, 8, 9})
	require.NoError(t, err)
	assert.Equal(t, map[int64]*planexport.StaffMember{
		7: {ID: 7, FirstName: "Franziska", LastName: "Kessener"},
		8: nil,
		9: {ID: 9},
	}, members, "a missing staff row keeps its slot so the sheet still prints Unbekannt for it")

	closing, err := (closingDayAdapter{source: fakeClosingDays{days: []*scheduleModel.ClosingDay{
		{StartDate: scheduleModel.Date(monday), EndDate: scheduleModel.Date(monday.AddDays(1)), Reason: "Betriebsferien"}, nil,
	}}}).ClosingDaysInRange(ctx, day(monday), day(monday.AddDays(6)))
	require.NoError(t, err)
	assert.Equal(t, []*planexport.ClosingPeriod{{StartDate: "2026-07-27", EndDate: "2026-07-28", Reason: "Betriebsferien"}}, closing)

	holidays, err := (holidayAdapter{source: fakeHolidays{days: []scheduleSvc.Holiday{{Date: monday.AddDays(2), Name: "Fronleichnam"}}}}).HolidaysInRange(ctx, day(monday), day(monday.AddDays(6)))
	require.NoError(t, err)
	assert.Equal(t, []planexport.Holiday{{Date: "2026-07-29", Name: "Fronleichnam"}}, holidays)
}

// Source failures surface unchanged, so the capability keeps deciding which
// of them fail the export and which only cost a detail on the sheet.
func TestAdaptersSurfaceSourceErrors(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	_, err := (overviewAdapter{source: &fakeOverview{err: errBoom}}).StaffScheduleOverview(ctx, day(monday), day(monday))
	require.ErrorIs(t, err, errBoom)
	_, err = (instanceAdapter{source: &fakeInstances{err: errBoom}}).InstancesInRange(ctx, day(monday), day(monday))
	require.ErrorIs(t, err, errBoom)
	_, err = (instanceStaffAdapter{source: fakeInstanceStaff{err: errBoom}}).InstanceStaffByInstanceIDs(ctx, nil)
	require.ErrorIs(t, err, errBoom)
	_, err = (roomAdapter{source: fakeRooms{err: errBoom}}).RoomsByIDs(ctx, nil)
	require.ErrorIs(t, err, errBoom)
	_, err = (staffAdapter{source: fakeStaff{err: errBoom}}).StaffByIDs(ctx, nil)
	require.ErrorIs(t, err, errBoom)
	_, err = (shiftTypeAdapter{source: fakeShiftTypes{err: errBoom}}).ListShiftTypes(ctx)
	require.ErrorIs(t, err, errBoom)
	_, err = (activityGroupAdapter{source: fakeActivityGroups{err: errBoom}}).ActivityGroupsByIDs(ctx, nil)
	require.ErrorIs(t, err, errBoom)
	_, err = (planningTrackAdapter{source: fakePlanningTracks{err: errBoom}}).ListPlanningTracks(ctx)
	require.ErrorIs(t, err, errBoom)
	_, err = (closingDayAdapter{source: fakeClosingDays{err: errBoom}}).ClosingDaysInRange(ctx, day(monday), day(monday))
	require.ErrorIs(t, err, errBoom)
	_, err = (holidayAdapter{source: fakeHolidays{err: errBoom}}).HolidaysInRange(ctx, day(monday), day(monday))
	require.ErrorIs(t, err, errBoom)
}

// The capability validates its own request days, so a malformed day reaching
// an adapter is a programming error and is reported instead of being widened
// to the zero date's week.
func TestAdaptersRefuseMalformedDays(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	_, err := (overviewAdapter{source: &fakeOverview{}}).StaffScheduleOverview(ctx, "27.07.2026", day(monday))
	require.Error(t, err)
	_, err = (instanceAdapter{source: &fakeInstances{}}).InstancesInRange(ctx, day(monday), "")
	require.Error(t, err)
	_, err = (closingDayAdapter{source: fakeClosingDays{}}).ClosingDaysInRange(ctx, "x", day(monday))
	require.Error(t, err)
	_, err = (holidayAdapter{source: fakeHolidays{}}).HolidaysInRange(ctx, day(monday), "x")
	require.Error(t, err)
}

// An unbound optional source leaves its port unbound, so the capability
// prints without that detail exactly as the retained service did; the two
// mandatory sources still fail the export when missing.
func TestNewLeavesUnboundSourcesOptional(t *testing.T) {
	t.Parallel()

	renderer := &captureRenderer{}
	service := New(Sources{
		Overview: &fakeOverview{overview: &scheduleSvc.StaffScheduleOverview{
			Staff:  []*usersModel.Staff{staffRow(7, "Franziska", "Kessener")},
			Shifts: []*scheduleModel.StaffShift{shiftRow(1, 7, monday)},
		}},
		Renderer: renderer,
	})
	params, err := planexport.ParseParams(monday.String(), monday.AddDays(4).String(), string(planexport.TemplateByPerson), "", "")
	require.NoError(t, err)
	_, err = service.ExportDienstplan(context.Background(), params)
	require.NoError(t, err)
	require.NotEmpty(t, renderer.doc.Rows)
	assert.Contains(t, renderer.doc.Rows[0].Values[listexport.ColumnPlanRowLabel], "Kessener, Franziska")

	params.Template = planexport.TemplateByOffering
	_, err = service.ExportBetreuungsplan(context.Background(), params)
	require.Error(t, err, "the care plan cannot print without its instance sources")
}

// The full binding renders the retained fixture shape through the real
// renderer: the same block, room, staff and colour end up on the sheet.
func TestNewRendersTheBetreuungsplanOverRetainedRows(t *testing.T) {
	t.Parallel()

	block := instanceRow(11, monday, "Mensa", scheduleModel.InstanceStatusPlanned)
	block.ActivityGroupID = ptr[int64](5)
	room := &facilitiesModel.Room{Name: "Speisesaal"}
	room.ID = 3
	group := &activitiesModel.Group{PlanningTrackID: ptr[int64](9)}
	group.ID = 5
	track := &scheduleModel.PlanningTrack{Color: "#5080D8"}
	track.ID = 9
	renderer := &captureRenderer{}
	service := New(Sources{
		Instances:      &fakeInstances{instances: []*scheduleModel.ActivityInstance{block}},
		InstanceStaff:  fakeInstanceStaff{rows: []*scheduleModel.InstanceStaff{{InstanceID: 11, StaffID: 7}}},
		Students:       fakeStudentCounts{counts: map[int64]int{11: 24}},
		Rooms:          fakeRooms{rooms: []*facilitiesModel.Room{room}},
		Staff:          fakeStaff{members: map[int64]*usersModel.Staff{7: staffRow(7, "Franziska", "Kessener")}},
		ActivityGroups: fakeActivityGroups{groups: []*activitiesModel.Group{group}},
		PlanningTracks: fakePlanningTracks{tracks: []*scheduleModel.PlanningTrack{track}},
		ClosingDays:    fakeClosingDays{days: []*scheduleModel.ClosingDay{{StartDate: scheduleModel.Date(monday.AddDays(1)), EndDate: scheduleModel.Date(monday.AddDays(1)), Reason: "Betriebsferien"}}},
		Holidays:       fakeHolidays{days: []scheduleSvc.Holiday{{Date: monday.AddDays(2), Name: "Fronleichnam"}}},
		Renderer:       renderer,
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
	assert.Equal(t, "#5080D8", anchor.Accent, "the Planungsspur colour reaches the sheet through the two retained readers")
	assert.Equal(t, "Schließtag: Betriebsferien", listexport.StripStyleMarkers(row.Values[listexport.ColumnPlanTuesday]))
	assert.Equal(t, "Feiertag: Fronleichnam", listexport.StripStyleMarkers(row.Values[listexport.ColumnPlanWednesday]))
}

type fakeStudentCounts struct {
	counts map[int64]int
}

func (f fakeStudentCounts) CountNonAbsentByInstanceIDs(context.Context, []int64) (map[int64]int, error) {
	return f.counts, nil
}
