package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The Jahresbescheide keep a Träger to its rows also below row level
// security (#6219): the Träger transaction drops its request role, so every
// assertion holds only because the adapter names the Träger in its own
// statement. Removing one filter turns its assertion red.

// noticeScope is a Träger with a released notice run and its notice for
// September 2029, and an open run.
type noticeScope struct {
	careScope
	noticeRun, openRun, notice int64
	noticeNumber               string
}

// noticeContract is the contract both Träger's notices name, so a missing
// Träger filter mixes their months.
const noticeContract = 4242

func seedNoticeScope(t *testing.T, ctx context.Context, tx bun.IDB, scope careScope, cents int64) noticeScope {
	t.Helper()
	s := noticeScope{careScope: scope, noticeNumber: scope.number + "-JB"}
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_notice_runs (organization_id, billing_year, period_from,
		period_until, school_ids, status, released_at, issued_on, notice_count, total_cents)
		VALUES (?, 2029, DATE '2029-09-01', DATE '2029-12-31', ARRAY[?]::bigint[], 'released', NOW(), DATE '2029-09-24', 1, ?)
		RETURNING id`, scope.organization, scope.school, cents).Scan(ctx, &s.noticeRun))
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_notice_runs (organization_id, billing_year, period_from,
		period_until, school_ids) VALUES (?, 2029, DATE '2029-10-01', DATE '2029-12-31', ARRAY[?]::bigint[]) RETURNING id`,
		scope.organization, scope.school).Scan(ctx, &s.openRun))
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_notices (organization_id, run_id, number, number_year,
		number_sequence, issued_on, billing_year, kind, care_contract_id, school_id, school_name, child_first_name,
		child_last_name, recipient_email, family_email, recipient, payment_method, period_from, period_until, lines,
		previous_cents, total_cents, delta_cents, legal_remedy)
		VALUES (?, ?, ?, 2029, 7, DATE '2029-09-24', 2029, 'initial', ?, ?, 'Schule', 'Lena', 'Muster', '', '',
		'{}'::jsonb, 'direct_debit', DATE '2029-09-01', DATE '2029-09-30', '[]'::jsonb, 0, ?, ?, 'Widerspruch binnen eines Monats.')
		RETURNING id`, scope.organization, s.noticeRun, s.noticeNumber, noticeContract, scope.school, cents, cents).Scan(ctx, &s.notice))
	_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.care_notice_months (organization_id, notice_id, care_contract_id,
		billing_month, due_on, immediately_due, previous_cents, amount_cents, snapshot)
		VALUES (?, ?, ?, DATE '2029-09-01', DATE '2029-09-24', true, 0, ?,
		jsonb_build_object('invoice', jsonb_build_object('positions', jsonb_build_array(jsonb_build_object('care_offering_id', 101,
			'amount_cents', ?::bigint)), 'cost_shares', jsonb_build_array(jsonb_build_object('kind', 'other_subsidy',
			'cost_bearer_id', 77, 'cost_bearer_name', 'Gemeinde', 'share_cents', 2000, 'scope', 'both', 'amount_cents', 2000))),
			'payable', jsonb_build_array(?::bigint)))`,
		scope.organization, s.notice, noticeContract, cents, cents, cents)
	require.NoError(t, err)
	return s
}

func TestCareNoticeFiltersKeepTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ownScope := newCareScope(t, db, "Träger Bescheid A")
	otherScope := newCareScope(t, db, "Träger Bescheid B")
	store := NewStore()
	rollback := errors.New("roll the notice rows back")

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, ownScope.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, ownScope.organization)
		require.NoError(t, err)
		var bypasses bool
		require.NoError(t, tx.NewRaw(`SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(ctx, &bypasses))
		require.True(t, bypasses, "the test runs below row level security")
		own := seedNoticeScope(t, ctx, tx, seedCareScope(t, ctx, tx, ownScope, "Träger Bescheid A"), 3200)
		other := seedNoticeScope(t, ctx, tx, seedCareScope(t, ctx, tx, otherScope, "Träger Bescheid B"), 9900)

		runs, err := store.ListCareNoticeRuns(ctx, own.organization)
		require.NoError(t, err)
		require.Len(t, runs, 2, "ListCareNoticeRuns")
		for _, run := range runs {
			assert.Contains(t, []int64{own.noticeRun, own.openRun}, run.ID)
		}

		_, err = store.CareNoticeRun(ctx, own.organization, other.noticeRun, false)
		assert.ErrorIs(t, err, parentbilling.ErrCareNoticeRunNotFound, "CareNoticeRun")
		_, err = store.CareNoticeRun(ctx, own.organization, other.openRun, true)
		assert.ErrorIs(t, err, parentbilling.ErrCareNoticeRunNotFound, "CareNoticeRun for update")

		effective, err := store.EffectiveCareNotices(ctx, own.organization, 2029)
		require.NoError(t, err)
		require.Len(t, effective, 1, "EffectiveCareNotices")
		assert.Equal(t, own.notice, effective[0].ID)
		require.Len(t, effective[0].Installments, 1, "withNoticeMonths")
		assert.Equal(t, int64(3200), effective[0].Installments[0].AmountCents)

		notices, err := store.CareNoticeRunNotices(ctx, own.organization, other.noticeRun)
		require.NoError(t, err)
		assert.Empty(t, notices, "CareNoticeRunNotices")

		invoiced, err := store.InvoicedContracts(ctx, own.organization, calendar.NewDate(2029, 9, 1))
		require.NoError(t, err)
		require.Contains(t, invoiced, int64(noticeContract), "InvoicedContracts names the noticed month")
		assert.Equal(t, int64(3200), invoiced[noticeContract].TotalCents, "InvoicedContracts reads the Träger's own notice months")
		assert.Equal(t, own.noticeNumber, invoiced[noticeContract].Number)
		require.Len(t, invoiced[noticeContract].Positions, 1)
		assert.Equal(t, int64(3200), invoiced[noticeContract].Positions[0].PayableCents)

		// The parts a cost bearer already took (ADR 0169): the notice month
		// carries its 20,00 € fixed share, the monthly invoice its 12,00 €.
		require.Len(t, invoiced[noticeContract].CostShares, 1, "InvoicedContracts reads the parts of a notice month")
		assert.Equal(t, int64(2000), invoiced[noticeContract].CostShares[0].AmountCents)
		for _, scope := range []noticeScope{own, other} {
			_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.care_invoice_cost_shares (organization_id, invoice_id, position,
				kind, cost_bearer_id, cost_bearer_name, share_cents, scope, amount_cents)
				SELECT organization_id, id, 1, 'other_subsidy', 77, 'Gemeinde', 2000, 'both', ? FROM parent_billing.care_invoices
				WHERE organization_id = ? AND care_contract_id = ?`, map[bool]int64{true: 1200, false: 1900}[scope.organization == own.organization],
				scope.organization, scope.contract)
			require.NoError(t, err)
		}
		monthly, err := store.InvoicedContracts(ctx, own.organization, calendar.NewDate(2029, 9, 1))
		require.NoError(t, err)
		require.Len(t, monthly[own.contract].CostShares, 1, "InvoicedContracts reads the parts of a monthly invoice")
		assert.Equal(t, int64(1200), monthly[own.contract].CostShares[0].AmountCents, "only the Träger's own invoice")

		assert.NoError(t, claimInvoiceNumbers(ctx, tx, own.organization, []string{other.noticeNumber}, careNumbersTable),
			"a number of another Träger's notice is free")
		assert.ErrorIs(t, claimInvoiceNumbers(ctx, tx, own.organization, []string{own.noticeNumber}, careNumbersTable),
			parentbilling.ErrInvoiceNumberTaken, "a monthly invoice never repeats a notice number")

		// The SEPA collection reads the rate of the effective notice (#4697):
		// the Träger's own, and after a change the change notice's.
		september := calendar.NewDate(2029, 9, 1)
		dues, err := store.EffectiveNoticeDues(ctx, own.organization, september)
		require.NoError(t, err)
		require.Len(t, dues, 1, "EffectiveNoticeDues")
		assert.Equal(t, own.notice, dues[0].NoticeID)
		assert.Equal(t, int64(3200), dues[0].AmountCents)
		var change int64
		require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_notices (organization_id, run_id, number, number_year,
			number_sequence, issued_on, billing_year, kind, amends_notice_id, care_contract_id, school_id, school_name,
			child_first_name, child_last_name, recipient_email, family_email, recipient, payment_method, period_from,
			period_until, lines, previous_cents, total_cents, delta_cents, legal_remedy)
			SELECT organization_id, run_id, number || '-A', number_year, number_sequence + 1, DATE '2029-10-01', billing_year,
				'change', id, care_contract_id, school_id, school_name, child_first_name, child_last_name, recipient_email,
				family_email, recipient, payment_method, period_from, period_until, lines, total_cents, 0, -total_cents, legal_remedy
			FROM parent_billing.care_notices WHERE id = ? RETURNING id`, own.notice).Scan(ctx, &change))
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.care_notice_months (organization_id, notice_id, care_contract_id,
			billing_month, due_on, immediately_due, previous_cents, amount_cents, snapshot)
			VALUES (?, ?, ?, DATE '2029-09-01', DATE '2029-09-24', false, 3200, 0, '{"invoice": {}}'::jsonb)`,
			own.organization, change, noticeContract)
		require.NoError(t, err)
		dues, err = store.EffectiveNoticeDues(ctx, own.organization, september)
		require.NoError(t, err)
		require.Len(t, dues, 1, "one entry per contract: the change replaces the notice it amends")
		assert.Equal(t, change, dues[0].NoticeID)
		assert.Zero(t, dues[0].AmountCents)

		_, err = store.MarkCareNoticeRunReleased(ctx, own.organization, other.openRun, calendar.NewDate(2029, 10, 1), 0, 0)
		assert.ErrorIs(t, err, parentbilling.ErrBillingRunReleased, "MarkCareNoticeRunReleased")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
