package owners

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/timetabletest"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// buildDienstplanWorkforce composes the Workforce capability over the test
// tenant runtime, the way the Workforce suites do.
func buildDienstplanWorkforce(t *testing.T, db *bun.DB) workforce.Capability {
	t.Helper()
	tenantRuntime := testpkg.ConfigRuntime(db)
	capability, err := workforceCompose.New(workforceCompose.Dependencies{LockStaffAssignment: tenantRuntime.LockStaffAssignment,
		DB:           db,
		LiveStaffIDs: tenantRuntime.LiveStaffIDs,
		Observe:      func(workforceCompose.Observation) {},
	})
	require.NoError(t, err)
	return capability
}

// The Dienstplan export over the Workforce staff schedule overview (#2706).
// The overview runs on the real Workforce capability for the shifts, the
// Timetable owner's reads for the blocks and their staff, and the root's room
// read; the staff roster is read over the tenant transaction under test, the
// way the root's retained staff read answers it. The plan export renders what
// the overview returns.

var dienstplanMonday = calendar.NewDate(2026, time.July, 27)

// dienstplanOverview builds the overview the root binds to the plan export,
// with the staff roster read over the given tenant transaction.
func dienstplanOverview(t *testing.T, db *bun.DB, tx bun.IDB, capability workforce.Capability, rooms workforceCompose.RoomReader) workforce.StaffScheduleOverviewQuery {
	t.Helper()
	instances := dienstplanInstances{rows: timetabletest.NewLegacyRows(t, NewTimetablePresenceProjection(testpkg.MustDatabase(db)))}
	return workforceCompose.NewStaffScheduleOverview(workforceCompose.StaffScheduleOverviewDependencies{
		Shifts:        capability,
		Instances:     instances,
		InstanceStaff: dienstplanInstanceStaff{timetable: timetabletest.New(t, db)},
		Rooms:         rooms,
		Staff:         txStaffRoster{tx: tx},
	})
}

