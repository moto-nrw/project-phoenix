package iot

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
)

// service keeps the legacy row-shaped API while Device Fleet owns the
// administration behavior and its stable error contract.
type service struct {
	devices        devicefleet.Capability
	administration devicefleet.Administration
}

func NewService(devices devicefleet.Capability) Service {
	return &service{devices: devices, administration: devicefleet.NewAdministration(devices)}
}
func (s *service) Fleet() devicefleet.Capability { return s.devices }

func (s *service) CreateDevice(ctx context.Context, row *iot.Device) error {
	if row == nil {
		return s.administration.CreateDevice(ctx, nil)
	}
	value := toAdministrationDevice(row)
	err := s.administration.CreateDevice(ctx, &value)
	*row = *toModel(value)
	return err
}

func (s *service) UpdateDevice(ctx context.Context, row *iot.Device) error {
	if row == nil {
		return s.administration.UpdateDevice(ctx, nil)
	}
	value := toAdministrationDevice(row)
	err := s.administration.UpdateDevice(ctx, &value)
	*row = *toModel(value)
	return err
}

func (s *service) GetDeviceByID(ctx context.Context, id int64) (*iot.Device, error) {
	row, err := s.administration.GetDeviceByID(ctx, id)
	if err != nil || row == nil {
		return nil, err
	}
	return toModel(*row), nil
}

func (s *service) GetDeviceByDeviceID(ctx context.Context, deviceID string) (*iot.Device, error) {
	row, err := s.administration.GetDeviceByDeviceID(ctx, deviceID)
	if err != nil || row == nil {
		return nil, err
	}
	return toModel(*row), nil
}

func (s *service) GetDevicesByType(ctx context.Context, deviceType string) ([]*iot.Device, error) {
	rows, err := s.administration.GetDevicesByType(ctx, deviceType)
	if err != nil {
		return nil, err
	}
	return administrationToModels(rows), nil
}

func (s *service) GetDevicesByStatus(ctx context.Context, status iot.DeviceStatus) ([]*iot.Device, error) {
	rows, err := s.administration.GetDevicesByStatus(ctx, devicefleet.DeviceStatus(status))
	if err != nil {
		return nil, err
	}
	return administrationToModels(rows), nil
}

func (s *service) GetDevicesByRegisteredBy(ctx context.Context, personID int64) ([]*iot.Device, error) {
	rows, err := s.administration.GetDevicesByRegisteredBy(ctx, personID)
	if err != nil {
		return nil, err
	}
	return administrationToModels(rows), nil
}

func (s *service) GetOfflineDevices(ctx context.Context, duration time.Duration) ([]*iot.Device, error) {
	rows, err := s.administration.GetOfflineDevices(ctx, duration)
	if err != nil {
		return nil, err
	}
	return administrationToModels(rows), nil
}

func (s *service) GetActiveDevices(ctx context.Context) ([]*iot.Device, error) {
	rows, err := s.administration.GetActiveDevices(ctx)
	if err != nil {
		return nil, err
	}
	return administrationToModels(rows), nil
}

func (s *service) GetDevicesRequiringMaintenance(ctx context.Context) ([]*iot.Device, error) {
	rows, err := s.administration.GetDevicesRequiringMaintenance(ctx)
	if err != nil {
		return nil, err
	}
	return administrationToModels(rows), nil
}

func (s *service) DetectNewDevices(ctx context.Context) ([]*iot.Device, error) {
	rows, err := s.administration.DetectNewDevices(ctx)
	if err != nil {
		return nil, err
	}
	return administrationToModels(rows), nil
}

func (s *service) ListDevices(ctx context.Context, filters map[string]interface{}) ([]*iot.Device, error) {
	filter, err := toFilter(filters)
	if err != nil {
		return nil, &IoTError{Op: "ListDevices", Err: err}
	}
	rows, err := s.administration.ListDevices(ctx, filter)
	if err != nil {
		return nil, err
	}
	return administrationToModels(rows), nil
}
func administrationToModels(rows []*devicefleet.Device) []*iot.Device {
	result := make([]*iot.Device, 0, len(rows))
	for _, row := range rows {
		result = append(result, toModel(*row))
	}
	return result
}
func (s *service) DeleteDevice(ctx context.Context, id int64) error {
	return s.administration.DeleteDevice(ctx, id)
}
func (s *service) PingDevice(ctx context.Context, id string) error {
	return s.administration.PingDevice(ctx, id)
}
func (s *service) UpdateDeviceStatus(ctx context.Context, id string, status iot.DeviceStatus) error {
	return s.administration.UpdateDeviceStatus(ctx, id, devicefleet.DeviceStatus(status))
}
func (s *service) GetDeviceTypeStatistics(ctx context.Context) (map[string]int, error) {
	return s.administration.GetDeviceTypeStatistics(ctx)
}
func (s *service) ScanNetwork(ctx context.Context) (map[string]string, error) {
	return s.administration.ScanNetwork(ctx)
}
func (s *service) DeviceOnlineWindow(ctx context.Context) time.Duration {
	return s.administration.DeviceOnlineWindow(ctx)
}
func (s *service) IsLastSeenOnline(ctx context.Context, lastSeen *time.Time) bool {
	return s.administration.IsLastSeenOnline(ctx, lastSeen)
}
func (s *service) IsDeviceOnline(ctx context.Context, row *iot.Device) bool {
	return s.IsDeviceOnlineAt(ctx, row, time.Now())
}
func (s *service) IsDeviceOnlineAt(ctx context.Context, row *iot.Device, now time.Time) bool {
	if row == nil {
		return false
	}
	value := toAdministrationDevice(row)
	return s.administration.IsDeviceOnlineAt(ctx, &value, now)
}
