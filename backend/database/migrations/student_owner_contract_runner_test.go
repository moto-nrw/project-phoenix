package migrations

import (
	"bytes"
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestStudentContractOrdinaryUpgradeContractsExistingStorage(t *testing.T) {
	t.Parallel()
	db, _ := studentContractFixture(t)
	_, err := db.ExecContext(t.Context(), `DELETE FROM public.bun_migrations WHERE name = '001015399'`)
	require.NoError(t, err)
	before := studentContractOwnerRows(t, db)
	var output bytes.Buffer
	evidence, _, _ := studentContractEvidenceFixture()
	evidence.WindowStart = evidence.WindowEnd
	ctx := WithStudentContractEvidence(t.Context(), evidence, evidence.ReleaseCommit)
	require.ErrorContains(t, migratePreflightTo(ctx, db, &output), "full rollback window")
	require.ErrorContains(t, studentOwnerContractUp(ctx, db), "full rollback window")
	require.NoError(t, migratePreflightTo(t.Context(), db, &output))
	require.NoError(t, Migrate(t.Context(), db))
	require.Equal(t, before, studentContractOwnerRows(t, db))
	var removed, rebound bool
	require.NoError(t, db.NewRaw(`SELECT
		to_regclass('users.students') IS NULL AND to_regclass('users.students_legacy') IS NULL
		AND to_regclass('users.expired_privacy_consents') IS NULL,
		position('users.students' in pg_get_functiondef('active.count_student_visits_for_deletion(bigint,bigint)'::regprocedure)) = 0
		AND position('users.student_profiles' in pg_get_functiondef('active.count_student_visits_for_deletion(bigint,bigint)'::regprocedure)) > 0`).Scan(t.Context(), &removed, &rebound))
	require.True(t, removed, "ordinary upgrades must perform cleanup, not silently defer it")
	require.True(t, rebound, "the deletion workflow must no longer depend on the removed view")
	var applied bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT FROM public.bun_migrations WHERE name = '001015399')`).Scan(t.Context(), &applied))
	require.True(t, applied, "ordinary deployments must not remain blocked by cleanup")
	require.NoError(t, migratePreflightTo(t.Context(), db, &output))
}

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

func TestStudentContractExplicitEvidenceStillRequiresValidation(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeContract(t)
	for _, operation := range []func(context.Context, *bun.DB) error{studentOwnerContractPrecondition, studentOwnerContractUp} {
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

func TestStudentContractOrdinaryUpgradeStillRejectsCompatibilityHits(t *testing.T) {
	t.Parallel()
	db := setupStudentStorageBeforeContract(t)
	_, err := db.ExecContext(t.Context(), `DELETE FROM public.bun_migrations WHERE name = '001015399'; SELECT nextval('users.student_compatibility_reads')`)
	require.NoError(t, err)
	var output bytes.Buffer
	require.ErrorContains(t, migratePreflightTo(t.Context(), db, &output), "compatibility hits")
	require.ErrorContains(t, Migrate(t.Context(), db), "compatibility hits")
	var retained, applied bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('users.students') IS NOT NULL AND to_regclass('users.students_legacy') IS NOT NULL,
		EXISTS (SELECT FROM public.bun_migrations WHERE name = '001015399')`).Scan(t.Context(), &retained, &applied))
	require.True(t, retained)
	require.False(t, applied)
}
