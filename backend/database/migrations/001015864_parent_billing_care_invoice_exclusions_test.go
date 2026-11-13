package migrations

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// TestCareExclusionsOfBeforeHoldBackBothServices pins the meaning of an
// exclusion written before #6346: it named the contract only and held back
// its care invoice and its Mensa invoice of its own, so the migration writes
// both services into it and into its return. An entry of #6346, which names
// its one service, keeps it; other trail entries stay as they are. The down
// migration takes the services off again, and the up migration runs twice
// without a change. It runs in a transaction that rolls back, so the trail
// rows of the package's other tests stay as they are.
func TestCareExclusionsOfBeforeHoldBackBothServices(t *testing.T) {
	t.Parallel()
	pool := testpkg.SetupTestDB(t)
	ctx := context.Background()
	rollback := errors.New("roll the migration back")
	err := pool.RunInTx(ctx, nil, func(ctx context.Context, db bun.Tx) error {
		checkCareExclusionMigration(t, ctx, pool, db)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

func checkCareExclusionMigration(t *testing.T, ctx context.Context, pool *bun.DB, db bun.Tx) {
	t.Helper()
	organization := testpkg.CreateTestOrganization(t, pool, &testpkg.Organization{
		Name: "Träger Ausschluss Bestand", Slug: fmt.Sprintf("traeger-ausschluss-bestand-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	var run int64
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_runs (organization_id, billing_month) VALUES (?, DATE '2029-10-01')
		RETURNING id`, organization).Scan(ctx, &run))
	event := func(action, details string) int64 {
		t.Helper()
		var id int64
		require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_run_events (organization_id, run_id, action, details)
			VALUES (?, ?, ?, ?::jsonb) RETURNING id`, organization, run, action, details).Scan(ctx, &id))
		return id
	}
	legacyExclude := event("exclude", `{"contract_ids": [41], "school_id": 7, "reason": "Kündigung wird geprüft"}`)
	legacyInclude := event("include", `{"contract_ids": [41]}`)
	mealOnly := event("exclude", `{"contract_ids": [42], "services": ["meal"], "reason": "Essen wird geprüft"}`)
	release := event("release", `{"document_count": 1}`)
	services := func(id int64) string {
		t.Helper()
		var value string
		require.NoError(t, db.NewRaw(`SELECT COALESCE(details->>'services', '') FROM parent_billing.care_run_events WHERE id = ?`, id).
			Scan(ctx, &value))
		return value
	}
	reason := func(id int64) string {
		t.Helper()
		var value string
		require.NoError(t, db.NewRaw(`SELECT COALESCE(details->>'reason', '') FROM parent_billing.care_run_events WHERE id = ?`, id).
			Scan(ctx, &value))
		return value
	}

	for range 2 {
		require.NoError(t, mapCareExclusionsOntoServices(ctx, db))
		assert.Equal(t, `["care", "meal"]`, services(legacyExclude), "an exclusion of before holds back both invoices")
		assert.Equal(t, `["care", "meal"]`, services(legacyInclude), "its return took both back")
		assert.Equal(t, "Kündigung wird geprüft", reason(legacyExclude), "the entry keeps its reason")
		assert.Equal(t, `["meal"]`, services(mealOnly), "an exclusion of one invoice keeps its service")
		assert.Empty(t, services(release), "other trail entries stay as they are")
	}

	require.NoError(t, takeServicesOffCareExclusions(ctx, db))
	assert.Empty(t, services(legacyExclude), "the down migration takes the services off again")
	assert.Empty(t, services(mealOnly))
	require.NoError(t, mapCareExclusionsOntoServices(ctx, db))
}

// TestCareInvoiceExclusionErasureExtendsTheErasureOf843 keeps the erasure of
// 1.15.864 the erasure of 1.15.843 with the statement of #6346 and nothing
// else, before the exclusions of the school's contracts go.
func TestCareInvoiceExclusionErasureExtendsTheErasureOf843(t *testing.T) {
	t.Parallel()
	require.Equal(t, 1, strings.Count(demoVisitorErasureOutboxKindsSQL, careInvoiceExclusionErasureAnchor))
	assert.Equal(t, strings.Replace(demoVisitorErasureOutboxKindsSQL, careInvoiceExclusionErasureAnchor,
		careInvoiceExclusionErasureStatement+careInvoiceExclusionErasureAnchor, 1), demoVisitorErasureCareInvoiceExclusionsSQL)
}
