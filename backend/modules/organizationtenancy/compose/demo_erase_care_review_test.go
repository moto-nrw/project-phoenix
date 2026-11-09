package compose

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// excludeVisitorContract leaves an open monthly run of the school's Träger
// with the school and an exclusion of the school's contract in its trail
// (#5916). The reason names no one: the row is personal only through the
// contract it names.
func excludeVisitorContract(t *testing.T, db *bun.DB, schoolID int64) int64 {
	t.Helper()
	var eventID int64
	require.NoError(t, db.NewRaw(`WITH contract AS (
			SELECT id, organization_id FROM care_contract.contracts WHERE school_id = ?0),
		run AS (
			INSERT INTO parent_billing.care_runs (organization_id, billing_month)
			SELECT organization_id, DATE '2026-10-01' FROM contract RETURNING id, organization_id),
		school AS (
			INSERT INTO parent_billing.care_run_schools (organization_id, run_id, school_id, billing_month)
			SELECT organization_id, id, ?0, DATE '2026-10-01' FROM run)
		INSERT INTO parent_billing.care_run_events (organization_id, run_id, action, details)
		SELECT run.organization_id, run.id, 'exclude', jsonb_build_object('contract_ids', jsonb_build_array(contract.id),
			'school_id', ?0, 'reason', 'Kündigung wird geprüft')
		FROM run JOIN contract ON contract.organization_id = run.organization_id
		RETURNING id`, schoolID).Scan(t.Context(), &eventID))
	return eventID
}

// An exclusion in the trail of a monthly run names the retired demo
// school's contract, but no name: it goes with the visitor all the same,
// before the contract it names. The exclusion of a school of the same
// Träger that is still open stays (#5916, rule of PR #6000).
func TestDemoSchoolQueueErasesTheExclusionsOfTheRetiredSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	openID := schoolOfOrganization(t, db, schoolID, false)
	accounts := leaveVisitorTraces(t, db, schoolID)
	leaveVisitorTraces(t, db, openID)
	retired := excludeVisitorContract(t, db, schoolID)
	open := excludeVisitorContract(t, db, openID)

	retireVisitorOrder(t, db, schoolID, accounts)

	assert.Zero(t, remainingRows(t, db, "parent_billing.care_run_events", []int64{retired}),
		"no exclusion naming the retired school's contract")
	assert.Equal(t, 1, remainingRows(t, db, "parent_billing.care_run_events", []int64{open}),
		"the open school keeps its exclusion")
}

// releaseExcludingBoth leaves a released run of the Träger whose release
// entry names the excluded invoices of both schools, by contract and by
// contract and service, as the release of #6346 writes it.
func releaseExcludingBoth(t *testing.T, db *bun.DB, retiredSchool, openSchool int64) (eventID, retiredContract, openContract int64) {
	t.Helper()
	ctx := t.Context()
	require.NoError(t, db.NewRaw(`SELECT id FROM care_contract.contracts WHERE school_id = ? ORDER BY id LIMIT 1`, retiredSchool).Scan(ctx, &retiredContract))
	require.NoError(t, db.NewRaw(`SELECT id FROM care_contract.contracts WHERE school_id = ? ORDER BY id LIMIT 1`, openSchool).Scan(ctx, &openContract))
	require.NoError(t, db.NewRaw(`WITH run AS (
			INSERT INTO parent_billing.care_runs (organization_id, billing_month)
			SELECT organization_id, DATE '2026-11-01' FROM care_contract.contracts WHERE id = ?0 RETURNING id, organization_id)
		INSERT INTO parent_billing.care_run_events (organization_id, run_id, action, details)
		SELECT run.organization_id, run.id, 'release', jsonb_build_object('document_count', 3,
			'excluded_contract_ids', jsonb_build_array(?0::bigint, ?1::bigint),
			'excluded_invoices', jsonb_build_array(
				jsonb_build_object('contract_id', ?0::bigint, 'service', 'care'),
				jsonb_build_object('contract_id', ?0::bigint, 'service', 'meal'),
				jsonb_build_object('contract_id', ?1::bigint, 'service', 'meal')))
		FROM run RETURNING id`, retiredContract, openContract).Scan(ctx, &eventID))
	return eventID, retiredContract, openContract
}

// The release entry of a monthly run names the invoices it left out by
// their contract (#6346). It belongs to the run, which also bills the open
// school of the Träger, so it stays; the retired demo school's contract
// leaves both lists, the open school's stays (rule 23, #6372).
func TestDemoSchoolQueueTakesTheRetiredContractsOutOfTheRelease(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	openID := schoolOfOrganization(t, db, schoolID, false)
	accounts := leaveVisitorTraces(t, db, schoolID)
	leaveVisitorTraces(t, db, openID)
	release, retired, open := releaseExcludingBoth(t, db, schoolID, openID)

	retireVisitorOrder(t, db, schoolID, accounts)

	var details struct {
		Contracts []int64          `json:"excluded_contract_ids"`
		Invoices  []map[string]any `json:"excluded_invoices"`
		Count     int              `json:"document_count"`
	}
	var raw []byte
	require.NoError(t, db.NewRaw(`SELECT details FROM parent_billing.care_run_events WHERE id = ?`, release).Scan(t.Context(), &raw))
	require.NoError(t, json.Unmarshal(raw, &details))
	assert.Equal(t, []int64{open}, details.Contracts, "only the open school's contract stays in excluded_contract_ids")
	assert.Equal(t, []map[string]any{{"contract_id": float64(open), "service": "meal"}}, details.Invoices,
		"only the open school's invoice stays in excluded_invoices")
	assert.Equal(t, 3, details.Count, "the rest of the release entry stays")
	assert.NotContains(t, details.Contracts, retired)
}
