package ports

import (
	"time"
)

type GroupSupervision struct {
	ID, TenantID, GroupID, StaffID int64
	CreatedAt, UpdatedAt           time.Time
	Role                           string
	StartDate                      Date
	EndDate                        *Date
}

type GroupSupervisionFilter struct {
	StaffIDs      []int64
	EndedBy       *Date
	Limit, Offset int
	IDs           []int64
	OpenOnly      bool
	StartedBefore *Date
	// ForUpdate requires a caller-owned tenant transaction.
	ForUpdate bool
	GroupIDs  []int64
	StaffID   *int64
	ActiveOn  *Date
}
