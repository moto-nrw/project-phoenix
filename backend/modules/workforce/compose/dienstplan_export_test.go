package compose_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/timetabletest"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The Dienstplan export over the Workforce staff schedule overview (#2706).
// The overview runs on the real Workforce capability for the shifts, the
// Timetable owner's reads for the blocks and their staff, and the root's room
// read; the staff roster is read over the tenant transaction under test, the
// way the root's retained staff read answers it. The plan export renders what
// the overview returns.

var dienstplanMonday = timezone.NewDate(2026, time.July, 27)

// dienstplanOverview builds the overview the root binds to the plan export,
// with the staff roster read over the given tenant transaction.
func dienstplanOverview(t *testing.T, db *bun.DB, tx bun.IDB, capability workforce.Capability, rooms compose.RoomReader) workforce.StaffScheduleOverviewQuery {
	t.Helper()
	instances := dienstplanInstances{rows: timetabletest.NewLegacyRows(t, db)}
	return compose.NewStaffScheduleOverview(compose.StaffScheduleOverviewDependencies{
		Shifts:        capability,
		Instances:     instances,
		InstanceStaff: dienstplanInstanceStaff{timetable: timetabletest.New(t, db)},
		Rooms:         rooms,
		Staff:         txStaffRoster{tx: tx},
	})
}

// dienstplanExport renders the Dienstplan over the overview. The overview
// binding mirrors the root's (modules/planexport/compose, whose unit tests
// pin every field); this role may not import that package.
func dienstplanExport(overview workforce.StaffScheduleOverviewQuery, renderer planexport.Renderer) planexport.Service {
	return planexport.NewService(planexport.Dependencies{Overview: overviewRecords{query: overview}, Renderer: renderer}, nil)
}

// TestDienstplanExportStaysInsideTheTenant proves the Dienstplan side of the
// plan export (#2706): two schools seed the same staff member, shift and
// supervised block, and each school's Dienstplan lists only its own staff
// member with their own shift and block, and nothing of the other school.
func TestDienstplanExportStaysInsideTheTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	type fixture struct {
		ctx      context.Context
		tenantID int64
		staff    *usersModels.Staff
		title    string
		room     string
	}
	var fixtures []fixture
	for _, side := range []string{"own", "foreign"} {
		t.Run(side, func(t *testing.T) {
			testpkg.OwnTenant(t)
			staff := testpkg.CreateTestStaff(t, db, "PlanRLS", side)
			room := testpkg.CreateTestRoom(t, db, "PlanRLS-"+side)
			activity := testpkg.CreateTestActivityGroup(t, db, "PlanRLS-"+side)
			instance := testpkg.CreateTestActivityInstance(t, db, dienstplanMonday, room.ID, testpkg.ActivityInstanceOpts{
				Title: "Block " + side, ActivityGroupID: &activity.ID, StartHHMM: "12:00", EndHHMM: "13:00",
			})
			testpkg.CreateTestInstanceStaff(t, db, instance.ID, staff.ID, testpkg.InstanceStaffOpts{})
			testpkg.CreateTestStaffShift(t, db, staff.ID, dienstplanMonday, testpkg.StaffShiftOpts{})
			fixtures = append(fixtures, fixture{ctx: testpkg.Ctx(t), tenantID: testpkg.Tenant(t), staff: staff, title: "Block " + side, room: room.Name})
		})
	}
	require.Len(t, fixtures, 2)

	capability := buildPlanningWorkforce(t, db)
	rooms := planningDependencies(t, db, capability, fixtures[0].staff).Rooms
	params, err := planexport.ParseParams(dienstplanMonday.String(), dienstplanMonday.AddDays(4).String(), string(planexport.TemplateByPerson), "", "")
	require.NoError(t, err)
	for _, fixture := range fixtures {
		t.Run("dienstplan-"+fixture.title, func(t *testing.T) {
			renderer := &dienstplanRenderer{}
			require.NoError(t, testpkg.WithTenantTx(t, fixture.ctx, db, fixture.tenantID, func(txCtx context.Context, tx bun.Tx) error {
				_, err := dienstplanExport(dienstplanOverview(t, db, tx, capability, rooms), renderer).ExportDienstplan(txCtx, params)
				return err
			}))
			require.Len(t, renderer.doc.Rows, 1, "exactly the tenant's own staff member is printed")
			row := renderer.doc.Rows[0]
			require.Equal(t, fixture.staff.Person.LastName+", "+fixture.staff.Person.FirstName, listexport.StripStyleMarkers(row.Values[listexport.ColumnPlanRowLabel]))
			cell := listexport.StripStyleMarkers(row.Values[listexport.ColumnPlanMonday])
			require.Contains(t, cell, "08:00–16:00", "the tenant's own shift is printed")
			require.Contains(t, cell, fixture.title, "the tenant's own block is printed under the shift")
			for _, other := range fixtures {
				if other.tenantID == fixture.tenantID {
					continue
				}
				require.NotContains(t, cell, other.title, "the other school's block leaks into the Dienstplan")
				require.NotContains(t, cell, other.room, "the other school's room leaks into the Dienstplan")
				require.Equal(t, 1, strings.Count(cell, "08:00–16:00"), "the other school's shift leaks into the Dienstplan")
			}
		})
	}
}

