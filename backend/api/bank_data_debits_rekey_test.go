package api

import (
	"context"
	"fmt"
	"testing"

	parentBillingCompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	securityRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/securityruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestBankDataRekeyIncludesFrozenDebitCopies(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.NewDatabase(t, db).UnitOfWork())
	// #6265 runs in parallel. Before its migration lands, this isolated schema
	// fixture provides its exact rekey columns; afterwards the real table wins.
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS parent_billing.sepa_debits (
  id BIGSERIAL PRIMARY KEY, organization_id BIGINT, mandate_id BIGINT, care_contract_id BIGINT,
  run_kind TEXT, run_id BIGINT, care_invoice_id BIGINT, invoice_number TEXT, amount_cents BIGINT,
  due_on DATE, sequence_type TEXT, creditor_id TEXT, mandate_reference TEXT, mandate_signed_on DATE,
  account_holder_sealed BYTEA, iban_sealed BYTEA, status TEXT DEFAULT 'pending')`)
	require.NoError(t, err)
	own := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Debit rekey", Slug: fmt.Sprintf("debit-rekey-%d", testpkg.UniqueTestTenantID(t))}).ID
	other := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Foreign debit", Slug: fmt.Sprintf("foreign-debit-%d", testpkg.UniqueTestTenantID(t))}).ID
	legacyKeys, bankKeys := testpkg.NewTestHealthDataKeys(), testpkg.NewTestHealthDataKeys()
	legacy, err := securityRuntimeCompose.NewFieldCipher(legacyKeys)
	require.NoError(t, err)
	require.NoError(t, db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, "SET LOCAL session_replication_role = replica"); err != nil {
			return err
		}
		for i, org := range []int64{own, other} {
			ref := fmt.Sprintf("COPY-%d", i)
			iban, err := legacy.Seal([]byte("DE89370400440532013000"), []byte(fmt.Sprintf("parent_billing.sepa_mandates.iban|%d|%s", org, ref)))
			if err != nil {
				return err
			}
			holder, err := legacy.Seal([]byte("Synthetic frozen holder"), []byte(fmt.Sprintf("parent_billing.sepa_mandates.account_holder|%d|%s", org, ref)))
			if err != nil {
				return err
			}
			_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.sepa_debits (organization_id, mandate_id, care_contract_id,
    run_kind, run_id, care_invoice_id, invoice_number, amount_cents, due_on, sequence_type, creditor_id,
    mandate_reference, mandate_signed_on, account_holder_sealed, iban_sealed)
    VALUES (?, ?, ?, 'care', ?, ?, 'TEST-COPY', 100, '2026-12-01', 'RCUR', 'DE98ZZZ09999999999', ?, '2026-09-01', ?, ?)`,
				org, testpkg.UniqueTestTenantID(t), testpkg.UniqueTestTenantID(t), testpkg.UniqueTestTenantID(t), testpkg.UniqueTestTenantID(t), ref, holder, iban)
			if err != nil {
				return err
			}
		}
		return nil
	}))
	var before, after, foreignBefore, foreignAfter string
	query := `SELECT (to_jsonb(d) - ARRAY['iban_sealed','account_holder_sealed'])::text FROM parent_billing.sepa_debits d WHERE organization_id = ?`
	foreignQuery := `SELECT to_jsonb(d)::text FROM parent_billing.sepa_debits d WHERE organization_id = ?`
	require.NoError(t, db.NewRaw(query, own).Scan(ctx, &before))
	require.NoError(t, db.NewRaw(foreignQuery, other).Scan(ctx, &foreignBefore))
	cipher, err := securityRuntimeCompose.NewBankDataCipher(bankKeys, legacyKeys)
	require.NoError(t, err)
	job := parentBillingCompose.NewBankDataRekey(cipher)
	batch, err := bankDataRekeyBatch(ctx, job, own, "debits", 0, 100)
	require.NoError(t, err)
	require.Equal(t, 1, batch.Changed)
	batch, err = bankDataRekeyBatch(ctx, job, own, "debits", 0, 100)
	require.NoError(t, err)
	require.Zero(t, batch.Changed)
	require.NoError(t, db.NewRaw(query, own).Scan(ctx, &after))
	require.Equal(t, before, after)
	require.NoError(t, db.NewRaw(foreignQuery, other).Scan(ctx, &foreignAfter))
	require.Equal(t, foreignBefore, foreignAfter)
	independent, err := securityRuntimeCompose.NewBankDataCipher(bankKeys, "")
	require.NoError(t, err)
	var values struct {
		IBAN   []byte `bun:"iban_sealed"`
		Holder []byte `bun:"account_holder_sealed"`
	}
	require.NoError(t, db.NewRaw(`SELECT iban_sealed, account_holder_sealed FROM parent_billing.sepa_debits WHERE organization_id = ?`, own).Scan(ctx, &values))
	for field, value := range map[string][]byte{"iban": values.IBAN, "account_holder": values.Holder} {
		_, err := independent.Open(value, []byte(fmt.Sprintf("parent_billing.sepa_mandates.%s|%d|COPY-0", field, own)))
		require.NoError(t, err)
	}
}
