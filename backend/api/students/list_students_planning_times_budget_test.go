package students_test

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// planningTimesTables are the six tables behind the two bulk effective-time
// loads (GetBulkEffectiveArrivalTimesForDate / GetBulkEffectivePickupTimesForDate,
// three SELECTs each: exceptions, schedules, notes). Before #2098 a list
// request with both include flags hit every one of them twice: once in
// enrichWithDayPlanning and again in the paginated arrival/pickup enrichment.
var planningTimesTables = []string{
	"student_arrival_exceptions",
	"student_arrival_schedules",
	"student_arrival_notes",
	"student_pickup_exceptions",
	"student_pickup_schedules",
	"student_pickup_notes",
}

// planningTimesCounter buckets SELECTs by the schedule table they read.
type planningTimesCounter struct {
	mu      sync.Mutex
	queries map[string][]string
}

func (c *planningTimesCounter) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	query := strings.ToLower(event.Query)
	if !strings.HasPrefix(strings.TrimSpace(query), "select") {
		return ctx
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, table := range planningTimesTables {
		if strings.Contains(query, table) {
			c.queries[table] = append(c.queries[table], event.Query)
		}
	}
	return ctx
}

func (*planningTimesCounter) AfterQuery(context.Context, *bun.QueryEvent) {}

func (c *planningTimesCounter) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queries = make(map[string][]string)
}

func (c *planningTimesCounter) captured(table string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.queries[table]...)
}

// TestListStudentsPlanningTimesQueryBudget is the #2098 acceptance test: one
// list request with include_pickup_times and include_arrival_times runs each
// bulk effective-time SELECT exactly once. Before the fix the paginated
// enrichment stage re-ran the same three SELECTs per kind that
// enrichWithDayPlanning had already issued, so every bucket held 2.
func TestListStudentsPlanningTimesQueryBudget(t *testing.T) {
	tc := setupTestContext(t)
	tc.resource.Now = func() time.Time {
		return time.Date(2026, time.June, 1, 10, 0, 0, 0, time.UTC)
	}

	studentA := testpkg.CreateTestStudent(t, tc.db, "TimesBudget", "KindA", "TB1")
	studentB := testpkg.CreateTestStudent(t, tc.db, "TimesBudget", "KindB", "TB1")
	defer testpkg.CleanupActivityFixtures(t, tc.db, studentA.ID, studentB.ID)

	counter := &planningTimesCounter{queries: make(map[string][]string)}
	tc.db.AddQueryHook(counter)
	counter.reset()

	req := testutil.NewRequest("GET", "/?include_pickup_times=true&include_arrival_times=true&page_size=50", nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

	for _, table := range planningTimesTables {
		queries := counter.captured(table)
		t.Logf("table %q: %d queries", table, len(queries))
		for _, q := range queries {
			t.Logf("  %s", q)
		}
		assert.Len(t, queries, 1, "bulk load of %s must run exactly once per list request (#2098)", table)
	}
}
