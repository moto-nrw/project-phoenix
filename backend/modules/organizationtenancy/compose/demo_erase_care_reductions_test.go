package compose

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The rule chain of the monthly contribution (#4685) keeps rows that name
// no one but belong to a contract or an invoice of the school: the evidence
// of a contract with its income level, the trail of every read and change
// of it, and the reduction of an issued invoice. They go with their contract
// and their invoice by cascade, so the erasure of a retired demo school
// (1.15.805, 1.15.808) removes them without a statement of its own. The
// income level is the Träger's rule and names no person: it stays.

// leaveCareReductions gives the school's contract an income level with its
// evidence and a read in its trail, and its invoice a reduction. It answers
// the rows it wrote.
func leaveCareReductions(t *testing.T, db *bun.DB, schoolID int64) (rule, evidence, event, reduction int64) {
	t.Helper()
	ctx := t.Context()
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.contribution_rules (organization_id, service, kind, name, valid_from,
			school_ids, discount_basis_points)
		SELECT organization_id, 'care', 'income_level', 'Stufe ' || ?0, DATE '2026-09-01', ARRAY[?0]::bigint[], 2000
		FROM care_contract.contracts WHERE school_id = ?0 LIMIT 1 RETURNING id`, schoolID).Scan(ctx, &rule))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_evidences (organization_id, care_contract_id, kind, rule_id,
			valid_from, evidence, checked_on)
		SELECT organization_id, id, 'income_level', ?1, DATE '2026-09-01', 'Einkommensteuerbescheid 2025', DATE '2026-10-01'
		FROM care_contract.contracts WHERE school_id = ?0 LIMIT 1 RETURNING id`, schoolID, rule).Scan(ctx, &evidence))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_evidence_events (organization_id, care_contract_id, evidence_id, action)
		SELECT organization_id, care_contract_id, id, 'read' FROM parent_billing.care_evidences WHERE id = ?0
		RETURNING id`, evidence).Scan(ctx, &event))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_invoice_reductions (organization_id, invoice_id, position, kind,
			rule_id, name, amount_cents)
		SELECT organization_id, id, 1, 'income_level', ?1, 'Stufe', -640
		FROM parent_billing.care_invoices WHERE school_id = ?0 LIMIT 1 RETURNING id`, schoolID, rule).Scan(ctx, &reduction))
	return rule, evidence, event, reduction
}

func TestDemoSchoolQueueErasesTheCareReductionsOfTheRetiredSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID, _ := testpkg.CreateTestTenant(t, db)
	openID := schoolOfOrganization(t, db, schoolID, false)
	accounts := leaveVisitorTraces(t, db, schoolID)
	leaveVisitorTraces(t, db, openID)
	rule, evidence, event, reduction := leaveCareReductions(t, db, schoolID)
	openRule, openEvidence, openEvent, openReduction := leaveCareReductions(t, db, openID)

	retireVisitorOrder(t, db, schoolID, accounts)

	assert.Zero(t, remainingRows(t, db, "parent_billing.care_evidences", []int64{evidence}), "no evidence of the retired school's contract")
	assert.Zero(t, remainingRows(t, db, "parent_billing.care_evidence_events", []int64{event}), "no read of that evidence")
	assert.Zero(t, remainingRows(t, db, "parent_billing.care_invoice_reductions", []int64{reduction}), "no reduction of its invoice")
	assert.Equal(t, 1, remainingRows(t, db, "parent_billing.contribution_rules", []int64{rule}), "the Träger keeps its income level")
	assert.Equal(t, 1, remainingRows(t, db, "parent_billing.care_evidences", []int64{openEvidence}), "the open school keeps its evidence")
	assert.Equal(t, 1, remainingRows(t, db, "parent_billing.care_evidence_events", []int64{openEvent}))
	assert.Equal(t, 1, remainingRows(t, db, "parent_billing.care_invoice_reductions", []int64{openReduction}))
	assert.Equal(t, 1, remainingRows(t, db, "parent_billing.contribution_rules", []int64{openRule}))
}