// overviewRecords translates the public staff week into the plan export's
// records, field for field as the root's binding does.
type overviewRecords struct {
	query workforce.StaffScheduleOverviewQuery
}

func (o overviewRecords) StaffScheduleOverview(ctx context.Context, from, to planexport.Date) (*planexport.StaffScheduleOverview, error) {
	overview, err := o.query.Overview(ctx, string(from), string(to))
	if err != nil {
		return nil, err
	}
	out := &planexport.StaffScheduleOverview{}
	for _, member := range overview.Staff {
		out.Staff = append(out.Staff, &planexport.StaffMember{ID: member.ID, FirstName: member.FirstName, LastName: member.LastName})
	}
	for _, shift := range overview.Shifts {
		out.Shifts = append(out.Shifts, &planexport.Shift{
			ID: shift.ID, StaffID: shift.StaffID, Date: planexport.Date(shift.Date),
			StartTime: overviewClock(shift.StartTime), EndTime: overviewClock(shift.EndTime),
			ShiftTypeID: shift.ShiftTypeID, OriginShiftID: shift.OriginShiftID, Cancelled: shift.Cancelled,
			ChangeReason: shift.ChangeReason, Notes: shift.Notes,
		})
	}
	for _, assignment := range overview.Assignments {
		record := planexport.Assignment{
			StaffID: assignment.StaffID, Date: planexport.Date(assignment.Date),
			StartTime: overviewClock(assignment.StartTime), EndTime: overviewClock(assignment.EndTime),
			ActivityTitle: assignment.ActivityTitle, ActivityGroupID: assignment.ActivityGroupID, RoomName: assignment.RoomName,
			IsSubstitute: assignment.IsSubstitute, IsAbsent: assignment.IsAbsent,
		}
		for _, gap := range assignment.UncoveredIntervals {
			record.UncoveredIntervals = append(record.UncoveredIntervals, planexport.Interval{StartTime: overviewClock(gap.StartTime), EndTime: overviewClock(gap.EndTime)})
		}
		out.Assignments = append(out.Assignments, record)
	}
	return out, nil
}

func overviewClock(value string) time.Time {
	parsed, _ := time.Parse(workforce.ClockLayout, value)
	return timezone.NormalizeWallClock(parsed)
}

type dienstplanRenderer struct {
	doc listexport.Document
}

func (c *dienstplanRenderer) Render(doc listexport.Document, _ listexport.Format, filenameBase string) (listexport.File, error) {
	c.doc = doc
	return listexport.File{Data: []byte("rendered"), ContentType: "application/pdf", Filename: filenameBase + ".pdf"}, nil
}

// txStaffRoster serves the overview's staff roster from the tenant
// transaction under test, in the retained staff shape its port reads.
type txStaffRoster struct {
	tx bun.IDB
}

