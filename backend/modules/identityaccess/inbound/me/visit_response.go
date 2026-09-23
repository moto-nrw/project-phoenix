package me

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// groupVisitResponse preserves the existing group-visits HTTP representation.
type groupVisitResponse struct {
	ID            int64      `json:"id"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	TenantID      int64      `json:"tenant_id"`
	StudentID     int64      `json:"student_id"`
	ActiveGroupID int64      `json:"active_group_id"`
	EntryTime     time.Time  `json:"entry_time"`
	ExitTime      *time.Time `json:"exit_time,omitempty"`
}

func groupVisitResponses(visits []identityaccess.CallerVisit) []groupVisitResponse {
	var result []groupVisitResponse
	for _, visit := range visits {
		result = append(result, groupVisitResponse(visit))
	}
	return result
}
