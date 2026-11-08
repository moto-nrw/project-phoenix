package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The creditor, the SEPA mandates and their trail keep a Träger to its rows
// also below row level security (#4695): the Träger transaction drops its
// request role, so every assertion holds only because the adapter names the
// Träger in its own statement. Removing one filter turns its assertion red.

type mandateScope struct {
	organization int64
	family       string
	mandate      int64
}

func newMandateScope(t *testing.T, db *bun.DB, name string) mandateScope {
	t.Helper()
	unique := testpkg.UniqueTestTenantID(t)
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name, Slug: fmt.Sprintf("mandate-scope-%d", unique)}).ID
	return mandateScope{organization: organization, family: fmt.Sprintf("familie.%d@example.org", unique)}
}

// seedMandateScope writes the creditor, a mandate in force and one entry
// of its trail. The mandate hangs on a contract; its key plays no part here.
func seedMandateScope(t *testing.T, ctx context.Context, tx bun.IDB, scope mandateScope, creditorID, reference string) mandateScope {
	t.Helper()
	_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.sepa_creditors (organization_id, creditor_id) VALUES (?, ?)`,
		scope.organization, creditorID)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = replica`)
	require.NoError(t, err)
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.sepa_mandates (organization_id, family_email, care_contract_id, reference,
		signed_on, kind, sequence, creditor_id, account_holder_sealed, iban_sealed, iban_masked, source)
		VALUES (?, ?, 1, ?, DATE '2026-09-01', 'recurring', 'first', 'DE98ZZZ09999999999', '\x02'::bytea, '\x01'::bytea, 'DE•• •••• •••• •••• ••30 00', 'office')
		RETURNING id`, scope.organization, scope.family, reference).Scan(ctx, &scope.mandate))
	_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.sepa_mandate_events (organization_id, mandate_id, care_contract_id, action, source)
		VALUES (?, ?, 1, 'create', 'office')`, scope.organization, scope.mandate)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = origin`)
	require.NoError(t, err)
	return scope
}

func TestSEPAMandateFiltersKeepTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newMandateScope(t, db, "Träger A")
	other := newMandateScope(t, db, "Träger B")
	store := NewStore()
	rollback := errors.New("roll the mandates back")

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own.organization)
		require.NoError(t, err)
		var bypasses bool
		require.NoError(t, tx.NewRaw(`SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(ctx, &bypasses))
		require.True(t, bypasses, "the test runs below row level security")
		own = seedMandateScope(t, ctx, tx, own, "DE98ZZZ09999999999", "SJR-A-1")
		other = seedMandateScope(t, ctx, tx, other, "DE98ABC09999999999", "SJR-B-1")

		creditor, err := store.SEPACreditor(ctx, own.organization)
		require.NoError(t, err)
		require.NotNil(t, creditor, "SEPACreditor")
		assert.Equal(t, "DE98ZZZ09999999999", creditor.CreditorID)

		mandates, err := store.SEPAMandates(ctx, own.organization, "")
		require.NoError(t, err)
		require.Len(t, mandates, 1, "SEPAMandates")
		assert.Equal(t, own.mandate, mandates[0].ID)
		mandates, err = store.SEPAMandates(ctx, own.organization, other.family)
		require.NoError(t, err)
		assert.Empty(t, mandates, "SEPAMandates of another Träger's family")

		_, err = store.SEPAMandate(ctx, own.organization, other.mandate, true)
		require.ErrorIs(t, err, parentbilling.ErrSEPAMandateNotFound, "SEPAMandate of another Träger")
		_, err = store.UpdateSEPAMandate(ctx, own.organization, 0, parentbilling.SealedSEPAMandate{
			Mandate: parentbilling.SEPAMandate{ID: other.mandate, AccountHolder: "Fremd", Sequence: parentbilling.SequenceFirst},
			IBAN:    []byte{2}, Holder: []byte{3}})
		require.ErrorIs(t, err, parentbilling.ErrSEPAMandateNotFound, "UpdateSEPAMandate of another Träger")
		_, err = store.RevokeSEPAMandate(ctx, own.organization, 0, other.mandate, calendar.NewDate(2026, 10, 1))
		require.ErrorIs(t, err, parentbilling.ErrSEPAMandateNotFound, "RevokeSEPAMandate of another Träger")

		events, err := store.SEPAMandateEvents(ctx, own.organization, other.mandate)
		require.NoError(t, err)
		assert.Empty(t, events, "SEPAMandateEvents of another Träger's mandate")
		sealed, err := store.SealedSEPAMandates(ctx, own.organization, []int64{own.mandate, other.mandate})
		require.NoError(t, err)
		require.Len(t, sealed, 1, "SealedSEPAMandates opens only the Träger's own")
		assert.Equal(t, own.mandate, sealed[0].Mandate.ID)

		debitCreditor, inForce, err := store.InvoiceDebitTerms(ctx, own.organization)
		require.NoError(t, err)
		require.NotNil(t, debitCreditor, "InvoiceDebitTerms: creditor")
		assert.Equal(t, "DE98ZZZ09999999999", debitCreditor.CreditorID)
		require.Len(t, inForce, 1, "InvoiceDebitTerms: mandates")
		assert.Equal(t, own.family, inForce[0].FamilyEmail)
		assert.Equal(t, calendar.NewDate(2026, 9, 1), inForce[0].SignedOn)

		require.NoError(t, store.SaveSEPACreditor(ctx, own.organization, 0, parentbilling.SEPACreditor{CreditorID: "",
			PrenotificationDays: 5, MissingMandate: parentbilling.MissingMandateBlock}))
		var kept string
		require.NoError(t, tx.NewRaw(`SELECT encode(holder.account_holder_sealed, 'hex') || '|' || creditor.creditor_id
			FROM parent_billing.sepa_mandates AS holder JOIN parent_billing.sepa_creditors AS creditor USING (organization_id)
			WHERE holder.id = ?`, other.mandate).Scan(ctx, &kept))
		assert.Equal(t, "02|DE98ABC09999999999", kept, "the other Träger keeps its mandate and creditor")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// A revoked mandate is history: the database refuses any change of it, and
