package ports

import (
	"errors"
	"time"
)

var ErrCombinedGroupNotFound = errors.New("combined group not found")

type CombinedGroup struct {
	ID, TenantID         int64
	CreatedAt, UpdatedAt time.Time
	StartTime            time.Time
	EndTime              *time.Time
}

// CombinedGroupFilter selects open combinations or those overlapping an inclusive time range.
type CombinedGroupFilter struct {
	// Active uses the current database time; future end times still count as active.
	Active        *bool
	ID            *int64
	Limit, Offset int
	OpenOnly      bool
	From, Until   *time.Time
}
