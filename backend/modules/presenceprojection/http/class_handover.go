package presenceprojectionhttp

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ClassHandover is the handover of a child to the OGS with its class
// (#4794): the class, the moment, the Lehrkraft's name (empty when she was
// removed since) and whether she marked the child "nicht da".
type ClassHandover struct {
	SchoolClass  string    `json:"school_class"`
	HandedOverAt time.Time `json:"handed_over_at"`
	HandedOverBy string    `json:"handed_over_by,omitempty"`
	Absent       bool      `json:"absent"`
}

// ClassHandoverReader returns, in one read, the handover on day of each
// child among studentIDs the Lehrkraft handed over to the OGS.
type ClassHandoverReader interface {
	ClassHandovers(ctx context.Context, studentIDs []int64, day calendar.Date) (map[int64]ClassHandover, error)
}

// enrichWithClassHandovers marks the page's children the Lehrkraft handed
// over on day (#4794). Like the status days it needs users:read; without a
// reader nothing is marked.
func (rs *Resource) enrichWithClassHandovers(ctx context.Context, responses []StudentResponse, day calendar.Date) error {
	if rs.ClassHandovers == nil || len(responses) == 0 || !canReadSensitiveStudentData(ctx) {
		return nil
	}
	ids := make([]int64, 0, len(responses))
	for i := range responses {
		ids = append(ids, responses[i].ID)
	}
	handovers, err := rs.ClassHandovers.ClassHandovers(ctx, ids, day)
	if err != nil {
		return fmt.Errorf("load class handovers: %w", err)
	}
	for i := range responses {
		if handover, ok := handovers[responses[i].ID]; ok {
			responses[i].ClassHandover = &handover
		}
	}
	return nil
}
