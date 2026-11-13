package delivery

import (
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

func TestSubscriptionPersistenceRequiresAmbientTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repository := &PushSubscriptionRepository{DB: db}
	testpkg.AssertAmbientDatabase(t, db, repository.database)
}
