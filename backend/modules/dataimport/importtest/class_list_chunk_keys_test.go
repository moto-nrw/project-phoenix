package integration

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/api"
	importModels "github.com/moto-ogs/project-phoenix/modules/dataimport"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// TestClassListChunkKeysFollowTheOwnerMatch: School Membership matches an
// entry by NFC-normalized, case-folded names, so a composed and a decomposed
// "José" are one entry. Their keys must put them in different segments: the
// second already exists, found by the match, without a failed batch. With a
// plain lower-case key (#5827) they shared a segment, the unique index
// refused the batch, and only the replay reported the row (#5552 step 5).
func TestClassListChunkKeysFollowTheOwnerMatch(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	rows := []importModels.ClassListEntryImportRow{
		{FirstName: "José", LastName: "Gómez", SchoolClass: "1a"},
		{FirstName: "Neu", LastName: "Eins", SchoolClass: "1a"},
		{FirstName: "Jose\u0301", LastName: "Go\u0301mez", SchoolClass: "1a"},
		{FirstName: "Neu", LastName: "Zwei", SchoolClass: "1a"},
		{FirstName: "\u212Aai", LastName: "Kelvin", SchoolClass: "1a"},
		{FirstName: "Kai", LastName: "Kelvin", SchoolClass: "1A"},
	}
	run := func(t *testing.T, options api.ImportTestOptions) (classListOutcome, importModels.ImportObservation) {
		testpkg.OwnTenant(t)
		module, err := api.NewImportTestModuleWithOptions(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), options)
		require.NoError(t, err)
		actor := newImporter(t, db)
		upload := awaitUpload(t, uploadClassList(t, module, db, actor, rows, "keys.csv"))
		require.NoError(t, upload.err)
		return classListOutcomeOf(t, db, upload.result), (*module.Observations)[0]
	}
	var batch, rowPath classListOutcome
	t.Run("batch path", func(t *testing.T) {
		var observation importModels.ImportObservation
		batch, observation = run(t, api.ImportTestOptions{})
		require.Zero(t, observation.BatchFallbacks, "the keys split the namesakes before the batch")
	})
	t.Run("row path", func(t *testing.T) { rowPath, _ = run(t, api.ImportTestOptions{RowPathOnly: true}) })
	require.Equal(t, rowPath, batch)
	require.Equal(t, 4, batch.Created)
	errs := strings.Join(batch.RowErrors, "\n")
	require.Contains(t, errs, "4 error duplicate already_exists", "the decomposed José")
	require.Contains(t, errs, "7 error first_name duplicate_in_file", "the Kelvin sign, already caught in validation")
}