func (r txStaffRoster) ListAllWithPerson(ctx context.Context) ([]*usersModels.Staff, error) {
	var rows []*usersModels.Staff
	if err := r.tx.NewSelect().Model(&rows).ModelTableExpr(`(SELECT m.id, m.tenant_id, m.person_id, m.created_at, m.updated_at, m.deleted_at, p.staff_notes, p.employment_type, p.work_time_model_id, p.personnel_number, p.rotation_anchor_date, p.birthday_display_opt_out FROM users.staff_school_memberships AS m JOIN users.staff_employment_profiles AS p ON p.tenant_id = m.tenant_id AND p.membership_id = m.id) AS "staff"`).Scan(ctx); err != nil {
		return nil, err
	}
	for _, row := range rows {
		person := &usersModels.Person{}
		if err := r.tx.NewSelect().Model(person).ModelTableExpr(`users.persons AS "person"`).Where(`"person".id = ?`, row.PersonID).Scan(ctx); err != nil {
			return nil, err
		}
		row.Person = person
	}
	return rows, nil
}

func (r txStaffRoster) FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*usersModels.Staff, error) {
	rows, err := r.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*usersModels.Staff, len(ids))
	for _, row := range rows {
		for _, id := range ids {
			if row.ID == id {
				out[id] = row
			}
		}
	}
	return out, nil
}

// dienstplanInstances serves the overview's block reads from the Timetable
// owner's presence-joined reads, in the scheduled-instance shape the
// Workforce overview speaks.
type dienstplanInstances struct {
	rows timetabletest.LegacyRows
}

func (s dienstplanInstances) FindByTenantAndDateRange(ctx context.Context, from, to timezone.Date) ([]*timetable.ScheduledInstance, error) {
	return scheduledDienstplanInstances(s.rows.InstancesBetween(ctx, from.String(), to.String()))
}

func (s dienstplanInstances) FindByIDs(ctx context.Context, ids []int64) ([]*timetable.ScheduledInstance, error) {
	return scheduledDienstplanInstances(s.rows.InstancesByID(ctx, ids))
}

func scheduledDienstplanInstances(rows []timetabletest.LegacyInstance, err error) ([]*timetable.ScheduledInstance, error) {
	if err != nil {
		return nil, err
	}
	out := make([]*timetable.ScheduledInstance, 0, len(rows))
	for _, row := range rows {
		out = append(out, &timetable.ScheduledInstance{
			ID: row.ID, Date: timezone.Date(row.Date), StartTime: row.StartTime, EndTime: row.EndTime,
			Title: row.Title, ActivityGroupID: row.ActivityGroupID, ActiveGroupID: row.ActiveGroupID,
			RoomID: row.RoomID, Status: row.Status, ListKind: row.ListKind,
			CancelReason: row.CancelReason, Notes: row.Notes, UnderstaffedNote: row.UnderstaffedNote,
		})
	}
	return out, nil
}

// dienstplanInstanceStaff serves the overview's staff-assignment reads from
// the Timetable owner with the filters and order of the root's reads.
type dienstplanInstanceStaff struct {
	timetable timetable.Capability
}

func (s dienstplanInstanceStaff) FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*timetable.InstanceStaff, error) {
	if len(instanceIDs) == 0 {
		return []*timetable.InstanceStaff{}, nil
	}
	return s.list(ctx, timetable.InstanceStaffFilter{InstanceIDs: instanceIDs, OrderByInstanceAndCreated: true})
}

func (s dienstplanInstanceStaff) FindByStaffAndDateRange(ctx context.Context, staffID int64, from, to timezone.Date) ([]*timetable.InstanceStaff, error) {
	fromText, toText := from.String(), to.String()
	return s.list(ctx, timetable.InstanceStaffFilter{
		StaffIDs: []int64{staffID}, FromDate: &fromText, ToDate: &toText, OrderByActivityDateTime: true,
	})
}

func (s dienstplanInstanceStaff) list(ctx context.Context, filter timetable.InstanceStaffFilter) ([]*timetable.InstanceStaff, error) {
	rows, err := s.timetable.ListInstanceStaff(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]*timetable.InstanceStaff, 0, len(rows))
	for index := range rows {
		out = append(out, &rows[index])
	}
	return out, nil
}
