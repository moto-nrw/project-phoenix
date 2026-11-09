package integration

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A row written on the row path keeps the row path's retry budget even inside
// the chunk executor (#5827 review): four 40P01 on a lone row end with the
// error and write nothing, as on the head before the executor. Only a failed
// batch command replays a chunk.

// requireRowPathDeadlockOutcome checks the row path's answer to four 40P01:
// the error after three retries, no replay, nothing written.
func requireRowPathDeadlockOutcome(t *testing.T, db *bun.DB, err error, observation importModels.ImportObservation) {
	t.Helper()
	var state interface{ Field(byte) string }
	require.True(t, errors.As(err, &state), "%v", err)
	require.Equal(t, "40P01", state.Field('C'))
	require.Equal(t, 3, observation.BatchesRetried, "the row path's three retries, no more")
	require.Zero(t, observation.BatchFallbacks, "no batch command ran, so nothing replays")
	require.Zero(t, countIn(t, db, `SELECT count(*) FROM users.class_list_entries WHERE tenant_id = ?`))
}

func runClassListDeadlocks(t *testing.T, rows []importModels.ClassListEntryImportRow) {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	installDeadlockScript(t, db, 4)
	module, err := api.NewImportTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	actor := newImporter(t, db)
	upload := awaitUpload(t, uploadClassList(t, module, db, actor, rows, "lone-row.csv"))
	requireRowPathDeadlockOutcome(t, db, upload.err, (*module.Observations)[0])
}

func TestOneRowChunkDeadlocksKeepTheRowPathBudget(t *testing.T) {
	t.Parallel()
	runClassListDeadlocks(t, []importModels.ClassListEntryImportRow{{FirstName: "Allein", LastName: "Zeile", SchoolClass: "1a"}})
}

// A chunk that validation filters down to one accepted row behaves alike.
func TestFilteredToOneRowChunkDeadlocksKeepTheRowPathBudget(t *testing.T) {
	t.Parallel()
	runClassListDeadlocks(t, []importModels.ClassListEntryImportRow{
		{FirstName: "Allein", LastName: "Zeile", SchoolClass: "1a"},
		{FirstName: "Ohne", LastName: "", SchoolClass: "1a"},
	})
}

// Two rows of one chunk that share a chunk key are two one-row segments: the
// chunk runs as a batch attempt, but no batch command runs, so a row's 40P01
// keeps the row path's budget there too.
func TestOneRowSegmentDeadlocksKeepTheRowPathBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	installDeadlockScript(t, db, 4)
	runtime, observations, err := api.NewImportTestRuntime(testpkg.NewDatabase(t, db), api.ImportTestOptions{})
	require.NoError(t, err)
	importer := probeImportOver(t, db, runtime, probedClassList{chunkKeyByName: map[string]string{"Gleich": "same"}}, 0)
	actor := newImporter(t, db)
	ctx := tenant.WithUnitOfWork(testpkg.Ctx(t), testpkg.TenantRuntime(t, db))
	_, err = importer.ImportBatches(ctx, importModels.ImportRequest[importModels.ClassListEntryImportRow]{
		Rows: []importModels.ClassListEntryImportRow{
			{FirstName: "Gleich", LastName: "Eins", SchoolClass: "1a"},
			{FirstName: "Gleich", LastName: "Zwei", SchoolClass: "1a"},
		},
		Mode: importModels.ImportModeCreate, UserID: actor.staffID, SkipInvalidRows: true,
	}, importModels.BatchAudit{EntityType: "class_list_entries", Filename: "segments.csv", AccountID: actor.accountID})
	requireRowPathDeadlockOutcome(t, db, err, (*observations)[0])
}
