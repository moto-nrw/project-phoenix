// Package application is the device-scan workflow (#2698): it turns one
// kiosk card scan into the authoritative attendance, visit and session
// transition of the request and renders the kiosk projection the public
// contract promises. Every foreign fact arrives through the public Device
// Fleet, Student Presence and Facilities capabilities or through the
// consumer-owned ports; the workflow never touches persistence itself.
package application

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
)

// Fleet is the part of the Device Fleet capability the scans use:
// online windows, the heartbeat write, and unregistered card bookkeeping.
type Fleet interface {
	IsDeviceOnline(context.Context, devicefleet.Device) bool
	FindDeviceByDeviceID(context.Context, string) (devicefleet.Device, error)
	UpdateDeviceLastSeen(context.Context, int64, time.Time) error
	RecordUnregisteredTagScan(context.Context, devicefleet.RecordUnregisteredTagScan) (devicefleet.UnregisteredTagScan, error)
}

// Presence is the part of the Student Presence capability the scans read:
// the open visit counts that drive capacity checks and the kiosk counter.
type Presence interface {
	CountOpenVisitsInRoom(context.Context, int64) (int, error)
	CountOpenVisitsInGroup(context.Context, int64) (int, error)
}

// Rooms is the part of the Facilities capability the scans use.
type Rooms interface {
	FindRoom(context.Context, int64) (facilities.Room, error)
	FindRoomByName(context.Context, string) (facilities.Room, error)
	FindToiletRoom(context.Context, int64) (facilities.Room, error)
	CreateRoom(context.Context, facilities.CreateRoom) (facilities.Room, error)
}

// Dependencies are the collaborators of the workflow. Fleet, Presence,
// Rooms, Principals, People, Visits, Sessions, Attendance, Settings,
// UnitOfWork and Clock are required; Activities, Groups and Pickups may be
// absent in narrow graphs, which disables the flows that need them.
type Dependencies struct {
	Fleet      Fleet
	Presence   Presence
	Rooms      Rooms
	Principals ports.Principals
	People     ports.People
	Visits     ports.Visits
	Sessions   ports.Sessions
	Attendance ports.Attendance
	Activities ports.Activities
	Groups     ports.Groups
	Pickups    ports.Pickups
	Settings   ports.Settings
	UnitOfWork ports.UnitOfWork
	Clock      ports.Clock
	Logger     *slog.Logger
}

// Service is the device-scan workflow.
type Service struct {
	fleet      Fleet
	presence   Presence
	rooms      Rooms
	principals ports.Principals
	people     ports.People
	visits     ports.Visits
	sessions   ports.Sessions
	attendance ports.Attendance
	activities ports.Activities
	groups     ports.Groups
	pickups    ports.Pickups
	settings   ports.Settings
	unit       ports.UnitOfWork
	clock      ports.Clock
	logger     *slog.Logger
}

var _ devicescan.DeviceScan = (*Service)(nil)

