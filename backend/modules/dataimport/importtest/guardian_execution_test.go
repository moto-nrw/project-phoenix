package integration

import (
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestStudentImportResolvesGuardiansFromEarlierWritesWithinEachTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	var guardianIDs []int64
	for _, city := range []string{"Köln", "Bonn"} {
		t.Run(city, func(t *testing.T) {
			testpkg.OwnTenant(t)
			actor := newImporter(t, db)
			rows := studentBatch(0, 3)
			for i := range rows {
				rows[i].Guardians[0].Email = "shared-import@example.test"
				rows[i].Guardians[0].AddressCity = ""
				rows[i].Guardians[0].PhoneNumbers = nil
			}
			rows[0].Guardians[0].AddressCity = city
			rows[0].Guardians[0].PhoneNumbers = []importModels.PhoneImportData{{PhoneNumber: "0171 1234567", PhoneType: "mobile"}}
			rows[1].Guardians[0].Email = strings.ToUpper(rows[1].Guardians[0].Email)
			rows[1].Guardians[0].Notes = "Abholung besprochen"
			rows[1].Guardians[0].PhoneNumbers = []importModels.PhoneImportData{{PhoneNumber: "0171 1234567", PhoneType: "mobile"}, {PhoneNumber: "0221 7654321", PhoneType: "home"}}

			result, err := runStudentImport(t, db, module, actor, importModels.ImportModeCreate, rows, nil)
			require.NoError(t, err)
			requireNoRowErrors(t, result)
			require.Equal(t, len(rows), result.CreatedCount)
			var guardians []struct {
				ID          int64
				LastName    string
				AddressCity string
				Notes       string
			}
			require.NoError(t, db.NewSelect().Table("users.guardian_profiles").
				Column("id", "last_name", "address_city", "notes").
				Where("tenant_id = ?", testpkg.Tenant(t)).Scan(testpkg.Ctx(t), &guardians))
			require.Len(t, guardians, 1, "later rows must reuse the guardian just created by the first row")
			guardian := guardians[0]
			require.Equal(t, rows[0].Guardians[0].LastName, guardian.LastName, "matching a profile must not replace its name")
			require.Equal(t, city, guardian.AddressCity, "empty cells must preserve the earlier row's address")
			require.Equal(t, rows[1].Guardians[0].Notes, guardian.Notes)
			var links []int64
			require.NoError(t, db.NewSelect().Table("users.student_guardian_relationships").Column("guardian_profile_id").
				Where("tenant_id = ?", testpkg.Tenant(t)).Scan(testpkg.Ctx(t), &links))
			require.Equal(t, []int64{guardian.ID, guardian.ID, guardian.ID}, links)
			var phones []string
			require.NoError(t, db.NewSelect().Table("users.guardian_phone_numbers").Column("phone_number").
				Where("tenant_id = ? AND guardian_profile_id = ?", testpkg.Tenant(t), guardian.ID).Scan(testpkg.Ctx(t), &phones))
			require.ElementsMatch(t, []string{"0171 1234567", "0221 7654321"}, phones)
			guardianIDs = append(guardianIDs, guardian.ID)
		})
	}
	require.Len(t, guardianIDs, 2)
	require.NotEqual(t, guardianIDs[0], guardianIDs[1], "the same e-mail must resolve independently in each tenant")
}

func TestStudentImportNewGuardianPhoneOrder(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	rows := studentBatch(0, 2)
	for i := range rows {
		rows[i].Guardians[0].Email = "phone-order@example.test"
	}
	rows[0].Guardians[0].PhoneNumbers = []importModels.PhoneImportData{
		{},
		{PhoneNumber: "0171 1234567", PhoneType: "unknown"},
		{PhoneNumber: "0171 1234567", PhoneType: "home", IsPrimary: true},
		{PhoneNumber: "0221 7654321", PhoneType: "home"},
		{PhoneNumber: "0221 1234567", PhoneType: "work", IsPrimary: true},
	}
	rows[1].Guardians[0].PhoneNumbers = []importModels.PhoneImportData{
		{PhoneNumber: "0171 1234567", IsPrimary: true},
		{PhoneNumber: "0172 7654321", PhoneType: "mobile"},
	}
	result, err := runStudentImport(t, db, module, actor, importModels.ImportModeCreate, rows, nil)
	require.NoError(t, err)
	requireNoRowErrors(t, result)
	require.Equal(t, len(rows), result.CreatedCount)
	var phones []struct {
		PhoneNumber string
		PhoneType   string
		Priority    int
		IsPrimary   bool
	}
	require.NoError(t, db.NewSelect().Table("users.guardian_phone_numbers").
		Column("phone_number", "phone_type", "priority", "is_primary").
		Where("tenant_id = ?", testpkg.Tenant(t)).OrderExpr("priority ASC").Scan(testpkg.Ctx(t), &phones))
	require.Len(t, phones, 4)
	for i, number := range []string{"0171 1234567", "0221 7654321", "0221 1234567", "0172 7654321"} {
		require.Equal(t, number, phones[i].PhoneNumber)
		require.Equal(t, i+1, phones[i].Priority)
		require.Equal(t, i == 2, phones[i].IsPrimary, "duplicates must not change primary; later adds must preserve it")
	}
	require.Equal(t, "other", phones[0].PhoneType)
	require.Equal(t, "home", phones[1].PhoneType)
	require.Equal(t, "work", phones[2].PhoneType)
	require.Equal(t, "mobile", phones[3].PhoneType)
}
