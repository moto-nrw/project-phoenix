package compose

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The cost bearer shares of the monthly contribution (#4687) keep rows that
// name no one but belong to a contract or an invoice of the school: the
// share of a contract, the trail of every read and change of it, and the
// share of an issued invoice. They go with their contract and their invoice
// by cascade, so the erasure of a retired demo school (1.15.805, 1.15.808)
// removes them without a statement of its own. The cost bearer is the
// Träger's and stays.

// leaveCareCostShares gives the school's contract a share of a cost bearer
// with a read in its trail, and its invoice an issued share. It answers the
// rows it wrote.
func leaveCareCostShares(t *testing.T, db *bun.DB, schoolID int64) (bearer, share, event, issued int64) {
	t.Helper()
	ctx := t.Context()
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.cost_bearers (organization_id, name)
		SELECT organization_id, 'Jugendamt ' || ?0 FROM care_contract.contracts WHERE school_id = ?0 LIMIT 1 RETURNING id`,
		schoolID).Scan(ctx, &bearer))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_cost_shares (organization_id, care_contract_id, kind,
			cost_bearer_id, share_basis_points, valid_from, valid_until, reference, checked_on)
		SELECT organization_id, id, 'youth_office', ?1, 10000, DATE '2026-09-01', DATE '2027-07-31', 'Bescheid 4-12', DATE '2026-10-01'
		FROM care_contract.contracts WHERE school_id = ?0 LIMIT 1 RETURNING id`, schoolID, bearer).Scan(ctx, &share))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_cost_share_events (organization_id, care_contract_id, share_id, action)
		SELECT organization_id, care_contract_id, id, 'read' FROM parent_billing.care_cost_shares WHERE id = ?0
		RETURNING id`, share).Scan(ctx, &event))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_invoice_cost_shares (organization_id, invoice_id, position, kind,
			cost_bearer_id, cost_bearer_name, share_basis_points, amount_cents)
		SELECT organization_id, id, 1, 'youth_office', ?1, 'Jugendamt', 10000, 640
		FROM parent_billing.care_invoices WHERE school_id = ?0 LIMIT 1 RETURNING id`, schoolID, bearer).Scan(ctx, &issued))
	return bearer, share, event, issued
}

func TestDemoSchoolQueueErasesTheCareCostSharesOfTheRetiredSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	openID := schoolOfOrganization(t, db, schoolID, false)
	accounts := leaveVisitorTraces(t, db, schoolID)
	leaveVisitorTraces(t, db, openID)
	bearer, share, event, issued := leaveCareCostShares(t, db, schoolID)
	openBearer, openShare, openEvent, openIssued := leaveCareCostShares(t, db, openID)

	retireVisitorOrder(t, db, schoolID, accounts)

	assert.Zero(t, remainingRows(t, db, "parent_billing.care_cost_shares", []int64{share}), "no share of the retired school's contract")
	assert.Zero(t, remainingRows(t, db, "parent_billing.care_cost_share_events", []int64{event}), "no read of that share")
	assert.Zero(t, remainingRows(t, db, "parent_billing.care_invoice_cost_shares", []int64{issued}), "no share of its invoice")
	assert.Equal(t, 1, remainingRows(t, db, "parent_billing.cost_bearers", []int64{bearer}), "the Träger keeps its cost bearer")
	assert.Equal(t, 1, remainingRows(t, db, "parent_billing.care_cost_shares", []int64{openShare}), "the open school keeps its share")
	assert.Equal(t, 1, remainingRows(t, db, "parent_billing.care_cost_share_events", []int64{openEvent}))
	assert.Equal(t, 1, remainingRows(t, db, "parent_billing.care_invoice_cost_shares", []int64{openIssued}))
	assert.Equal(t, 1, remainingRows(t, db, "parent_billing.cost_bearers", []int64{openBearer}))
}
