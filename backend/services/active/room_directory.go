package active

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/active"
)

// AttendanceRooms supplies room facts and the lock used by capacity checks.
// Attendance does not create, update, or delete facility records.
type AttendanceRooms interface {
	FindByID(context.Context, interface{}) (*SessionRoom, error)
	FindByIDForUpdate(context.Context, int64) (*SessionRoom, error)
	FindByIDs(context.Context, []int64) ([]*SessionRoom, error)
	List(context.Context, map[string]interface{}) ([]*SessionRoom, error)
}

type SessionRoom = active.SessionRoom
