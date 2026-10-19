package devicefleet

import "context"

// SessionDeviceRecords supplies device identity and room attribution without rows.
type SessionDeviceRecords interface {
	LookupSessionDevice(context.Context, string) (id int64, found bool, err error)
	UpdateRoomID(context.Context, int64, int64) error
}

// SessionDeviceDirectory is the attendance boundary's device port.
type SessionDeviceDirectory interface {
	ManualAttendanceDeviceID(context.Context) (int64, error)
	UpdateRoomID(context.Context, int64, int64) error
}
