package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	dataimport "github.com/moto-nrw/project-phoenix/modules/dataimport"
	importCompose "github.com/moto-nrw/project-phoenix/modules/dataimport/compose"
	imports "github.com/moto-nrw/project-phoenix/modules/dataimport/compose/imports"
	"github.com/moto-nrw/project-phoenix/modules/dataimport/ports"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Hiding only the preview capability keeps the original scalar path available
// as a behavioral reference, including its batch validation and quota rule.
type scalarStudentPreview struct {
	ports.ImportConfig[dataimport.StudentImportRow]
}

func (c scalarStudentPreview) ValidateBatch(ctx context.Context, rows []dataimport.StudentImportRow) map[int][]dataimport.ValidationError {
	return c.ImportConfig.(ports.BatchValidator[dataimport.StudentImportRow]).ValidateBatch(ctx, rows)
}

func (c scalarStudentPreview) AddsToChildQuota(ctx context.Context, row dataimport.StudentImportRow, id *int64) (bool, error) {
	return c.ImportConfig.(ports.ChildQuotaCounter[dataimport.StudentImportRow]).AddsToChildQuota(ctx, row, id)
}

func previewConfigForTest(persons ports.PersonDirectory, students ports.StudentDirectory) ports.ImportConfig[dataimport.StudentImportRow] {
	empty := func(context.Context) ([]dataimport.Reference, error) { return nil, nil }
	return imports.NewStudentImportConfig(ports.StudentImportDeps{
		Transactions: importCompose.NewTransactions(), Persons: persons, Students: students,
		Resolver: imports.NewRelationshipResolver(empty, empty), CountsTowardChildQuota: api.ImportChildQuotaRule(),
	})
}

func runPreviewForTest(t *testing.T, ctx context.Context, config ports.ImportConfig[dataimport.StudentImportRow], request dataimport.ImportRequest[dataimport.StudentImportRow]) *dataimport.ImportResult[dataimport.StudentImportRow] {
	t.Helper()
	encoded, err := json.Marshal(request.Rows)
	require.NoError(t, err)
	request.Rows = nil
	require.NoError(t, json.Unmarshal(encoded, &request.Rows))
	result, err := imports.NewImportService(config, ports.ImportRuntime{}).Import(ctx, request)
	require.NoError(t, err)
	result.StartedAt, result.CompletedAt = time.Time{}, time.Time{}
	for index := range result.Errors {
		result.Errors[index].Timestamp = time.Time{}
	}
	return result
}

func TestStudentPreviewBatchPreservesScalarResultsAndRowErrorOrder(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	_, people, _, err := api.NewImportTestModuleWithOwners(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.ImportTestOptions{})
	require.NoError(t, err)
	for _, name := range []struct{ first, last string }{{"Mila", "Match"}, {"Doppelt", "Match"}, {"Doppelt", "Match"}, {"Nora", "Other"}} {
		testpkg.CreateTestStudent(t, db, name.first, name.last, "1A")
	}
	before := countImportRows(t, db, testpkg.Tenant(t))
	for _, mode := range []dataimport.ImportMode{dataimport.ImportModeCreate, dataimport.ImportModeUpdate, dataimport.ImportModeUpsert} {
		for _, stop := range []bool{false, true} {
			request := dataimport.ImportRequest[dataimport.StudentImportRow]{Mode: mode, DryRun: true, StopOnError: stop, Rows: []dataimport.StudentImportRow{
				{FirstName: " mila ", LastName: " MATCH ", SchoolClass: "1a", DataRetentionDays: 30},
				{FirstName: "Neu", LastName: "Match", SchoolClass: "1A", DataRetentionDays: 30},
				{FirstName: "Doppelt", LastName: "Match", SchoolClass: "1A", DataRetentionDays: 30},
				{LastName: "Invalid", SchoolClass: "", Birthday: "invalid", DataRetentionDays: 90},
				// Neither cross-combining the name pairs nor matching a person
				// of another class may turn these rows into an update.
				{FirstName: "Mila", LastName: "Other", SchoolClass: "1A", DataRetentionDays: 30},
				{FirstName: "Nora", LastName: "Other", SchoolClass: "2B", DataRetentionDays: 30},
			}}
			require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
				want := runPreviewForTest(t, ctx, scalarStudentPreview{previewConfigForTest(people, people)}, request)
				got := runPreviewForTest(t, ctx, previewConfigForTest(people, people), request)
				require.Equal(t, want, got, "mode=%s stop=%t", mode, stop)
				return nil
			}))
		}
	}
	require.Equal(t, before, countImportRows(t, db, testpkg.Tenant(t)), "preview does not mutate any imported owner")
}

