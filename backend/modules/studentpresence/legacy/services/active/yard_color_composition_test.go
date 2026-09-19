package active_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/services/active"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestGroupsCompositionPreservesYardColor(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	room := testpkg.CreateTestRoom(t, db, "yard color composition")
	color := "#A3D977"
	_, err := db.NewUpdate().Table("facilities.rooms").
		Set("name = ?", "Schulhof").Set("is_system = TRUE").Set("color = ?", color).
		Where("id = ? AND tenant_id = ?", room.ID, testpkg.Tenant(t)).Exec(ctx)
	require.NoError(t, err)
	module, err := services.NewGroupsTestModule(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	got := active.ResolveYardRoomColor(ctx, module.Active)
	require.NotNil(t, got)
	require.Equal(t, color, *got)
}
