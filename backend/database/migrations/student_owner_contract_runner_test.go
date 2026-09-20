package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestStudentContractInitialMigrationReachesContractedSchema(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	var applied, removed bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM public.bun_migrations WHERE name = '001015399'),
		to_regclass('users.students') IS NULL AND to_regclass('users.students_legacy') IS NULL`).Scan(t.Context(), &applied, &removed))
	require.True(t, applied)
	require.True(t, removed)
	ctx, err := studentContractRunContext(t.Context(), db)
	require.NoError(t, err)
	require.Equal(t, false, ctx.Value(freshStudentStorageKey{}), "an already initialized empty database is not a fresh replay")
}

func TestStudentContractExistingInstallationRequiresEvidence(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeContract(t)
	for _, operation := range []func(context.Context, *bun.DB) error{studentOwnerContractPrecondition, studentOwnerContractUp} {
		require.ErrorContains(t, operation(t.Context(), db), "--student-contract-evidence")
		evidence, _, _ := studentContractEvidenceFixture()
		evidence.WindowStart = evidence.WindowEnd // A point sample cannot authorize the real migration.
		ctx := WithStudentContractEvidence(t.Context(), evidence, evidence.ReleaseCommit)
		require.ErrorContains(t, operation(ctx, db), "full rollback window")
	}
	require.NoError(t, studentOwnerContractDataPreflight(t.Context(), db), "failed gates leave compatibility storage intact")
}

func TestStudentContractFreshReplayRefusesUnexpectedStudentData(t *testing.T) {
	t.Parallel()
	db, _ := studentContractFixture(t)
	ctx := context.WithValue(t.Context(), freshStudentStorageKey{}, true)
	require.ErrorContains(t, studentOwnerContractUp(ctx, db), "initial replay contains student data")
	require.NoError(t, studentOwnerContractDataPreflight(t.Context(), db))
}
