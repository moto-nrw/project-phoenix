// Package repositoryadapter serves the retired device repository shape from
// the Device Fleet capability, so callers that still hold
// models/iot.DeviceRepository keep working while they migrate.
package repositoryadapter

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
)

// DeviceRepository delegates every operation to the owner capability. The
// lookup and command halves are embedded, so UpdateRoomID and
// CountDevicesByType, whose shapes already match, are promoted unchanged;
// the translating methods call devices, the same value.
type DeviceRepository struct {
	devicefleet.DeviceLookup
	devicefleet.DeviceCommand
	devices devicefleet.Capability
}

// NewDeviceRepository wraps the capability in the retired repository shape.
func NewDeviceRepository(devices devicefleet.Capability) *DeviceRepository {
	if devices == nil {
		panic("devicefleet repository adapter: capability is required")
	}
	return &DeviceRepository{DeviceLookup: devices, DeviceCommand: devices, devices: devices}
}

// notFound reports absence the way the retired repository did, so callers
// matching on sql.ErrNoRows keep the same branch.
func notFound(operation string, err error) error {
	return fmt.Errorf("%s: %w: %w", operation, err, sql.ErrNoRows)
}

func (r *DeviceRepository) translate(operation string, device devicefleet.Device, err error) (*devicefleet.Device, error) {
	if err != nil {
		if errors.Is(err, devicefleet.ErrDeviceNotFound) {
			return nil, notFound(operation, err)
		}
		return nil, err
	}
	return toModel(device), nil
}

func (r *DeviceRepository) translateList(devices []devicefleet.Device, err error) ([]*devicefleet.Device, error) {
	if err != nil {
		return nil, err
	}
	result := make([]*devicefleet.Device, 0, len(devices))
	for _, device := range devices {
		result = append(result, toModel(device))
	}
	return result, nil
}

// Create registers the device and writes the stored row back into entity.
func (r *DeviceRepository) Create(ctx context.Context, entity *devicefleet.Device) error {
	if entity == nil {
		return errors.New("device cannot be nil or zero value")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	created, err := r.devices.CreateDevice(ctx, devicefleet.CreateDevice{
		DeviceID: entity.DeviceID, DeviceType: entity.DeviceType, Name: entity.Name,
		Status: devicefleet.DeviceStatus(entity.Status), APIKey: entity.APIKey, APIKeyHash: entity.APIKeyHash, MaskedAPIKey: entity.MaskedAPIKey,
		LastSeen: entity.LastSeen, RegisteredByID: entity.RegisteredByID, RoomID: entity.RoomID,
	})
	if err != nil {
		return err
	}
	*entity = *toModel(created)
	return nil
}

// Update replaces every writable column of the device.
func (r *DeviceRepository) Update(ctx context.Context, entity *devicefleet.Device) error {
	if entity == nil {
		return errors.New("device cannot be nil or zero value")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	updated, err := r.devices.UpdateDevice(ctx, devicefleet.UpdateDevice{
		ID: entity.ID, DeviceID: entity.DeviceID, DeviceType: entity.DeviceType, Name: entity.Name,
		Status: devicefleet.DeviceStatus(entity.Status), APIKey: entity.APIKey, APIKeyHash: entity.APIKeyHash, MaskedAPIKey: entity.MaskedAPIKey, LastSeen: entity.LastSeen,
		RegisteredByID: entity.RegisteredByID, RoomID: entity.RoomID, ArchivedAt: entity.ArchivedAt,
		TransferredToDeviceID: entity.TransferredToDeviceID,
	})
	if err != nil {
		if errors.Is(err, devicefleet.ErrDeviceNotFound) {
			return notFound("update Device", err)
		}
		return err
	}
	*entity = *toModel(updated)
	return nil
}

// Delete removes the device. A row that is already gone is not an error.
func (r *DeviceRepository) Delete(ctx context.Context, id any) error {
	key, err := int64ID(id)
	if err != nil {
		return err
	}
	return r.devices.DeleteDevice(ctx, key)
}

// FindByID reads one live device including its room name.
func (r *DeviceRepository) FindByID(ctx context.Context, id any) (*devicefleet.Device, error) {
	key, err := int64ID(id)
	if err != nil {
		return nil, err
	}
	device, err := r.devices.FindDevice(ctx, key)
	return r.translate("find by id", device, err)
}

// FindByIDForUpdate reads and locks one live device.
func (r *DeviceRepository) FindByIDForUpdate(ctx context.Context, id int64) (*devicefleet.Device, error) {
	device, err := r.devices.FindDeviceForUpdate(ctx, id)
	return r.translate("find by id for update", device, err)
}

// FindByDeviceID reads one live device by its tenant-unique device_id.
func (r *DeviceRepository) FindByDeviceID(ctx context.Context, deviceID string) (*devicefleet.Device, error) {
	device, err := r.devices.FindDeviceByDeviceID(ctx, deviceID)
	return r.translate("find by device ID", device, err)
}

// LookupSessionDevice preserves the retained lookup result and failure identity.
func (r *DeviceRepository) LookupSessionDevice(ctx context.Context, code string) (int64, bool, error) {
	device, err := r.FindByDeviceID(ctx, code)
	if err != nil {
		return 0, false, err
	}
	if device == nil {
		return 0, false, nil
	}
	return device.ID, true, nil
}

// FindByAPIKey reads one live device by its API key.
func (r *DeviceRepository) FindByAPIKey(ctx context.Context, apiKey string) (*devicefleet.Device, error) {
	device, err := r.devices.FindDeviceByAPIKey(ctx, apiKey)
	return r.translate("find by API key", device, err)
}

// List reads every live device matching the retired filter map.
func (r *DeviceRepository) List(ctx context.Context, filters map[string]any) ([]*devicefleet.Device, error) {
	filter, err := toFilter(filters)
	if err != nil {
		return nil, err
	}
	return r.translateList(r.devices.ListDevices(ctx, filter))
}

func int64ID(id any) (int64, error) {
	switch value := id.(type) {
	case int64:
		return value, nil
	case int:
		return int64(value), nil
	case int32:
		return int64(value), nil
	default:
		return 0, fmt.Errorf("devicefleet repository adapter: unsupported device ID %T", id)
	}
}

func toModel(device devicefleet.Device) *devicefleet.Device { return &device }
