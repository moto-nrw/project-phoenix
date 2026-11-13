package compose

import (
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

func TestPersistenceRequiresAmbientTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.AssertAmbientDatabase(t, db, PersistenceRuntimeFor(db).Database)
}
