package compose

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/timetabletest"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// betreuungsplanSources binds the Betreuungsplan's reads to the Timetable
// owner's presence-joined block reads and staff-assignment reads, the room
// names and roster counts of the tenant transaction under test and the given
// staff names. The
// Dienstplan-only and colour reads stay empty; Workforce's behaviour suite
// proves the Dienstplan side (#2706).
func betreuungsplanSources(t *testing.T, db *bun.DB, tx bun.IDB, staff planexport.StaffNameReader, renderer planexport.Renderer) Sources {
	t.Helper()
	return Sources{
		Overview:       &fakeOverview{},
		ShiftTypes:     fakeShiftTypes{},
		Instances:      newOwnerInstances(t, db),
		InstanceStaff:  newOwnerInstanceStaff(t, db),
		Students:       txRosterCounts{tx: tx},
		Rooms:          txRooms{tx: tx},
		Staff:          staff,
		ActivityGroups: fakeGroups{},
		PlanningTracks: &fakeTracks{},
		ClosingDays:    fakeClosingDays{},
		Holidays:       fakeHolidays{},
		Renderer:       renderer,
	}
}

// staffNames serves the staff-name port from fixture names, the way the root
// translates the retained staff rows.
func staffNames(members ...*planexport.StaffMember) fakeStaff {
	out := make(map[int64]*planexport.StaffMember, len(members))
	for _, member := range members {
		out[member.ID] = member
	}
	return fakeStaff{members: out}
}

// TestPlanExportInputsEnforceRLS proves the plan export capability (#2706)
// stays inside the tenant: two schools seed the same shape in every table the
// two renderers read, those rows are invisible across the tenant boundary
// under the least-privilege role, and each school's Betreuungsplan, rendered
// over the owners' reads, lists only its own block, room, staff member and
// child.
func TestPlanExportInputsEnforceRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	type fixture struct {
		ctx      context.Context
		tenantID int64
		staff    *planexport.StaffMember
		title    string
		room     string
		rows     map[string]int64
	}
	var fixtures []fixture
	for _, side := range []string{"own", "foreign"} {
		t.Run(side, func(t *testing.T) {
			testpkg.OwnTenant(t)
			ctx := testpkg.Ctx(t)
			staff := testpkg.CreateTestStaff(t, db, "PlanRLS", side)
			room := testpkg.CreateTestRoom(t, db, "PlanRLS-"+side)
			activity := testpkg.CreateTestActivityGroup(t, db, "PlanRLS-"+side)
			instance := testpkg.CreateTestActivityInstance(t, db, monday, room.ID, testpkg.ActivityInstanceOpts{
				Title: "Block " + side, ActivityGroupID: &activity.ID, StartHHMM: "12:00", EndHHMM: "13:00",
			})
			assignment := testpkg.CreateTestInstanceStaff(t, db, instance.ID, staff.ID, testpkg.InstanceStaffOpts{})
			student := testpkg.CreateTestStudent(t, db, "PlanRLS", side, "PR1")
			roster := testpkg.CreateTestInstanceStudent(t, db, instance.ID, student.ID, "")
			shift := testpkg.CreateTestStaffShift(t, db, staff.ID, monday, testpkg.StaffShiftOpts{})
			closing := testpkg.CreateTestClosingDay(t, db, monday.AddDays(1), monday.AddDays(1), "PlanRLS-"+side)
			var shiftTypeID, trackID int64
			require.NoError(t, db.NewRaw("INSERT INTO schedule.shift_types (tenant_id, name, color) VALUES (?, ?, ?) RETURNING id",
				testpkg.Tenant(t), "PlanRLS-"+side, "#83CD2D").Scan(ctx, &shiftTypeID))
			require.NoError(t, db.NewRaw("INSERT INTO schedule.planning_tracks (tenant_id, name, color, sort_order) VALUES (?, ?, ?, 0) RETURNING id",
				testpkg.Tenant(t), "PlanRLS-"+side, "#5080D8").Scan(ctx, &trackID))
			fixtures = append(fixtures, fixture{
				ctx: ctx, tenantID: testpkg.Tenant(t), title: "Block " + side, room: room.Name,
				staff: &planexport.StaffMember{ID: staff.ID, FirstName: staff.Person.FirstName, LastName: staff.Person.LastName},
				rows: map[string]int64{
					"users.staff_school_memberships": staff.ID,
					"facilities.rooms":               room.ID,
					"activities.groups":              activity.ID,
					"schedule.activity_instances":    instance.ID,
					"schedule.instance_staff":        assignment.ID,
					"schedule.instance_students":     roster.ID,
					"schedule.staff_shifts":          shift.ID,
					"schedule.shift_types":           shiftTypeID,
					"schedule.planning_tracks":       trackID,
					"schedule.closing_days":          closing.ID,
				},
			})
		})
	}
	require.Len(t, fixtures, 2)
	for table, ownID := range fixtures[0].rows {
		t.Run(table, func(t *testing.T) {
			for _, fixture := range fixtures {
				require.NoError(t, testpkg.WithTenantTx(t, fixture.ctx, db, fixture.tenantID, func(txCtx context.Context, tx bun.Tx) error {
					var bypass bool
					require.NoError(t, tx.NewRaw("SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user").Scan(txCtx, &bypass))
					require.False(t, bypass, "the tenant transaction must run under the least-privilege role")
					var ids []int64
					require.NoError(t, tx.NewSelect().Table(table).Column("id").Where("id IN (?, ?)", ownID, fixtures[1].rows[table]).Scan(txCtx, &ids))
					require.Equal(t, []int64{fixture.rows[table]}, ids, "%s leaks across the tenant boundary", table)
					return nil
				}))
			}
		})
	}

	names := staffNames(fixtures[0].staff, fixtures[1].staff)
	for _, fixture := range fixtures {
		t.Run("export-"+fixture.title, func(t *testing.T) {
			renderer := &captureRenderer{}
			params, err := planexport.ParseParams(monday.String(), monday.AddDays(4).String(), string(planexport.TemplateByOffering), "", "")
			require.NoError(t, err)
			require.NoError(t, testpkg.WithTenantTx(t, fixture.ctx, db, fixture.tenantID, func(txCtx context.Context, tx bun.Tx) error {
				_, err := New(betreuungsplanSources(t, db, tx, names, renderer)).ExportBetreuungsplan(txCtx, params)
				return err
			}))
			require.Len(t, renderer.doc.Rows, 1, "exactly the tenant's own block is printed")
			row := renderer.doc.Rows[0]
			require.Equal(t, fixture.title, listexport.StripStyleMarkers(row.Values[listexport.ColumnPlanRowLabel]))
			cell := listexport.StripStyleMarkers(row.Values[listexport.ColumnPlanMonday])
			require.Equal(t, "12:00–13:00 · "+fixture.room+"\n"+fixture.staff.LastName+", "+string([]rune(fixture.staff.FirstName)[:1])+".\n1 Kind", cell)
		})
	}
}

