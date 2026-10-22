package httpadapter_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type deviceRepository interface {
	Create(context.Context, *devicefleet.Device) error
	Update(context.Context, *devicefleet.Device) error
	Delete(context.Context, any) error
	FindByID(context.Context, any) (*devicefleet.Device, error)
	FindByIDForUpdate(context.Context, int64) (*devicefleet.Device, error)
	FindByDeviceID(context.Context, string) (*devicefleet.Device, error)
	FindByAPIKey(context.Context, string) (*devicefleet.Device, error)
	List(context.Context, map[string]any) ([]*devicefleet.Device, error)
	UpdateRoomID(context.Context, int64, int64) error
	CountDevicesByType(context.Context) (map[string]int, error)
}

// newDeviceRepository serves the retained device repository shape from the
// real Device Fleet owner, so these behaviour tests still cover the exact
// contract their callers hold.
func newDeviceRepository(t *testing.T, db *bun.DB) deviceRepository {
	t.Helper()
	repository, err := api.NewDeviceRepository(testpkg.NewDatabase(t, db))
	require.NoError(t, err, "Failed to compose the device repository")
	return repository
}

// ============================================================================
// CRUD Tests
// ============================================================================

func TestDeviceRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newDeviceRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("creates device with valid data", func(t *testing.T) {
		uniqueDeviceID := fmt.Sprintf("device-%d", time.Now().UnixNano())
		device := &devicefleet.Device{
			DeviceID:   uniqueDeviceID,
			DeviceType: "terminal",
			Status:     devicefleet.DeviceStatusActive,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)
		assert.NotZero(t, device.ID)

	})

	t.Run("creates inactive device", func(t *testing.T) {
		uniqueDeviceID := fmt.Sprintf("inactive-device-%d", time.Now().UnixNano())
		device := &devicefleet.Device{
			DeviceID:   uniqueDeviceID,
			DeviceType: "terminal",
			Status:     devicefleet.DeviceStatusInactive,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)
		assert.Equal(t, devicefleet.DeviceStatusInactive, device.Status)

	})
}

func TestDeviceRepository_FindByID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newDeviceRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("finds existing device", func(t *testing.T) {
		device := testpkg.CreateTestDevice(t, db, "findbyid")

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newDeviceRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("finds device by device_id string", func(t *testing.T) {
		device := testpkg.CreateTestDevice(t, db, "bydeviceid")

		found, err := repo.FindByDeviceID(ctx, device.DeviceID)
		require.NoError(t, err)
		assert.Equal(t, device.ID, found.ID)
	})

	t.Run("returns error for non-existent device_id", func(t *testing.T) {
		_, err := repo.FindByDeviceID(ctx, "nonexistent-device-12345")
		require.Error(t, err)
	})
}

func TestDeviceRepository_ArchivedDevicesAreHistoricalOnly(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	scope := testpkg.NewTenantScope(t, db)
	repo := newDeviceRepository(t, db)
	ctx := testpkg.TenantContext(scope.TenantID)
	deviceID := fmt.Sprintf("transfer-history-%d", time.Now().UnixNano())
	apiKey := fmt.Sprintf("transfer-key-%d", time.Now().UnixNano())
	source := &devicefleet.Device{DeviceID: deviceID, DeviceType: "terminal", Status: devicefleet.DeviceStatusInactive, APIKey: &apiKey}
	require.NoError(t, repo.Create(ctx, source))

	_, err := db.NewRaw(`UPDATE iot.devices SET archived_at = NOW(), api_key = NULL WHERE id = ?`, source.ID).Exec(ctx)
	require.NoError(t, err)

	_, err = repo.FindByID(ctx, source.ID)
	require.Error(t, err)
	_, err = repo.FindByDeviceID(ctx, deviceID)
	require.Error(t, err)
	_, err = repo.FindByAPIKey(ctx, apiKey)
	require.Error(t, err)
	_, err = repo.FindByIDForUpdate(ctx, source.ID)
	require.Error(t, err)

	target := &devicefleet.Device{DeviceID: deviceID, DeviceType: "terminal", Status: devicefleet.DeviceStatusActive}
	require.NoError(t, repo.Create(ctx, target), "an archived row must not reserve the current tenant/device identity")
	found, err := repo.FindByDeviceID(ctx, deviceID)
	require.NoError(t, err)
	assert.Equal(t, target.ID, found.ID)
}

func TestDeviceRepository_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newDeviceRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates device status", func(t *testing.T) {
		device := ownerDevice(testpkg.CreateTestDevice(t, db, "update"))

		device.Status = devicefleet.DeviceStatusInactive
		err := repo.Update(ctx, device)
		require.NoError(t, err)

		found, err := repo.FindByID(ctx, device.ID)
		require.NoError(t, err)
		assert.Equal(t, devicefleet.DeviceStatusInactive, found.Status)
	})
}

func TestDeviceRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newDeviceRepository(t, db)
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newDeviceRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("lists all devices", func(t *testing.T) {
		testpkg.CreateTestDevice(t, db, "list")

		devices, err := repo.List(ctx, nil)
		require.NoError(t, err)
		assert.NotEmpty(t, devices)
	})
}

// ============================================================================
// Validation Tests
// ============================================================================

func TestDeviceRepository_Create_Validation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newDeviceRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns error when device is nil", func(t *testing.T) {
		err := repo.Create(ctx, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be nil")
	})

	t.Run("returns error when device_id is missing", func(t *testing.T) {
		device := &devicefleet.Device{
			DeviceType: "terminal",
			Status:     devicefleet.DeviceStatusActive,
		}

		err := repo.Create(ctx, device)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "device ID is required")
	})

	t.Run("returns error when device_type is missing", func(t *testing.T) {
		device := &devicefleet.Device{
			DeviceID: fmt.Sprintf("no-type-%d", time.Now().UnixNano()),
			Status:   devicefleet.DeviceStatusActive,
		}

		err := repo.Create(ctx, device)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "device type is required")
	})
}

func TestDeviceRepository_Update_Validation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newDeviceRepository(t, db)
	ctx := testpkg.Ctx(t)

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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newDeviceRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("finds device by API key", func(t *testing.T) {
		apiKey := fmt.Sprintf("test-api-key-%d", time.Now().UnixNano())
		device := &devicefleet.Device{
			DeviceID:   fmt.Sprintf("device-with-key-%d", time.Now().UnixNano()),
			DeviceType: "terminal",
			Status:     devicefleet.DeviceStatusActive,
			APIKey:     &apiKey,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)

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

// ============================================================================
// Specialized Query Tests
// ============================================================================

func TestDeviceRepository_CountDevicesByType(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newDeviceRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("counts devices grouped by type", func(t *testing.T) {
		uniqueType := fmt.Sprintf("count-type-%d", time.Now().UnixNano())

		// Create multiple devices of the same type
		device1 := &devicefleet.Device{
			DeviceID:   fmt.Sprintf("count-device-1-%d", time.Now().UnixNano()),
			DeviceType: uniqueType,
			Status:     devicefleet.DeviceStatusActive,
		}
		device2 := &devicefleet.Device{
			DeviceID:   fmt.Sprintf("count-device-2-%d", time.Now().UnixNano()),
			DeviceType: uniqueType,
			Status:     devicefleet.DeviceStatusActive,
		}

		err := repo.Create(ctx, device1)
		require.NoError(t, err)

		err = repo.Create(ctx, device2)
		require.NoError(t, err)

		counts, err := repo.CountDevicesByType(ctx)
		require.NoError(t, err)
		assert.NotEmpty(t, counts)

		// Our unique type should have at least 2 devices
		count, exists := counts[uniqueType]
		assert.True(t, exists)
		assert.GreaterOrEqual(t, count, 2)
	})
}

func TestDeviceRepository_UpdateRoomID(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newDeviceRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("updates room_id on existing device", func(t *testing.T) {
		device := testpkg.CreateTestDevice(t, db, "updateroomid")
		room := testpkg.CreateTestRoom(t, db, "UpdateRoomID-Room")

		err := repo.UpdateRoomID(ctx, device.ID, room.ID)
		require.NoError(t, err)

		// Verify the update persisted
		updated, err := repo.FindByID(ctx, device.ID)
		require.NoError(t, err)
		require.NotNil(t, updated.RoomID)
		assert.Equal(t, room.ID, *updated.RoomID)
	})

	t.Run("updates updated_at timestamp", func(t *testing.T) {
		device := testpkg.CreateTestDevice(t, db, "updateroomid-ts")
		room := testpkg.CreateTestRoom(t, db, "UpdateRoomID-Timestamp")

		beforeUpdate, err := repo.FindByID(ctx, device.ID)
		require.NoError(t, err)
		originalUpdatedAt := beforeUpdate.UpdatedAt

		// Small delay to ensure timestamps differ
		time.Sleep(10 * time.Millisecond)

		err = repo.UpdateRoomID(ctx, device.ID, room.ID)
		require.NoError(t, err)

		afterUpdate, err := repo.FindByID(ctx, device.ID)
		require.NoError(t, err)
		assert.True(t, afterUpdate.UpdatedAt.After(originalUpdatedAt),
			"updated_at should be later than original; got %v vs %v", afterUpdate.UpdatedAt, originalUpdatedAt)
	})

	t.Run("overwrites existing room_id", func(t *testing.T) {
		device := testpkg.CreateTestDevice(t, db, "updateroomid-overwrite")
		room1 := testpkg.CreateTestRoom(t, db, "UpdateRoomID-First")
		room2 := testpkg.CreateTestRoom(t, db, "UpdateRoomID-Second")

		// Set initial room
		err := repo.UpdateRoomID(ctx, device.ID, room1.ID)
		require.NoError(t, err)

		// Overwrite with second room
		err = repo.UpdateRoomID(ctx, device.ID, room2.ID)
		require.NoError(t, err)

		updated, err := repo.FindByID(ctx, device.ID)
		require.NoError(t, err)
		require.NotNil(t, updated.RoomID)
		assert.Equal(t, room2.ID, *updated.RoomID)
	})

	t.Run("returns error for non-existent device", func(t *testing.T) {
		room := testpkg.CreateTestRoom(t, db, "UpdateRoomID-NoDevice")

		err := repo.UpdateRoomID(ctx, int64(999999), room.ID)
		require.Error(t, err)
	})
}

// ============================================================================
// Filter Tests
// ============================================================================

func TestDeviceRepository_List_WithFilters(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newDeviceRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("filters by status", func(t *testing.T) {
		device := &devicefleet.Device{
			DeviceID:   fmt.Sprintf("filter-status-%d", time.Now().UnixNano()),
			DeviceType: "terminal",
			Status:     devicefleet.DeviceStatusInactive,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)

		devices, err := repo.List(ctx, map[string]interface{}{
			"status": devicefleet.DeviceStatusInactive,
		})
		require.NoError(t, err)

		var found bool
		for _, d := range devices {
			assert.Equal(t, devicefleet.DeviceStatusInactive, d.Status)
			if d.ID == device.ID {
				found = true
			}
		}
		assert.True(t, found)
	})

	t.Run("filters by device_type", func(t *testing.T) {
		uniqueType := fmt.Sprintf("filter-type-%d", time.Now().UnixNano())
		device := &devicefleet.Device{
			DeviceID:   fmt.Sprintf("filter-device-type-%d", time.Now().UnixNano()),
			DeviceType: uniqueType,
			Status:     devicefleet.DeviceStatusActive,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)

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
		device := &devicefleet.Device{
			DeviceID:   fmt.Sprintf("%s-device", uniquePrefix),
			DeviceType: "terminal",
			Status:     devicefleet.DeviceStatusActive,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)

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
		device := &devicefleet.Device{
			DeviceID:   fmt.Sprintf("namefilter-%d", time.Now().UnixNano()),
			DeviceType: "terminal",
			Status:     devicefleet.DeviceStatusActive,
			Name:       &uniqueName,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)

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
		device := &devicefleet.Device{
			DeviceID:   fmt.Sprintf("hasname-%d", time.Now().UnixNano()),
			DeviceType: "terminal",
			Status:     devicefleet.DeviceStatusActive,
			Name:       &deviceName,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)

		devices, err := repo.List(ctx, map[string]interface{}{
			"has_name": true,
		})
		require.NoError(t, err)

		for _, d := range devices {
			assert.NotNil(t, d.Name)
		}
	})

	t.Run("filters by has_name false", func(t *testing.T) {
		device := &devicefleet.Device{
			DeviceID:   fmt.Sprintf("noname-%d", time.Now().UnixNano()),
			DeviceType: "terminal",
			Status:     devicefleet.DeviceStatusActive,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)

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
		device := &devicefleet.Device{
			DeviceID:   fmt.Sprintf("seenafter-%d", time.Now().UnixNano()),
			DeviceType: "terminal",
			Status:     devicefleet.DeviceStatusActive,
			LastSeen:   &recentTime,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)

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
		device := &devicefleet.Device{
			DeviceID:   fmt.Sprintf("seenbefore-%d", time.Now().UnixNano()),
			DeviceType: "terminal",
			Status:     devicefleet.DeviceStatusActive,
			LastSeen:   &oldTime,
		}

		err := repo.Create(ctx, device)
		require.NoError(t, err)

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

	t.Run("filters by exclude_device_id", func(t *testing.T) {
		webManualDevice := testpkg.EnsureWebManualDevice(t, db)

		virtualDevice := &devicefleet.Device{
			DeviceID:   fmt.Sprintf("virtual-visible-%d", time.Now().UnixNano()),
			DeviceType: devicefleet.DeviceTypeVirtual,
			Status:     devicefleet.DeviceStatusActive,
		}
		err := repo.Create(ctx, virtualDevice)
		require.NoError(t, err)

		normalDevice := &devicefleet.Device{
			DeviceID:   fmt.Sprintf("exclude-type-%d", time.Now().UnixNano()),
			DeviceType: "terminal",
			Status:     devicefleet.DeviceStatusActive,
		}
		err = repo.Create(ctx, normalDevice)
		require.NoError(t, err)

		devices, err := repo.List(ctx, map[string]interface{}{
			"exclude_device_id": devicefleet.WebManualDeviceID,
		})
		require.NoError(t, err)

		var foundWebManual bool
		var foundVirtual bool
		var foundNormal bool
		for _, d := range devices {
			if d.ID == webManualDevice.ID {
				foundWebManual = true
			}
			if d.ID == virtualDevice.ID {
				foundVirtual = true
			}
			if d.ID == normalDevice.ID {
				foundNormal = true
			}
		}
		assert.False(t, foundWebManual, "reserved web manual device should be excluded")
		assert.True(t, foundVirtual, "tenant-managed virtual devices should still be listed")
		assert.True(t, foundNormal, "normal device should not be excluded")
	})
}

// ownerDevice carries a fixture row into the owner model the repository and
// service write, field for field.
func ownerDevice(row *testpkg.Device) *devicefleet.Device {
	return &devicefleet.Device{
		ID:                    row.ID,
		CreatedAt:             row.CreatedAt,
		UpdatedAt:             row.UpdatedAt,
		TenantID:              row.TenantID,
		DeviceID:              row.DeviceID,
		DeviceType:            row.DeviceType,
		Name:                  row.Name,
		Status:                devicefleet.DeviceStatus(row.Status),
		APIKey:                row.APIKey,
		LastSeen:              row.LastSeen,
		RegisteredByID:        row.RegisteredByID,
		RoomID:                row.RoomID,
		ArchivedAt:            row.ArchivedAt,
		TransferredToDeviceID: row.TransferredToDeviceID,
	}
}
