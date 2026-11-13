package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/api"
	importModels "github.com/moto-ogs/project-phoenix/modules/dataimport"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// One student import serves every school of the process. A group name one
// school's upload resolved must neither resolve nor be suggested in another
// school's preview, and its group ID must never reach that school's child.
func TestStudentImportResolvesGroupNamesOnlyInTheUploadingSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	importBatches := func(t *testing.T, row importModels.StudentImportRow, dryRun bool) *importModels.ImportResult[importModels.StudentImportRow] {
		t.Helper()
		actor := newImporter(t, db)
		result, err := module.Import.ImportBatches(testpkg.Ctx(t), importModels.ImportRequest[importModels.StudentImportRow]{
			Rows: []importModels.StudentImportRow{row}, Mode: importModels.ImportModeCreate, UserID: actor.staffID, SkipInvalidRows: true, DryRun: dryRun,
		}, importModels.BatchAudit{EntityType: "student", Filename: "groups.csv", AccountID: actor.accountID})
		require.NoError(t, err)
		return result
	}
	groupWarnings := func(result *importModels.ImportResult[importModels.StudentImportRow]) []importModels.ValidationError {
		var warnings []importModels.ValidationError
		for _, rowErr := range result.Errors {
			for _, e := range rowErr.Errors {
				if e.Field == "group" {
					warnings = append(warnings, e)
				}
			}
		}
		return warnings
	}

	groupCodes := func(warnings []importModels.ValidationError) []string {
		codes := []string{}
		for _, warning := range warnings {
			codes = append(codes, warning.Code)
		}
		return codes
	}

	var groupName string
	t.Run("school A resolves its own group", func(t *testing.T) {
		testpkg.OwnTenant(t)
		groupName = testpkg.CreateTestEducationGroup(t, db, "Sonnengruppe").Name
		result := importBatches(t, importModels.StudentImportRow{FirstName: "Ada", LastName: "Eigene", SchoolClass: "1A", DataRetentionDays: 30, GroupName: groupName}, true)
		require.Empty(t, groupWarnings(result), "the uploading school resolves its own group")
	})

	t.Run("school B without groups", func(t *testing.T) {
		testpkg.OwnTenant(t)
		preview := importBatches(t, importModels.StudentImportRow{FirstName: "Ben", LastName: "Fremde", SchoolClass: "1A", DataRetentionDays: 30, GroupName: groupName}, true)
		assert.Equal(t, []string{"group_not_found"}, groupCodes(groupWarnings(preview)), "another school's group must not resolve")

		typo := importBatches(t, importModels.StudentImportRow{FirstName: "Ben", LastName: "Fremde", SchoolClass: "1A", DataRetentionDays: 30, GroupName: groupName + "x"}, true)
		warnings := groupWarnings(typo)
		assert.Equal(t, []string{"group_not_found"}, groupCodes(warnings), "another school's group name must not be suggested")
		for _, warning := range warnings {
			assert.Empty(t, warning.Suggestions, "another school's group name must not be suggested")
		}

		commit := importBatches(t, importModels.StudentImportRow{FirstName: "Ben", LastName: "Fremde", SchoolClass: "1A", DataRetentionDays: 30, GroupName: groupName}, false)
		requireNoRowErrors(t, commit)
		require.Equal(t, 1, commit.CreatedCount)
		var memberships, grouped int
		require.NoError(t, db.NewRaw(`SELECT count(*), count(m.group_id) FROM users.student_school_memberships m JOIN users.student_profiles s ON s.id = m.student_profile_id JOIN users.persons p ON p.id = s.person_id WHERE m.tenant_id = ? AND p.last_name = 'Fremde'`, testpkg.Tenant(t)).Scan(testpkg.Ctx(t), &memberships, &grouped))
		require.Equal(t, 1, memberships)
		assert.Zero(t, grouped, "the child must not carry another school's group")
	})
}
