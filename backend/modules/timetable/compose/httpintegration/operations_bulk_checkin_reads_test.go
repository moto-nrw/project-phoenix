package httpintegration_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTimetableCheckInStudentsReadsCurrentVisitsOnce pins the shared reads of
// the batch check-in (#5815): every child's open visit is read once for the
// batch, while fresh children get a visit, children in another running
// session move over, a repeated ID resolves as before and the final roster
// lists every child present.
func TestTimetableCheckInStudentsReadsCurrentVisitsOnce(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	for _, size := range []int{3, 8} {
		t.Run(fmt.Sprintf("N%d", size), func(t *testing.T) {
			ctx := testpkg.OwnCtx(t)
			db := testpkg.SetupIsolatedTestDB(t)
			staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Batch", "Operator")
			ctx = testpkg.IdentityContext(ctx, account.ID, testpkg.Tenant(t), "tenant", []string{"schedules:read"})
			module, _ := api.NewActiveTestTimetable(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
			source := testpkg.CreateTestActiveGroup(t, db,
				testpkg.CreateTestActivityGroup(t, db, "Batch Source").ID,
				testpkg.CreateTestRoom(t, db, "Batch Source").ID)
			room := testpkg.CreateTestRoom(t, db, "Batch Target")
			activity := testpkg.CreateTestActivityGroup(t, db, "Batch Target")
			target := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
			instance := testpkg.CreateTestActivityInstance(t, db, testpkg.TodayDate(), room.ID, testpkg.ActivityInstanceOpts{
				Status: "active", ActivityGroupID: &activity.ID, ActiveGroupID: &target.ID, Title: "Batch Target",
			})
			testpkg.CreateTestInstanceStaff(t, db, instance.ID, staff.ID, testpkg.InstanceStaffOpts{IsPrimary: true})
			testpkg.EnsureWebManualDevice(t, db)
			device := testpkg.CreateTestDevice(t, db, fmt.Sprintf("batch-checkin-%d", size))
			now := time.Now().Add(-time.Minute)
			var request, children []int64
			for i := range size {
				fresh := testpkg.CreateTestStudent(t, db, "Fresh", fmt.Sprintf("Child%d", i), "3a")
				testpkg.CreateTestInstanceStudent(t, db, instance.ID, fresh.ID, "expected")
				moving := testpkg.CreateTestStudent(t, db, "Moving", fmt.Sprintf("Child%d", i), "3a")
				testpkg.CreateTestInstanceStudent(t, db, instance.ID, moving.ID, "expected")
				testpkg.CreateTestAttendance(t, db, moving.ID, staff.ID, device.ID, now, nil)
				testpkg.CreateTestVisit(t, db, moving.ID, source.ID, now, nil)
				request = append(request, fresh.ID, moving.ID)
				children = append(children, fresh.ID, moving.ID)
			}
			// A repeated child finds the visit its first check-in opened.
			request = append(request, children[0])

			counter := testpkg.CaptureQueries(t, db)
			require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
				counter.Reset()
				roster, err := module.TimetableOperations.CheckInStudents(txCtx, account.ID, false, instance.ID, request)
				counter.Stop()
				require.NoError(t, err)
				assert.Nil(t, roster.MovedFrom, "a batch names no single previous session")
				present := 0
				for _, row := range roster.Rows {
					if row.CurrentlyPresent {
						present++
					}
				}
				assert.Equal(t, len(children), present)
				return nil
			}))
			t.Logf("batch check-in N%d (%d children): %d statements", size, len(children), counter.Total())
			singleReads := counter.Matching(func(sql string) bool {
				return strings.HasPrefix(sql, `select "visit"."id"`) && strings.HasSuffix(strings.TrimSpace(sql), "order by visit.entry_time desc limit 1")
			})
			sharedReads := counter.Matching(func(sql string) bool {
				return strings.HasPrefix(sql, `select "visit"."id"`) && strings.HasSuffix(strings.TrimSpace(sql), "order by visit.entry_time desc") &&
					strings.Contains(sql, fmt.Sprintf("student_id in (%d, ", children[0]))
			})
			assert.Len(t, sharedReads, 1, "one open-visit read for the whole batch")
			assert.Len(t, singleReads, 1, "only the repeated child reads its open visit again")
			visits := api.NewStudentPresenceForTests(testpkg.NewDatabase(t, db))
			require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
				open, err := visits.ListVisits(txCtx, studentpresence.VisitFilter{StudentIDs: children, OpenOnly: true})
				require.NoError(t, err)
				inTarget := map[int64]bool{}
				for _, visit := range open {
					assert.Equal(t, target.ID, visit.ActiveGroupID, "child %d", visit.StudentID)
					inTarget[visit.StudentID] = true
				}
				assert.Len(t, inTarget, len(children), "every child has one open visit in the block")
				all, err := visits.ListVisits(txCtx, studentpresence.VisitFilter{StudentIDs: []int64{children[0]}})
				require.NoError(t, err)
				assert.Len(t, all, 1, "the repeated child keeps one visit")
				return nil
			}))
		})
	}
}
