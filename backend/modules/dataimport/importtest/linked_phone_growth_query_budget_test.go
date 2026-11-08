package integration

import (
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestStudentImportLinkedPhoneGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	for _, n := range []int{3, 8} {
		t.Run(fmt.Sprintf("N%d", n), func(t *testing.T) {
			testpkg.OwnTenant(t)
			actor := newImporter(t, db)
			row := importModels.StudentImportRow{FirstName: "Linked", LastName: "Phones", SchoolClass: "1A", DataRetentionDays: 30}
			for i := range n {
				row.Guardians = append(row.Guardians, importModels.GuardianImportData{
					FirstName: "Guardian", LastName: fmt.Sprintf("Contact%d", i), Email: fmt.Sprintf("linked%d@example.test", i), RelationshipType: "Mutter",
					PhoneNumbers: []importModels.PhoneImportData{{PhoneNumber: fmt.Sprintf("0151 %06d", i), PhoneType: "mobile"}},
				})
			}
			result, err := runStudentImport(t, db, module, actor, importModels.ImportModeCreate, []importModels.StudentImportRow{row}, nil)
			require.NoError(t, err)
			requireNoRowErrors(t, result)
			before := countImportRows(t, db, testpkg.Tenant(t))
			// Only the first linked guardian is merged. Loading its siblings'
			// phones must not issue one statement per relationship.
			row.Guardians = row.Guardians[:1]
			row.Guardians[0].PhoneNumbers = nil
			counter := testpkg.CaptureQueriesForContext(t, db)
			result, err = module.Import.ImportBatches(counter.Context(testpkg.Ctx(t)), importModels.ImportRequest[importModels.StudentImportRow]{
				Rows: []importModels.StudentImportRow{row}, Mode: importModels.ImportModeUpdate, UserID: actor.staffID, SkipInvalidRows: true,
			}, importModels.BatchAudit{EntityType: "student", Filename: "linked-phones.csv", AccountID: actor.accountID})
			require.NoError(t, err)
			requireNoRowErrors(t, result)
			require.Equal(t, 1, result.UpdatedCount)
			t.Logf("linked guardians=%d statements=%d phone_reads=%d", n, counter.Total(), len(counter.Selects("users.guardian_phone_numbers")))
			querybudgets.AssertQueryBudget(t, "modules.dataimport.student_batch.linked_phones", counter.Measurement(counter.Queries()))
			require.Len(t, counter.Selects("users.guardian_phone_numbers"), 1, "reuse the ID-set phone read of ListStudentGuardians")
			after := countImportRows(t, db, testpkg.Tenant(t))
			require.Equal(t, before.guardians, after.guardians)
			require.Equal(t, before.links, after.links)
			require.Equal(t, before.phones, after.phones)
		})
	}
}