// The binding over the real block reads honours the widened week: a block
// on the Sunday of the requested week is loaded and printed in its own
// column, a block in the following week is not.
func TestInstanceBindingHonoursTheRequestedWindow(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	room := testpkg.CreateTestRoom(t, db, "PlanWindow")
	sunday := monday.AddDays(6)
	testpkg.CreateTestActivityInstance(t, db, sunday, room.ID, testpkg.ActivityInstanceOpts{Title: "Sonntag", StartHHMM: "09:00", EndHHMM: "10:00"})
	testpkg.CreateTestActivityInstance(t, db, monday.AddDays(7), room.ID, testpkg.ActivityInstanceOpts{Title: "Nächste Woche", StartHHMM: "09:00", EndHHMM: "10:00"})

	renderer := &captureRenderer{}
	params, err := planexport.ParseParams(monday.String(), monday.String(), string(planexport.TemplateByOffering), "", "")
	require.NoError(t, err)
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
		_, err := New(betreuungsplanSources(t, db, tx, fakeStaff{}, renderer)).ExportBetreuungsplan(txCtx, params)
		return err
	}))
	require.Len(t, renderer.doc.Rows, 1)
	require.Equal(t, "Sonntag", listexport.StripStyleMarkers(renderer.doc.Rows[0].Values[listexport.ColumnPlanRowLabel]))
	require.Len(t, renderer.doc.Columns, 8, "the Sunday block widens the sheet to the full week")
	require.Contains(t, listexport.StripStyleMarkers(renderer.doc.Rows[0].Values[listexport.ColumnPlanSunday]), "09:00–10:00")
}

