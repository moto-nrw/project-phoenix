package compose

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestPersistenceRequiresAmbientTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.AssertAmbientDatabase(t, db, NewLegacyRepositoryRuntime(db).DB)
}
