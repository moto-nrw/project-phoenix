package peopledirectory_test

import (
	"context"
	"testing"

	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/stretchr/testify/require"
)

// TestVerifyStudentSchema_FullyMigrated pins the startup guard's happy path:
// against a fully migrated schema (the test database) it must pass, so the
// server boots. The missing-column rejection is covered in
// TestStudentRepository_CompanionNoteSchemaCompatibility.
func TestVerifyStudentSchema_FullyMigrated(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	require.NoError(t, peopleCompose.VerifyRetainedStudentSchema(context.Background(), db))
}
