package compose_test

import (
	"testing"

	settingsCompose "github.com/moto-ogs/project-phoenix/modules/settings/compose"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

func TestPersistenceRequiresAmbientTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.AssertAmbientDatabase(t, db, settingsCompose.NewTenantSettingsRuntime(db, nil).DB)
}
