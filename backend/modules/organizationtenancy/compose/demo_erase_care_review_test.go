package compose

import (
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
