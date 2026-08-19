package facilities_test

import (
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	facilitiesSvc "github.com/moto-nrw/project-phoenix/services/facilities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFacilitiesService_ToiletteAliasSystemProtection(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupFacilitiesService(t, db)
	tenantID := createFacilityTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)

	t.Run("blocks renaming system room Toilette", func(t *testing.T) {
		room := createRoomWithExactName(t, db, tenantID, constants.WCRoomAliasName)
		defer cleanupRoom(t, db, tenantID, room.ID)

		room.Name = "Ruheraum"
		err := service.UpdateRoom(ctx, room)

		require.Error(t, err)
		require.True(t, errors.Is(err, facilitiesSvc.ErrSystemRoomProtected))
	})

	t.Run("blocks deletion of system room Toilette", func(t *testing.T) {
		room := createRoomWithExactName(t, db, tenantID, constants.WCRoomAliasName)
		defer cleanupRoom(t, db, tenantID, room.ID)

		err := service.DeleteRoom(ctx, room.ID)

		require.Error(t, err)
		require.True(t, errors.Is(err, facilitiesSvc.ErrSystemRoomProtected))
	})

	t.Run("blocks color change on system room Toilette", func(t *testing.T) {
		room := createRoomWithExactName(t, db, tenantID, constants.WCRoomAliasName)
		defer cleanupRoom(t, db, tenantID, room.ID)

		newColor := "#A3D977"
		room.Color = &newColor
		err := service.UpdateRoom(ctx, room)

		require.Error(t, err)
		require.True(t, errors.Is(err, facilitiesSvc.ErrSystemRoomProtected))
	})
}

func TestFacilitiesService_CreateRoom_RejectsWCRoomAliasDuplicates(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupFacilitiesService(t, db)
	tenantID := createFacilityTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)

	t.Run("rejects Toilette when WC already exists", func(t *testing.T) {
		room := createRoomWithExactName(t, db, tenantID, constants.WCRoomName)
		defer cleanupRoom(t, db, tenantID, room.ID)

		aliasRoom := &facilities.Room{Name: constants.WCRoomAliasName, Building: "Test Building"}
		err := service.CreateRoom(ctx, aliasRoom)

		require.Error(t, err)
		assert.True(t, errors.Is(err, facilitiesSvc.ErrDuplicateToiletRoom))
	})

	t.Run("rejects WC when Toilette already exists", func(t *testing.T) {
		room := createRoomWithExactName(t, db, tenantID, constants.WCRoomAliasName)
		defer cleanupRoom(t, db, tenantID, room.ID)

		aliasRoom := &facilities.Room{Name: constants.WCRoomName, Building: "Test Building"}
		err := service.CreateRoom(ctx, aliasRoom)

		require.Error(t, err)
		assert.True(t, errors.Is(err, facilitiesSvc.ErrDuplicateToiletRoom))
	})
}

func TestFacilitiesService_UpdateRoom_RejectsWCRoomAliasDuplicates(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	service := setupFacilitiesService(t, db)
	tenantID := createFacilityTestTenant(t, db)
	ctx := testpkg.TenantContext(tenantID)

	t.Run("rejects renaming to Toilette when WC already exists", func(t *testing.T) {
		wcRoom := createRoomWithExactName(t, db, tenantID, constants.WCRoomName)
		defer cleanupRoom(t, db, tenantID, wcRoom.ID)

		room := testpkg.CreateTestRoomForTenant(t, db, tenantID, "AliasTarget")

		room.Name = constants.WCRoomAliasName
		err := service.UpdateRoom(ctx, room)

		require.Error(t, err)
		assert.True(t, errors.Is(err, facilitiesSvc.ErrDuplicateToiletRoom))
	})

	t.Run("rejects renaming to WC when Toilette already exists", func(t *testing.T) {
		aliasRoom := createRoomWithExactName(t, db, tenantID, constants.WCRoomAliasName)
		defer cleanupRoom(t, db, tenantID, aliasRoom.ID)

		room := testpkg.CreateTestRoomForTenant(t, db, tenantID, "CanonicalTarget")

		room.Name = constants.WCRoomName
		err := service.UpdateRoom(ctx, room)

		require.Error(t, err)
		assert.True(t, errors.Is(err, facilitiesSvc.ErrDuplicateToiletRoom))
	})
}
