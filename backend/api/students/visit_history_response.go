package students

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// studentVisitResponse preserves the current-visit and deprecated visit-history HTTP shape.
type studentVisitResponse struct {
	ID            int64      `json:"id"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	TenantID      int64      `json:"tenant_id"`
	StudentID     int64      `json:"student_id"`
	ActiveGroupID int64      `json:"active_group_id"`
	EntryTime     time.Time  `json:"entry_time"`
	ExitTime      *time.Time `json:"exit_time,omitempty"`
}

func newStudentVisitResponse(visit studentpresence.Visit) studentVisitResponse {
	return studentVisitResponse{
		ID: visit.ID, TenantID: visit.TenantID, CreatedAt: visit.CreatedAt, UpdatedAt: visit.UpdatedAt,
		StudentID: visit.StudentID, ActiveGroupID: visit.ActiveGroupID, EntryTime: visit.EntryTime, ExitTime: visit.ExitTime,
	}
}
