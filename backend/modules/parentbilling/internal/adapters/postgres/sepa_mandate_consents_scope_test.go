package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The proof of an online mandate (#4696) keeps a Träger to its rows below
// row level security, and never changes: the Träger transaction drops its
// request role, so the read holds only because the adapter names the
// Träger in its own statement, and the trigger refuses a change for every
// role. Removing the filter or the trigger turns an assertion red.

const consentChecksum = "4f3c1f0e9b8a7d6c5b4a39281706f5e4d3c2b1a09f8e7d6c5b4a392817060504"

// seedConsent writes the proof of a mandate, keys and contract bypassed.
func seedConsent(t *testing.T, ctx context.Context, tx bun.IDB, scope mandateScope) {
	t.Helper()
	_, err := tx.ExecContext(ctx, `SET LOCAL session_replication_role = replica`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.sepa_mandate_consents (organization_id, mandate_id, care_contract_id,
		channel, text_version, text_sha256, text_sealed, holder_confirmed, creditor_name, creditor_address, debtor_street, debtor_postal_code, debtor_city)
		VALUES (?, ?, 1, 'parent_account', 'sepa-core-de-1', ?, 'sealed-consent-text'::bytea, true, 'Jugendring', 'Hauptstraße 1, 75175 Pforzheim',
		'Gartenweg 2', '75175', 'Pforzheim')`, scope.organization, scope.mandate, consentChecksum)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = origin`)
	require.NoError(t, err)
}

func TestSEPAMandateConsentFilterKeepsTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newMandateScope(t, db, "Träger Nachweis A")
	other := newMandateScope(t, db, "Träger Nachweis B")
	store := NewStore()
	rollback := errors.New("roll the proofs back")

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own.organization)
		require.NoError(t, err)
		own = seedMandateScope(t, ctx, tx, own, "DE98ZZZ09999999999", "SJR-N-A")
		other = seedMandateScope(t, ctx, tx, other, "DE98ABC09999999999", "SJR-N-B")
		seedConsent(t, ctx, tx, other)

		consent, err := store.SEPAMandateConsent(ctx, own.organization, other.mandate)
		require.NoError(t, err)
		assert.Nil(t, consent, "SEPAMandateConsent of another Träger's mandate")

		seedConsent(t, ctx, tx, own)
		consent, err = store.SEPAMandateConsent(ctx, own.organization, own.mandate)
		require.NoError(t, err)
		require.NotNil(t, consent, "SEPAMandateConsent of the Träger's own mandate")
		assert.Equal(t, consentChecksum, consent.TextSHA256)
		assert.Equal(t, []byte("sealed-consent-text"), consent.TextSealed)
		assert.True(t, consent.HolderConfirmed)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// The proof never changes, not even below row level security (the trigger
// lets only the erasure of the account clear it). The request role may
// neither change nor delete a proof.
func TestSEPAMandateConsentNeverChanges(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newMandateScope(t, db, "Träger Nachweis fest")
	rollback := errors.New("roll the proof back")

	for name, statement := range map[string]string{
		"text":     `UPDATE parent_billing.sepa_mandate_consents SET text_sealed = 'changed'::bytea WHERE mandate_id = ?`,
		"checksum": `UPDATE parent_billing.sepa_mandate_consents SET text_sha256 = repeat('0', 64) WHERE mandate_id = ?`,
		"holder":   `UPDATE parent_billing.sepa_mandate_consents SET debtor_street = 'Anderswo 9' WHERE mandate_id = ?`,
		"time":     `UPDATE parent_billing.sepa_mandate_consents SET confirmed_at = confirmed_at - interval '1 day' WHERE mandate_id = ?`,
	} {
		ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
		err := testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
			require.NoError(t, testpkg.WithoutRequestRole(ctx))
			tx, err := traegerDB(ctx, own.organization)
			require.NoError(t, err)
			scope := seedMandateScope(t, ctx, tx, own, "DE98ZZZ09999999999", "SJR-FEST-"+strings.ToUpper(name))
			seedConsent(t, ctx, tx, scope)
			_, err = tx.ExecContext(ctx, statement, scope.mandate)
			require.Error(t, err, name)
			assert.Contains(t, err.Error(), "the proof of a SEPA mandate never changes", name)
			return rollback
		})
		require.ErrorIs(t, err, rollback, name)
	}

	var update, remove bool
	require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_tenant', 'parent_billing.sepa_mandate_consents', 'UPDATE'),
		has_table_privilege('phoenix_tenant', 'parent_billing.sepa_mandate_consents', 'DELETE')`).Scan(context.Background(), &update, &remove))
	assert.False(t, update, "the request role changes no proof")
	assert.False(t, remove, "the request role deletes no proof")
}
