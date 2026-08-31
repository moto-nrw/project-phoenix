package students_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// identityStageCounter buckets the SELECTs that resolve the identity chain
// (Account → Person → Staff → Teacher → substitutions). The markers pair the
// table with the lookup column of the identity resolution so that business
// queries which merely JOIN the same tables (e.g. substitute names on the
// transfers projection) do not pollute the buckets.
type identityStageCounter struct {
	mu      sync.Mutex
	queries map[string][]string
}

// identityStageMatchers maps a stage to a predicate over the lowercased SQL.
// The substitution stage accepts both SQL shapes: the hand-written finder
// emits `.substitute_staff_id = ?`, the Filter-based one
// `"substitute_staff_id" = ?`; substitution reads filtered by group (the
// transfers projection) don't match either.
var identityStageMatchers = map[string]func(string) bool{
	"person": func(q string) bool {
		return strings.Contains(q, "users.persons") && strings.Contains(q, "account_id = ")
	},
	"staff": func(q string) bool {
		return strings.Contains(q, "users.staff") && strings.Contains(q, "person_id = ")
	},
	"teacher": func(q string) bool {
		return strings.Contains(q, "users.teachers") && strings.Contains(q, "staff_id = ")
	},
	"substitutions": func(q string) bool {
		hasTable := strings.Contains(q, "education.group_substitution") ||
			strings.Contains(q, `"education"."group_substitution"`)
		return hasTable &&
			(strings.Contains(q, "substitute_staff_id = ") || strings.Contains(q, `substitute_staff_id" = `))
	},
}

func (c *identityStageCounter) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	query := strings.ToLower(event.Query)
	if !strings.HasPrefix(strings.TrimSpace(query), "select") {
		return ctx
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for stage, matches := range identityStageMatchers {
		if matches(query) {
			c.queries[stage] = append(c.queries[stage], event.Query)
		}
	}
	return ctx
}

func (*identityStageCounter) AfterQuery(context.Context, *bun.QueryEvent) {}

func (c *identityStageCounter) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queries = make(map[string][]string)
}

func (c *identityStageCounter) captured(stage string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.queries[stage]...)
}

// TestOGSGroupLiveIdentityQueryBudget is the endpoint-level #2099 acceptance
// test: one request through the production Router() (including the
// RequestIdentityCacheMiddleware mounted by ProtectedTenantGroup) resolves
// each identity-chain stage at most once, even though the aggregate calls
// GetMyGroups, GetSubstitutedGroupIDs, and ResolveStudentAccess internally.
//
// Before #2099 the same request resolved the person+staff pair ~4 times.
// The existing TestOGSGroupLive_QueryBudget bounds the total; this test pins
// the identity share specifically.
// Deliberately NOT parallel: the test installs a query hook on the SHARED
// package pool and asserts a query budget, so any test running beside it is
// counted too.
func TestOGSGroupLiveIdentityQueryBudget(t *testing.T) {
	tc := setupStudentsRoute(t)

	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "IdentityBudget", "Leader")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "IdentityBudgetGroup")
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)

	// Substitution with an unassigned regular slot so both substitution
	// projections (GetMyGroups relations + GetSubstitutedGroupIDs) have a row.
	today := timezone.TodayDate()
	subGroup := testpkg.CreateTestEducationGroup(t, tc.db, "IdentityBudgetSubGroup")
	testpkg.CreateTestGroupSubstitution(t, tc.db, subGroup.ID, nil, teacher.StaffID,
		today.AddDays(-1), today.AddDays(3))

	for i := range 3 {
		student := testpkg.CreateTestStudent(t, tc.db, "IdentityBudget", fmt.Sprintf("Kind%d", i), "IB1")
		testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
	}

	counter := &identityStageCounter{queries: make(map[string][]string)}
	tc.db.AddQueryHook(counter)
	counter.reset()

	req := testutil.NewRequest("GET", fmt.Sprintf("/ogs-group-live?group_id=%d", group.ID), nil)
	rr := authExec(t, tc, req, testutil.TeacherTestClaims(int(account.ID)), ogsLivePerms)
	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())

	for stage, queries := range map[string][]string{
		"person":        counter.captured("person"),
		"staff":         counter.captured("staff"),
		"teacher":       counter.captured("teacher"),
		"substitutions": counter.captured("substitutions"),
	} {
		t.Logf("identity stage %q: %d queries", stage, len(queries))
		for _, q := range queries {
			t.Logf("  %s", q)
		}
	}

	assert.Len(t, counter.captured("person"), 1, "person stage must be resolved exactly once per request")
	assert.Len(t, counter.captured("staff"), 1, "staff stage must be resolved exactly once per request")
	assert.Len(t, counter.captured("teacher"), 1, "teacher stage must be resolved exactly once per request")
	// Two distinct substitution projections exist (GetMyGroups relations,
	// GetSubstitutedGroupIDs id-set); each memoizes its own result, so 2 is
	// the per-request floor.
	assert.Len(t, counter.captured("substitutions"), 2, "substitution lookups must run once per projection")
}
