package studentpresence

import (
	"errors"
	"time"
)

var (
	ErrGroupNotFound      = errors.New("active group not found")
	ErrGroupEnded         = errors.New("cannot claim ended group")
	ErrAlreadySupervising = errors.New("staff already supervising group")
)

// UnclaimedGroup is an open session without current supervision. Room and
// activity display data remain the responsibility of their owners.
type UnclaimedGroup struct {
	ID, TenantID                                  int64
	CreatedAt, UpdatedAt, StartTime, LastActivity time.Time
	EndTime                                       *time.Time
	TimeoutMinutes                                int
	GroupID, DeviceID                             *int64
	RoomID                                        int64
}

type GroupClaim struct {
	GroupID, StaffID int64
	Role, Date       string
}

type ClaimedSupervision struct {
	ID, TenantID, GroupID, StaffID int64
	CreatedAt, UpdatedAt           time.Time
	Role, StartDate                string
}
