package iot_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres"
	"github.com/moto-nrw/project-phoenix/models/iot"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// CRUD Tests
// ============================================================================

func TestDeviceRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Device
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

		testpkg.CleanupTableRecords(t, db, "iot.devices", device.ID)
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

		testpkg.CleanupTableRecords(t, db, "iot.devices", device.ID)
	})
}

func TestDeviceRepository_FindByID(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Device
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

	repo := repositories.NewFactory(db).Device
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

	repo := repositories.NewFactory(db).Device
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

	repo := repositories.NewFactory(db).Device
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

	repo := repositories.NewFactory(db).Device
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

	repo := repositories.NewFactory(db).Device
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

// ============================================================================
// Validation Tests
// ============================================================================

func TestDeviceRepository_Create_Validation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Device
	ctx := context.Background()

	t.Run("returns error when device is nil", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("returns error when device_id is missing", func(t *testing.T) {
		device := &iot.Device{
			DeviceType: "rfid_reader",
			Status:     iot.DeviceStatusActive,
		}

		err := repo.Create(ctx, device)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "device ID is required")
	})

	t.Run("returns error when device_type is missing", func(t *testing.T) {
		device := &iot.Device{
			DeviceID: fmt.Sprintf("no-type-%d", time.Now().UnixNano()),
			Status:   iot.DeviceStatusActive,
		}

		err := repo.Create(ctx, device)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "device type is required")
	})
}

func TestDeviceRepository_Update_Validation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Device
	ctx := context.Background()

	t.Run("returns error when device is nil", func(t *testing.T) {
		err := repo.Update(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})
}

// ============================================================================
// FindBy Tests
// ============================================================================

func TestDeviceRepository_FindByAPIKey(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Device
	ctx := context.Background()

	t.Run("finds device by API key", func(t *testing.T) {
		apiKey := fmt.Sprintf("test-api-key-%d", time.Now().UnixNano())
		device := &iot.Device{
			DeviceID:   fmt.Sprintf("device-with-key-%d", time.Now().UnixNano()),
			DeviceType: "rfid_reader",
			Status:     iot.DeviceStatusActive,
			APIKey:     &apiKey,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "iot.devices", device.ID)

		found, err := repo.FindByAPIKey(ctx, apiKey)
		require.NoError(t, err)
		assert.Equal(t, device.ID, found.ID)
		assert.NotNil(t, found.APIKey)
		assert.Equal(t, apiKey, *found.APIKey)
	})

	t.Run("returns error for non-existent API key", func(t *testing.T) {
		_, err := repo.FindByAPIKey(ctx, "nonexistent-api-key-12345")
		require.Error(t, err)
	})
}

