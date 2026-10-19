package iot

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
)

// service keeps the legacy row-shaped API while Device Fleet owns the
// administration behavior and its stable error contract.
type service struct {
	devicefleet.Administration
}

func NewService(devices devicefleet.Capability) Service {
	return &service{Administration: devicefleet.NewAdministration(devices)}
}

func (s *service) CreateDevice(ctx context.Context, row *devicefleet.Device) error {
	if row == nil {
		return s.Administration.CreateDevice(ctx, nil)
	}
	value := toAdministrationDevice(row)
	err := s.Administration.CreateDevice(ctx, &value)
	*row = *toModel(value)
	return err
}

func (s *service) UpdateDevice(ctx context.Context, row *devicefleet.Device) error {
	if row == nil {
		return s.Administration.UpdateDevice(ctx, nil)
	}
	value := toAdministrationDevice(row)
	err := s.Administration.UpdateDevice(ctx, &value)
	*row = *toModel(value)
	return err
}

func (s *service) GetDeviceByID(ctx context.Context, id int64) (*devicefleet.Device, error) {
	row, err := s.Administration.GetDeviceByID(ctx, id)
	if err != nil || row == nil {
		return nil, err
	}
	return toModel(*row), nil
}

func (s *service) GetDeviceByDeviceID(ctx context.Context, deviceID string) (*devicefleet.Device, error) {
	row, err := s.Administration.GetDeviceByDeviceID(ctx, deviceID)
	if err != nil || row == nil {
		return nil, err
	}
	return toModel(*row), nil
}

func (s *service) GetDevicesByType(ctx context.Context, deviceType string) ([]*devicefleet.Device, error) {
	rows, err := s.Administration.GetDevicesByType(ctx, deviceType)
	if err != nil {
		return nil, err
	}
	return administrationToModels(rows), nil
}

func (s *service) GetDevicesByStatus(ctx context.Context, status devicefleet.DeviceStatus) ([]*devicefleet.Device, error) {
	rows, err := s.Administration.GetDevicesByStatus(ctx, devicefleet.DeviceStatus(status))
	if err != nil {
		return nil, err
	}
	return administrationToModels(rows), nil
}

func (s *service) GetDevicesByRegisteredBy(ctx context.Context, personID int64) ([]*devicefleet.Device, error) {
	rows, err := s.Administration.GetDevicesByRegisteredBy(ctx, personID)
	if err != nil {
		return nil, err
	}
	return administrationToModels(rows), nil
}

func (s *service) GetOfflineDevices(ctx context.Context, duration time.Duration) ([]*devicefleet.Device, error) {
	rows, err := s.Administration.GetOfflineDevices(ctx, duration)
	if err != nil {
		return nil, err
	}
	return administrationToModels(rows), nil
}

func (s *service) GetActiveDevices(ctx context.Context) ([]*devicefleet.Device, error) {
	rows, err := s.Administration.GetActiveDevices(ctx)
	if err != nil {
		return nil, err
	}
	return administrationToModels(rows), nil
}

func (s *service) GetDevicesRequiringMaintenance(ctx context.Context) ([]*devicefleet.Device, error) {
	rows, err := s.Administration.GetDevicesRequiringMaintenance(ctx)
	if err != nil {
		return nil, err
	}
	return administrationToModels(rows), nil
}

func (s *service) DetectNewDevices(ctx context.Context) ([]*devicefleet.Device, error) {
	rows, err := s.Administration.DetectNewDevices(ctx)
	if err != nil {
		return nil, err
	}
	return administrationToModels(rows), nil
}

func (s *service) ListDevices(ctx context.Context, filters map[string]interface{}) ([]*devicefleet.Device, error) {
	filter, err := toFilter(filters)
	if err != nil {
		return nil, &IoTError{Op: "ListDevices", Err: err}
	}
	rows, err := s.Administration.ListDevices(ctx, filter)
	if err != nil {
		return nil, err
	}
	return administrationToModels(rows), nil
}
func administrationToModels(rows []*devicefleet.Device) []*devicefleet.Device {
	result := make([]*devicefleet.Device, 0, len(rows))
	for _, row := range rows {
		result = append(result, toModel(*row))
	}
	return result
}

func (s *service) UpdateDeviceStatus(ctx context.Context, id string, status devicefleet.DeviceStatus) error {
	return s.Administration.UpdateDeviceStatus(ctx, id, devicefleet.DeviceStatus(status))
}

func (s *service) IsDeviceOnline(ctx context.Context, row *devicefleet.Device) bool {
	return s.IsDeviceOnlineAt(ctx, row, time.Now())
}
func (s *service) IsDeviceOnlineAt(ctx context.Context, row *devicefleet.Device, now time.Time) bool {
	if row == nil {
		return false
	}
	value := toAdministrationDevice(row)
	return s.Administration.IsDeviceOnlineAt(ctx, &value, now)
}
