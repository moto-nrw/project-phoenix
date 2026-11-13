package cmd

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	exportTransferCompose "github.com/moto-ogs/project-phoenix/modules/exporttransfer/compose"
	parentBillingCompose "github.com/moto-ogs/project-phoenix/modules/parentbilling/compose"
	securityRuntimeCompose "github.com/moto-ogs/project-phoenix/modules/securityruntime/compose"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Exercise the CLI's complete traversal, including immutable consent text:
// omitting a source leaves ciphertext the bank-only reader rejects.
func TestBankDataRekeyJobOpensBothSourcesWithoutLegacy(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.NewDatabase(t, db).UnitOfWork())
	org := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "CLI rekey", Slug: fmt.Sprintf("cli-rekey-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	legacyKeys, bankKeys := testpkg.NewTestHealthDataKeys(), testpkg.NewTestHealthDataKeys()
	legacy, err := securityRuntimeCompose.NewFieldCipher(legacyKeys)
	require.NoError(t, err)
	const reference = "CLI-REKEY"
	aad := func(field string) []byte {
		return []byte(fmt.Sprintf("parent_billing.sepa_mandates.%s|%d|%s", field, org, reference))
	}
	iban, err := legacy.Seal([]byte("DE89370400440532013000"), aad("iban"))
	require.NoError(t, err)
	holder, err := legacy.Seal([]byte("Synthetic CLI holder"), aad("account_holder"))
	require.NoError(t, err)
	consentAAD := []byte(fmt.Sprintf("parent_billing.sepa_mandate_consents.text|%d|%s", org, reference))
	consentText, err := legacy.Seal([]byte("Exactly the confirmed text"), consentAAD)
	require.NoError(t, err)
	recipientAAD := []byte(fmt.Sprintf("config.traeger_export_recipients.password|%d", org))
	recipientPassword, err := legacy.Seal([]byte("synthetic-sftp-secret"), recipientAAD)
	require.NoError(t, err)
	require.NoError(t, db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, "SET LOCAL session_replication_role = replica"); err != nil {
			return err
		}
		contractID, invoiceID := testpkg.UniqueTestTenantID(t), testpkg.UniqueTestTenantID(t)
		var mandateID int64
		if err := tx.NewRaw(`INSERT INTO parent_billing.sepa_mandates
 (organization_id, family_email, care_contract_id, reference, signed_on, kind, sequence,
  creditor_id, account_holder_sealed, iban_sealed, iban_masked, source)
 VALUES (?, 'cli-rekey@example.invalid', ?, ?, '2026-09-01', 'recurring', 'first',
 'DE98ZZZ09999999999', ?, ?, 'masked', 'import') RETURNING id`,
			org, contractID, reference, holder, iban).Scan(ctx, &mandateID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.sepa_mandate_consents
 (organization_id, mandate_id, care_contract_id, channel, text_version, text_sha256, text_sealed,
 holder_confirmed, creditor_name, creditor_address, debtor_street, debtor_postal_code, debtor_city)
 VALUES (?, ?, ?, 'parent_account', 'sepa-core-de-1', repeat('a', 64), ?, true,
 'Synthetic creditor', 'Creditor street 1', 'Debtor street 2', '75175', 'City')`, org, mandateID, contractID, consentText); err != nil {
			return err
		}
		// Two recipients (ADR 0222): batches of one reach both.
		if _, err := tx.ExecContext(ctx, `INSERT INTO config.traeger_export_recipients (organization_id, name, password_sealed)
 VALUES (?, 'Kämmerei', ?), (?, 'Steuerbüro', ?)`, org, recipientPassword, org, recipientPassword); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.sepa_debits
 (organization_id, mandate_id, care_contract_id, run_kind, run_id, care_invoice_id,
  invoice_number, amount_cents, due_on, sequence_type, creditor_id, mandate_reference,
  mandate_signed_on, account_holder_sealed, iban_sealed, status)
 VALUES (?, ?, ?, 'care', ?, ?, 'CLI-COPY', 100, '2026-12-01', 'RCUR',
 'DE98ZZZ09999999999', ?, '2026-09-01', ?, ?, 'pending')`,
			org, mandateID, contractID, testpkg.UniqueTestTenantID(t), invoiceID, reference, holder, iban)
		return err
	}))
	transition, err := securityRuntimeCompose.NewBankDataCipher(bankKeys, legacyKeys)
	require.NoError(t, err)
	var output bytes.Buffer
	require.NoError(t, runBankDataRekeyJob(ctx, parentBillingCompose.NewBankDataRekey(transition), exportTransferCompose.NewRecipientRekey(transition), org, 1, &output))
	require.Contains(t, output.String(), "source=export_recipients scanned_total=2 changed_total=2")
	bankOnly, err := securityRuntimeCompose.NewBankDataCipher(bankKeys, "")
	require.NoError(t, err)
	var rows []struct {
		IBAN   []byte `bun:"iban_sealed"`
		Holder []byte `bun:"account_holder_sealed"`
	}
	require.NoError(t, db.NewRaw(`SELECT iban_sealed, account_holder_sealed
 FROM parent_billing.sepa_mandates WHERE organization_id = ?
 UNION ALL SELECT iban_sealed, account_holder_sealed
 FROM parent_billing.sepa_debits WHERE organization_id = ?`, org, org).Scan(ctx, &rows))
	require.Len(t, rows, 2)
	for _, row := range rows {
		plain, err := bankOnly.Open(row.IBAN, aad("iban"))
		require.NoError(t, err)
		require.Equal(t, "DE89370400440532013000", string(plain))
		plain, err = bankOnly.Open(row.Holder, aad("account_holder"))
		require.NoError(t, err)
		require.Equal(t, "Synthetic CLI holder", string(plain))
	}
	var sealedProof []byte
	require.NoError(t, db.NewRaw(`SELECT text_sealed FROM parent_billing.sepa_mandate_consents WHERE organization_id = ?`, org).Scan(ctx, &sealedProof))
	plain, err := bankOnly.Open(sealedProof, consentAAD)
	require.NoError(t, err)
	require.Equal(t, "Exactly the confirmed text", string(plain))
	var sealedPasswords [][]byte
	require.NoError(t, db.NewRaw(`SELECT password_sealed FROM config.traeger_export_recipients WHERE organization_id = ? ORDER BY id`, org).
		Scan(ctx, &sealedPasswords))
	require.Len(t, sealedPasswords, 2)
	for _, sealedPassword := range sealedPasswords {
		plain, err = bankOnly.Open(sealedPassword, recipientAAD)
		require.NoError(t, err)
		require.Equal(t, "synthetic-sftp-secret", string(plain))
	}
	output.Reset()
	require.NoError(t, runBankDataRekeyJob(ctx, parentBillingCompose.NewBankDataRekey(bankOnly), exportTransferCompose.NewRecipientRekey(bankOnly), org, 1, &output))
	require.Contains(t, output.String(), "source=export_recipients scanned_total=2 changed_total=0")
	require.Contains(t, output.String(), "source=mandates scanned_total=1 changed_total=0")
	require.Contains(t, output.String(), "source=debits scanned_total=1 changed_total=0")
	require.Contains(t, output.String(), "source=consents scanned_total=1 changed_total=0")
}