func TestDeviceRepository_FindByType(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Device
	ctx := context.Background()

	t.Run("finds devices by type", func(t *testing.T) {
		uniqueType := fmt.Sprintf("test-type-%d", time.Now().UnixNano())
		device := &iot.Device{
			DeviceID:   fmt.Sprintf("device-by-type-%d", time.Now().UnixNano()),
			DeviceType: uniqueType,
			Status:     iot.DeviceStatusActive,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "iot.devices", device.ID)

		devices, err := repo.FindByType(ctx, uniqueType)
		require.NoError(t, err)
		assert.NotEmpty(t, devices)

		var found bool
		for _, d := range devices {
			assert.Equal(t, uniqueType, d.DeviceType)
			if d.ID == device.ID {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("returns empty list for type with no devices", func(t *testing.T) {
		devices, err := repo.FindByType(ctx, "nonexistent-type-12345")
		require.NoError(t, err)
		assert.Empty(t, devices)
	})
}

func TestDeviceRepository_FindByStatus(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Device
	ctx := context.Background()

	t.Run("finds devices by status", func(t *testing.T) {
		device := &iot.Device{
			DeviceID:   fmt.Sprintf("maintenance-device-%d", time.Now().UnixNano()),
			DeviceType: "rfid_reader",
			Status:     iot.DeviceStatusMaintenance,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "iot.devices", device.ID)

		devices, err := repo.FindByStatus(ctx, iot.DeviceStatusMaintenance)
		require.NoError(t, err)
		assert.NotEmpty(t, devices)

		var found bool
		for _, d := range devices {
			assert.Equal(t, iot.DeviceStatusMaintenance, d.Status)
			if d.ID == device.ID {
				found = true
			}
		}
		assert.True(t, found)
	})
}

func TestDeviceRepository_FindByRegisteredBy(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Device
	ctx := context.Background()

	t.Run("finds devices registered by person", func(t *testing.T) {
		// Create a person to register the device
		person := testpkg.CreateTestStaff(t, db, "Device", "Registrar")
		defer testpkg.CleanupActivityFixtures(t, db, 0, person.ID, 0, 0, 0)

		device := &iot.Device{
			DeviceID:       fmt.Sprintf("registered-device-%d", time.Now().UnixNano()),
			DeviceType:     "rfid_reader",
			Status:         iot.DeviceStatusActive,
			RegisteredByID: &person.PersonID,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "iot.devices", device.ID)

		devices, err := repo.FindByRegisteredBy(ctx, person.PersonID)
		require.NoError(t, err)
		assert.NotEmpty(t, devices)

		var found bool
		for _, d := range devices {
			assert.NotNil(t, d.RegisteredByID)
			assert.Equal(t, person.PersonID, *d.RegisteredByID)
			if d.ID == device.ID {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("returns empty list for person with no registered devices", func(t *testing.T) {
		devices, err := repo.FindByRegisteredBy(ctx, int64(999999))
		require.NoError(t, err)
		assert.Empty(t, devices)
	})
}

// ============================================================================
// Update Tests
// ============================================================================

func TestDeviceRepository_UpdateLastSeen(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Device
	ctx := context.Background()

	t.Run("updates last_seen timestamp", func(t *testing.T) {
		device := testpkg.CreateTestDevice(t, db, "lastseen")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, device.ID, 0, 0)

		newLastSeen := time.Now().Add(-10 * time.Minute)
		err := repo.UpdateLastSeen(ctx, device.DeviceID, newLastSeen)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, device.ID)
		require.NoError(t, err)
		assert.NotNil(t, found.LastSeen)
		assert.WithinDuration(t, newLastSeen, *found.LastSeen, time.Second)
	})
}

func TestDeviceRepository_UpdateStatus(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Device
	ctx := context.Background()

	t.Run("updates device status", func(t *testing.T) {
		device := testpkg.CreateTestDevice(t, db, "statusupdate")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, device.ID, 0, 0)

		err := repo.UpdateStatus(ctx, device.DeviceID, iot.DeviceStatusOffline)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, device.ID)
		require.NoError(t, err)
		assert.Equal(t, iot.DeviceStatusOffline, found.Status)
	})

	t.Run("updates status to maintenance", func(t *testing.T) {
		device := testpkg.CreateTestDevice(t, db, "maintenance")
		defer testpkg.CleanupActivityFixtures(t, db, 0, 0, device.ID, 0, 0)

		err := repo.UpdateStatus(ctx, device.DeviceID, iot.DeviceStatusMaintenance)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, device.ID)
		require.NoError(t, err)
		assert.Equal(t, iot.DeviceStatusMaintenance, found.Status)
	})
}

// ============================================================================
// Specialized Query Tests
// ============================================================================

func TestDeviceRepository_FindDevicesRequiringMaintenance(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Device
	ctx := context.Background()

	t.Run("finds devices requiring maintenance", func(t *testing.T) {
		device := &iot.Device{
			DeviceID:   fmt.Sprintf("needs-maintenance-%d", time.Now().UnixNano()),
			DeviceType: "rfid_reader",
			Status:     iot.DeviceStatusMaintenance,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "iot.devices", device.ID)

		devices, err := repo.FindDevicesRequiringMaintenance(ctx)
		require.NoError(t, err)

		var found bool
		for _, d := range devices {
			assert.Equal(t, iot.DeviceStatusMaintenance, d.Status)
			if d.ID == device.ID {
				found = true
			}
		}
		assert.True(t, found)
	})
}

func TestDeviceRepository_FindOfflineDevices(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Device
	ctx := context.Background()

	t.Run("finds devices offline for specified duration", func(t *testing.T) {
		// Create device with old last_seen timestamp
		oldTime := time.Now().Add(-2 * time.Hour)
		device := &iot.Device{
			DeviceID:   fmt.Sprintf("offline-device-%d", time.Now().UnixNano()),
			DeviceType: "rfid_reader",
			Status:     iot.DeviceStatusActive,
			LastSeen:   &oldTime,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "iot.devices", device.ID)

		// Find devices offline for more than 1 hour
		devices, err := repo.FindOfflineDevices(ctx, 1*time.Hour)
		require.NoError(t, err)

		var found bool
		for _, d := range devices {
			if d.ID == device.ID {
				found = true
				assert.NotNil(t, d.LastSeen)
				assert.True(t, d.LastSeen.Before(time.Now().Add(-1*time.Hour)))
				break
			}
		}
		assert.True(t, found)
	})

	t.Run("finds devices never seen (null last_seen)", func(t *testing.T) {
		// Create device with no last_seen (will use old created_at)
		device := &iot.Device{
			DeviceID:   fmt.Sprintf("never-seen-%d", time.Now().UnixNano()),
			DeviceType: "rfid_reader",
			Status:     iot.DeviceStatusActive,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "iot.devices", device.ID)

		// Update created_at to be old (requires direct DB update with schema)
		oldTime := time.Now().Add(-2 * time.Hour)
		_, err = db.NewUpdate().
			Model((*iot.Device)(nil)).
			ModelTableExpr("iot.devices").
			Set("created_at = ?", oldTime).
			Where("id = ?", device.ID).
			Exec(ctx)
		require.NoError(t, err)

		// Find devices offline for more than 1 hour
		devices, err := repo.FindOfflineDevices(ctx, 1*time.Hour)
		require.NoError(t, err)

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

func TestDeviceRepository_CountDevicesByType(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Device
	ctx := context.Background()

	t.Run("counts devices grouped by type", func(t *testing.T) {
		uniqueType := fmt.Sprintf("count-type-%d", time.Now().UnixNano())

		// Create multiple devices of the same type
		device1 := &iot.Device{
			DeviceID:   fmt.Sprintf("count-device-1-%d", time.Now().UnixNano()),
			DeviceType: uniqueType,
			Status:     iot.DeviceStatusActive,
		}
		device2 := &iot.Device{
			DeviceID:   fmt.Sprintf("count-device-2-%d", time.Now().UnixNano()),
			DeviceType: uniqueType,
			Status:     iot.DeviceStatusActive,
		}

		err := repo.Create(ctx, device1)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "iot.devices", device1.ID)

		err = repo.Create(ctx, device2)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "iot.devices", device2.ID)

		counts, err := repo.CountDevicesByType(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, counts)

		// Our unique type should have at least 2 devices
		count, exists := counts[uniqueType]
		assert.True(t, exists)
		assert.GreaterOrEqual(t, count, 2)
	})
}

// ============================================================================
// Filter Tests
// ============================================================================

func TestDeviceRepository_List_WithFilters(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).Device
	ctx := context.Background()

	t.Run("filters by status", func(t *testing.T) {
		device := &iot.Device{
			DeviceID:   fmt.Sprintf("filter-status-%d", time.Now().UnixNano()),
			DeviceType: "rfid_reader",
			Status:     iot.DeviceStatusInactive,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "iot.devices", device.ID)

		devices, err := repo.List(ctx, map[string]interface{}{
			"status": iot.DeviceStatusInactive,
		})
		require.NoError(t, err)

		var found bool
		for _, d := range devices {
			assert.Equal(t, iot.DeviceStatusInactive, d.Status)
			if d.ID == device.ID {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("filters by device_type", func(t *testing.T) {
		uniqueType := fmt.Sprintf("filter-type-%d", time.Now().UnixNano())
		device := &iot.Device{
			DeviceID:   fmt.Sprintf("filter-device-type-%d", time.Now().UnixNano()),
			DeviceType: uniqueType,
			Status:     iot.DeviceStatusActive,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "iot.devices", device.ID)

		devices, err := repo.List(ctx, map[string]interface{}{
			"device_type": uniqueType,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, devices)

		var found bool
		for _, d := range devices {
			assert.Equal(t, uniqueType, d.DeviceType)
			if d.ID == device.ID {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("filters by device_id_like", func(t *testing.T) {
		uniquePrefix := fmt.Sprintf("like-%d", time.Now().UnixNano())
		device := &iot.Device{
			DeviceID:   fmt.Sprintf("%s-device", uniquePrefix),
			DeviceType: "rfid_reader",
			Status:     iot.DeviceStatusActive,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "iot.devices", device.ID)

		devices, err := repo.List(ctx, map[string]interface{}{
			"device_id_like": uniquePrefix,
		})
		require.NoError(t, err)

		var found bool
		for _, d := range devices {
			assert.Contains(t, d.DeviceID, uniquePrefix)
			if d.ID == device.ID {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("filters by name_like", func(t *testing.T) {
		uniqueName := fmt.Sprintf("NameFilter-%d", time.Now().UnixNano())
		device := &iot.Device{
			DeviceID:   fmt.Sprintf("namefilter-%d", time.Now().UnixNano()),
			DeviceType: "rfid_reader",
			Status:     iot.DeviceStatusActive,
			Name:       &uniqueName,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "iot.devices", device.ID)

		devices, err := repo.List(ctx, map[string]interface{}{
			"name_like": "NameFilter",
		})
		require.NoError(t, err)

		var found bool
		for _, d := range devices {
			if d.Name != nil {
				assert.Contains(t, *d.Name, "NameFilter")
			}
			if d.ID == device.ID {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("filters by has_name true", func(t *testing.T) {
		deviceName := fmt.Sprintf("Named Device %d", time.Now().UnixNano())
		device := &iot.Device{
			DeviceID:   fmt.Sprintf("hasname-%d", time.Now().UnixNano()),
			DeviceType: "rfid_reader",
			Status:     iot.DeviceStatusActive,
			Name:       &deviceName,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "iot.devices", device.ID)

		devices, err := repo.List(ctx, map[string]interface{}{
			"has_name": true,
		})
		require.NoError(t, err)

		for _, d := range devices {
			assert.NotNil(t, d.Name)
		}
	})

	t.Run("filters by has_name false", func(t *testing.T) {
		device := &iot.Device{
			DeviceID:   fmt.Sprintf("noname-%d", time.Now().UnixNano()),
			DeviceType: "rfid_reader",
			Status:     iot.DeviceStatusActive,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "iot.devices", device.ID)

		devices, err := repo.List(ctx, map[string]interface{}{
			"has_name": false,
		})
		require.NoError(t, err)

		var found bool
		for _, d := range devices {
			assert.Nil(t, d.Name)
			if d.ID == device.ID {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("filters by seen_after", func(t *testing.T) {
		recentTime := time.Now().Add(-5 * time.Minute)
		device := &iot.Device{
			DeviceID:   fmt.Sprintf("seenafter-%d", time.Now().UnixNano()),
			DeviceType: "rfid_reader",
			Status:     iot.DeviceStatusActive,
			LastSeen:   &recentTime,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "iot.devices", device.ID)

		cutoff := time.Now().Add(-10 * time.Minute)
		devices, err := repo.List(ctx, map[string]interface{}{
			"seen_after": cutoff,
		})
		require.NoError(t, err)

		var found bool
		for _, d := range devices {
			if d.LastSeen != nil {
				assert.True(t, d.LastSeen.After(cutoff))
			}
			if d.ID == device.ID {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("filters by seen_before", func(t *testing.T) {
		oldTime := time.Now().Add(-20 * time.Minute)
		device := &iot.Device{
			DeviceID:   fmt.Sprintf("seenbefore-%d", time.Now().UnixNano()),
			DeviceType: "rfid_reader",
			Status:     iot.DeviceStatusActive,
			LastSeen:   &oldTime,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)
		defer testpkg.CleanupTableRecords(t, db, "iot.devices", device.ID)

		cutoff := time.Now().Add(-15 * time.Minute)
		devices, err := repo.List(ctx, map[string]interface{}{
			"seen_before": cutoff,
		})
		require.NoError(t, err)

		var found bool
		for _, d := range devices {
			if d.LastSeen != nil {
				assert.True(t, d.LastSeen.Before(cutoff))
			}
			if d.ID == device.ID {
				found = true
			}
		}
		assert.True(t, found)
	})
}