// txRooms serves the room names from the tenant transaction under test, in
// the public Facilities shape the root's room read returns. Composing the
// Facilities module itself is that owner's test concern.
type txRooms struct {
	tx bun.IDB
}

func (r txRooms) ListRoomsByID(ctx context.Context, ids []int64) ([]facilities.Room, error) {
	if len(ids) == 0 {
		return []facilities.Room{}, nil
	}
	var rows []struct {
		ID   int64  `bun:"id"`
		Name string `bun:"name"`
	}
	if err := r.tx.NewSelect().TableExpr("facilities.rooms").Column("id", "name").Where("id IN (?)", bun.List(ids)).Scan(ctx, &rows); err != nil {
		return nil, err
	}
	out := make([]facilities.Room, 0, len(rows))
	for _, row := range rows {
		out = append(out, facilities.Room{ID: row.ID, Name: row.Name})
	}
	return out, nil
}

// txRosterCounts serves the head count per block from the roster rows of the
// tenant transaction under test. The fixtures record no absence, so every
// planned child counts; which children count as absent is Timetable's rule,
// pinned by its own tests.
type txRosterCounts struct {
	tx bun.IDB
}

func (r txRosterCounts) CountNonAbsentByInstanceIDs(ctx context.Context, instanceIDs []int64) (map[int64]int, error) {
	var rows []struct {
		InstanceID int64 `bun:"instance_id"`
		Children   int   `bun:"children"`
	}
	if err := r.tx.NewSelect().TableExpr("schedule.instance_students").ColumnExpr("instance_id, count(*) AS children").
		Where("instance_id IN (?)", bun.List(instanceIDs)).GroupExpr("instance_id").Scan(ctx, &rows); err != nil {
		return nil, err
	}
	out := make(map[int64]int, len(rows))
	for _, row := range rows {
		out[row.InstanceID] = row.Children
	}
	return out, nil
}

// ownerInstances serves the block read from the Timetable owner's
// presence-joined reads, the rows the root's instance reads return.
type ownerInstances struct {
	rows timetabletest.LegacyRows
}

func newOwnerInstances(t *testing.T, db *bun.DB) ownerInstances {
	t.Helper()
	return ownerInstances{rows: timetabletest.NewLegacyRows(t, db)}
}

func (s ownerInstances) FindByTenantAndDateRange(ctx context.Context, from, to calendar.Date) ([]*timetable.ScheduledInstance, error) {
	rows, err := s.rows.InstancesBetween(ctx, from.String(), to.String())
	if err != nil {
		return nil, err
	}
	out := make([]*timetable.ScheduledInstance, 0, len(rows))
	for _, row := range rows {
		out = append(out, &timetable.ScheduledInstance{
			ID: row.ID, Date: calendar.Date(row.Date), StartTime: row.StartTime, EndTime: row.EndTime,
			Title: row.Title, ActivityGroupID: row.ActivityGroupID, ActiveGroupID: row.ActiveGroupID,
			RoomID: row.RoomID, Status: row.Status, ListKind: row.ListKind,
			CancelReason: row.CancelReason, Notes: row.Notes, UnderstaffedNote: row.UnderstaffedNote,
		})
	}
	return out, nil
}

// ownerInstanceStaff serves the staff-assignment read from the Timetable
// owner with the filter and order of the root's instance staff reads.
type ownerInstanceStaff struct {
	timetable timetable.Capability
}

func newOwnerInstanceStaff(t *testing.T, db *bun.DB) ownerInstanceStaff {
	t.Helper()
	return ownerInstanceStaff{timetable: timetabletest.New(t, db)}
}

func (s ownerInstanceStaff) FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*timetable.InstanceStaff, error) {
	if len(instanceIDs) == 0 {
		return []*timetable.InstanceStaff{}, nil
	}
	rows, err := s.timetable.ListInstanceStaff(ctx, timetable.InstanceStaffFilter{InstanceIDs: instanceIDs, OrderByInstanceAndCreated: true})
	if err != nil {
		return nil, err
	}
	out := make([]*timetable.InstanceStaff, 0, len(rows))
	for index := range rows {
		out = append(out, &rows[index])
	}
	return out, nil
}
