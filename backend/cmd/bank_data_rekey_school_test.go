package cmd

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-ogs/project-phoenix/modules/peopledirectory/compose"
	securityRuntimeCompose "github.com/moto-ogs/project-phoenix/modules/securityruntime/compose"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-ogs/project-phoenix/modules/workforce/compose"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Synthetic values; none is a real account or person.
const (
	schoolRekeyIBAN   = "DE89370400440532013000"
	schoolRekeyHolder = "Synthetic school holder"
	schoolRekeyTaxID  = "86095742719"
	schoolRekeySocial = "65170839J003"
)

type schoolBankFixture struct {
	own, other              int64
	guardians, staff        []int64 // own school: plaintext row, legacy-sealed row
	otherGuardian, otherStf int64
}

// #6332: the school traversal of the CLI rekeys the legacy-sealed rows of
// both new sources (1.15.900 dropped the plaintext columns the backfill read). Afterwards the bank ring
// alone opens every value of all four sources (the Träger sources are covered
// by TestBankDataRekeyJobOpensBothSourcesWithoutLegacy), a second complete
// pass changes nothing, and another school stays untouched although the
// maintenance login bypasses row security.
func TestSchoolBankDataRekeyBackfillsAndOpensWithoutLegacy(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.NewDatabase(t, db).UnitOfWork())
	legacyKeys, bankKeys := testpkg.NewTestHealthDataKeys(), testpkg.NewTestHealthDataKeys()
	legacy, err := securityRuntimeCompose.NewFieldCipher(legacyKeys)
	require.NoError(t, err)
	f := arrangeSchoolBankRows(t, ctx, db, legacy)
	otherBefore := schoolBankSnapshot(t, ctx, db, f.other, true)
	businessBefore := schoolBankSnapshot(t, ctx, db, f.own, false)

	var logged bytes.Buffer
	testpkg.NewDatabase(t, db).LogQueries(slog.New(slog.NewJSONHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug})))
	transition, err := securityRuntimeCompose.NewBankDataCipher(bankKeys, legacyKeys)
	require.NoError(t, err)
	guardians, staff := peopleCompose.NewGuardianBankDataRekey(transition), workforceCompose.NewStaffBankDataRekey(transition)

	// Neither an HTTP login nor a bare context reaches the maintenance path.
	serveCtx := tenant.WithUnitOfWork(context.Background(), testpkg.NewDatabase(t, testpkg.SetupServeTestDB(t)).UnitOfWork())
	require.Error(t, runSchoolBankDataRekeyJob(serveCtx, guardians, staff, f.own, 100, &bytes.Buffer{}))
	_, err = guardians.Batch(context.Background(), f.own, 0, 100)
	require.Error(t, err)
	_, err = staff.Batch(context.Background(), f.own, 0, 100)
	require.Error(t, err)

	var output bytes.Buffer
	require.NoError(t, runSchoolBankDataRekeyJob(ctx, guardians, staff, f.own, 1, &output))
	require.Contains(t, output.String(), "source=guardian_financial scanned_total=2 changed_total=2")
	require.Contains(t, output.String(), "source=staff_financial scanned_total=2 changed_total=2")

	bankOnly, err := securityRuntimeCompose.NewBankDataCipher(bankKeys, "")
	require.NoError(t, err)
	assertSchoolBankRowsOpen(t, ctx, db, f, bankOnly, legacy)
	require.Equal(t, businessBefore, schoolBankSnapshot(t, ctx, db, f.own, false), "business fields and timestamps stay put")
	require.Equal(t, otherBefore, schoolBankSnapshot(t, ctx, db, f.other, true), "another school's rows are not touched")

	output.Reset()
	require.NoError(t, runSchoolBankDataRekeyJob(ctx, peopleCompose.NewGuardianBankDataRekey(bankOnly), workforceCompose.NewStaffBankDataRekey(bankOnly), f.own, 1, &output))
	require.Contains(t, output.String(), "source=guardian_financial scanned_total=2 changed_total=0")
	require.Contains(t, output.String(), "source=staff_financial scanned_total=2 changed_total=0")

	for _, secret := range []string{schoolRekeyIBAN, "532013000", schoolRekeyHolder, schoolRekeyTaxID, schoolRekeySocial, legacyKeys, bankKeys} {
		require.NotContains(t, output.String(), secret)
		require.NotContains(t, logged.String(), secret, "no bank value or key in the SQL debug log")
	}

	// The other school keeps its legacy values until its own run.
	output.Reset()
	require.NoError(t, runSchoolBankDataRekeyJob(ctx, guardians, staff, f.other, 100, &output))
	require.Contains(t, output.String(), "source=guardian_financial scanned_total=1 changed_total=1")
	require.Contains(t, output.String(), "source=staff_financial scanned_total=1 changed_total=1")
}