// dienstplanExport renders the Dienstplan over the overview alone. The
// overview binding mirrors the root's (plan_export.go, whose unit tests pin
// every field) without the reads only the Betreuungsplan needs.
func dienstplanExport(overview workforce.StaffScheduleOverviewQuery, renderer planexport.Renderer) planexport.Service {
	return planexport.NewService(planexport.Dependencies{Overview: dienstplanOverviewRecords{query: overview}, Renderer: renderer}, nil)
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
		staff    *testpkg.Staff
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

	capability := buildDienstplanWorkforce(t, db)
	rooms := NewTimetableRoomNames(MustNewUnobservedRooms(testpkg.NewDatabase(t, db)))
	params, err := planexport.ParseParams(dienstplanMonday.String(), dienstplanMonday.AddDays(4).String(), string(planexport.TemplateByPerson), "", "")
	require.NoError(t, err)
	for _, fixture := range fixtures {
		t.Run("dienstplan-"+fixture.title, func(t *testing.T) {
			renderer := &planExportCaptureRenderer{}
			require.NoError(t, testpkg.WithTenantTx(t, fixture.ctx, db, fixture.tenantID, func(txCtx context.Context, tx bun.Tx) error {
				_, err := dienstplanExport(dienstplanOverview(t, db, tx, capability, rooms), renderer).ExportDienstplan(txCtx, params)
				return err
			}))
			require.Len(t, renderer.doc.Rows, 1, "exactly the tenant's own staff member is printed")
			row := renderer.doc.Rows[0]
			require.Equal(t, fixture.staff.Person.LastName+", "+fixture.staff.Person.FirstName, lists.SanitizeUserText(row.Values[lists.ColumnPlanRowLabel]))
			cell := lists.SanitizeUserText(row.Values[lists.ColumnPlanMonday])
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

// dienstplanOverviewRecords translates the public staff week into the plan export's
// records, field for field as the root's binding does.
type dienstplanOverviewRecords struct {
	query workforce.StaffScheduleOverviewQuery
}

func (o dienstplanOverviewRecords) StaffScheduleOverview(ctx context.Context, from, to planexport.Date) (*planexport.StaffScheduleOverview, error) {
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
			StartTime: dienstplanClock(shift.StartTime), EndTime: dienstplanClock(shift.EndTime),
			ShiftTypeID: shift.ShiftTypeID, OriginShiftID: shift.OriginShiftID, Cancelled: shift.Cancelled,
			ChangeReason: shift.ChangeReason, Notes: shift.Notes,
		})
	}
	for _, assignment := range overview.Assignments {
		record := planexport.Assignment{
			StaffID: assignment.StaffID, Date: planexport.Date(assignment.Date),
			StartTime: dienstplanClock(assignment.StartTime), EndTime: dienstplanClock(assignment.EndTime),
			ActivityTitle: assignment.ActivityTitle, ActivityGroupID: assignment.ActivityGroupID, RoomName: assignment.RoomName,
			IsSubstitute: assignment.IsSubstitute, IsAbsent: assignment.IsAbsent,
		}
		for _, gap := range assignment.UncoveredIntervals {
			record.UncoveredIntervals = append(record.UncoveredIntervals, planexport.Interval{StartTime: dienstplanClock(gap.StartTime), EndTime: dienstplanClock(gap.EndTime)})
		}
		out.Assignments = append(out.Assignments, record)
	}
	return out, nil
}

func dienstplanClock(value string) time.Time {
	parsed, _ := time.Parse(workforce.ClockLayout, value)
	return calendar.NormalizeWallClock(parsed)
}

// txStaffRoster serves the overview's staff roster from the tenant
// transaction under test, in the retained staff shape its port reads.
type txStaffRoster struct {
	tx bun.IDB
}

// txStaffRow and txPersonRow scan the staff and person columns the roster
// reads.
type txStaffRow struct {
	bun.BaseModel      `bun:"alias:staff"`
	ID                 int64          `bun:"id"`
	PersonID           int64          `bun:"person_id"`
	WorkTimeModelID    *int64         `bun:"work_time_model_id"`
	RotationAnchorDate *calendar.Date `bun:"rotation_anchor_date,type:date"`
}

type txPersonRow struct {
	bun.BaseModel `bun:"alias:person"`
	FirstName     string `bun:"first_name"`
	LastName      string `bun:"last_name"`
	AccountID     *int64 `bun:"account_id"`
}

func (r txStaffRoster) ListAllWithPerson(ctx context.Context) ([]*workforce.PlanStaff, error) {
	var rows []txStaffRow
	if err := r.tx.NewSelect().Model(&rows).ModelTableExpr(`(SELECT m.id, m.tenant_id, m.person_id, m.created_at, m.updated_at, m.deleted_at, p.staff_notes, p.employment_type, p.work_time_model_id, p.personnel_number, p.rotation_anchor_date, p.birthday_display_opt_out FROM users.staff_school_memberships AS m JOIN users.staff_employment_profiles AS p ON p.tenant_id = m.tenant_id AND p.membership_id = m.id) AS "staff"`).Where(`"staff".deleted_at IS NULL`).Scan(ctx); err != nil {
		return nil, err
	}
	staff := make([]*workforce.PlanStaff, 0, len(rows))
	for _, row := range rows {
		person := &txPersonRow{}
		if err := r.tx.NewSelect().Model(person).ModelTableExpr(`users.persons AS "person"`).Where(`"person".id = ?`, row.PersonID).Scan(ctx); err != nil {
			return nil, err
		}
		staff = append(staff, &workforce.PlanStaff{
			ID: row.ID, PersonID: row.PersonID, WorkTimeModelID: row.WorkTimeModelID, RotationAnchorDate: row.RotationAnchorDate,
			Person: &workforce.PlanStaffPerson{FirstName: person.FirstName, LastName: person.LastName, AccountID: person.AccountID},
		})
	}
	return staff, nil
}

func (r txStaffRoster) FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*workforce.PlanStaff, error) {
	rows, err := r.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*workforce.PlanStaff, len(ids))
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

