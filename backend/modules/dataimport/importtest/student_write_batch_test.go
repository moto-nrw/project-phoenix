package integration

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/api"
	dataimport "github.com/moto-ogs/project-phoenix/modules/dataimport"
	"github.com/moto-ogs/project-phoenix/modules/dataimport/ports"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

func TestStudentWriteBatchSeesEarlierNamesakesAndPreservesErrorOrder(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, people, _, err := api.NewImportTestModuleWithOwners(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.ImportTestOptions{})
	require.NoError(t, err)
	actor := newImporter(t, db)
	testpkg.CreateTestStudent(t, db, "Twin", "Stored", "1A")
	testpkg.CreateTestStudent(t, db, "Twin", "Stored", "1A")
	testpkg.CreateTestStudent(t, db, "Existing", "Stored", "1A")
	before := countImportRows(t, db, testpkg.Tenant(t))
	rows := []dataimport.StudentImportRow{
		{FirstName: "Batch", LastName: "Same", SchoolClass: "1A", DataRetentionDays: 30},
		{FirstName: " batch ", LastName: " SAME ", SchoolClass: "1a", DataRetentionDays: 30},
		{FirstName: "Batch", LastName: "Same", SchoolClass: "2B", DataRetentionDays: 30},
		{FirstName: "BATCH", LastName: "SAME", SchoolClass: "2b", DataRetentionDays: 30},
		{FirstName: "Twin", LastName: "Stored", SchoolClass: "1A", DataRetentionDays: 30},
		{FirstName: "Existing", LastName: "Stored", SchoolClass: "1A", DataRetentionDays: 30},
		{FirstName: "Last", LastName: "Created", SchoolClass: "3C", DataRetentionDays: 30},
		{FirstName: "Other", LastName: "Created", SchoolClass: "3C", DataRetentionDays: 30},
		{LastName: "Invalid", SchoolClass: "1A", DataRetentionDays: 30},
	}
	result, err := module.Import.ImportBatches(testpkg.Ctx(t), dataimport.ImportRequest[dataimport.StudentImportRow]{
		Rows: rows, Mode: dataimport.ImportModeCreate, UserID: actor.staffID, SkipInvalidRows: true,
	}, dataimport.BatchAudit{EntityType: "student", Filename: "namesakes.csv", AccountID: actor.accountID})
	require.NoError(t, err)
	require.Equal(t, 4, result.CreatedCount)
	require.Equal(t, 5, result.ErrorCount)
	require.Len(t, result.Errors, 5)
	// Validation errors precede write-phase match errors; file row numbers
	// and the original duplicate/ambiguity distinction remain unchanged.
	for i, row := range []int{10, 3, 5, 6, 7} {
		require.Equal(t, row, result.Errors[i].RowNumber)
	}
	for i, code := range []string{"already_exists", "already_exists", "duplicate_check_failed", "already_exists"} {
		require.Equal(t, code, result.Errors[i+1].Errors[0].Code)
	}
	after := countImportRows(t, db, testpkg.Tenant(t))
	require.Equal(t, before.persons+4, after.persons)
	require.Equal(t, before.students+4, after.students)
	persons, err := people.SearchPersons(testpkg.Ctx(t), ports.PersonFilter{Names: []ports.PersonName{
		{FirstName: "Batch", LastName: "Same"}, {FirstName: "Last", LastName: "Created"},
	}})
	require.NoError(t, err)
	require.Len(t, persons, 3)
	var first, second, last int64
	for _, person := range persons {
		if person.FirstName == "Last" {
			last = person.ID
		} else if first == 0 || person.ID < first {
			second, first = first, person.ID
		} else {
			second = person.ID
		}
	}
	require.Positive(t, first)
	require.Greater(t, second, first, "same-name children retain upload insertion order")
	require.Greater(t, last, second)
}

func TestStudentWriteBatchKeepsCanonicalNameEquality(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	rows := []dataimport.StudentImportRow{
		{FirstName: "José", LastName: "Müller", SchoolClass: "1A", DataRetentionDays: 30},
		{FirstName: "Jose\u0301", LastName: "Mu\u0308ller", SchoolClass: "1A", DataRetentionDays: 30},
		{FirstName: "Jose", LastName: "Muller", SchoolClass: "1A", DataRetentionDays: 30},
		{FirstName: "Another", LastName: "Child", SchoolClass: "1A", DataRetentionDays: 30},
		{FirstName: "Last", LastName: "Child", SchoolClass: "1A", DataRetentionDays: 30},
	}
	before := countImportRows(t, db, testpkg.Tenant(t))
	result, err := module.Import.ImportBatches(testpkg.Ctx(t), dataimport.ImportRequest[dataimport.StudentImportRow]{
		Rows: rows, Mode: dataimport.ImportModeCreate, UserID: actor.staffID, SkipInvalidRows: true,
	}, dataimport.BatchAudit{EntityType: "student", Filename: "canonical.csv", AccountID: actor.accountID})
	require.NoError(t, err)
	require.Equal(t, 4, result.CreatedCount)
	require.Equal(t, 1, result.ErrorCount)
	require.Len(t, result.Errors, 1)
	require.Equal(t, 3, result.Errors[0].RowNumber)
	require.Equal(t, "already_exists", result.Errors[0].Errors[0].Code)
	require.Equal(t, before.students+4, countImportRows(t, db, testpkg.Tenant(t)).students)
}
