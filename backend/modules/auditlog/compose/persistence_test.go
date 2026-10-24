package compose

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The Audit composition builds the persistence the repository factory used to
// hold (ADR 0049).
func TestNewPersistenceBuildsTheRetainedRepositories(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	audits, err := NewPersistence(PersistenceDependencies{DB: db})
	require.NoError(t, err)

	assert.NotNil(t, audits.AppendStore)
	assert.NotNil(t, audits.DataDeletion)
	assert.NotNil(t, audits.AuthEvent)
	assert.NotNil(t, audits.DataImport)
	assert.NotNil(t, audits.OperatorAuditLog)
	assert.NotNil(t, audits.RetentionSummaries)
	assert.Nil(t, audits.BookingConsistency, "the booking audit needs the Enrollment projection")
}

func TestNewPersistenceRequiresADatabaseOrRuntime(t *testing.T) {
	t.Parallel()

	_, err := NewPersistence(PersistenceDependencies{})
	require.Error(t, err)
}
