package checkin_test

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Wait for an observed PostgreSQL lock wait, not an assumed goroutine order.
func awaitPresenceWait(t *testing.T, ctx context.Context, db *bun.DB, holder int) {
	t.Helper()
	require.Eventually(t, func() bool {
		var waiting bool
		err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity
   WHERE ?::int = ANY(pg_blocking_pids(pid)))`, holder).Scan(ctx, &waiting)
		require.NoError(t, err)
		return waiting
	}, 5*time.Second, 10*time.Millisecond)
}

func TestDeviceCheckinLocksStudentBeforeSessions(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"checkin", "batch checkout", "staff move"} {
		t.Run(operation, func(t *testing.T) {
			testpkg.OwnTenant(t)
			k := setupCheckinRoute(t)
			ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 15*time.Second)
			defer cancel()
			presence, err := api.NewTestActivePresence(testpkg.NewDatabase(t, k.db), slog.Default())
			require.NoError(t, err)
			device, deviceID := k.deviceRow(t, "student-lock-order")
			staff, staffID, _ := k.staff(t, "Lock", "Order")
			tag, studentID, _ := k.studentCard(t, "Lock", "Student", "1a")
			roomID, _, groupID := k.roomWithSession(t, "Lock order source")
			targetRoomID, _, targetID := k.roomWithSession(t, "Lock order target")
			scanRoomID := roomID
			if operation != "checkin" {
				scanRoomID = targetRoomID
			}
			testpkg.LinkDeviceToActiveGroup(t, k.db, groupID, deviceID)
			if operation != "checkin" {
				testpkg.CreateTestAttendance(t, k.db, studentID, staffID, deviceID, time.Now().Add(-time.Minute), nil)
				testpkg.CreateTestVisit(t, k.db, studentID, groupID, time.Now().Add(-time.Minute), nil)
			}
			response := make(chan *httptest.ResponseRecorder, 1)
			err = testpkg.WithTenantTx(t, ctx, k.db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
				var pid int
				if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(txCtx, &pid); err != nil {
					return err
				}
				var locked int64
				if err := tx.NewRaw("SELECT id FROM users.student_profiles WHERE id = ? AND tenant_id = ? FOR UPDATE", studentID, testpkg.Tenant(t)).Scan(txCtx, &locked); err != nil {
					return err
				}
				go func() { response <- k.call(t, "POST", "/checkin", checkinBody(tag, scanRoomID), device, staff) }()
				awaitPresenceWait(t, ctx, k.db, pid)
				// These production callers already own the student before taking a
				// session. A waiting scan must not hold that session in return.
				txCtx = api.WithAttendanceDevice(api.WithAttendanceStaff(txCtx, staffID, testpkg.Tenant(t)), deviceID, testpkg.Tenant(t))
				switch operation {
				case "checkin":
					return presence.CreateVisit(txCtx, &studentpresence.Visit{StudentID: studentID, ActiveGroupID: groupID, EntryTime: time.Now()})
				case "batch checkout":
					result, err := presence.ProcessSchoolCheckinBatch(txCtx, []int64{studentID}, staffID, "out")
					if err == nil {
						require.Equal(t, 1, result.Succeeded)
						require.True(t, result.Results[0].Changed)
					}
					return err
				default:
					result, err := presence.MoveStudentsToActiveGroupAuthorized(txCtx, []int64{studentID}, targetID, studentpresence.StudentMoveAuthorization{BypassResourceChecks: true})
					if err == nil {
						require.Equal(t, []int64{studentID}, result.Moved)
					}
					return err
				}
			})
			select {
			case rr := <-response:
				require.NoError(t, err)
				testutil.AssertSuccessResponse(t, rr, 200)
			case <-ctx.Done():
				t.Fatal("scan did not finish after the presence writer committed")
			}
		})
	}
}