// A batch whose row does not open rolls back as a whole and reports no value.
func TestSchoolBankDataRekeyRollsBackABatchThatDoesNotOpen(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.NewDatabase(t, db).UnitOfWork())
	legacyKeys := testpkg.NewTestHealthDataKeys()
	legacy, err := securityRuntimeCompose.NewFieldCipher(legacyKeys)
	require.NoError(t, err)
	f := arrangeSchoolBankRows(t, ctx, db, legacy)
	_, err = db.NewRaw(`UPDATE users.guardian_financial_data SET account_holder_sealed = ? WHERE guardian_profile_id = ?`, []byte("invalid sealed holder"), f.guardians[1]).Exec(ctx)
	require.NoError(t, err)
	before := schoolBankSnapshot(t, ctx, db, f.own, true)
	transition, err := securityRuntimeCompose.NewBankDataCipher(testpkg.NewTestHealthDataKeys(), legacyKeys)
	require.NoError(t, err)
	var output bytes.Buffer
	err = runSchoolBankDataRekeyJob(ctx, peopleCompose.NewGuardianBankDataRekey(transition), workforceCompose.NewStaffBankDataRekey(transition), f.own, 100, &output)
	require.Error(t, err, "the invalid row does not open")
	require.NotContains(t, err.Error(), schoolRekeyIBAN)
	require.Equal(t, before, schoolBankSnapshot(t, ctx, db, f.own, true), "the row rewrapped earlier in the batch rolls back")
}

// The ports demand the bank ring: Security Runtime's health cipher cannot be
// bound to them, the bank cipher can (#6332, ADR 0183).
func TestHealthCipherCannotServeTheSchoolBankPorts(t *testing.T) {
	t.Parallel()
	health, err := securityRuntimeCompose.NewFieldCipher(testpkg.NewTestHealthDataKeys())
	require.NoError(t, err)
	bank, err := securityRuntimeCompose.NewBankDataCipher(testpkg.NewTestHealthDataKeys(), "")
	require.NoError(t, err)
	for name, port := range map[string]func(any) bool{
		"people directory": func(c any) bool { _, ok := c.(peopledirectory.BankDataCipher); return ok },
		"people rekey":     func(c any) bool { _, ok := c.(peopledirectory.BankDataRekeyCipher); return ok },
		"workforce":        func(c any) bool { _, ok := c.(workforce.BankDataCipher); return ok },
		"workforce rekey":  func(c any) bool { _, ok := c.(workforce.BankDataRekeyCipher); return ok },
	} {
		require.False(t, port(health), name)
		require.True(t, port(bank), name)
	}
}

func arrangeSchoolBankRows(t *testing.T, ctx context.Context, db *bun.DB, legacy interface {
	Seal(plaintext, context []byte) ([]byte, error)
},
) schoolBankFixture {
	t.Helper()
	f := schoolBankFixture{own: testpkg.NewTenantScope(t, db).TenantID, other: testpkg.NewTenantScope(t, db).TenantID}
	seal := func(field string, school, owner int64, value string) []byte {
		sealed, err := legacy.Seal([]byte(value), []byte(fmt.Sprintf("users.%s|%d|%d", field, school, owner)))
		require.NoError(t, err)
		return sealed
	}
	insertGuardian := func(school int64, name string) int64 {
		profile := testpkg.CreateTestGuardianProfileForTenant(t, db, school, "Rekey", name, fmt.Sprintf("rekey-%s-%d@example.invalid", strings.ToLower(name), school))
		_, err := db.NewRaw(`INSERT INTO users.guardian_financial_data (tenant_id, guardian_profile_id, iban_sealed, iban_masked, account_holder_sealed)
   VALUES (?, ?, ?, '•••• 3000', ?)`, school, profile.ID,
			seal("guardian_financial_data.iban", school, profile.ID, schoolRekeyIBAN),
			seal("guardian_financial_data.account_holder", school, profile.ID, schoolRekeyHolder)).Exec(ctx)
		require.NoError(t, err)
		return profile.ID
	}
	insertStaff := func(school int64, name string) int64 {
		member := testpkg.CreateTestStaffForTenant(t, db, school, "Rekey", name)
		_, err := db.NewRaw(`INSERT INTO users.staff_financial_data (tenant_id, staff_id, iban_sealed, iban_masked, tax_id_sealed, social_security_number_sealed)
   VALUES (?, ?, ?, '•••• 3000', ?, ?)`, school, member.ID,
			seal("staff_financial_data.iban", school, member.ID, schoolRekeyIBAN),
			seal("staff_financial_data.tax_id", school, member.ID, schoolRekeyTaxID),
			seal("staff_financial_data.social_security_number", school, member.ID, schoolRekeySocial)).Exec(ctx)
		require.NoError(t, err)
		return member.ID
	}
	f.guardians = []int64{insertGuardian(f.own, "First"), insertGuardian(f.own, "Second")}
	f.staff = []int64{insertStaff(f.own, "First"), insertStaff(f.own, "Second")}
	f.otherGuardian, f.otherStf = insertGuardian(f.other, "Other"), insertStaff(f.other, "Other")
	return f
}

