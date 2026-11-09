package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	securityRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/securityruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The Träger package as the CLI binds it (#6314, #4720, ADR 0187): the
// PAIN.008 file of each submitted message Parent Billing opens with the bank
// key, only of this Träger, and the run in the operator audit.
func TestTenantExportOrganizationReturnsOnlyItsPAIN008FilesAndIsAudited(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	database := testpkg.NewDatabase(t, db)
	ctx := tenant.WithUnitOfWork(context.Background(), database.UnitOfWork())
	keys := testpkg.TestBankDataKeys // the ring the root opens the catalog rows with (ADR 0184)
	cipher, err := securityRuntimeCompose.NewBankDataCipher(keys, "")
	require.NoError(t, err)
	operator := testpkg.CreateTestOperator(t, db)
	seed := func(name, iban, messageID string) int64 {
		org := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
			Name: name, Slug: fmt.Sprintf("rueckgabe-%d", testpkg.UniqueTestTenantID(t))}).ID
		reference := "REF-" + messageID[:6]
		aad := func(field string) []byte {
			return []byte(fmt.Sprintf("parent_billing.sepa_mandates.%s|%d|%s", field, org, reference))
		}
		sealedIBAN, err := cipher.Seal([]byte(iban), aad("iban"))
		require.NoError(t, err)
		sealedHolder, err := cipher.Seal([]byte("Synthetic Holder "+name), aad("account_holder"))
		require.NoError(t, err)
		require.NoError(t, db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, "SET LOCAL session_replication_role = replica"); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO platform.traeger_bank_accounts (organization_id, label, account_holder, iban, bic, is_default)
				VALUES (?, 'Hauptkonto', ?, 'DE75512108001245126199', 'SOLADES1PFO', true)`, org, name+" e. V."); err != nil {
				return err
			}
			contract := testpkg.UniqueTestTenantID(t)
			var mandate int64
			if err := tx.NewRaw(`INSERT INTO parent_billing.sepa_mandates (organization_id, family_email, care_contract_id, reference,
				signed_on, kind, sequence, creditor_id, account_holder_sealed, iban_sealed, iban_masked, source)
				VALUES (?, 'rueckgabe@example.invalid', ?, ?, '2026-09-01', 'recurring', 'recurring', 'DE98ZZZ09999999999', ?, ?, 'masked', 'office')
				RETURNING id`, org, contract, reference, sealedHolder, sealedIBAN).Scan(ctx, &mandate); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.sepa_debits (organization_id, mandate_id, care_contract_id, run_kind,
				run_id, care_invoice_id, invoice_number, amount_cents, due_on, sequence_type, creditor_id, mandate_reference,
				mandate_signed_on, account_holder_sealed, iban_sealed, status, message_id, submitted_at, submitted_amount_cents, collect_on)
				VALUES (?, ?, ?, 'care', 3, ?, 'RE-2027-0001', 7800, '2027-01-01', 'FRST', 'DE98ZZZ09999999999', ?, '2026-09-01', ?, ?,
					'submitted', ?, '2026-11-20T09:00:00Z', 7800, '2027-01-01')`,
				org, mandate, contract, testpkg.UniqueTestTenantID(t), reference, sealedHolder, sealedIBAN, messageID)
			return err
		}))
		return org
	}
	a := seed("Träger A", "DE89370400440532013000", "MOTO-20261120-AAAAAAAAAAAAAAAAAAAA")
	seed("Träger B", "DE02120300000000202051", "MOTO-20261120-BBBBBBBBBBBBBBBBBBBB")

	out := filepath.Join(t.TempDir(), "export")
	exporter, err := newTraegerTenantExporter(database, returnSealedFields(t), tenantExportProjectedFields(), cipher)
	require.NoError(t, err)
	require.NoError(t, exporter.Export(ctx, organizationtenancy.TenantExportOptions{OrganizationID: a, OperatorID: operator.ID, Output: out, Uploads: t.TempDir()}))

	bank := filepath.Join(out, "SEPA-LASTSCHRIFTDATEIEN")
	file, err := os.ReadFile(filepath.Join(bank, "MOTO-20261120-AAAAAAAAAAAAAAAAAAAA.xml"))
	require.NoError(t, err)
	for _, want := range []string{"<MsgId>MOTO-20261120-AAAAAAAAAAAAAAAAAAAA</MsgId>", "<IBAN>DE89370400440532013000</IBAN>",
		"<IBAN>DE75512108001245126199</IBAN>", `<InstdAmt Ccy="EUR">78.00</InstdAmt>`, "<ReqdColltnDt>2027-01-01</ReqdColltnDt>",
		"<CreDtTm>2026-11-20T10:00:00</CreDtTm>"} {
		require.Contains(t, string(file), want)
	}
	entries, err := os.ReadDir(bank)
	require.NoError(t, err)
	require.Len(t, entries, 2, "only the file of Träger A, and its notice")
	require.NoError(t, filepath.WalkDir(out, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NotContains(t, string(data), "DE02120300000000202051", "no bank data of Träger B: %s", path)
		require.NotContains(t, string(data), "BBBBBBBBBBBBBBBBBBBB", "no file of Träger B: %s", path)
		return nil
	}))

	marker, err := os.ReadFile(filepath.Join(out, "COMPLETE"))
	require.NoError(t, err)
	var audit struct {
		Changes json.RawMessage `bun:"changes"`
	}
	var outcomes []string
	require.NoError(t, db.NewRaw(`SELECT changes->>'outcome' FROM platform.operator_audit_log
		WHERE operator_id = ? AND action = 'export' AND resource_type = 'organization' AND resource_id = ? ORDER BY id`, operator.ID, a).Scan(ctx, &outcomes))
	require.Equal(t, []string{"started", "completed"}, outcomes, "the attempt before the extraction, the outcome after it")
	require.NoError(t, db.NewRaw(`SELECT changes FROM platform.operator_audit_log
		WHERE operator_id = ? AND action = 'export' AND resource_type = 'organization' AND resource_id = ?
			AND changes->>'outcome' = 'completed'`, operator.ID, a).Scan(ctx, &audit))
	var record organizationtenancy.TenantExportRecord
	require.NoError(t, json.Unmarshal(audit.Changes, &record))
	require.Equal(t, strings.TrimSpace(string(marker)), record.ManifestSHA256)
	require.Equal(t, a, record.OrganizationID)
	require.NotNil(t, record.SEPADebitFiles)
	require.Len(t, record.SEPADebitFiles.Files, 1)
	require.NotContains(t, string(audit.Changes), "DE89370400440532013000", "the audit holds no bank data")
}

