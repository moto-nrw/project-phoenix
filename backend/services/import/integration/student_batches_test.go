package integration

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	importModels "github.com/moto-nrw/project-phoenix/models/import"
	"github.com/moto-nrw/project-phoenix/services"
	importService "github.com/moto-nrw/project-phoenix/services/import"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestImportBatches_StudentCrossBatchGuardianAndExplicitPermissionRevocation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := services.NewImportTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	rows := make([]importModels.StudentImportRow, 101)
	for i := range rows {
		rows[i] = importModels.StudentImportRow{
			FirstName: fmt.Sprintf("Kind%d", i), LastName: "Sharedguardian", SchoolClass: "1A", DataRetentionDays: 30,
			Guardians: []importModels.GuardianImportData{{FirstName: "Maria", LastName: "Sharedguardian", Email: "shared@example.test", RelationshipType: "Mutter", CanPickup: true, IsPrimary: true, IsEmergencyContact: true}},
		}
	}
	audit := importService.BatchAudit{EntityType: "student", Filename: "siblings.csv", AccountID: actor.accountID}
	request := importModels.ImportRequest[importModels.StudentImportRow]{Rows: rows, Mode: importModels.ImportModeCreate, UserID: actor.staffID, SkipInvalidRows: true}
	result, err := module.Import.ImportBatches(testpkg.Ctx(t), request, audit)
	require.NoError(t, err)
	requireNoRowErrors(t, result)
	assert.Equal(t, 101, result.CreatedCount)
	counts := countImportRows(t, db, testpkg.Tenant(t))
	assert.Equal(t, 1, counts.guardians, "later rows and batches reuse the guardian written by the first row")
	assert.Equal(t, 101, counts.links)
	assert.Equal(t, 2, counts.audits)
	result, err = module.Import.ImportBatches(testpkg.Ctx(t), request, audit)
	require.NoError(t, err)
	assert.Equal(t, 101, result.CreatedCount)
	assert.Equal(t, counts, countImportRows(t, db, testpkg.Tenant(t)))

	// Blank columns preserve permissions. Explicit false values must be a
	// different checkpoint identity even though their public JSON is equal.
	update := rows[0]
	update.Guardians = append([]importModels.GuardianImportData(nil), update.Guardians...)
	update.Guardians[0].CanPickup = false
	update.Guardians[0].IsPrimary = false
	update.Guardians[0].IsEmergencyContact = false
	request.Rows, request.Mode = []importModels.StudentImportRow{update}, importModels.ImportModeUpdate
	check := func(want bool) {
		t.Helper()
		var flags struct{ CanPickup, IsPrimary, IsEmergencyContact bool }
		require.NoError(t, db.NewSelect().TableExpr("users.students_guardians sg").
			ColumnExpr("sg.can_pickup, sg.is_primary, sg.is_emergency_contact").
			Join("JOIN users.students s ON s.id = sg.student_id").Join("JOIN users.persons p ON p.id = s.person_id").
			Where("sg.tenant_id = ? AND p.first_name = ?", testpkg.Tenant(t), rows[0].FirstName).Scan(testpkg.Ctx(t), &flags))
		assert.Equal(t, want, flags.CanPickup)
		assert.Equal(t, want, flags.IsPrimary)
		assert.Equal(t, want, flags.IsEmergencyContact)
	}
	result, err = module.Import.ImportBatches(testpkg.Ctx(t), request, audit)
	require.NoError(t, err)
	requireNoRowErrors(t, result)
	check(true)
	request.Rows[0].Guardians[0].CanPickupSet = true
	request.Rows[0].Guardians[0].IsPrimarySet = true
	request.Rows[0].Guardians[0].IsEmergencyContactSet = true
	result, err = module.Import.ImportBatches(testpkg.Ctx(t), request, audit)
	require.NoError(t, err)
	requireNoRowErrors(t, result)
	assert.Equal(t, 1, result.UpdatedCount)
	check(false)
	assert.Equal(t, 4, countImportRows(t, db, testpkg.Tenant(t)).audits, "absent and explicit false are separate imports")
}
