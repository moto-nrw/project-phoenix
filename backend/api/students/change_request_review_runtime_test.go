package students_test

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

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

	plans := make(map[string][]requestReviewPlan)
	for _, scenario := range []struct {
		name     string
		children int
		// explain captures the query plans of this scenario's requests.
		explain bool
	}{{"empty", 0, false}, {"children-8", 8, false}, {"children-32", 32, true}} {
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
					if scenario.explain {
						// The counter still holds the last measured request.
						plans[req.name] = explainRequestReviewShapes(t, tc.db, counter.Queries())
					}
				})
			}
		})
	}
	report, err := json.Marshal(plans)
	require.NoError(t, err)
	t.Logf("request-review-plans: %s", report)
}

// requestReviewPlan is the EXPLAIN (ANALYZE, BUFFERS) evidence for one
// statement shape of a measured request: how often the request issued it,
// what one execution cost and how many rows it scanned for the rows it
// returned. Rows scanned sums actual rows times loops over every scan node.
type requestReviewPlan struct {
	Shape            string          `json:"shape"`
	Executions       int             `json:"executions"`
	PlanningMS       float64         `json:"planning_ms"`
	ExecutionMS      float64         `json:"execution_ms"`
	RowsReturned     float64         `json:"rows_returned"`
	RowsScanned      float64         `json:"rows_scanned"`
	SharedHitBlocks  float64         `json:"shared_hit_blocks"`
	SharedReadBlocks float64         `json:"shared_read_blocks"`
	SeqScans         []string        `json:"seq_scans,omitempty"`
	Plan             json.RawMessage `json:"plan"`
}

var (
	planLiteral = regexp.MustCompile(`'(?:[^']|'')*'|\b\d+(?:\.\d+)?\b`)
	planList    = regexp.MustCompile(`\(\?(?:, \?)*\)`)
)

// explainRequestReviewShapes explains one exemplar of every distinct read
// shape under the least-privilege tenant role, most expensive first. Bun
// inlines the arguments, so the exemplar replays the request's own values;
// literals are folded only to group the executions of one shape.
func explainRequestReviewShapes(t *testing.T, db *bun.DB, statements []string) []requestReviewPlan {
	t.Helper()
	exemplars := make(map[string]string)
	executions := make(map[string]int)
	var order []string
	for _, statement := range statements {
		head := strings.ToUpper(strings.TrimSpace(statement))
		if (!strings.HasPrefix(head, "SELECT") && !strings.HasPrefix(head, "WITH")) || strings.Contains(statement, "set_config(") {
			continue
		}
		shape := planList.ReplaceAllString(planLiteral.ReplaceAllString(statement, "?"), "(?…)")
		if _, seen := exemplars[shape]; !seen {
			exemplars[shape] = statement
			order = append(order, shape)
		}
		executions[shape]++
	}
	plans := make([]requestReviewPlan, 0, len(order))
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
		var bypass bool
		require.NoError(t, tx.NewRaw("SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user").Scan(ctx, &bypass))
		require.False(t, bypass, "plans must run under the least-privilege role")
		for _, shape := range order {
			var raw string
			require.NoError(t, tx.NewRaw("EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) "+exemplars[shape]).Scan(ctx, &raw))
			plans = append(plans, summarizeRequestReviewPlan(t, shape, executions[shape], raw))
		}
		return nil
	}))
	slices.SortStableFunc(plans, func(a, b requestReviewPlan) int {
		return cmp.Compare(b.ExecutionMS*float64(b.Executions), a.ExecutionMS*float64(a.Executions))
	})
	return plans
}

type explainNode struct {
	NodeType         string        `json:"Node Type"`
	RelationName     string        `json:"Relation Name"`
	ActualRows       float64       `json:"Actual Rows"`
	ActualLoops      float64       `json:"Actual Loops"`
	SharedHitBlocks  float64       `json:"Shared Hit Blocks"`
	SharedReadBlocks float64       `json:"Shared Read Blocks"`
	Plans            []explainNode `json:"Plans"`
}

func summarizeRequestReviewPlan(t *testing.T, shape string, executions int, raw string) requestReviewPlan {
	t.Helper()
	var explained []struct {
		Plan          explainNode `json:"Plan"`
		PlanningTime  float64     `json:"Planning Time"`
		ExecutionTime float64     `json:"Execution Time"`
	}
	require.NoError(t, json.Unmarshal([]byte(raw), &explained))
	require.Len(t, explained, 1)
	root := explained[0]
	plan := requestReviewPlan{
		Shape: shape, Executions: executions,
		PlanningMS: root.PlanningTime, ExecutionMS: root.ExecutionTime,
		RowsReturned:    root.Plan.ActualRows * root.Plan.ActualLoops,
		SharedHitBlocks: root.Plan.SharedHitBlocks, SharedReadBlocks: root.Plan.SharedReadBlocks,
		Plan: json.RawMessage(raw),
	}
	var walk func(node explainNode)
	walk = func(node explainNode) {
		if strings.HasSuffix(node.NodeType, "Scan") && node.NodeType != "Subquery Scan" && node.NodeType != "CTE Scan" {
			plan.RowsScanned += node.ActualRows * node.ActualLoops
			if node.NodeType == "Seq Scan" {
				plan.SeqScans = append(plan.SeqScans, node.RelationName)
			}
		}
		for _, child := range node.Plans {
			walk(child)
		}
	}
	walk(root.Plan)
	return plan
}
