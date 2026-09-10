package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestRequestReviewRuntimeEvidence measures the staff request-review reads
// (#2705) through the production router: the open list, the history and the
// pending-count badge, each over the four parent-request queues. The same
// harness ran against the pre-cutover fan-out handler; the numbers are
// recorded in docs/runtime-checkpoints/request-review-2705.md. Fixtures are
// outside the timer.
func TestRequestReviewRuntimeEvidence(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	ctx := testpkg.Ctx(t)
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Runtime", "Reviewer")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "RuntimeReviewGroup")
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)
	claims := testutil.AdminTestClaims(int(account.ID))
	perms := []string{"users:read", "users:update"}

	var postgres string
	require.NoError(t, tc.db.NewRaw("SHOW server_version").Scan(ctx, &postgres))
	// Scoped to this test's requests: the package pool is shared with the
	// parallel tests, whose statements must not land in these counts.
	counter := testpkg.CaptureQueriesForContext(t, tc.db)
	deadlocks := func() int64 {
		var n int64
		require.NoError(t, tc.db.NewRaw("SELECT deadlocks FROM pg_stat_database WHERE datname = current_database()").Scan(ctx, &n))
		return n
	}

	decidedBase := time.Now().UTC().Add(-2 * time.Hour)
	seeded := 0
	seedChild := func(t *testing.T, n int) {
		t.Helper()
		student := testpkg.CreateTestStudent(t, tc.db, "Runtime", fmt.Sprintf("Child%03d", n), "3a")
		testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
		tenantID := student.TenantID

		masterData := &userModels.StudentDataChangeRequest{
			StudentID: student.ID, SubmittedBy: account.ID,
			Target: userModels.DataChangeTargetPerson, FieldKey: "first_name",
			NewValue: json.RawMessage(`"Neu"`), Status: userModels.DataChangeStatusPending,
		}
		masterData.TenantID = tenantID
		_, err := tc.db.NewInsert().Model(masterData).Exec(ctx)
		require.NoError(t, err)

		care := &scheduleModels.CareScheduleChangeRequest{
			StudentID: student.ID, SubmittedBy: account.ID, RequestKind: "weekly_schedule",
			Payload: map[string]any{"weekdays": []any{
				map[string]any{"weekday": 1, "arrival": "08:00", "pickup": "15:00"},
				map[string]any{"weekday": 3, "pickup": "16:00"},
			}},
			Status: "pending",
		}
		care.TenantID = tenantID
		_, err = tc.db.NewInsert().Model(care).Exec(ctx)
		require.NoError(t, err)

		excused := &activeModels.ExcusedAbsenceRequest{
			StudentID: student.ID, SubmittedBy: account.ID,
			Dates: []timezone.Date{timezone.TodayDate().AddDays(3)}, Note: "Arzttermin",
			AbsenceStatus: "excused", Status: "pending",
		}
		excused.TenantID = tenantID
		_, err = tc.db.NewInsert().Model(excused).Exec(ctx)
		require.NoError(t, err)

		fixture := setupCorrectionFixture(t, tc, student.ID, tenantID, fmt.Sprintf("Runtime%03d", n))
		insertPendingOfferingChangeRequest(t, tc, fixture, student.ID, account.ID)

		for i, status := range []string{userModels.DataChangeStatusApproved, userModels.DataChangeStatusRejected} {
			insertDecidedMasterDataRequest(t, tc, student.ID, tenantID, account.ID, status,
				decidedBase.Add(-time.Duration(n*10+i)*time.Minute))
		}
		reviewed := decidedBase.Add(-time.Duration(n*10+5) * time.Minute)
		decidedExcused := &activeModels.ExcusedAbsenceRequest{
			StudentID: student.ID, SubmittedBy: account.ID,
			Dates: []timezone.Date{timezone.TodayDate().AddDays(-3)}, Note: "Krank",
			AbsenceStatus: "sick", Status: "approved", ReviewedBy: &account.ID, ReviewedAt: &reviewed,
		}
		decidedExcused.TenantID = tenantID
		decidedExcused.CreatedAt = reviewed.Add(-time.Hour)
		decidedExcused.UpdatedAt = reviewed
		_, err = tc.db.NewInsert().Model(decidedExcused).Exec(ctx)
		require.NoError(t, err)
	}

	type request struct {
		name string
		path string
		want func(t *testing.T, body []byte, children int)
	}
	requests := []request{
		{"open", "/change-requests?view=open&search=Runtime&limit=25", func(t *testing.T, body []byte, children int) {
			var env aggListEnvelope
			require.NoError(t, json.Unmarshal(body, &env))
			require.Len(t, env.Data.Items, min(children*4, 25))
		}},
		{"history", "/change-requests?view=history&search=Runtime&limit=25", func(t *testing.T, body []byte, children int) {
			var env aggListEnvelope
			require.NoError(t, json.Unmarshal(body, &env))
			require.Len(t, env.Data.Items, min(children*3, 25))
		}},
		{"pending-count", "/change-requests/pending-count", func(t *testing.T, body []byte, _ int) {
			var env struct {
				Data struct {
					PendingCount int `json:"pending_count"`
				} `json:"data"`
			}
			require.NoError(t, json.Unmarshal(body, &env))
			require.GreaterOrEqual(t, env.Data.PendingCount, 0)
		}},
	}

	for _, scenario := range []struct {
		name     string
		children int
	}{{"empty", 0}, {"children-8", 8}, {"children-32", 32}} {
		t.Run(scenario.name, func(t *testing.T) {
			for seeded < scenario.children {
				seedChild(t, seeded)
				seeded++
			}
			for _, req := range requests {
				t.Run(req.name, func(t *testing.T) {
					beforeDeadlocks := deadlocks()
					stopLocks := testpkg.SampleCheckpointLocks(func(sampleCtx context.Context) (int, error) {
						var n int
						err := tc.db.NewRaw("SELECT count(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'").Scan(sampleCtx, &n)
						return n, err
					})
					var samples []testpkg.RuntimeCheckpointSample
					var durations []float64
					for i := range 35 {
						counter.Reset()
						before := tc.db.Stats()
						start := time.Now()
						httpReq := testutil.NewRequest("GET", req.path, nil)
						httpReq = httpReq.WithContext(counter.Context(httpReq.Context()))
						rr := authExec(t, tc, httpReq, claims, perms)
						elapsed := float64(time.Since(start)) / float64(time.Millisecond)
						after := tc.db.Stats()
						require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
						req.want(t, rr.Body.Bytes(), scenario.children)
						require.Zero(t, counter.WriteRows(), "the review reads must not persist state")
						if i >= 5 {
							affected, statements := counter.Rows()
							zero := counter.WriteRows()
							samples = append(samples, testpkg.RuntimeCheckpointSample{
								DurationMS: elapsed, Queries: counter.Total(), Status: rr.Code,
								RowsAffected: affected, StatementsWithRows: statements, WriteRowsAffected: &zero,
								PoolWaitCount: after.WaitCount - before.WaitCount,
								PoolWaitMS:    float64(after.WaitDuration-before.WaitDuration) / float64(time.Millisecond),
							})
							durations = append(durations, elapsed)
						}
					}
					locks := stopLocks()
					require.Empty(t, locks.Error)
					slices.Sort(durations)
					report, err := json.Marshal(map[string]any{
						"scenario": scenario.name, "request": req.name, "children": scenario.children,
						"samples": samples, "p50_ms": durations[14], "p95_ms": durations[28], "max_ms": durations[29],
						"queries": samples[len(samples)-1].Queries,
						"go":      runtime.Version(), "postgres": postgres, "warmup": 5, "concurrency": 1,
						"unexpected_errors": 0, "lock_samples": locks, "deadlocks": deadlocks() - beforeDeadlocks,
						"cache": "no projection cache",
					})
					require.NoError(t, err)
					t.Logf("request-review-runtime: %s", report)
				})
			}
		})
	}
}
