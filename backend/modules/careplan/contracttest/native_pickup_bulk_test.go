package contracttest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type bulkPickupDirectory struct {
	rows        map[int64]compose.PickupBulkStudent
	locked      []int64
	departed    int64
	nameFailure int64
	err         error
	beforeName  func()
}

func (d *bulkPickupDirectory) FindByIDs(context.Context, []int64, calendar.Date) (map[int64]compose.PickupBulkStudent, error) {
	return d.rows, nil
}
func (d *bulkPickupDirectory) LockByID(_ context.Context, id int64, _ calendar.Date) (compose.PickupBulkStudent, error) {
	d.locked = append(d.locked, id)
	row := d.rows[id]
	if id == d.departed {
		row.Eligible = false
	}
	return row, nil
}
func (d *bulkPickupDirectory) Name(_ context.Context, row compose.PickupBulkStudent) (string, error) {
	d.beforeName()
	if row.ID == d.nameFailure {
		return "", d.err
	}
	return "Pickup Student", nil
}

func TestNativeBulkPickupRollsBackTheWholeSelection(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"authorization", "departure after selection", "name after first write"} {
		t.Run(failure, func(t *testing.T) {
			t.Parallel()
			db := testpkg.SetupTestDB(t)
			ctx := testpkg.Ctx(t)
			owner := careplantest.NewCarePlan(t, db)
			first := testpkg.CreateTestStudent(t, db, "First", "Pickup", "1a")
			second := testpkg.CreateTestStudent(t, db, "Second", "Pickup", "1a")
			staff := testpkg.CreateTestStaff(t, db, "Pickup", "Staff")
			require.Less(t, first.ID, second.ID)
			clock := time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC)
			note := "Keep this note"
			directory := &bulkPickupDirectory{rows: make(map[int64]compose.PickupBulkStudent), err: errors.New("directory unavailable")}
			for _, id := range []int64{first.ID, second.ID} {
				directory.rows[id] = compose.PickupBulkStudent{ScheduleStudent: careplan.ScheduleStudent{ID: id, TenantID: testpkg.Tenant(t)}, Eligible: true}
				_, err := owner.UpsertPickupSchedule(ctx, careplan.PickupSchedule{StudentID: id, Weekday: 1, PickupTime: clock, Notes: &note, CreatedBy: staff.ID})
				require.NoError(t, err)
			}
			directory.beforeName = func() {
				require.Equal(t, []int64{first.ID, second.ID}, directory.locked, "all student locks precede any patch")
			}
			filter := careplan.PickupBulkFilter{StudentIDs: []int64{second.ID, first.ID}}
			want := careplan.ErrBulkStudentUnauthorized
			switch failure {
			case "authorization":
				filter.Authorize = func(_ context.Context, row careplan.ScheduleStudent) (bool, error) {
					require.Equal(t, testpkg.Tenant(t), row.TenantID)
					if row.ID == second.ID {
						return false, directory.err
					}
					return true, nil
				}
			case "departure after selection":
				directory.departed = second.ID
				want = careplan.ErrBulkStudentNotFound
			case "name after first write":
				directory.nameFailure = second.ID
				want = directory.err
			}
			service, err := compose.NewPickupSchedules(db, owner, excusalBaseline{}, nil, directory, nil)
			require.NoError(t, err)
			result, err := service.BulkUpsertPickupSchedules(ctx, filter, []careplan.PickupScheduleInput{{Weekday: 1, PickupTime: "16:00"}}, staff.ID)
			require.ErrorIs(t, err, want)
			require.Nil(t, result)
			rows, err := owner.ListPickupSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: filter.StudentIDs})
			require.NoError(t, err)
			require.Len(t, rows, 2)
			for _, row := range rows {
				require.Equal(t, "14:00", row.PickupTime.Format("15:04"))
				require.Equal(t, &note, row.Notes)
			}
		})
	}
}
