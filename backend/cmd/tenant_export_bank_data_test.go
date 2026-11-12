package cmd

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	parentBillingCompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	securityRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/securityruntime/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// bankSeal seals like the owners do (#6332, #4696): the context names
// field, scope and row, so the test pins the contexts the openers bind.
func bankSeal(t *testing.T, plain, context string) []byte {
	t.Helper()
	cipher, err := securityRuntimeCompose.NewBankDataCipher(testpkg.TestBankDataKeys, "")
	require.NoError(t, err)
	sealed, err := cipher.Seal([]byte(plain), []byte(context))
	require.NoError(t, err)
	return sealed
}

// requireNoCiphertext walks the whole package: no sealed value may leave in
// any encoding the export or a reader could produce.
func requireNoCiphertext(t *testing.T, out string, sealed ...[]byte) {
	t.Helper()
	require.NoError(t, filepath.WalkDir(out, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, value := range sealed {
			for _, encoded := range []string{hex.EncodeToString(value), base64.StdEncoding.EncodeToString(value)} {
				require.NotContains(t, string(data), encoded, "ciphertext in %s", path)
			}
		}
		return nil
	}))
}

// exportSchool runs the school return as the root binds it, with the
// operator audit #6326 requires.
func exportSchool(t *testing.T, db *bun.DB, school int64) (string, error) {
	t.Helper()
	sealed, err := tenantExportSealedFields()
	require.NoError(t, err)
	database := testpkg.NewDatabase(t, db)
	exporter, err := newTenantExporter(database, sealed, tenantExportProjectedFields(), false)
	require.NoError(t, err)
	operator := testpkg.CreateTestOperator(t, db)
	out := filepath.Join(t.TempDir(), "export")
	return out, exporter.Export(tenant.WithUnitOfWork(t.Context(), database.UnitOfWork()),
		organizationtenancy.TenantExportOptions{SchoolID: school, OperatorID: operator.ID, Output: out, Uploads: t.TempDir()})
}

func openedValues(t *testing.T, path string, columns ...string) []string {
	t.Helper()
	var values []string
	for _, row := range readExportRows(t, path) {
		for _, column := range columns {
			var value *string
			require.NoError(t, json.Unmarshal(row[column], &value), column)
			if value != nil {
				values = append(values, column+"="+*value)
			}
		}
		for name := range row {
			require.False(t, strings.HasSuffix(name, "_sealed"), "the sealed column %s never leaves", name)
		}
	}
	return values
}

// schoolBankData seals the bank values of one guardian and one staff member
// of a school as People Directory and Workforce do (#6332) and stores them.
type schoolBankData struct {
	school, guardian, staff int64
	sealed                  [][]byte
}

