// Package iot_test tests the IoT service layer with hermetic testing pattern.
package iot_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/iot"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupIoTService creates an IoT Service with real database connection
func setupIoTService(t *testing.T, db *bun.DB) iot.Service {
	repoFactory := repositories.NewFactory(db)
	serviceFactory, err := services.NewFactory(repoFactory, db)
	require.NoError(t, err, "Failed to create service factory")
	return serviceFactory.IoT
}

// =============================================================================
// CreateDevice Tests
// =============================================================================

func TestIoTService_CreateDevice(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := setupIoTService(t, db)
	ctx := context.Background()

	t.Run("creates device successfully", func(t *testing.T) {
		// ARRANGE
		device := &iotModels.Device{
			DeviceID:   fmt.Sprintf("test-device-%d", time.Now().UnixNano()),
			DeviceType: "rfid_reader",
			Name:       stringPtr("Test Device"),
		}

		// ACT
		err := service.CreateDevice(ctx, device)

		// ASSERT
		require.NoError(t, err)
		assert.Greater(t, device.ID, int64(0))
		assert.NotNil(t, device.APIKey, "API key should be auto-generated")
		assert.Equal(t, iotModels.DeviceStatusActive, device.Status, "Status should default to active")
		assert.NotNil(t, device.LastSeen, "LastSeen should be set")

		// Cleanup
		testpkg.CleanupActivityFixtures(t, db, device.ID)
	})

	t.Run("creates device with provided API key", func(t *testing.T) {
		// ARRANGE
		providedAPIKey := fmt.Sprintf("custom-api-key-%d", time.Now().UnixNano())
		device := &iotModels.Device{
			DeviceID:   fmt.Sprintf("test-device-custom-%d", time.Now().UnixNano()),
			DeviceType: "rfid_reader",
			APIKey:     &providedAPIKey,
		}

		// ACT
		err := service.CreateDevice(ctx, device)

		// ASSERT
		require.NoError(t, err)
		assert.Equal(t, providedAPIKey, *device.APIKey, "Should use provided API key")

		// Cleanup
		testpkg.CleanupActivityFixtures(t, db, device.ID)
	})

	t.Run("returns error for nil device", func(t *testing.T) {
		// ACT
		err := service.CreateDevice(ctx, nil)

		// ASSERT
		require.Error(t, err)
		assert.True(t, errors.Is(err, iot.ErrInvalidDeviceData))
	})

	t.Run("returns error for empty device ID", func(t *testing.T) {
		// ARRANGE
		device := &iotModels.Device{
			DeviceID:   "", // invalid
			DeviceType: "rfid_reader",
		}

		// ACT
		err := service.CreateDevice(ctx, device)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for empty device type", func(t *testing.T) {
		// ARRANGE
		device := &iotModels.Device{
			DeviceID:   fmt.Sprintf("test-device-%d", time.Now().UnixNano()),
			DeviceType: "", // invalid
		}

		// ACT
		err := service.CreateDevice(ctx, device)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for duplicate device ID", func(t *testing.T) {
		// ARRANGE - create first device
		existingDevice := testpkg.CreateTestDevice(t, db, "duplicate-test", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, existingDevice.ID)

		// Try to create another device with the same device ID
		duplicateDevice := &iotModels.Device{
			DeviceID:   existingDevice.DeviceID,
			DeviceType: "rfid_reader",
		}

		// ACT
		err := service.CreateDevice(ctx, duplicateDevice)

		// ASSERT
		require.Error(t, err)
		assert.True(t, errors.Is(err, iot.ErrDuplicateDeviceID))
	})
}

// =============================================================================
// GetDeviceByID Tests
// =============================================================================

