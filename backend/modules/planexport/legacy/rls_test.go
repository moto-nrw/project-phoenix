package legacy

import (
	"context"
	"testing"

	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// facadeRooms binds the retained room-batch signature to the public
// Facilities facade, the way the root's room directory does.
type facadeRooms struct {
	facade *facilities.Module
}

func (r facadeRooms) FindByIDs(ctx context.Context, ids []int64) ([]*facilitiesModel.Room, error) {
	rows, err := r.facade.ListRoomsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]*facilitiesModel.Room, 0, len(rows))
	for _, row := range rows {
		room := &facilitiesModel.Room{Name: row.Name}
		room.ID = row.ID
		out = append(out, room)
	}
	return out, nil
}

func newFacilities(t *testing.T, db *bun.DB) *facilities.Module {
	t.Helper()
	module, err := facilitiesCompose.New(facilitiesCompose.Dependencies{
		DB:            db,
		DeletionLock:  func(context.Context) error { return nil },
		DeletionGuard: func(context.Context, int64) error { return nil },
		Observe:       func(facilitiesCompose.Observation) {},
	})
	require.NoError(t, err)
	return module
}

// TestPlanExportInputsEnforceRLS proves the plan export capability (#2706)
// stays inside the tenant: two schools seed the same shape in every table the
// two renderers read, those rows are invisible across the tenant boundary
// under the least-privilege role, and each school's Betreuungsplan, rendered
// over the real retained instance, staff and room sources, lists only its own
// block, room and staff member.
func TestPlanExportInputsEnforceRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	type fixture struct {
		ctx      context.Context
		tenantID int64
		staff    *usersModel.Staff
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
			shiftType := &scheduleModel.ShiftType{Name: "PlanRLS-" + side, Color: "#83CD2D", IsActive: true}
			shiftType.SetTenantID(testpkg.Tenant(t))
			require.NoError(t, db.NewInsert().Model(shiftType).ModelTableExpr("schedule.shift_types").Scan(ctx))
			track := &scheduleModel.PlanningTrack{Name: "PlanRLS-" + side, Color: "#5080D8"}
			track.SetTenantID(testpkg.Tenant(t))
			require.NoError(t, db.NewInsert().Model(track).ModelTableExpr("schedule.planning_tracks").Scan(ctx))
			fixtures = append(fixtures, fixture{
				ctx: ctx, tenantID: testpkg.Tenant(t), staff: staff, title: "Block " + side, room: room.Name,
				rows: map[string]int64{
					"users.staff":                 staff.ID,
					"facilities.rooms":            room.ID,
					"activities.groups":           activity.ID,
					"schedule.activity_instances": instance.ID,
					"schedule.instance_staff":     assignment.ID,
					"schedule.instance_students":  roster.ID,
					"schedule.staff_shifts":       shift.ID,
					"schedule.shift_types":        shiftType.ID,
					"schedule.planning_tracks":    track.ID,
					"schedule.closing_days":       closing.ID,
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

	rooms := newFacilities(t, db)
	for _, fixture := range fixtures {
		t.Run("export-"+fixture.title, func(t *testing.T) {
			renderer := &captureRenderer{}
			service := New(Sources{
				Instances:     scheduleRepo.NewActivityInstanceRepository(db),
				InstanceStaff: scheduleRepo.NewInstanceStaffRepository(db),
				Rooms:         facadeRooms{facade: rooms},
				Staff:         fakeStaff{members: map[int64]*usersModel.Staff{fixtures[0].staff.ID: fixtures[0].staff, fixtures[1].staff.ID: fixtures[1].staff}},
				Renderer:      renderer,
			})
			params, err := planexport.ParseParams(monday.String(), monday.AddDays(4).String(), string(planexport.TemplateByOffering), "", "")
			require.NoError(t, err)
			require.NoError(t, testpkg.WithTenantTx(t, fixture.ctx, db, fixture.tenantID, func(txCtx context.Context, _ bun.Tx) error {
				_, err := service.ExportBetreuungsplan(txCtx, params)
				return err
			}))
			require.Len(t, renderer.doc.Rows, 1, "exactly the tenant's own block is printed")
			row := renderer.doc.Rows[0]
			require.Equal(t, fixture.title, listexport.StripStyleMarkers(row.Values[listexport.ColumnPlanRowLabel]))
			cell := listexport.StripStyleMarkers(row.Values[listexport.ColumnPlanMonday])
			require.Equal(t, "12:00–13:00 · "+fixture.room+"\n"+fixture.staff.Person.LastName+", "+string([]rune(fixture.staff.Person.FirstName)[:1])+".", cell)
		})
	}
}

// The adapter over the real instance repository honours the widened week:
// a block on the Sunday of the requested week is loaded and printed in its
// own column, a block in the following week is not.
func TestInstanceSourceHonoursTheRequestedWindow(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	room := testpkg.CreateTestRoom(t, db, "PlanWindow")
	sunday := monday.AddDays(6)
	testpkg.CreateTestActivityInstance(t, db, sunday, room.ID, testpkg.ActivityInstanceOpts{Title: "Sonntag", StartHHMM: "09:00", EndHHMM: "10:00"})
	testpkg.CreateTestActivityInstance(t, db, monday.AddDays(7), room.ID, testpkg.ActivityInstanceOpts{Title: "Nächste Woche", StartHHMM: "09:00", EndHHMM: "10:00"})

	renderer := &captureRenderer{}
	service := New(Sources{
		Instances:     scheduleRepo.NewActivityInstanceRepository(db),
		InstanceStaff: scheduleRepo.NewInstanceStaffRepository(db),
		Renderer:      renderer,
	})
	params, err := planexport.ParseParams(monday.String(), monday.String(), string(planexport.TemplateByOffering), "", "")
	require.NoError(t, err)
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		_, err := service.ExportBetreuungsplan(txCtx, params)
		return err
	}))
	require.Len(t, renderer.doc.Rows, 1)
	require.Equal(t, "Sonntag", listexport.StripStyleMarkers(renderer.doc.Rows[0].Values[listexport.ColumnPlanRowLabel]))
	require.Len(t, renderer.doc.Columns, 8, "the Sunday block widens the sheet to the full week")
	require.Contains(t, listexport.StripStyleMarkers(renderer.doc.Rows[0].Values[listexport.ColumnPlanSunday]), "09:00–10:00")
}
