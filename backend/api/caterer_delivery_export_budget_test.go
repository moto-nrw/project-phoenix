package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// checkCatererDeliveryExportQueryBudget pins the statements of the caterer's
// delivery list through the production router (ADR 0077, #3873, #2940). The
// list reads every school the caterer supplies in its own transaction, so it
// grows by a fixed number of statements per school, never with the children
// or the days beyond the week the window is bounded to, and it writes
// nothing and takes no order lock. A longer window is refused before any
// school is read. It runs serially: the counter sees the router's pool.
func checkCatererDeliveryExportQueryBudget(t *testing.T, api *goldenRuntime) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	account := testpkg.CreateTestAccount(t, db, "caterer-export-budget")
	var catererID int64
	require.NoError(t, db.NewRaw(`INSERT INTO auth.caterers (name) VALUES ('Budgetküche') RETURNING id`).Scan(ctx, &catererID))
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `DELETE FROM auth.caterers WHERE id = ?`, catererID)
		assert.NoError(t, err)
	})
	_, err := db.ExecContext(ctx, `INSERT INTO auth.caterer_accounts (account_id, caterer_id) VALUES (?, ?)`, account.ID, catererID)
	require.NoError(t, err)
	tokenAuth, err := newSessionTokenAuth()
	require.NoError(t, err)
	token, err := tokenAuth.CreateJWT(sessiontoken.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"caterer"}, Scope: tenant.ScopeCaterer})
	require.NoError(t, err)

	addSchool := func(children int) {
		t.Helper()
		school := testpkg.UniqueTestTenantID(t)
		testpkg.EnsureTestTenant(t, db, school)
		_, err := db.ExecContext(ctx, `
			INSERT INTO config.setting_values (tenant_id, setting_key, value)
			VALUES (?, 'operations.meal_plan_enabled', 'true'::jsonb), (?, 'operations.meal_registration_enabled', 'true'::jsonb)
			ON CONFLICT (tenant_id, setting_key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW()`, school, school)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `INSERT INTO auth.caterer_schools (caterer_id, tenant_id, delivery_address) VALUES (?, ?, 'Hof')`, catererID, school)
		require.NoError(t, err)
		for range children {
			testpkg.CreateTestStudentForTenant(t, db, school, "Budget", "Kind", "2a")
		}
	}

	// A week far ahead: every day is open and lies after the supply started.
	monday := calendar.NewDate(2030, 3, 4)
	week := fmt.Sprintf("/caterer/orders/export?from=%s&to=%s&format=csv", monday, monday.AddDays(4))
	counter := testpkg.CaptureQueries(t, api.db.Pool())
	export := func(path string) (int, []string) {
		t.Helper()
		counter.Reset()
		response := checkpointRequest(api, checkpointScenario{Method: http.MethodGet, Path: path, Authenticated: true}, token)
		return response.Code, counter.Queries()
	}

	addSchool(1)
	status, one := export(week)
	require.Equal(t, http.StatusOK, status)
	addSchool(3)
	_, two := export(week)
	addSchool(5)
	status, three := export(week)
	require.Equal(t, http.StatusOK, status)
	t.Logf("caterer delivery list: 1 school → %d statements, 2 → %d, 3 → %d", len(one), len(two), len(three))
	assert.Equal(t, len(two)-len(one), len(three)-len(two), "a fixed number of statements per school, flat in its children")
	assert.Empty(t, counter.Matching(func(sql string) bool {
		return strings.HasPrefix(strings.TrimSpace(sql), "insert") || strings.HasPrefix(strings.TrimSpace(sql), "update") || strings.HasPrefix(strings.TrimSpace(sql), "delete")
	}), "the caterer's list writes nothing")
	assert.Empty(t, counter.Matching(func(sql string) bool { return strings.Contains(sql, "pg_advisory") }), "and takes no lock")
	testpkg.AssertQueryBudget(t, "workflows.catererportal.delivery_export", three)

	status, refused := export(fmt.Sprintf("/caterer/orders/export?from=%s&to=%s&format=csv", monday, monday.AddDays(7)))
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Less(t, len(refused), len(one), "a window over a week is refused before any school is read")
}
