package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// CaregiverPlannedSupervisionBlockers serves People Directory's caregiver
// capability the planned activity supervisions a staff member holds, as its
// own blocker rows (#3754).
type CaregiverPlannedSupervisionBlockers struct {
	Supervisors timetable.PlannedSupervisorQuery
}

// ListPlannedSupervisionBlockers returns the converted rows, or the owner's
// error.
func (b CaregiverPlannedSupervisionBlockers) ListPlannedSupervisionBlockers(ctx context.Context, staffID, tenantID int64) ([]peopledirectory.BlockerActivity, error) {
	rows, err := b.Supervisors.ListPlannedSupervisionBlockers(ctx, staffID, tenantID)
	if err != nil {
		return nil, err
	}
	result := make([]peopledirectory.BlockerActivity, 0, len(rows))
	for _, row := range rows {
		result = append(result, peopledirectory.BlockerActivity{
			ID: row.ID, ActivityID: row.ActivityID, ActivityName: row.ActivityName, IsPrimary: row.IsPrimary,
		})
	}
	return result, nil
}
