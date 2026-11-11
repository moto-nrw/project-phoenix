package integration

import (
	"fmt"
	"slices"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// One import chunk updates the staff rows of several people (#6702). The
// staff rows are one ascending class of the unit of work's lock order, so
// the chunk takes all of them, ascending, before its first row: a file that
// lists a higher person before a lower one, each with a membership change
// and a teacher row behind it, imports instead of being refused.
func TestStaffImportUpdatesPeopleInAnyFileOrder(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	role := testpkg.CreateTestRoleForTenant(t, db, "Betreuungskraft", testpkg.Tenant(t))
	ctx := tenant.WithUnitOfWork(testpkg.Ctx(t), testpkg.TenantRuntime(t, db))
	ctx = importModels.ContextWithImporterPermissions(ctx, []string{"admin:*"})
	counter := testpkg.CaptureQueriesForContext(t, db)
	runner := importBudgetRun[importModels.StaffImportRow]{
		importer: module.StaffImport, ctx: counter.Context(ctx), counter: counter, userID: actor.accountID,
		audit: importModels.BatchAudit{EntityType: "staff", Filename: "order.csv", AccountID: actor.accountID},
	}
	rows := make([]importModels.StaffImportRow, 0, 3)
	for i := range 3 {
		rows = append(rows, importModels.StaffImportRow{
			FirstName: fmt.Sprintf("Order%d", i), LastName: "Staff", RoleName: role.Name,
			PersonnelNumber: fmt.Sprintf("O-%d", i), Position: "Erzieherin",
		})
	}
	runner.run(t, rows, importModels.ImportModeCreate, false, allCreated(3))
	slices.Reverse(rows)
	for i := range rows {
		rows[i].StaffNotes = "Neu im Team"
		rows[i].Position = "Leitung"
	}
	runner.run(t, rows, importModels.ImportModeUpdate, false, allUpdated(3))
	var notes int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.staff_school_memberships s JOIN users.persons p ON p.id = s.person_id
		JOIN users.staff_employment_profiles ep ON ep.membership_id = s.id AND ep.tenant_id = s.tenant_id
		WHERE s.tenant_id = ? AND p.first_name LIKE 'Order%' AND ep.staff_notes = 'Neu im Team'`, testpkg.Tenant(t)).Scan(ctx, &notes))
	require.Equal(t, 3, notes, "every person's membership was updated")
}
