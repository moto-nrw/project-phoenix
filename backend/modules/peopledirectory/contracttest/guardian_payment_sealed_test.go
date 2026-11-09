package contracttest_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"testing"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGuardianPaymentAccessAuditSurvivesCallerRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)
	profile := testpkg.CreateTestGuardianProfile(t, db, "audited-failure")
	actor := testpkg.CreateTestAccount(t, db, fmt.Sprintf("bank-audit-%d@example.test", testpkg.Tenant(t)))
	require.NoError(t, testpkg.TenantCommand4(t, ctx, db, service.UpdateGuardianPayment, profile.ID,
		peoplePorts.GuardianPaymentInput{IBAN: strPtr("DE89370400440532013000")}, actor.ID, ""))
	_, err := db.NewRaw(`UPDATE users.guardian_financial_data SET iban_sealed = ? WHERE guardian_profile_id = ?`,
		[]byte("damaged IBAN"), profile.ID).Exec(ctx)
	require.NoError(t, err)

	err = testpkg.WithinTenantContext(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context) error {
		_, err := service.RevealGuardianPayment(txCtx, profile.ID, actor.ID, "admin")
		return err // The caller rolls back on the decryption error.
	})
	require.Error(t, err)
	var count int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM audit.data_access_log
WHERE tenant_id = ? AND resource_type = 'guardian_financial_reveal' AND metadata->>'guardian_profile_id' = ?`,
		testpkg.Tenant(t), fmt.Sprint(profile.ID)).Scan(ctx, &count))
	assert.Equal(t, 1, count)
}

type storedGuardianBank struct {
	IBAN                *string `bun:"iban"`
	AccountHolder       *string `bun:"account_holder"`
	IBANSealed          []byte  `bun:"iban_sealed"`
	IBANMasked          *string `bun:"iban_masked"`
	AccountHolderSealed []byte  `bun:"account_holder_sealed"`
}

// #6332: the service writes IBAN and account holder only sealed with the
// bank ring, keeps the mask for the default read, and a sealed value opens
// only in its own field, row and school.
func TestGuardianPaymentIsStoredSealedAndOpensOnlyInItsOwnRow(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupGuardianService(t, db)
	ctx := testpkg.Ctx(t)
	const iban, holder = "DE89370400440532013000", "Sabine Schneider"
	stored := func(t *testing.T, profileID int64) storedGuardianBank {
		t.Helper()
		var row storedGuardianBank
		require.NoError(t, db.NewRaw(`SELECT iban, account_holder, iban_sealed, iban_masked, account_holder_sealed
   FROM users.guardian_financial_data WHERE guardian_profile_id = ?`, profileID).Scan(ctx, &row))
		return row
	}
	write := func(t *testing.T, profileID int64, value, name string) {
		t.Helper()
		require.NoError(t, testpkg.TenantCommand4(t, ctx, db, service.UpdateGuardianPayment, profileID,
			peoplePorts.GuardianPaymentInput{IBAN: strPtr(value), AccountHolder: strPtr(name)}, 1, ""))
	}

	owner := testpkg.CreateTestGuardianProfile(t, db, "sealed-owner")
	write(t, owner.ID, iban, holder)
	row := stored(t, owner.ID)
	assert.Nil(t, row.IBAN, "no plaintext IBAN column value")
	assert.Nil(t, row.AccountHolder, "no plaintext account holder column value")
	require.NotEmpty(t, row.IBANSealed)
	require.NotEmpty(t, row.AccountHolderSealed)
	assert.False(t, bytes.Contains(row.IBANSealed, []byte(iban)), "the sealed IBAN holds no plaintext")
	assert.False(t, bytes.Contains(row.IBANSealed, []byte("532013000")), "not even the account number")
	assert.False(t, bytes.Contains(row.AccountHolderSealed, []byte("Schneider")), "the sealed holder holds no plaintext")
	require.NotNil(t, row.IBANMasked)
	assert.Equal(t, "•••• 3000", *row.IBANMasked)

	masked, err := testpkg.TenantQuery3(t, ctx, db, service.GetGuardianPaymentMasked, owner.ID, int64(1), "admin")
	require.NoError(t, err)
	assert.Equal(t, "•••• 3000", *masked.IBANMasked)
	assert.Equal(t, holder, *masked.AccountHolder, "decision 3: the holder stays unmasked in the default read")
	plain, err := testpkg.TenantQuery3(t, ctx, db, service.RevealGuardianPayment, owner.ID, int64(1), "admin")
	require.NoError(t, err)
	assert.Equal(t, iban, *plain.IBAN)
	assert.Equal(t, holder, *plain.AccountHolder)

	copyInto := func(t *testing.T, profileID int64, column string, value []byte) {
		t.Helper()
		_, err := db.NewRaw(`UPDATE users.guardian_financial_data SET `+column+` = ? WHERE guardian_profile_id = ?`, value, profileID).Exec(ctx)
		require.NoError(t, err)
	}
	assertNoValue := func(t *testing.T, err error) {
		t.Helper()
		require.Error(t, err)
		assert.NotContains(t, err.Error(), iban)
		assert.NotContains(t, err.Error(), "Schneider")
	}

	t.Run("another guardian of the school", func(t *testing.T) {
		sibling := testpkg.CreateTestGuardianProfile(t, db, "sealed-sibling")
		write(t, sibling.ID, "DE84120300000000203000", "Peter Schneider")
		copyInto(t, sibling.ID, "iban_sealed", row.IBANSealed)
		_, err := testpkg.TenantQuery3(t, ctx, db, service.RevealGuardianPayment, sibling.ID, int64(1), "admin")
		assertNoValue(t, err)
		// The default read serves the stored mask and never opens the IBAN.
		masked, err := testpkg.TenantQuery3(t, ctx, db, service.GetGuardianPaymentMasked, sibling.ID, int64(1), "admin")
		require.NoError(t, err)
		assert.Equal(t, "•••• 3000", *masked.IBANMasked)
		copyInto(t, sibling.ID, "account_holder_sealed", row.AccountHolderSealed)
		_, err = testpkg.TenantQuery3(t, ctx, db, service.GetGuardianPaymentMasked, sibling.ID, int64(1), "admin")
		assertNoValue(t, err)
	})

	t.Run("another field of the same guardian", func(t *testing.T) {
		swapped := testpkg.CreateTestGuardianProfile(t, db, "sealed-swapped")
		write(t, swapped.ID, iban, holder)
		own := stored(t, swapped.ID)
		copyInto(t, swapped.ID, "account_holder_sealed", own.IBANSealed)
		_, err := testpkg.TenantQuery3(t, ctx, db, service.GetGuardianPaymentMasked, swapped.ID, int64(1), "admin")
		assertNoValue(t, err)
	})

	t.Run("the same guardian ID at another school", func(t *testing.T) {
		scope := testpkg.NewTenantScope(t, db)
		foreign := testpkg.CreateTestGuardianProfileForTenant(t, db, scope.TenantID, "Sealed", "Foreign", "sealed-foreign@example.invalid")
		require.NoError(t, testpkg.TenantCommand4(t, scope.Context(), db, service.UpdateGuardianPayment, foreign.ID,
			peoplePorts.GuardianPaymentInput{IBAN: strPtr("DE84120300000000203000")}, 1, ""))
		copyInto(t, foreign.ID, "iban_sealed", row.IBANSealed)
		_, err := testpkg.TenantQuery3(t, scope.Context(), db, service.RevealGuardianPayment, foreign.ID, int64(1), "admin")
		assertNoValue(t, err)
	})
}

// #6332: with SQL debug logging on, a write and a reveal of guardian bank
// data put neither the account holder nor the IBAN nor the sealed bytes into
// the log. The holder is not IBAN-shaped, so only the sealing keeps it out.
func TestGuardianPaymentWriteLogsNoBankValue(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupIsolatedTestDB(t)
	service := setupGuardianService(t, db)
	scope := testpkg.NewTenantScope(t, db)
	profile := testpkg.CreateTestGuardianProfileForTenant(t, db, scope.TenantID, "Logged", "Guardian", "sealed-log@example.invalid")
	var logged bytes.Buffer
	testpkg.NewDatabase(t, db).LogQueries(slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug})))
	const holder = "Quirin Kontoinhaberlog"
	require.NoError(t, testpkg.TenantCommand4(t, scope.Context(), db, service.UpdateGuardianPayment, profile.ID,
		peoplePorts.GuardianPaymentInput{IBAN: strPtr("DE89370400440532013000"), AccountHolder: strPtr(holder)}, 1, ""))
	plain, err := testpkg.TenantQuery3(t, scope.Context(), db, service.RevealGuardianPayment, profile.ID, int64(1), "admin")
	require.NoError(t, err)
	require.Equal(t, holder, *plain.AccountHolder)
	require.Contains(t, logged.String(), "guardian_financial_data", "the write was logged")
	for _, secret := range []string{"Kontoinhaberlog", "532013000"} {
		require.NotContains(t, logged.String(), secret)
	}
}
