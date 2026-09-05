package active_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/ptrtest"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestActiveGroupRepository_FindActiveByDeviceIDWithNamesInAdminTransaction(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db).ActiveGroup
	ctx := testpkg.Ctx(t)
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "Administrative activity")
	room := testpkg.CreateTestRoom(t, db, "Administrative room")
	device := testpkg.CreateTestDevice(t, db, "administrative-device")
	now := time.Now()
	require.NoError(t, repo.Create(ctx, &active.Group{
		StartTime: now, LastActivity: now, TimeoutMinutes: 30,
		GroupID: ptrtest.Ptr(activityGroup.ID), DeviceID: &device.ID, RoomID: room.ID,
	}))

	require.NoError(t, testpkg.WithAdminTx(t, context.Background(), db, func(adminCtx context.Context, _ bun.Tx) error {
		found, err := repo.FindActiveByDeviceIDWithNames(tenant.WithTenantID(adminCtx, testpkg.Tenant(t)), device.ID)
		require.NoError(t, err)
		require.NotNil(t, found)
		require.NotNil(t, found.ActualGroup)
		assert.Equal(t, activityGroup.Name, found.ActualGroup.Name)
		return nil
	}))
}
