package ports

import (
	"context"
	"errors"
	"time"
)

var (
	ErrGroupNotFound      = errors.New("active group not found")
	ErrGroupEnded         = errors.New("cannot claim ended group")
	ErrAlreadySupervising = errors.New("staff already supervising group")
)

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
	Role             string
	Date             Date
}

type ClaimedSupervision struct {
	ID, TenantID, GroupID, StaffID int64
	CreatedAt, UpdatedAt           time.Time
	Role                           string
	StartDate                      Date
}

type UnclaimedStore interface {
	UnclaimedGroups(context.Context, Date) ([]UnclaimedGroup, Stats, error)
	ClaimGroup(context.Context, GroupClaim) (ClaimedSupervision, Stats, error)
}