// NewService composes the workflow over its dependencies.
func NewService(deps Dependencies) *Service {
	if deps.Fleet == nil || deps.Presence == nil || deps.Rooms == nil || deps.Principals == nil ||
		deps.People == nil || deps.Visits == nil || deps.Sessions == nil || deps.Attendance == nil ||
		deps.Settings == nil || deps.UnitOfWork == nil || deps.Clock == nil {
		panic("device scan: fleet, presence, rooms, principals, people, visits, sessions, attendance, settings, unit of work and clock are required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		fleet: deps.Fleet, presence: deps.Presence, rooms: deps.Rooms, principals: deps.Principals,
		people: deps.People, visits: deps.Visits, sessions: deps.Sessions, attendance: deps.Attendance,
		activities: deps.Activities, groups: deps.Groups, pickups: deps.Pickups, settings: deps.Settings,
		unit: deps.UnitOfWork, clock: deps.Clock, logger: logger,
	}
}

func (s *Service) now() time.Time { return s.clock.Now() }

// device resolves the authenticated device of the request.
func (s *Service) device(ctx context.Context) (*ports.Device, error) {
	device, ok := s.principals.Device(ctx)
	if !ok || device == nil {
		return nil, devicescan.ErrDeviceUnauthorized
	}
	return device, nil
}

func publicDevice(device *ports.Device) devicescan.Device {
	return devicescan.Device{
		ID: device.ID, DeviceID: device.DeviceID, DeviceType: device.DeviceType,
		Name: device.Name, Status: device.Status, LastSeen: device.LastSeen, Active: device.Active,
	}
}

// Device returns the authenticated device or ErrDeviceUnauthorized.
func (s *Service) Device(ctx context.Context) (devicescan.Device, error) {
	device, err := s.device(ctx)
	if err != nil {
		return devicescan.Device{}, err
	}
	return publicDevice(device), nil
}

// Ping records the heartbeat of the device and, when it runs a session,
// keeps that session alive. The session refresh is best-effort: the device
// learns that a session exists even when the heartbeat write failed.
func (s *Service) Ping(ctx context.Context) (*devicescan.DevicePing, error) {
	device, err := s.device(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.pingDevice(ctx, device.DeviceID); err != nil {
		return nil, err
	}

	sessionActive := false
	if session, err := s.sessions.Current(ctx, device.ID); err == nil && session != nil {
		sessionActive = true
		if err := s.sessions.Touch(ctx, session.ID); err != nil {
			s.logger.WarnContext(ctx, "failed to update session activity during ping",
				slog.Int64("session_id", session.ID),
				slog.String("error", err.Error()),
			)
		}
	}

	return &devicescan.DevicePing{
		Device:        publicDevice(device),
		IsOnline:      s.fleet.IsDeviceOnline(ctx, devicefleet.Device{LastSeen: device.LastSeen}),
		SessionActive: sessionActive,
		PingTime:      s.now(),
	}, nil
}

// pingDevice writes the device's last-seen instant by its globally unique
// row id, so the ping stays cross-tenant safe.
func (s *Service) pingDevice(ctx context.Context, deviceID string) error {
	// Preserve the legacy PingDevice wire message and classification without
	// importing the retained IoT service into this workflow.
	const prefix = "IoT service error in PingDevice: "
	if deviceID == "" {
		return devicescan.Internal(prefix+"device ID cannot be empty", nil)
	}
	existing, err := s.fleet.FindDeviceByDeviceID(ctx, deviceID)
	if err != nil {
		return devicescan.Internal(prefix+err.Error(), err)
	}
	if existing.ID <= 0 {
		return devicescan.NotFound(prefix + "device not found: " + deviceID)
	}
	if err := s.fleet.UpdateDeviceLastSeen(ctx, existing.ID, s.now()); err != nil {
		return devicescan.Internal(prefix+err.Error(), err)
	}
	return nil
}

// Status reports the authenticated device's own view.
func (s *Service) Status(ctx context.Context) (*devicescan.DeviceStatus, error) {
	device, err := s.device(ctx)
	if err != nil {
		return nil, err
	}
	return &devicescan.DeviceStatus{
		Device:          publicDevice(device),
		IsOnline:        s.fleet.IsDeviceOnline(ctx, devicefleet.Device{LastSeen: device.LastSeen}),
		AuthenticatedAt: s.now(),
	}, nil
}

// recordUnregisteredScan best-effort persists a scan of a card that
// resolves to no person, stamped with the scanning device. A persistence
// error only logs: the scan response must not fail because bookkeeping did.
func (s *Service) recordUnregisteredScan(ctx context.Context, device *ports.Device, tag string) {
	var deviceID *int64
	if device != nil && device.ID > 0 {
		id := device.ID
		deviceID = &id
	}
	if _, err := s.fleet.RecordUnregisteredTagScan(ctx, devicefleet.RecordUnregisteredTagScan{TagUID: tag, DeviceID: deviceID}); err != nil {
		s.logger.ErrorContext(ctx, "failed to record unregistered RFID scan",
			slog.String("rfid", tag),
			slog.String("error", err.Error()),
		)
	}
}
