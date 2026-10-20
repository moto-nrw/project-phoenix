package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

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
	UnclaimedGroups(context.Context, Date) ([]studentpresence.UnclaimedGroup, Stats, error)
	ClaimGroup(context.Context, GroupClaim) (ClaimedSupervision, Stats, error)
}
