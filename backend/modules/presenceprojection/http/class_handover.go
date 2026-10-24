package presenceprojectionhttp

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ClassHandoverAbsence is a child the Lehrkraft marked "nicht da" when she
// handed her class over to the OGS (#4794): the class, the moment and her
// name (empty when she was removed since).
type ClassHandoverAbsence struct {
	SchoolClass  string    `json:"school_class"`
	HandedOverAt time.Time `json:"handed_over_at"`
	HandedOverBy string    `json:"handed_over_by,omitempty"`
}

// ClassHandoverReader returns the children among studentIDs a class handover
// on day marked "nicht da", in one read.
type ClassHandoverReader interface {
	HandoverAbsences(ctx context.Context, studentIDs []int64, day calendar.Date) (map[int64]ClassHandoverAbsence, error)
}

// enrichWithHandoverAbsences marks the page's children a class handover
// marked "nicht da" on day (#4794). Like the status days it needs users:read;
// without a reader nothing is marked.
func (rs *Resource) enrichWithHandoverAbsences(ctx context.Context, responses []StudentResponse, day calendar.Date) error {
	if rs.ClassHandovers == nil || len(responses) == 0 || !canReadSensitiveStudentData(ctx) {
		return nil
	}
	ids := make([]int64, 0, len(responses))
	for i := range responses {
		ids = append(ids, responses[i].ID)
	}
	absences, err := rs.ClassHandovers.HandoverAbsences(ctx, ids, day)
	if err != nil {
		return fmt.Errorf("load class handover absences: %w", err)
	}
	for i := range responses {
		if absence, ok := absences[responses[i].ID]; ok {
			responses[i].HandoverAbsent = &absence
		}
	}
	return nil
}