// of the reference of one in force, also below row level security.
func TestSEPAMandateHistoryNeverChanges(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newMandateScope(t, db, "Träger Historie")
	creator := testpkg.CreateTestAccount(t, db, "sepa-creator@example.org")
	revoker := testpkg.CreateTestAccount(t, db, "sepa-revoker@example.org")
	store := NewStore()
	rollback := errors.New("roll the mandate back")

	school := testpkg.CreateTestSchoolInOrganization(t, db, own.organization, "OGS Historie")
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own.organization)
		require.NoError(t, err)
		// A real contract: a change of the mandate checks its contract
		// again in the transaction that inserted it.
		var contract int64
		require.NoError(t, tx.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id, status, sent_at)
			VALUES (?, ?, ?, 'versendet', NOW()) RETURNING id`, own.organization, school, testpkg.UniqueTestTenantID(t)).Scan(ctx, &contract))
		require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.sepa_mandates (organization_id, family_email, care_contract_id, reference,
			signed_on, kind, sequence, creditor_id, account_holder_sealed, iban_sealed, iban_masked, source)
			VALUES (?, ?, ?, 'SJR-H-1', DATE '2026-09-01', 'recurring', 'first', 'DE98ZZZ09999999999', '\x02'::bytea, '\x01'::bytea, 'DE•• 30 00', 'office')
			RETURNING id`, own.organization, own.family, contract).Scan(ctx, &own.mandate))

		_, err = tx.ExecContext(ctx, `SAVEPOINT reference`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `UPDATE parent_billing.sepa_mandates SET reference = 'SJR-H-2' WHERE id = ?`, own.mandate)
		require.ErrorContains(t, err, "keeps its family, reference", "the reference never changes")
		_, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT reference`)
		require.NoError(t, err)

		_, err = tx.ExecContext(ctx, `UPDATE parent_billing.sepa_mandates
			SET created_by_account_id = ?, updated_by_account_id = ? WHERE id = ?`, creator.ID, creator.ID, own.mandate)
		require.NoError(t, err)
		revoked, err := store.RevokeSEPAMandate(ctx, own.organization, revoker.ID, own.mandate, calendar.NewDate(2026, 10, 1))
		require.NoError(t, err)
		require.NotNil(t, revoked.RevokedOn)
		_, err = tx.ExecContext(ctx, `SAVEPOINT revoked`)
		require.NoError(t, err)
		_, err = store.UpdateSEPAMandate(ctx, own.organization, 0, parentbilling.SealedSEPAMandate{
			Mandate: parentbilling.SEPAMandate{ID: own.mandate, Sequence: parentbilling.SequenceFirst}, Holder: []byte{4}})
		require.ErrorContains(t, err, "revoked SEPA mandate never changes")
		_, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT revoked`)
		require.NoError(t, err)

		for _, statement := range []string{
			`UPDATE parent_billing.sepa_mandates SET created_by_account_id = ? WHERE id = ?`,
			`UPDATE parent_billing.sepa_mandates SET created_by_account_id = NULL, updated_by_account_id = ?, bic = 'NEUBICXX'
				WHERE id = ?`,
		} {
			_, err = tx.ExecContext(ctx, `SAVEPOINT actor_rewrite`)
			require.NoError(t, err)
			_, err = tx.ExecContext(ctx, statement, revoker.ID, own.mandate)
			require.ErrorContains(t, err, "revoked SEPA mandate never changes")
			_, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT actor_rewrite`)
			require.NoError(t, err)
		}

		// Deleting each actor clears only its FK, even on a revoked mandate.
		var before, after string
		const historicalData = `SELECT (to_jsonb(m) - ARRAY['created_by_account_id', 'updated_by_account_id', 'revoked_by_account_id'])::text
			FROM parent_billing.sepa_mandates m WHERE id = ?`
		require.NoError(t, tx.NewRaw(historicalData, own.mandate).Scan(ctx, &before))
		for _, accountID := range []int64{creator.ID, revoker.ID} {
			_, err = tx.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id = ?`, accountID)
			require.NoError(t, err)
		}
		require.NoError(t, tx.NewRaw(historicalData, own.mandate).Scan(ctx, &after))
		assert.Equal(t, before, after, "all historical mandate data survives actor deletion")
		var actorsCleared bool
		require.NoError(t, tx.NewRaw(`SELECT created_by_account_id IS NULL AND updated_by_account_id IS NULL
			AND revoked_by_account_id IS NULL FROM parent_billing.sepa_mandates WHERE id = ?`, own.mandate).Scan(ctx, &actorsCleared))
		assert.True(t, actorsCleared)

		// The trail is only ever appended: the request role may not change
		// or delete an entry.
		var grants []string
		require.NoError(t, tx.NewRaw(`SELECT privilege_type FROM information_schema.role_table_grants
			WHERE grantee = 'phoenix_tenant' AND table_schema = 'parent_billing' AND table_name = 'sepa_mandate_events'
			ORDER BY privilege_type`).Scan(ctx, &grants))
		assert.Equal(t, []string{"INSERT", "SELECT"}, grants)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
