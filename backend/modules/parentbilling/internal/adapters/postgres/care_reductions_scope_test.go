package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The rule chain's own filters keep a Träger to its rules, evidences and
// reductions below row level security (#4685): the Träger transaction drops
// its request role, so each assertion holds only because the adapter names
// the Träger in its statement. Removing one filter turns its assertion red.

type reductionScope struct {
	careScope
	holidayRule int64
	rule        int64
	evidence    int64
	invoice     int64
	care        int64
}

// seedReductionScope writes a contract, a holiday and a care rule, an
// evidence and an issued invoice with a reduction for the Träger in tx.
func seedReductionScope(t *testing.T, ctx context.Context, tx bun.IDB, scope careScope, name string) reductionScope {
	t.Helper()
	scope = seedCareScope(t, ctx, tx, scope, name)
	out := reductionScope{careScope: scope}
	var contract int64
	require.NoError(t, tx.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id)
		VALUES (?, ?, ?) RETURNING id`, scope.organization, scope.school, scope.contract).Scan(ctx, &contract))
	out.contract = contract
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.contribution_rules (organization_id, service, kind, valid_from,
		sibling_from_child, discount_basis_points) VALUES (?, 'holiday', 'sibling', DATE '2029-09-01', 2, 5000) RETURNING id`,
		scope.organization).Scan(ctx, &out.holidayRule))
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.contribution_rules (organization_id, service, kind, name, valid_from,
		school_ids, discount_basis_points) VALUES (?, 'care', 'income_level', ?, DATE '2029-09-01', ARRAY[?]::bigint[], 2000)
		RETURNING id`, scope.organization, "Stufe "+name, scope.school).Scan(ctx, &out.care))
	out.rule = out.care
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_evidences (organization_id, care_contract_id, kind, rule_id,
		valid_from, evidence, checked_on) VALUES (?, ?, 'income_level', ?, DATE '2029-09-01', 'Bescheid', DATE '2026-10-01')
		RETURNING id`, scope.organization, contract, out.care).Scan(ctx, &out.evidence))
	require.NoError(t, tx.NewRaw(`SELECT id FROM parent_billing.care_invoices WHERE organization_id = ?`, scope.organization).
		Scan(ctx, &out.invoice))
	_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.care_invoice_reductions (organization_id, invoice_id, position, kind,
		rule_id, name, amount_cents) VALUES (?, ?, 1, 'income_level', ?, 'Stufe', -640)`, scope.organization, out.invoice, out.care)
	require.NoError(t, err)
	return out
}

func TestCareReductionFiltersKeepTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ownBase := newCareScope(t, db, "Träger A")
	otherBase := newCareScope(t, db, "Träger B")
	store := NewStore()
	rollback := errors.New("roll the rows back")
	september := calendar.NewDate(2029, 9, 1)

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, ownBase.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, ownBase.organization)
		require.NoError(t, err)
		var bypasses bool
		require.NoError(t, tx.NewRaw(`SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(ctx, &bypasses))
		require.True(t, bypasses, "the test runs below row level security")
		own := seedReductionScope(t, ctx, tx, ownBase, "A")
		other := seedReductionScope(t, ctx, tx, otherBase, "B")

		rules, err := store.ListContributionRules(ctx, own.organization, parentbilling.ServiceCare)
		require.NoError(t, err)
		require.Len(t, rules, 1, "ListContributionRules of the Träger and the service")
		assert.Equal(t, own.care, rules[0].ID)
		assert.Equal(t, []int64{own.school}, rules[0].SchoolIDs)

		all, err := store.CareEvidences(ctx, own.organization, nil)
		require.NoError(t, err)
		require.Len(t, all, 1, "CareEvidences of the Träger")
		assert.Equal(t, own.evidence, all[0].ID)
		named, err := store.CareEvidences(ctx, own.organization, []int64{own.contract, other.contract})
		require.NoError(t, err)
		assert.Len(t, named, 1, "CareEvidences never answers another Träger's contract")

		input := parentbilling.CareEvidenceInput{ContractID: other.contract, Kind: parentbilling.EvidenceSingleParent,
			ValidFrom: september, Evidence: "Erklärung", CheckedOn: september}
		_, err = store.UpdateCareEvidence(ctx, own.organization, 1, other.evidence, input)
		assert.ErrorIs(t, err, parentbilling.ErrCareEvidenceNotFound, "UpdateCareEvidence of another Träger")
		_, err = store.DeleteCareEvidence(ctx, own.organization, other.evidence)
		assert.ErrorIs(t, err, parentbilling.ErrCareEvidenceNotFound, "DeleteCareEvidence of another Träger")

		care := parentbilling.CareRuleInput{Kind: parentbilling.RuleCap, Name: "Deckel", ValidFrom: september,
			SchoolIDs: []int64{own.school}, CapCents: new(int64)}
		assert.ErrorIs(t, store.UpdateCareRule(ctx, own.organization, other.care, care), parentbilling.ErrContributionRuleNotFound,
			"UpdateCareRule of another Träger")
		assert.ErrorIs(t, store.DeleteCareRule(ctx, own.organization, other.care), parentbilling.ErrContributionRuleNotFound,
			"DeleteCareRule of another Träger")
		assert.ErrorIs(t, store.UpdateCareRule(ctx, own.organization, own.holidayRule, care), parentbilling.ErrContributionRuleNotFound,
			"UpdateCareRule leaves a holiday rule")
		assert.ErrorIs(t, store.DeleteCareRule(ctx, own.organization, own.holidayRule), parentbilling.ErrContributionRuleNotFound,
			"DeleteCareRule leaves a holiday rule")
		assert.ErrorIs(t, store.DeleteContributionRule(ctx, own.organization, own.care), parentbilling.ErrContributionRuleNotFound,
			"the holiday deletion leaves a care rule")
		_, err = store.UpdateContributionRule(ctx, own.organization, own.care, parentbilling.ContributionRuleInput{
			Service: parentbilling.ServiceHoliday, Kind: parentbilling.RuleBenefitsExemption, ValidFrom: september, Note: "Befreit"})
		assert.ErrorIs(t, err, parentbilling.ErrContributionRuleNotFound, "the holiday change leaves a care rule")

		reductions, err := careReductionsOf(ctx, tx, own.organization, []int64{own.invoice, other.invoice})
		require.NoError(t, err)
		assert.Len(t, reductions, 1, "careReductionsOf of the Träger")
		assert.Contains(t, reductions, own.invoice)

		var kept int
		require.NoError(t, tx.NewRaw(`SELECT count(*) FROM parent_billing.care_evidences WHERE organization_id = ?`, other.organization).
			Scan(ctx, &kept))
		assert.Equal(t, 1, kept, "the other Träger keeps its evidence")
		require.NoError(t, tx.NewRaw(`SELECT count(*) FROM parent_billing.contribution_rules
			WHERE organization_id = ? AND service = 'care'`, other.organization).Scan(ctx, &kept))
		assert.Equal(t, 1, kept, "the other Träger keeps its rule")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
