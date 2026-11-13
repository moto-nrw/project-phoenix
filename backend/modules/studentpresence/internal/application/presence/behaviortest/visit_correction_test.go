package behaviortest_test

import (
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdateVisit_EarlierRoomEndPreservesWholeStay(t *testing.T) {
	t.Parallel()
	for _, completed := range []bool{false, true} {
		name := "open later visit"
		if completed {
			name = "later daily checkout"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			svc := setupVisitHelperService(t, db)
			student := testpkg.CreateTestStudent(t, db, "Earlier", "Room", "2a")
			staff := testpkg.CreateTestStaff(t, db, "Daily", "Recorder")
			device := testpkg.CreateTestDevice(t, db, "correction-device")
			activity := testpkg.CreateTestActivityGroup(t, db, "correction")
			room := testpkg.CreateTestRoom(t, db, "Correction")
			group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
			entry := calendar.NewDate(2026, 8, 24).BerlinMidnight().Add(8 * time.Hour)
			firstEnd := entry.Add(time.Hour)
			laterEntry := entry.Add(2 * time.Hour)
			laterEnd := entry.Add(3 * time.Hour)
			first := testpkg.CreateTestVisit(t, db, student.ID, group.ID, entry, &firstEnd)
			var checkout *time.Time
			if completed {
				checkout = &laterEnd
			}
			later := testpkg.CreateTestVisit(t, db, student.ID, group.ID, laterEntry, checkout)
			_, err := db.ExecContext(testpkg.Ctx(t), `INSERT INTO active.attendance
     (tenant_id, student_id, date, check_in_time, check_out_time, checked_in_by, device_id, checked_out_by, check_out_reason)
     VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, testpkg.Tenant(t), student.ID, calendar.DateFromTime(entry), entry, checkout, staff.ID, device.ID,
				func() *int64 {
					if completed {
						return &staff.ID
					}
					return nil
				}(),
				func() *string {
					if completed {
						v := studentpresence.CheckoutReasonClass
						return &v
					}
					return nil
				}())
			require.NoError(t, err)
			before := getAttendanceForStudent(t, db, student.ID, calendar.DateFromTime(entry))
			revisedEnd := firstEnd.Add(5 * time.Minute)
			first.ExitTime = &revisedEnd
			ctx := api.WithAttendanceStaff(testpkg.Ctx(t), staff.ID, staff.TenantID)
			require.NoError(t, svc.UpdateVisit(ctx, first))
			after := getAttendanceForStudent(t, db, student.ID, calendar.DateFromTime(entry))
			assert.Equal(t, before.CheckOutTime, after.CheckOutTime)
			assert.Equal(t, before.CheckedOutBy, after.CheckedOutBy)
			assert.Equal(t, before.CheckOutReason, after.CheckOutReason)
			assert.Equal(t, before.CheckInTime, after.CheckInTime)
			var storedExit *time.Time
			require.NoError(t, db.NewRaw(`SELECT exit_time FROM active.visits WHERE id = ?`, later.ID).Scan(testpkg.Ctx(t), &storedExit))
			if checkout == nil {
				assert.Nil(t, storedExit)
			} else {
				require.NotNil(t, storedExit)
				assert.True(t, checkout.Equal(*storedExit))
			}
		})
	}
}
