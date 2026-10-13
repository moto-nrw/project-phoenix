package compose

import (
	"context"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

// CaregiverPlannedSupervisionBlockers serves People Directory's caregiver
// capability the planned activity supervisions a staff member holds, as its
// own blocker rows (#3754).
type CaregiverPlannedSupervisionBlockers struct {
	Supervisors activitiesModels.SupervisorPlannedRepository
}

// ListPlannedSupervisionBlockers returns the converted rows, or the owner's
// error.
func (b CaregiverPlannedSupervisionBlockers) ListPlannedSupervisionBlockers(ctx context.Context, staffID, tenantID int64) ([]usersModels.BlockerActivity, error) {
	rows, err := b.Supervisors.ListPlannedSupervisionBlockers(ctx, staffID, tenantID)
	if err != nil {
		return nil, err
	}
	result := make([]usersModels.BlockerActivity, 0, len(rows))
	for _, row := range rows {
		result = append(result, usersModels.BlockerActivity{
			ID: row.ID, ActivityID: row.ActivityID, ActivityName: row.ActivityName, IsPrimary: row.IsPrimary,
		})
	}
	return result, nil
}
