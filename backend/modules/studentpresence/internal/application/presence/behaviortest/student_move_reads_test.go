package behaviortest_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/studentpresence"
	activeService "github.com/moto-ogs/project-phoenix/modules/studentpresence/internal/application/presence"
	presenceevents "github.com/moto-ogs/project-phoenix/modules/studentpresence/internal/application/presence/behaviortest"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// moveCohort is one child of each kind a bulk move meets, in request order:
// present in another session, in transit, already in the target and absent.
type moveCohort struct {
	moving, transit, unchanged, absent []int64
	request                            []int64
}

// TestBulkMoveReadsGuardsOnce pins the per-child guard reads of the bulk
// moves (#5848): the target session and the locked children are checked once
// per move, not once per child, while the outcome lists, the rows and the
// session heartbeat stay as before.
func TestBulkMoveReadsGuardsOnce(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	for _, operation := range []string{"move", "assign"} {
		for _, size := range []int{3, 8} {
			t.Run(fmt.Sprintf("%s/N%d", operation, size), func(t *testing.T) {
				testpkg.OwnTenant(t)
				db := testpkg.SetupIsolatedTestDB(t)
				module := testSchoolPresence(t, db)
				svc, broadcaster := newServiceWithBroadcaster(t, db, module)
				activeService.ConfigureForTest(svc, activeService.WithTenantRuntime(testpkg.TenantRuntime(t, db)))
				ctx := testpkg.Ctx(t)
				staff := testpkg.CreateTestStaff(t, db, "Bulk", "Mover")
				device := testpkg.CreateTestDevice(t, db, fmt.Sprintf("bulk-move-%s-%d", operation, size))
				source := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
				target := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
				var cohort moveCohort
				for i := range size {
					child := func(kind string) int64 {
						student := testpkg.CreateTestStudent(t, db, "Bulk", fmt.Sprintf("%s%d", kind, i), "3a")
						cohort.request = append(cohort.request, student.ID)
						if kind != "absent" {
							testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, time.Now().Add(-time.Hour), nil)
						}
						return student.ID
					}
					id := child("moving")
					testpkg.CreateTestVisit(t, db, id, source.ID, time.Now().Add(-time.Hour), nil)
					cohort.moving = append(cohort.moving, id)
					cohort.transit = append(cohort.transit, child("transit"))
					id = child("unchanged")
					testpkg.CreateTestVisit(t, db, id, target.ID, time.Now().Add(-time.Hour), nil)
					cohort.unchanged = append(cohort.unchanged, id)
					cohort.absent = append(cohort.absent, child("absent"))
				}

				// The first moving and the first transit child belong to an
				// education group; their check-in events must name it.
				eduGroup := testpkg.CreateTestEducationGroup(t, db, fmt.Sprintf("Bulk-%s-%d", operation, size))
				grouped := map[string]bool{}
				for _, id := range []int64{cohort.moving[0], cohort.transit[0]} {
					assignStudentToEducationGroup(t, db, ctx, id, eduGroup.ID)
					grouped[strconv.FormatInt(id, 10)] = true
				}

				counter := testpkg.CaptureQueries(t, db)
				if operation == "move" {
					result, err := svc.MoveStudentsToActiveGroupAuthorized(ctx, cohort.request, target.ID, activeSvcBypassAuth)
					counter.Stop()
					require.NoError(t, err)
					assert.Equal(t, interleave(cohort.moving, cohort.transit), result.Moved)
					assert.Equal(t, cohort.unchanged, result.Unchanged)
					skipped := make([]int64, 0, len(result.Skipped))
					for _, s := range result.Skipped {
						assert.Equal(t, activeService.StudentMoveSkipNotPresent, s.Reason)
						skipped = append(skipped, s.StudentID)
					}
					assert.Equal(t, cohort.absent, skipped)
					for _, id := range cohort.moving {
						assert.Equal(t, source.ID, result.PreviousActiveGroupIDs[id])
					}
				} else {
					result, err := svc.AssignTransitStudentsToActiveGroup(ctx, cohort.request, target.ID)
					counter.Stop()
					require.NoError(t, err)
					assert.Equal(t, cohort.transit, result.Assigned)
					assert.Len(t, result.Skipped, 3*size)
				}
				t.Logf("%s N%d: %d statements", operation, size, counter.Total())

				open, err := module.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: cohort.request, OpenOnly: true})
				require.NoError(t, err)
				inTarget := 0
				for _, visit := range open {
					if visit.ActiveGroupID == target.ID {
						inTarget++
					}
				}
				want := 2 * size
				if operation == "move" {
					want = 3 * size
				}
				assert.Equal(t, want, inTarget, "every moved child and every unchanged child is in the target")

				checkins := broadcaster.EventsOfType(presenceevents.EventStudentCheckIn)
				require.NotEmpty(t, checkins)
				for _, event := range checkins {
					require.NotNil(t, event.Data.StudentID)
					if grouped[*event.Data.StudentID] {
						require.NotNil(t, event.Data.GroupIDs, "child %s", *event.Data.StudentID)
						assert.Equal(t, []string{strconv.FormatInt(eduGroup.ID, 10)}, *event.Data.GroupIDs)
					} else {
						assert.Nil(t, event.Data.GroupIDs, "child %s has no education group", *event.Data.StudentID)
					}
				}

				// The child reads left are the lock (move) or the existence
				// check (assign) and the checkout events of the moved
				// children; no visit reads its child or its group again.
				studentReads := counter.Matching(func(sql string) bool {
					return strings.HasPrefix(sql, `select "student".id,`)
				})
				targetReads := counter.Matching(func(sql string) bool {
					return strings.HasPrefix(sql, `select "id", "tenant_id", "created_at"`) && strings.Contains(sql, fmt.Sprintf("(id in (%d))", target.ID)) && !strings.Contains(sql, "for update")
				})
				if operation == "move" {
					assert.Len(t, studentReads, 1+len(cohort.moving), "one child lock plus one checkout event per moved child")
				} else {
					assert.Len(t, studentReads, len(cohort.transit)+1, "one existence check per assigned child plus one group read")
				}
				assert.Len(t, targetReads, 1, "the target session is checked once per move")
			})
		}
	}
}

func interleave(a, b []int64) []int64 {
	out := make([]int64, 0, len(a)+len(b))
	for i := range a {
		out = append(out, a[i], b[i])
	}
	return out
}