// schoolBankSnapshot renders a school's rows; without bank columns it keeps
// only the business fields and timestamps.
func schoolBankSnapshot(t *testing.T, ctx context.Context, db *bun.DB, school int64, withBank bool) string {
	t.Helper()
	strip := `- ARRAY['iban_sealed','iban_masked','account_holder_sealed','tax_id_sealed','social_security_number_sealed']`
	if withBank {
		strip = ""
	}
	var guardians, staff string
	require.NoError(t, db.NewRaw(`SELECT coalesce(jsonb_agg(to_jsonb(g) `+strip+` ORDER BY id), '[]')::text FROM users.guardian_financial_data g WHERE tenant_id = ?`, school).Scan(ctx, &guardians))
	require.NoError(t, db.NewRaw(`SELECT coalesce(jsonb_agg(to_jsonb(s) `+strip+` ORDER BY id), '[]')::text FROM users.staff_financial_data s WHERE tenant_id = ?`, school).Scan(ctx, &staff))
	return guardians + staff
}

func assertSchoolBankRowsOpen(t *testing.T, ctx context.Context, db *bun.DB, f schoolBankFixture, bank, legacy interface {
	Open(sealed, context []byte) ([]byte, error)
},
) {
	t.Helper()
	open := func(field string, owner int64, sealed []byte, want string) {
		aad := []byte(fmt.Sprintf("users.%s|%d|%d", field, f.own, owner))
		plain, err := bank.Open(sealed, aad)
		require.NoError(t, err, field)
		require.Equal(t, want, string(plain), field)
		_, err = legacy.Open(sealed, aad)
		require.Error(t, err, "every value is on the bank ring now: %s", field)
	}
	for _, id := range f.guardians {
		var row struct {
			IBANSealed, HSealed []byte
			Masked              *string
		}
		require.NoError(t, db.NewRaw(`SELECT iban_sealed, account_holder_sealed AS h_sealed, iban_masked AS masked
   FROM users.guardian_financial_data WHERE tenant_id = ? AND guardian_profile_id = ?`, f.own, id).Scan(ctx, &row))
		require.Equal(t, "•••• 3000", *row.Masked)
		open("guardian_financial_data.iban", id, row.IBANSealed, schoolRekeyIBAN)
		open("guardian_financial_data.account_holder", id, row.HSealed, schoolRekeyHolder)
	}
	for _, id := range f.staff {
		var row struct {
			IBANSealed, TaxSealed, SocialSealed []byte
			Masked                              *string
		}
		require.NoError(t, db.NewRaw(`SELECT iban_sealed, tax_id_sealed AS tax_sealed,
       social_security_number_sealed AS social_sealed, iban_masked AS masked
   FROM users.staff_financial_data WHERE tenant_id = ? AND staff_id = ?`, f.own, id).Scan(ctx, &row))
		require.Equal(t, "•••• 3000", *row.Masked)
		open("staff_financial_data.iban", id, row.IBANSealed, schoolRekeyIBAN)
		open("staff_financial_data.tax_id", id, row.TaxSealed, schoolRekeyTaxID)
		open("staff_financial_data.social_security_number", id, row.SocialSealed, schoolRekeySocial)
	}
}