type failingPreviewPersons struct {
	ports.PersonDirectory
	fail func(context.Context) error
}

func (p failingPreviewPersons) SearchPersons(ctx context.Context, filter ports.PersonFilter) ([]ports.Person, error) {
	if len(filter.Names) > 0 {
		return nil, p.fail(ctx)
	}
	return p.PersonDirectory.SearchPersons(ctx, filter)
}

type failingPreviewStudents struct {
	ports.StudentDirectory
	fail func(context.Context) error
}

func (s failingPreviewStudents) ListStudentsByPersonID(ctx context.Context, ids []int64) ([]ports.Student, error) {
	if len(ids) > 1 {
		return nil, s.fail(ctx)
	}
	return s.StudentDirectory.ListStudentsByPersonID(ctx, ids)
}

func TestStudentPreviewBulkReadFailureRestoresScalarValidation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	_, people, _, err := api.NewImportTestModuleWithOwners(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.ImportTestOptions{})
	require.NoError(t, err)
	testpkg.CreateTestStudent(t, db, "Mila", "First", "1A")
	testpkg.CreateTestStudent(t, db, "Nora", "Second", "1A")
	request := dataimport.ImportRequest[dataimport.StudentImportRow]{Mode: dataimport.ImportModeUpdate, DryRun: true, Rows: []dataimport.StudentImportRow{
		{FirstName: "Mila", LastName: "First", SchoolClass: "1A", DataRetentionDays: 30},
		{FirstName: "Nora", LastName: "Second", SchoolClass: "1A", Birthday: "invalid", DataRetentionDays: 90},
	}}
	for _, stage := range []string{"persons", "students"} {
		require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
			want := runPreviewForTest(t, ctx, scalarStudentPreview{previewConfigForTest(people, people)}, request)
			calls := 0
			fail := func(ctx context.Context) error {
				calls++
				_, err := tx.NewRaw("SELECT 1 / 0").Exec(ctx)
				require.Error(t, err)
				return err
			}
			var persons ports.PersonDirectory = people
			var students ports.StudentDirectory = people
			if stage == "persons" {
				persons = failingPreviewPersons{PersonDirectory: people, fail: fail}
			} else {
				students = failingPreviewStudents{StudentDirectory: people, fail: fail}
			}
			got := runPreviewForTest(t, ctx, previewConfigForTest(persons, students), request)
			require.Equal(t, 1, calls)
			require.Equal(t, want, got, stage)
			return nil
		}))
	}
}

func TestStudentPreviewNameSetKeepsExactPairsAndTenantScope(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	_, people, _, err := api.NewImportTestModuleWithOwners(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), api.ImportTestOptions{})
	require.NoError(t, err)
	first := testpkg.CreateTestPerson(t, db, "Mila", "Match")
	second := testpkg.CreateTestPerson(t, db, "Nora", "Other")
	namesake := testpkg.CreateTestPerson(t, db, "Mila", "Match")
	testpkg.CreateTestPerson(t, db, "Mila", "Other")
	testpkg.CreateTestPerson(t, db, "Norabelle", "Other")
	foreign := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreign)
	testpkg.CreateTestPersonForTenant(t, db, foreign, "Mila", "Match")
	matches, err := people.SearchPersons(testpkg.Ctx(t), ports.PersonFilter{Names: []ports.PersonName{
		{FirstName: " mila ", LastName: "MATCH"}, {FirstName: "NORA", LastName: " other "},
	}})
	require.NoError(t, err)
	ids := make([]int64, 0, len(matches))
	for _, person := range matches {
		require.Equal(t, testpkg.Tenant(t), person.TenantID)
		ids = append(ids, person.ID)
	}
	require.ElementsMatch(t, []int64{first.ID, second.ID, namesake.ID}, ids)
}
