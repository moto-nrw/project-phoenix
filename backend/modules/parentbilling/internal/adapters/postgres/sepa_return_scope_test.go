package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The debits behind the PAIN.008 files of the Rückgabe (#6314) are one
// Träger's own, also below
// row level security: the Träger transaction drops its request role, so
// only the adapter's own Träger filter keeps the other Träger out.
func TestSEPAReturnReadsOnlyTheTraegersOwnBankData(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ownScope := newMandateScope(t, db, "Träger Rückgabe A")
	otherScope := newMandateScope(t, db, "Träger Rückgabe B")
	ownSchool := testpkg.CreateTestSchoolInOrganization(t, db, ownScope.organization, "OGS Rückgabe A")
	otherSchool := testpkg.CreateTestSchoolInOrganization(t, db, otherScope.organization, "OGS Rückgabe B")
	store := NewStore()
	rollback := errors.New("roll the bank data back")

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, ownScope.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, ownScope.organization)
		require.NoError(t, err)
		own := seedDebitScope(t, ctx, tx, ownScope, ownSchool, "DE98ZZZ09999999999", "SJR-RA-1")
		other := seedDebitScope(t, ctx, tx, otherScope, otherSchool, "DE98ABC09999999999", "SJR-RB-1")
		collectOn := calendar.NewDate(2029, 6, 4)
		require.NoError(t, store.SubmitSEPADebits(ctx, own.organization, 0, fmt.Sprintf("MOTO-RUECKGABE-%d", own.organization),
			[]parentbilling.SEPADebit{{ID: own.debit, AmountCents: 7000, CollectOn: collectOn}},
			map[int64]calendar.Date{own.mandate: collectOn}))
		_, err = tx.ExecContext(ctx, `UPDATE parent_billing.sepa_debits SET status = 'submitted', message_id = ?, submitted_at = NOW(),
			submitted_amount_cents = 7000, collect_on = ? WHERE id = ?`, fmt.Sprintf("MOTO-RUECKGABE-%d", other.organization), collectOn, other.debit)
		require.NoError(t, err)

		messages, err := store.ReturnSEPAMessages(ctx, own.organization)
		require.NoError(t, err)
		require.Equal(t, []string{fmt.Sprintf("MOTO-RUECKGABE-%d", own.organization)}, messages, "only the Träger's own message")
		foreign, err := store.ReturnSEPAMessageDebits(ctx, own.organization, fmt.Sprintf("MOTO-RUECKGABE-%d", other.organization))
		require.NoError(t, err)
		require.Empty(t, foreign, "no debit of the other Träger by its message ID")
		debits, err := store.ReturnSEPAMessageDebits(ctx, own.organization, messages[0])
		require.NoError(t, err)
		require.Len(t, debits, 1, "only the Träger's own debit")
		debit := debits[0]
		assert.Equal(t, own.debit, debit.Debit.ID)
		assert.Equal(t, parentbilling.DebitSubmitted, debit.Debit.Status)
		assert.Equal(t, int64(7000), debit.Debit.AmountCents, "the submitted amount")
		assert.Equal(t, collectOn, debit.Debit.CollectOn)
		assert.NotEmpty(t, debit.Debit.MessageID)
		assert.NotNil(t, debit.Debit.SubmittedAt)
		assert.Equal(t, "DE98ZZZ09999999999", debit.CreditorID, "the creditor identifier of the release")
		assert.Equal(t, calendar.NewDate(2026, 9, 1), debit.MandateSignedOn)
		assert.Equal(t, []byte{1}, debit.IBAN)
		assert.Equal(t, []byte{2}, debit.Holder)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
