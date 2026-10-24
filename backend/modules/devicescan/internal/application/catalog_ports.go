package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

// Activities is the activity catalog the special rooms provision from.
type ActivityCatalog interface {
	Find(ctx context.Context, id int64) (*devicescan.Activity, error)
	ListByName(ctx context.Context, name string) ([]devicescan.Activity, error)
	ListCategories(ctx context.Context) ([]devicescan.Category, error)
	CreateCategory(ctx context.Context, category devicescan.NewCategory) (devicescan.Category, error)
	CreateActivity(ctx context.Context, activity devicescan.NewActivity) (devicescan.Activity, error)
}

// Sessions are the room session transitions of the retained presence service.
type SessionOperations interface {
	// ListOpenInRoom returns the running sessions of a room, without names.
	ListOpenInRoom(ctx context.Context, roomID int64) ([]devicescan.Session, error)
	// FindDeviceSessionInRoom is nil, nil when the device runs no session there.
	FindDeviceSessionInRoom(ctx context.Context, roomID, deviceID int64) (*devicescan.Session, error)
	// Current is the device's running session with its names; nil, nil when none.
	Current(ctx context.Context, deviceID int64) (*devicescan.Session, error)
	Start(ctx context.Context, session devicescan.NewSession) (devicescan.Session, error)
	// EnsureRoomSession returns the room's open session of a system
	// activity, creating a device-less one next to running activity
	// sessions (ADR 0018). It does not report whether it created the row.
	EnsureRoomSession(ctx context.Context, session devicescan.NewSession) (devicescan.Session, error)
	DeleteActiveGroup(ctx context.Context, sessionID int64) error
	// End closes a session; an already ended session is not an error.
	End(ctx context.Context, sessionID int64) error
	// UpdateSessionActivity refreshes the session heartbeat.
	UpdateSessionActivity(ctx context.Context, sessionID int64) error
	Supervisors(ctx context.Context, sessionID int64) ([]devicescan.Supervisor, error)
	ReplaceSupervisors(ctx context.Context, sessionID int64, staffIDs []int64) error
}
