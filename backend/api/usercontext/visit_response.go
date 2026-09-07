package usercontext

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
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

func groupVisitResponses(visits []studentpresence.Visit) []groupVisitResponse {
	var result []groupVisitResponse
	for _, visit := range visits {
		result = append(result, groupVisitResponse{
			ID: visit.ID, CreatedAt: visit.CreatedAt, UpdatedAt: visit.UpdatedAt,
			TenantID: visit.TenantID, StudentID: visit.StudentID, ActiveGroupID: visit.ActiveGroupID,
			EntryTime: visit.EntryTime, ExitTime: visit.ExitTime,
		})
	}
	return result
}