// Like the erasure, the Rückgabe needs an existing, active operator before
// anything is read: an unknown or deactivated one gets no file and no row.
func TestTenantExportOrganizationRefusesAnUnknownOrInactiveOperator(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	database := testpkg.NewDatabase(t, db)
	ctx := tenant.WithUnitOfWork(context.Background(), database.UnitOfWork())
	cipher, err := securityRuntimeCompose.NewBankDataCipher(testpkg.TestBankDataKeys, "")
	require.NoError(t, err)
	org := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Inaktiv", Slug: fmt.Sprintf("rueckgabe-inaktiv-%d", testpkg.UniqueTestTenantID(t))}).ID
	inactive := testpkg.CreateTestOperator(t, db)
	_, err = db.ExecContext(ctx, `UPDATE platform.operators SET active = false, deactivated_at = now() WHERE id = ?`, inactive.ID)
	require.NoError(t, err)
	exporter, err := newTraegerTenantExporter(database, returnSealedFields(t), tenantExportProjectedFields(), cipher)
	require.NoError(t, err)
	for name, operator := range map[string]int64{"inactive": inactive.ID, "unknown": 1 << 60} {
		out := filepath.Join(t.TempDir(), "export")
		require.Error(t, exporter.Export(ctx, organizationtenancy.TenantExportOptions{OrganizationID: org, OperatorID: operator, Output: out, Uploads: t.TempDir()}), name)
		require.NoDirExists(t, out, name)
		var rows int
		require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.operator_audit_log
			WHERE action = 'export' AND resource_type = 'organization' AND resource_id = ?`, org).Scan(ctx, &rows))
		require.Zero(t, rows, name)
	}
}

// A submitted file whose download moto stored (ADR 0187) is returned as that
// original, byte for byte: changing the Träger's default account afterwards
// does not rewrite its history. LIESMICH.txt and the manifest name it.

// returnSealedFields opens the bank values of the catalog rows as the root
// does (ADR 0184), with the key ring of the test.
func returnSealedFields(t *testing.T) organizationtenancy.TenantExportSealedFields {
	t.Helper()
	sealed, err := tenantExportSealedFields()
	require.NoError(t, err)
	return sealed
}

func TestTenantExportOrganizationReturnsTheStoredOriginalAfterTheAccountChanged(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	database := testpkg.NewDatabase(t, db)
	ctx := tenant.WithUnitOfWork(context.Background(), database.UnitOfWork())
	cipher, err := securityRuntimeCompose.NewBankDataCipher(testpkg.TestBankDataKeys, "")
	require.NoError(t, err)
	operator := testpkg.CreateTestOperator(t, db)
	org := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Original", Slug: fmt.Sprintf("rueckgabe-original-%d", testpkg.UniqueTestTenantID(t))}).ID
	const messageID = "MOTO-20261120-ORIGINALORIGINALORIG"
	original := []byte("<Document><MsgId>" + messageID + "</MsgId><IBAN>DE75512108001245126199</IBAN><IBAN>DE89370400440532013000</IBAN></Document>\n")
	sealedFile, err := cipher.Seal(original, []byte(fmt.Sprintf("parent_billing.sepa_debit_files.file|%d|%s", org, messageID)))
	require.NoError(t, err)
	reference := "REF-ORIGINAL"
	sealedIBAN, err := cipher.Seal([]byte("DE89370400440532013000"), []byte(fmt.Sprintf("parent_billing.sepa_mandates.iban|%d|%s", org, reference)))
	require.NoError(t, err)
	sealedHolder, err := cipher.Seal([]byte("Synthetic Holder"), []byte(fmt.Sprintf("parent_billing.sepa_mandates.account_holder|%d|%s", org, reference)))
	require.NoError(t, err)
	require.NoError(t, db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, "SET LOCAL session_replication_role = replica"); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO platform.traeger_bank_accounts (organization_id, label, account_holder, iban, bic, is_default)
			VALUES (?, 'Hauptkonto', 'Träger Original e. V.', 'DE75512108001245126199', 'SOLADES1PFO', true)`, org); err != nil {
			return err
		}
		contract := testpkg.UniqueTestTenantID(t)
		var mandate int64
		if err := tx.NewRaw(`INSERT INTO parent_billing.sepa_mandates (organization_id, family_email, care_contract_id, reference,
			signed_on, kind, sequence, creditor_id, account_holder_sealed, iban_sealed, iban_masked, source)
			VALUES (?, 'original@example.invalid', ?, ?, '2026-09-01', 'recurring', 'recurring', 'DE98ZZZ09999999999', ?, ?, 'masked', 'office')
			RETURNING id`, org, contract, reference, sealedHolder, sealedIBAN).Scan(ctx, &mandate); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.sepa_debits (organization_id, mandate_id, care_contract_id, run_kind,
			run_id, care_invoice_id, invoice_number, amount_cents, due_on, sequence_type, creditor_id, mandate_reference,
			mandate_signed_on, account_holder_sealed, iban_sealed, status, message_id, submitted_at, submitted_amount_cents, collect_on)
			VALUES (?, ?, ?, 'care', 3, ?, 'RE-2027-0001', 7800, '2027-01-01', 'FRST', 'DE98ZZZ09999999999', ?, '2026-09-01', ?, ?,
				'submitted', ?, '2026-11-20T09:00:00Z', 7800, '2027-01-01')`,
			org, mandate, contract, testpkg.UniqueTestTenantID(t), reference, sealedHolder, sealedIBAN, messageID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.sepa_debit_files (organization_id, message_id, run_kind, run_id,
			file_sealed, file_sha256, creditor_name, creditor_iban, creditor_bic, creditor_id, downloaded_at, submitted_at)
			VALUES (?, ?, 'care', 3, ?, ?, 'Träger Original e. V.', 'DE75512108001245126199', 'SOLADES1PFO', 'DE98ZZZ09999999999', now(), now())`,
			org, messageID, sealedFile, testpkg.ContentFingerprint(original))
		return err
	}))
	// After the submission the Träger changes its default account.
	_, err = db.ExecContext(ctx, `UPDATE platform.traeger_bank_accounts SET iban = 'DE12500105170648489890', bic = 'INGDDEFFXXX'
		WHERE organization_id = ?`, org)
	require.NoError(t, err)

	out := filepath.Join(t.TempDir(), "export")
	exporter, err := newTraegerTenantExporter(database, returnSealedFields(t), tenantExportProjectedFields(), cipher)
	require.NoError(t, err)
	require.NoError(t, exporter.Export(ctx, organizationtenancy.TenantExportOptions{OrganizationID: org, OperatorID: operator.ID, Output: out, Uploads: t.TempDir()}))
	returned, err := os.ReadFile(filepath.Join(out, "SEPA-LASTSCHRIFTDATEIEN", messageID+".xml"))
	require.NoError(t, err)
	require.Equal(t, original, returned, "the stored original, not a file with today's account")
	readme, err := os.ReadFile(filepath.Join(out, "SEPA-LASTSCHRIFTDATEIEN", "LIESMICH.txt"))
	require.NoError(t, err)
	require.Contains(t, string(readme), messageID+".xml: Original")
	manifest, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	require.NoError(t, err)
	var parsed organizationtenancy.ExportManifest
	require.NoError(t, json.Unmarshal(manifest, &parsed))
	require.NotNil(t, parsed.SEPADebitFiles)
	require.Equal(t, []organizationtenancy.ExportSEPADebitFileOrigin{{Path: "SEPA-LASTSCHRIFTDATEIEN/" + messageID + ".xml", Origin: "Original",
		Reason: parsed.SEPADebitFiles.Files[0].Reason}}, parsed.SEPADebitFiles.Files)
	require.NotContains(t, string(manifest), "DE89370400440532013000", "the manifest names no bank data of a family")
}
