package presence

import (
	"context"
	"time"
)

// SessionDeviceDirectory supplies the device operations used by attendance.
type SessionDeviceRecords interface {
	ManualAttendanceDeviceID(context.Context) (int64, error)
	UpdateRoomID(context.Context, int64, int64) error
}

type SessionDeviceDirectory interface {
	SessionDeviceRecords
	OnlineWindow(context.Context) time.Duration
}