func seedSchoolBankData(t *testing.T, db *bun.DB, school int64, guardianIBAN, holder, staffIBAN, taxID, social string) schoolBankData {
	t.Helper()
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, school, "Synthetic", "Payer", fmt.Sprintf("export-bank-%d", school))
	staff := testpkg.CreateTestStaffForTenant(t, db, school, "Synthetic", "Payroll")
	guardianContext := func(field string) string {
		return fmt.Sprintf("users.guardian_financial_data.%s|%d|%d", field, school, guardian.ID)
	}
	staffContext := func(field string) string {
		return fmt.Sprintf("users.staff_financial_data.%s|%d|%d", field, school, staff.ID)
	}
	data := schoolBankData{school: school, guardian: guardian.ID, staff: staff.ID, sealed: [][]byte{
		bankSeal(t, guardianIBAN, guardianContext("iban")), bankSeal(t, holder, guardianContext("account_holder")),
		bankSeal(t, staffIBAN, staffContext("iban")), bankSeal(t, taxID, staffContext("tax_id")), bankSeal(t, social, staffContext("social_security_number")),
	}}
	_, err := db.NewRaw(`INSERT INTO users.guardian_financial_data (tenant_id, guardian_profile_id, iban_sealed, iban_masked, account_holder_sealed)
		VALUES (?, ?, ?, '****0000', ?)`, school, guardian.ID, data.sealed[0], data.sealed[1]).Exec(t.Context())
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO users.staff_financial_data (tenant_id, staff_id, iban_sealed, iban_masked, tax_id_sealed, social_security_number_sealed)
		VALUES (?, ?, ?, '****0000', ?, ?)`, school, staff.ID, data.sealed[2], data.sealed[3], data.sealed[4]).Exec(t.Context())
	require.NoError(t, err)
	return data
}

// The return of a school carries the IBAN and account holder of a guardian
// and the IBAN, tax ID and social security number of a staff member opened
// with the bank ring, never their ciphertext (#6412), and only those of the
// school it returns: the values of another school leave in no form (#6386).
func TestTenantExportOpensGuardianAndStaffBankDataWithTheRootBindings(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	otherSchool := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherSchool)
	own := seedSchoolBankData(t, db, testpkg.Tenant(t), "DE89370400440532013000", "Synthetic Holder", "DE02120300000000202051", "12345678995", "65170839J003")
	other := seedSchoolBankData(t, db, otherSchool, "DE75512108001245126199", "Other Holder", "DE12500105170648489890", "86095742719", "12150379M512")

	out, err := exportSchool(t, db, own.school)
	require.NoError(t, err)
	require.FileExists(t, filepath.Join(out, "COMPLETE"))
	require.Equal(t, []string{"iban_opened=DE89370400440532013000", "account_holder_opened=Synthetic Holder"},
		openedValues(t, filepath.Join(out, "users.guardian_financial_data.jsonl"), "iban_opened", "account_holder_opened"))
	require.Equal(t, []string{"iban_opened=DE02120300000000202051", "tax_id_opened=12345678995", "social_security_number_opened=65170839J003"},
		openedValues(t, filepath.Join(out, "users.staff_financial_data.jsonl"), "iban_opened", "tax_id_opened", "social_security_number_opened"))
	requireNoCiphertext(t, out, append(own.sealed, other.sealed...)...)
	require.NoError(t, filepath.WalkDir(out, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		for _, value := range []string{"DE75512108001245126199", "Other Holder", "DE12500105170648489890", "86095742719", "12150379M512"} {
			require.NotContains(t, string(data), value, "bank data of another school in %s", path)
		}
		return nil
	}))
}

// A sealed value that does not open in its row stops the return (#6386):
// copied from another guardian or another school, moved into another
// field, or sealed with a ring the installation does not hold. The package
// never completes, carries neither ciphertext nor a gap, the error names no
// value, and the operator audit holds the attempt and the failure.
func TestTenantExportRefusesBankDataThatDoesNotOpenInItsRow(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	const iban = "DE89370400440532013000"
	foreignRing, err := securityRuntimeCompose.NewBankDataCipher(testpkg.NewTestHealthDataKeys(), "")
	require.NoError(t, err)
	for name, sealFor := range map[string]func(school, owner, other int64) (column string, sealed []byte){
		"another guardian": func(school, owner, _ int64) (string, []byte) {
			return "iban_sealed", bankSeal(t, iban, fmt.Sprintf("users.guardian_financial_data.iban|%d|%d", school, owner))
		},
		"another school": func(_, _, other int64) (string, []byte) {
			return "iban_sealed", bankSeal(t, iban, fmt.Sprintf("users.guardian_financial_data.iban|%d|%d", testpkg.UniqueTestTenantID(t), other))
		},
		"another field": func(school, _, other int64) (string, []byte) {
			return "account_holder_sealed", bankSeal(t, iban, fmt.Sprintf("users.guardian_financial_data.iban|%d|%d", school, other))
		},
		"another ring": func(school, _, other int64) (string, []byte) {
			sealed, err := foreignRing.Seal([]byte(iban), fmt.Appendf(nil, "users.guardian_financial_data.iban|%d|%d", school, other))
			require.NoError(t, err)
			return "iban_sealed", sealed
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			school := testpkg.UniqueTestTenantID(t) // one school per case: the audit is per school
			testpkg.EnsureTestTenant(t, db, school)
			owner := testpkg.CreateTestGuardianProfileForTenant(t, db, school, "Synthetic", "Owner", fmt.Sprintf("export-bank-owner-%d", school))
			other := testpkg.CreateTestGuardianProfileForTenant(t, db, school, "Synthetic", "Other", fmt.Sprintf("export-bank-other-%d", school))
			column, sealed := sealFor(school, owner.ID, other.ID)
			_, err := db.NewRaw(`INSERT INTO users.guardian_financial_data (tenant_id, guardian_profile_id, iban_masked, ?)
				VALUES (?, ?, '****3000', ?)`, bun.Ident(column), school, other.ID, sealed).Exec(t.Context())
			require.NoError(t, err)

			out, err := exportSchool(t, db, school)
			require.Error(t, err)
			require.Contains(t, err.Error(), "cannot open a sealed value of users.guardian_financial_data")
			require.NotContains(t, err.Error(), "DE89")
			require.NoFileExists(t, filepath.Join(out, "COMPLETE"))
			requireNoCiphertext(t, out, sealed)

			var audit []string
			require.NoError(t, db.NewRaw(`SELECT changes::text FROM platform.operator_audit_log
				WHERE action = 'export' AND resource_type = 'school' AND resource_id = ? ORDER BY id`, school).Scan(t.Context(), &audit))
			require.Len(t, audit, 2, "the attempt and the failure")
			var failure organizationtenancy.TenantExportRecord
			require.NoError(t, json.Unmarshal([]byte(audit[1]), &failure))
			require.Equal(t, organizationtenancy.TenantExportFailed, failure.Outcome)
			require.Equal(t, "snapshot", failure.ErrorClass)
			for _, entry := range audit {
				require.NotContains(t, entry, "DE89", "the audit holds no bank data")
			}
		})
	}
}

// The root can bind only the bank ring to the bank openers (#6386): the
// health cipher does not satisfy the port any of them takes.
func TestTenantExportBankOpenersRefuseTheHealthCipher(t *testing.T) {
	t.Parallel()
	health, err := securityRuntimeCompose.NewFieldCipher(testpkg.TestHealthDataKeys)
	require.NoError(t, err)
	bank, err := securityRuntimeCompose.NewBankDataCipher(testpkg.TestBankDataKeys, "")
	require.NoError(t, err)
	for name, constructor := range map[string]any{
		"guardian": peopleCompose.NewGuardianBankExportFields,
		"staff":    workforceCompose.NewStaffBankExportFields,
		"sepa":     parentBillingCompose.NewSEPAExportFields,
	} {
		port := reflect.TypeOf(constructor).In(0)
		require.True(t, reflect.TypeOf(bank).Implements(port), name)
		require.False(t, reflect.TypeOf(health).Implements(port), name)
	}
}

// The Träger package carries the confirmed text of an online mandate opened
// with the reference of its mandate from the same snapshot (#6412); the
// reference serves the opening only and the ciphertext never leaves.
func TestTenantExportOrganizationPackageOpensTheMandateText(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	org := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Export mandate text", Slug: fmt.Sprintf("export-mandate-text-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	const reference, text = "EXPORT-TEXT", "Synthetischer bestätigter Mandatstext"
	iban := bankSeal(t, "DE89370400440532013000", fmt.Sprintf("parent_billing.sepa_mandates.iban|%d|%s", org, reference))
	holder := bankSeal(t, "Synthetic Holder", fmt.Sprintf("parent_billing.sepa_mandates.account_holder|%d|%s", org, reference))
	consentText := bankSeal(t, text, fmt.Sprintf("parent_billing.sepa_mandate_consents.text|%d|%s", org, reference))
	ctx := context.Background()
	require.NoError(t, db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, "SET LOCAL session_replication_role = replica"); err != nil {
			return err
		}
		contractID := testpkg.UniqueTestTenantID(t)
		var mandateID int64
		if err := tx.NewRaw(`INSERT INTO parent_billing.sepa_mandates
 (organization_id, family_email, care_contract_id, reference, signed_on, kind, sequence,
  creditor_id, account_holder_sealed, iban_sealed, iban_masked, source)
 VALUES (?, 'export-text@example.invalid', ?, ?, '2026-09-01', 'recurring', 'first',
 'DE98ZZZ09999999999', ?, ?, 'masked', 'import') RETURNING id`,
			org, contractID, reference, holder, iban).Scan(ctx, &mandateID); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.sepa_mandate_consents
 (organization_id, mandate_id, care_contract_id, channel, text_version, text_sha256, text_sealed,
 holder_confirmed, creditor_name, creditor_address, debtor_street, debtor_postal_code, debtor_city)
 VALUES (?, ?, ?, 'parent_account', 'sepa-core-de-1', repeat('a', 64), ?, true,
 'Synthetic creditor', 'Creditor street 1', 'Debtor street 2', '75175', 'City')`, org, mandateID, contractID, consentText)
		return err
	}))

	sealed, err := tenantExportSealedFields()
	require.NoError(t, err)
	database := testpkg.NewDatabase(t, db)
	exporter, err := newTenantExporter(database, sealed, tenantExportProjectedFields(), true)
	require.NoError(t, err)
	operator := testpkg.CreateTestOperator(t, db)
	out := filepath.Join(t.TempDir(), "traeger")
	require.NoError(t, exporter.Export(tenant.WithUnitOfWork(t.Context(), database.UnitOfWork()),
		organizationtenancy.TenantExportOptions{OrganizationID: org, OperatorID: operator.ID, Output: out, Uploads: t.TempDir()}))
	require.FileExists(t, filepath.Join(out, "COMPLETE"))

	rows := readExportRows(t, filepath.Join(out, "parent_billing.sepa_mandate_consents.jsonl"))
	require.Len(t, rows, 1)
	require.NotContains(t, rows[0], "mandate_reference", "the binding serves the opening only")
	require.Equal(t, []string{"text=" + text}, openedValues(t, filepath.Join(out, "parent_billing.sepa_mandate_consents.jsonl"), "text"))
	requireNoCiphertext(t, out, consentText, iban, holder)
}