func TestIoTService_GetDeviceByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := setupIoTService(t, db)
	ctx := context.Background()

	t.Run("returns device when found", func(t *testing.T) {
		// ARRANGE
		device := testpkg.CreateTestDevice(t, db, "get-by-id", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, device.ID)

		// ACT
		result, err := service.GetDeviceByID(ctx, device.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, device.ID, result.ID)
		assert.Equal(t, device.DeviceID, result.DeviceID)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		result, err := service.GetDeviceByID(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
		assert.True(t, errors.Is(err, iot.ErrDeviceNotFound))
	})

	t.Run("returns error for invalid ID", func(t *testing.T) {
		// ACT
		result, err := service.GetDeviceByID(ctx, 0)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns error for negative ID", func(t *testing.T) {
		// ACT
		result, err := service.GetDeviceByID(ctx, -1)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// =============================================================================
// GetDeviceByDeviceID Tests
// =============================================================================

func TestIoTService_GetDeviceByDeviceID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)

	service := setupIoTService(t, db)
	ctx := context.Background()

	t.Run("returns device when found", func(t *testing.T) {
		// ARRANGE
		device := testpkg.CreateTestDevice(t, db, "get-by-device-id", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, device.ID)

		// ACT
		result, err := service.GetDeviceByDeviceID(ctx, device.DeviceID)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, device.ID, result.ID)
		assert.Equal(t, device.DeviceID, result.DeviceID)
	})

	t.Run("returns error when not found", func(t *testing.T) {
		// ACT
		result, err := service.GetDeviceByDeviceID(ctx, "non-existent-device-id")

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns error for empty device ID", func(t *testing.T) {
		// ACT
		result, err := service.GetDeviceByDeviceID(ctx, "")

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// =============================================================================
// UpdateDevice Tests
// =============================================================================

func TestIoTService_UpdateDevice(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)
	_ = testpkg.SetupTestOGS(t, db)

	service := setupIoTService(t, db)
	ctx := context.Background()

	t.Run("updates device successfully", func(t *testing.T) {
		// ARRANGE
		device := testpkg.CreateTestDevice(t, db, "to-update", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, device.ID)

		// Modify device
		newName := "Updated Device Name"
		device.Name = &newName
		device.DeviceType = "temperature_sensor"

		// ACT
		err := service.UpdateDevice(ctx, device)

		// ASSERT
		require.NoError(t, err)

		// Verify update persisted
		updated, err := service.GetDeviceByID(ctx, device.ID)
		require.NoError(t, err)
		assert.Equal(t, newName, *updated.Name)
		assert.Equal(t, "temperature_sensor", updated.DeviceType)
	})

	t.Run("returns error for nil device", func(t *testing.T) {
		// ACT
		err := service.UpdateDevice(ctx, nil)

		// ASSERT
		require.Error(t, err)
		assert.True(t, errors.Is(err, iot.ErrInvalidDeviceData))
	})

	t.Run("returns error for device with zero ID", func(t *testing.T) {
		// ARRANGE
		device := &iotModels.Device{
			DeviceID:   "some-device",
			DeviceType: "rfid_reader",
		}
		device.ID = 0 // Set ID via embedded base.Model

		// ACT
		err := service.UpdateDevice(ctx, device)

		// ASSERT
		require.Error(t, err)
		assert.True(t, errors.Is(err, iot.ErrInvalidDeviceData))
	})

	t.Run("returns error when device not found", func(t *testing.T) {
		// ARRANGE
		device := &iotModels.Device{
			DeviceID:   "non-existent",
			DeviceType: "rfid_reader",
		}
		device.ID = 99999999 // Set ID via embedded base.Model

		// ACT
		err := service.UpdateDevice(ctx, device)

		// ASSERT
		require.Error(t, err)
		assert.True(t, errors.Is(err, iot.ErrDeviceNotFound))
	})

	t.Run("returns error for invalid device data", func(t *testing.T) {
		// ARRANGE
		device := testpkg.CreateTestDevice(t, db, "invalid-update", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, device.ID)

		// Make device invalid
		device.DeviceType = "" // invalid

		// ACT
		err := service.UpdateDevice(ctx, device)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error when changing to duplicate device ID", func(t *testing.T) {
		// ARRANGE - create two devices
		device1 := testpkg.CreateTestDevice(t, db, "first-device", ogsID)
		device2 := testpkg.CreateTestDevice(t, db, "second-device", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, device1.ID, device2.ID)

		// Try to change device2's DeviceID to match device1's
		device2.DeviceID = device1.DeviceID

		// ACT
		err := service.UpdateDevice(ctx, device2)

		// ASSERT
		require.Error(t, err)
		assert.True(t, errors.Is(err, iot.ErrDuplicateDeviceID))
	})
}

// =============================================================================
// DeleteDevice Tests
// =============================================================================

func TestIoTService_DeleteDevice(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)
	_ = testpkg.SetupTestOGS(t, db)

	service := setupIoTService(t, db)
	ctx := context.Background()

	t.Run("deletes device successfully", func(t *testing.T) {
		// ARRANGE
		device := testpkg.CreateTestDevice(t, db, "to-delete", ogsID)

		// ACT
		err := service.DeleteDevice(ctx, device.ID)

		// ASSERT
		require.NoError(t, err)

		// Verify device is deleted
		_, err = service.GetDeviceByID(ctx, device.ID)
		require.Error(t, err)
		assert.True(t, errors.Is(err, iot.ErrDeviceNotFound))
	})

	t.Run("returns error when device not found", func(t *testing.T) {
		// ACT
		err := service.DeleteDevice(ctx, 99999999)

		// ASSERT
		require.Error(t, err)
		assert.True(t, errors.Is(err, iot.ErrDeviceNotFound))
	})

	t.Run("returns error for invalid ID", func(t *testing.T) {
		// ACT
		err := service.DeleteDevice(ctx, 0)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for negative ID", func(t *testing.T) {
		// ACT
		err := service.DeleteDevice(ctx, -1)

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// ListDevices Tests
// =============================================================================

func TestIoTService_ListDevices(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)
	_ = testpkg.SetupTestOGS(t, db)

	service := setupIoTService(t, db)
	ctx := context.Background()

	t.Run("returns devices with no filters", func(t *testing.T) {
		// ARRANGE - create test devices
		device1 := testpkg.CreateTestDevice(t, db, "list-1", ogsID)
		device2 := testpkg.CreateTestDevice(t, db, "list-2", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, device1.ID, device2.ID)

		// ACT
		result, err := service.ListDevices(ctx, nil)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		// Should have at least our two devices (might have seeded data too)
		assert.GreaterOrEqual(t, len(result), 2)
	})

	t.Run("returns devices with status filter", func(t *testing.T) {
		// ARRANGE - create test device with specific status
		device := testpkg.CreateTestDevice(t, db, "list-status", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, device.ID)

		// ACT
		filters := map[string]interface{}{
			"status": string(iotModels.DeviceStatusActive),
		}
		result, err := service.ListDevices(ctx, filters)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		// All returned devices should be active
		for _, d := range result {
			assert.Equal(t, iotModels.DeviceStatusActive, d.Status)
		}
	})

	t.Run("returns empty list when no devices match", func(t *testing.T) {
		// ACT
		filters := map[string]interface{}{
			"device_type": "non_existent_type_xyz",
		}
		result, err := service.ListDevices(ctx, filters)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

// =============================================================================
// UpdateDeviceStatus Tests
// =============================================================================

func TestIoTService_UpdateDeviceStatus(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)
	_ = testpkg.SetupTestOGS(t, db)

	service := setupIoTService(t, db)
	ctx := context.Background()

	t.Run("updates status successfully", func(t *testing.T) {
		// ARRANGE
		device := testpkg.CreateTestDevice(t, db, "status-update", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, device.ID)

		// ACT
		err := service.UpdateDeviceStatus(ctx, device.DeviceID, iotModels.DeviceStatusMaintenance)

		// ASSERT
		require.NoError(t, err)

		// Verify status updated
		updated, err := service.GetDeviceByDeviceID(ctx, device.DeviceID)
		require.NoError(t, err)
		assert.Equal(t, iotModels.DeviceStatusMaintenance, updated.Status)
	})

	t.Run("returns error for empty device ID", func(t *testing.T) {
		// ACT
		err := service.UpdateDeviceStatus(ctx, "", iotModels.DeviceStatusActive)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error when device not found", func(t *testing.T) {
		// ACT
		err := service.UpdateDeviceStatus(ctx, "non-existent-device", iotModels.DeviceStatusActive)

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error for invalid status", func(t *testing.T) {
		// ARRANGE
		device := testpkg.CreateTestDevice(t, db, "invalid-status", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, device.ID)

		// ACT
		err := service.UpdateDeviceStatus(ctx, device.DeviceID, "invalid_status")

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// PingDevice Tests
// =============================================================================

func TestIoTService_PingDevice(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)
	_ = testpkg.SetupTestOGS(t, db)

	service := setupIoTService(t, db)
	ctx := context.Background()

	t.Run("updates last seen time successfully", func(t *testing.T) {
		// ARRANGE
		device := testpkg.CreateTestDevice(t, db, "ping-test", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, device.ID)

		originalLastSeen := device.LastSeen

		// Wait a bit to ensure time difference
		time.Sleep(10 * time.Millisecond)

		// ACT
		err := service.PingDevice(ctx, device.DeviceID)

		// ASSERT
		require.NoError(t, err)

		// Verify last seen was updated
		updated, err := service.GetDeviceByDeviceID(ctx, device.DeviceID)
		require.NoError(t, err)
		assert.NotNil(t, updated.LastSeen)
		if originalLastSeen != nil {
			assert.True(t, updated.LastSeen.After(*originalLastSeen) || updated.LastSeen.Equal(*originalLastSeen))
		}
	})

	t.Run("returns error for empty device ID", func(t *testing.T) {
		// ACT
		err := service.PingDevice(ctx, "")

		// ASSERT
		require.Error(t, err)
	})

	t.Run("returns error when device not found", func(t *testing.T) {
		// ACT
		err := service.PingDevice(ctx, "non-existent-device")

		// ASSERT
		require.Error(t, err)
	})
}

// =============================================================================
// GetDevicesByType Tests
// =============================================================================

func TestIoTService_GetDevicesByType(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)
	_ = testpkg.SetupTestOGS(t, db)

	service := setupIoTService(t, db)
	ctx := context.Background()

	t.Run("returns devices of specified type", func(t *testing.T) {
		// ARRANGE - CreateTestDevice creates rfid_reader type
		device := testpkg.CreateTestDevice(t, db, "type-filter", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, device.ID)

		// ACT
		result, err := service.GetDevicesByType(ctx, "rfid_reader")

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
		for _, d := range result {
			assert.Equal(t, "rfid_reader", d.DeviceType)
		}
	})

	t.Run("returns empty list for non-existent type", func(t *testing.T) {
		// ACT
		result, err := service.GetDevicesByType(ctx, "non_existent_type_xyz")

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("returns error for empty type", func(t *testing.T) {
		// ACT
		result, err := service.GetDevicesByType(ctx, "")

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// =============================================================================
// GetDevicesByStatus Tests
// =============================================================================

func TestIoTService_GetDevicesByStatus(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)
	_ = testpkg.SetupTestOGS(t, db)

	service := setupIoTService(t, db)
	ctx := context.Background()

	t.Run("returns devices with specified status", func(t *testing.T) {
		// ARRANGE - CreateTestDevice creates active devices
		device := testpkg.CreateTestDevice(t, db, "status-filter", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, device.ID)

		// ACT
		result, err := service.GetDevicesByStatus(ctx, iotModels.DeviceStatusActive)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
		for _, d := range result {
			assert.Equal(t, iotModels.DeviceStatusActive, d.Status)
		}
	})

	t.Run("returns error for invalid status", func(t *testing.T) {
		// ACT
		result, err := service.GetDevicesByStatus(ctx, "invalid_status")

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// =============================================================================
// GetDevicesByRegisteredBy Tests
// =============================================================================

func TestIoTService_GetDevicesByRegisteredBy(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)
	_ = testpkg.SetupTestOGS(t, db)

	service := setupIoTService(t, db)
	ctx := context.Background()

	t.Run("returns devices registered by person", func(t *testing.T) {
		// ARRANGE - create a person and device with that person as registerer
		person := testpkg.CreateTestPerson(t, db, "Device", "Registerer", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)

		device := &iotModels.Device{
			DeviceID:       fmt.Sprintf("registered-device-%d", time.Now().UnixNano()),
			DeviceType:     "rfid_reader",
			RegisteredByID: &person.ID,
		}
		err := service.CreateDevice(ctx, device)
		require.NoError(t, err)
		defer testpkg.CleanupActivityFixtures(t, db, device.ID)

		// ACT
		result, err := service.GetDevicesByRegisteredBy(ctx, person.ID)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
		// Verify at least one device is registered by this person
		found := false
		for _, d := range result {
			if d.RegisteredByID != nil && *d.RegisteredByID == person.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected to find device registered by test person")
	})

	t.Run("returns empty list when no devices registered by person", func(t *testing.T) {
		// ARRANGE - create a person with no devices
		person := testpkg.CreateTestPerson(t, db, "No", "Devices", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, person.ID)

		// ACT
		result, err := service.GetDevicesByRegisteredBy(ctx, person.ID)

		// ASSERT
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("returns error for invalid person ID", func(t *testing.T) {
		// ACT
		result, err := service.GetDevicesByRegisteredBy(ctx, 0)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns error for negative person ID", func(t *testing.T) {
		// ACT
		result, err := service.GetDevicesByRegisteredBy(ctx, -1)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// =============================================================================
// GetActiveDevices Tests
// =============================================================================

func TestIoTService_GetActiveDevices(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)
	_ = testpkg.SetupTestOGS(t, db)

	service := setupIoTService(t, db)
	ctx := context.Background()

	t.Run("returns active devices", func(t *testing.T) {
		// ARRANGE - CreateTestDevice creates active devices by default
		device := testpkg.CreateTestDevice(t, db, "active-device", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, device.ID)

		// ACT
		result, err := service.GetActiveDevices(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.NotEmpty(t, result)
		for _, d := range result {
			assert.Equal(t, iotModels.DeviceStatusActive, d.Status)
		}
	})
}

// =============================================================================
// GetDevicesRequiringMaintenance Tests
// =============================================================================

func TestIoTService_GetDevicesRequiringMaintenance(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)
	_ = testpkg.SetupTestOGS(t, db)

	service := setupIoTService(t, db)
	ctx := context.Background()

	t.Run("returns devices in maintenance status", func(t *testing.T) {
		// ARRANGE - create device and set to maintenance
		device := testpkg.CreateTestDevice(t, db, "maintenance-device", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, device.ID)

		// Update to maintenance status
		err := service.UpdateDeviceStatus(ctx, device.DeviceID, iotModels.DeviceStatusMaintenance)
		require.NoError(t, err)

		// ACT
		result, err := service.GetDevicesRequiringMaintenance(ctx)

		// ASSERT
		require.NoError(t, err)
		// Should contain at least our device
		found := false
		for _, d := range result {
			if d.ID == device.ID {
				found = true
				assert.Equal(t, iotModels.DeviceStatusMaintenance, d.Status)
				break
			}
		}
		assert.True(t, found, "Expected to find the maintenance device")
	})
}

// =============================================================================
// GetOfflineDevices Tests
// =============================================================================

func TestIoTService_GetOfflineDevices(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)
	_ = testpkg.SetupTestOGS(t, db)

	service := setupIoTService(t, db)
	ctx := context.Background()

	t.Run("returns devices offline for specified duration", func(t *testing.T) {
		// ARRANGE - create device with old last seen
		device := testpkg.CreateTestDevice(t, db, "offline-device", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, device.ID)

		// Update last_seen to be old (we'll check for devices offline > 1 second)
		// Note: This test is timing-dependent, so we use a very short duration
		oldTime := time.Now().Add(-1 * time.Hour)
		_, err := db.NewUpdate().
			Model((*iotModels.Device)(nil)).
			ModelTableExpr("iot.devices").
			Set("last_seen = ?", oldTime).
			Where("id = ?", device.ID).
			Exec(ctx)
		require.NoError(t, err)

		// ACT - get devices offline for more than 30 minutes
		result, err := service.GetOfflineDevices(ctx, 30*time.Minute)

		// ASSERT
		require.NoError(t, err)
		// Should find our device
		found := false
		for _, d := range result {
			if d.ID == device.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected to find the offline device")
	})

	t.Run("returns error for invalid duration", func(t *testing.T) {
		// ACT
		result, err := service.GetOfflineDevices(ctx, 0)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns error for negative duration", func(t *testing.T) {
		// ACT
		result, err := service.GetOfflineDevices(ctx, -1*time.Second)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// =============================================================================
// GetDeviceTypeStatistics Tests
// =============================================================================

func TestIoTService_GetDeviceTypeStatistics(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)
	_ = testpkg.SetupTestOGS(t, db)

	service := setupIoTService(t, db)
	ctx := context.Background()

	t.Run("returns device type statistics", func(t *testing.T) {
		// ARRANGE - create test devices (they have rfid_reader type)
		device1 := testpkg.CreateTestDevice(t, db, "stats-1", ogsID)
		device2 := testpkg.CreateTestDevice(t, db, "stats-2", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, device1.ID, device2.ID)

		// ACT
		result, err := service.GetDeviceTypeStatistics(ctx)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		// Should have rfid_reader type with count >= 2
		count, exists := result["rfid_reader"]
		assert.True(t, exists, "Expected rfid_reader type in statistics")
		assert.GreaterOrEqual(t, count, 2)
	})
}

// =============================================================================
// Network Operation Tests (Placeholder Implementations)
// =============================================================================

func TestIoTService_DetectNewDevices(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)
	_ = testpkg.SetupTestOGS(t, db)

	service := setupIoTService(t, db)
	ctx := context.Background()

	t.Run("returns not implemented error", func(t *testing.T) {
		// ACT
		result, err := service.DetectNewDevices(ctx)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not implemented")
	})
}

func TestIoTService_ScanNetwork(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)
	_ = testpkg.SetupTestOGS(t, db)

	service := setupIoTService(t, db)
	ctx := context.Background()

	t.Run("returns not implemented error", func(t *testing.T) {
		// ACT
		result, err := service.ScanNetwork(ctx)

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not implemented")
	})
}

// =============================================================================
// GetDeviceByAPIKey Tests
// =============================================================================

func TestIoTService_GetDeviceByAPIKey(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	ogsID := testpkg.SetupTestOGS(t, db)
	_ = testpkg.SetupTestOGS(t, db)

	service := setupIoTService(t, db)
	ctx := context.Background()

	t.Run("returns device when found by API key", func(t *testing.T) {
		// ARRANGE - CreateTestDevice generates an API key
		device := testpkg.CreateTestDevice(t, db, "api-key-test", ogsID)
		defer testpkg.CleanupActivityFixtures(t, db, device.ID)

		// ACT
		result, err := service.GetDeviceByAPIKey(ctx, *device.APIKey)

		// ASSERT
		require.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, device.ID, result.ID)
		assert.Equal(t, device.DeviceID, result.DeviceID)
	})

	t.Run("returns error when API key not found", func(t *testing.T) {
		// ACT
		result, err := service.GetDeviceByAPIKey(ctx, "non-existent-api-key")

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("returns error for empty API key", func(t *testing.T) {
		// ACT
		result, err := service.GetDeviceByAPIKey(ctx, "")

		// ASSERT
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

// =============================================================================
// Transaction Support Tests
// =============================================================================

func TestIoTService_WithTx(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	_ = testpkg.SetupTestOGS(t, db)

	service := setupIoTService(t, db)
	ctx := context.Background()

	t.Run("returns service instance with transaction", func(t *testing.T) {
		// ARRANGE
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()

		// ACT
		txService := service.WithTx(tx)

		// ASSERT - verify it returns a valid service interface
		_, ok := txService.(iot.Service)
		require.True(t, ok, "WithTx should return a valid Service interface")
	})
}

// Helper function for string pointers
func stringPtr(s string) *string {
	return &s
}
