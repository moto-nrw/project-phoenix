package iot_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/iot"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ============================================================================
// Setup Helpers
// ============================================================================

func setupDeviceRepo(t *testing.T, db *bun.DB) iot.DeviceRepository {
	repoFactory := repositories.NewFactory(db)
	return repoFactory.Device
}

// cleanupDeviceRecords removes devices directly
func cleanupDeviceRecords(t *testing.T, db *bun.DB, deviceIDs ...int64) {
	t.Helper()
	if len(deviceIDs) == 0 {
		return
	}

	ctx := context.Background()
	_, err := db.NewDelete().
		TableExpr("iot.devices").
		Where("id IN (?)", bun.In(deviceIDs)).
		Exec(ctx)
	if err != nil {
		t.Logf("Warning: failed to cleanup devices: %v", err)
	}
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestDeviceRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupDeviceRepo(t, db)
	ctx := context.Background()

	t.Run("creates device with valid data", func(t *testing.T) {
		uniqueDeviceID := fmt.Sprintf("device-%d", time.Now().UnixNano())
		device := &iot.Device{
			DeviceID:   uniqueDeviceID,
			DeviceType: "rfid_reader",
			Status:     iot.DeviceStatusActive,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)
		assert.NotZero(t, device.ID)

		cleanupDeviceRecords(t, db, device.ID)
	})

	t.Run("creates inactive device", func(t *testing.T) {
		uniqueDeviceID := fmt.Sprintf("inactive-device-%d", time.Now().UnixNano())
		device := &iot.Device{
			DeviceID:   uniqueDeviceID,
			DeviceType: "rfid_reader",
			Status:     iot.DeviceStatusInactive,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)
		assert.Equal(t, iot.DeviceStatusInactive, device.Status)

		cleanupDeviceRecords(t, db, device.ID)
	})
}

func TestDeviceRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupDeviceRepo(t, db)
	ctx := context.Background()

	t.Run("finds existing device", func(t *testing.T) {
		device := testpkg.CreateTestDevice(t, db, "findbyid")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, device.ID, 0, 0)

		found, err := repo.FindByID(ctx, device.ID)
		require.NoError(t, err)
		assert.Equal(t, device.ID, found.ID)
	})

	t.Run("returns error for non-existent device", func(t *testing.T) {
		_, err := repo.FindByID(ctx, int64(999999))
		require.Error(t, err)
	})
}

func TestDeviceRepository_FindByDeviceID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupDeviceRepo(t, db)
	ctx := context.Background()

	t.Run("finds device by device_id string", func(t *testing.T) {
		device := testpkg.CreateTestDevice(t, db, "bydeviceid")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, device.ID, 0, 0)

		found, err := repo.FindByDeviceID(ctx, device.DeviceID)
		require.NoError(t, err)
		assert.Equal(t, device.ID, found.ID)
	})

	t.Run("returns error for non-existent device_id", func(t *testing.T) {
		_, err := repo.FindByDeviceID(ctx, "nonexistent-device-12345")
		require.Error(t, err)
	})
}

func TestDeviceRepository_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupDeviceRepo(t, db)
	ctx := context.Background()

	t.Run("updates device status", func(t *testing.T) {
		device := testpkg.CreateTestDevice(t, db, "update")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, device.ID, 0, 0)

		device.Status = iot.DeviceStatusInactive
		err := repo.Update(ctx, device)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, device.ID)
		require.NoError(t, err)
		assert.Equal(t, iot.DeviceStatusInactive, found.Status)
	})
}

func TestDeviceRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupDeviceRepo(t, db)
	ctx := context.Background()

	t.Run("deletes existing device", func(t *testing.T) {
		device := testpkg.CreateTestDevice(t, db, "delete")

		err := repo.Delete(ctx, device.ID)
		require.NoError(t, err)

		_, err = repo.FindByID(ctx, device.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Query Tests
// ============================================================================

func TestDeviceRepository_List(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupDeviceRepo(t, db)
	ctx := context.Background()

	t.Run("lists all devices", func(t *testing.T) {
		device := testpkg.CreateTestDevice(t, db, "list")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, device.ID, 0, 0)

		devices, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, devices)
	})
}

func TestDeviceRepository_FindActiveDevices(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := setupDeviceRepo(t, db)
	ctx := context.Background()

	t.Run("finds only active devices", func(t *testing.T) {
		device := testpkg.CreateTestDevice(t, db, "activedevice")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, device.ID, 0, 0)

		devices, err := repo.FindActiveDevices(ctx)
		require.NoError(t, err)

		// All returned devices should be active
		for _, d := range devices {
			assert.Equal(t, iot.DeviceStatusActive, d.Status)
		}

		// Our device should be in the results
		var found bool
		for _, d := range devices {
			if d.ID == device.ID {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}