func (s dienstplanInstances) FindByTenantAndDateRange(ctx context.Context, from, to calendar.Date) ([]*timetable.ScheduledInstance, error) {
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
			ID: row.ID, Date: calendar.Date(row.Date), StartTime: row.StartTime, EndTime: row.EndTime,
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

func (s dienstplanInstanceStaff) FindByStaffAndDateRange(ctx context.Context, staffID int64, from, to calendar.Date) ([]*timetable.InstanceStaff, error) {
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

// Measures the Dienstplan export over the Workforce staff schedule overview,
// independently of HTTP. The shifts come from the real Workforce capability,
// the blocks and their staff from the Timetable owner's reads, the rooms from
// the root's room read and the staff roster from the tenant transaction
// (txStaffRoster).
func TestDienstplanRuntimeEvidence(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	room := testpkg.CreateTestRoom(t, db, "Runtime-room")
	staffIDs := make([]int64, 0, 4)
	first := testpkg.CreateTestStaff(t, db, "Runtime", "Staff0")
	staffIDs = append(staffIDs, first.ID)
	for i := 1; i < 4; i++ {
		staffIDs = append(staffIDs, testpkg.CreateTestStaff(t, db, "Runtime", fmt.Sprintf("Staff%d", i)).ID)
	}
	capability := buildDienstplanWorkforce(t, db)
	rooms := NewTimetableRoomNames(MustNewUnobservedRooms(testpkg.NewDatabase(t, db)))
	export := func(txCtx context.Context, tx bun.Tx, renderer planexport.Renderer, params planexport.Params) (lists.File, error) {
		return dienstplanExport(dienstplanOverview(t, db, tx, capability, rooms), renderer).ExportDienstplan(txCtx, params)
	}
	var postgres string
	require.NoError(t, db.NewRaw("SHOW server_version").Scan(ctx, &postgres))
	counter := testpkg.CaptureQueriesForContext(t, db)
	deadlocks := func() int64 {
		var n int64
		require.NoError(t, db.NewRaw("SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(ctx, &n))
		return n
	}
	seededWeeks := 0
	for _, scenario := range []struct {
		name  string
		weeks int
		pdf   bool
	}{{"empty-week", 0, false}, {"week", 1, false}, {"weeks-4", 4, false}, {"pdf-4", 4, true}} {
		t.Run(scenario.name, func(t *testing.T) {
			for seededWeeks < scenario.weeks {
				// Every staff member works every weekday and supervises one
				// block per day: 20 shifts and 20 blocks per printed week.
				for weekday := range 5 {
					date := dienstplanMonday.AddDays(7*seededWeeks + weekday)
					for i, staffID := range staffIDs {
						testpkg.CreateTestStaffShift(t, db, staffID, date, testpkg.StaffShiftOpts{})
						instance := testpkg.CreateTestActivityInstance(t, db, date, room.ID, testpkg.ActivityInstanceOpts{
							Title:     fmt.Sprintf("Block %d", i),
							StartHHMM: fmt.Sprintf("%02d:00", 12+i), EndHHMM: fmt.Sprintf("%02d:00", 13+i),
						})
						testpkg.CreateTestInstanceStaff(t, db, instance.ID, staffID, testpkg.InstanceStaffOpts{})
					}
				}
				seededWeeks++
			}
			weeks := max(scenario.weeks, 1)
			params, err := planexport.ParseParams(dienstplanMonday.String(), dienstplanMonday.AddDays(7*weeks-3).String(), string(planexport.TemplateByPerson), "", "")
			require.NoError(t, err)
			beforeDeadlocks := deadlocks()
			stopLocks := testpkg.SampleCheckpointLocks(func(sampleCtx context.Context) (int, error) {
				var n int
				err := db.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(sampleCtx, &n)
				return n, err
			})
			var samples []testpkg.RuntimeCheckpointSample
			var durations []float64
			expectedRows := -1
			for i := range 35 {
				counter.Reset()
				before := db.Stats()
				start := time.Now()
				var file lists.File
				err := testpkg.WithTenantTx(t, counter.Context(ctx), db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
					var err error
					if scenario.pdf {
						file, err = export(txCtx, tx, lists.NewRendererService(), params)
						return err
					}
					file, err = export(txCtx, tx, planExportDocumentProbe{}, params)
					return err
				})
				elapsed := float64(time.Since(start)) / float64(time.Millisecond)
				after := db.Stats()
				require.NoError(t, err)
				if scenario.pdf {
					require.True(t, strings.HasPrefix(string(file.Data), "%PDF-"))
				} else {
					var rows int
					require.NoError(t, json.Unmarshal(file.Data, &rows))
					if expectedRows < 0 {
						expectedRows = rows
						if scenario.weeks == 0 {
							require.Equal(t, 1, rows, "an empty week prints its explicit empty row")
						} else {
							require.GreaterOrEqual(t, rows, len(staffIDs)*scenario.weeks, "every staff member prints in every week")
						}
					}
					require.Equal(t, expectedRows, rows, "the row count is stable across samples")
				}
				require.Zero(t, counter.WriteRows(), "the plan export must not persist anything")
				if i >= 5 {
					affected, statements := counter.Rows()
					zero := counter.WriteRows()
					samples = append(samples, testpkg.RuntimeCheckpointSample{DurationMS: elapsed, Queries: counter.Total(), RowsAffected: affected, StatementsWithRows: statements, WriteRowsAffected: &zero, PoolWaitCount: after.WaitCount - before.WaitCount, PoolWaitMS: float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond)})
					durations = append(durations, elapsed)
				}
			}
			locks := stopLocks()
			require.Empty(t, locks.Error)
			slices.Sort(durations)
			report, err := json.Marshal(map[string]any{"scenario": scenario.name, "weeks": scenario.weeks, "shifts": 20 * scenario.weeks, "blocks": 20 * scenario.weeks, "samples": samples, "p50_ms": durations[14], "p95_ms": durations[28], "max_ms": durations[29], "go": runtime.Version(), "postgres": postgres, "warmup": 5, "concurrency": 1, "unexpected_errors": 0, "lock_samples": locks, "deadlocks": deadlocks() - beforeDeadlocks, "cache": "no projection cache; staff roster read over the tenant transaction"})
			require.NoError(t, err)
			t.Logf("dienstplan-runtime: %s", report)
		})
	}
}
