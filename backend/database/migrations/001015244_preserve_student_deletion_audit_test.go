package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreserveStudentDeletionAuditMigration(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	var foreignKeyExists bool
	require.NoError(t, db.NewRaw(`
		SELECT EXISTS (
			SELECT 1
			FROM pg_constraint
			WHERE conrelid = 'audit.data_deletions'::regclass
			  AND conname = 'fk_data_deletions_student'
		)
	`).Scan(context.Background(), &foreignKeyExists))
	assert.False(t, foreignKeyExists, "permanent deletion audit rows must outlive their student")

	var functionExists bool
	require.NoError(t, db.NewRaw(`
		SELECT EXISTS (
			SELECT 1
			FROM pg_proc
			WHERE oid = 'active.count_student_visits_for_deletion(bigint,bigint)'::regprocedure
		)
	`).Scan(context.Background(), &functionExists))
	assert.True(t, functionExists)
}
