package repositories

import (
	"context"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// CaregiverSupervisionBlockers serves People Directory's caregiver capability
// the Student Presence supervisions a staff member still holds today, as its
// own blocker rows (#3754).
type CaregiverSupervisionBlockers struct {
	Records interface {
		ListActiveSupervisionBlockers(ctx context.Context, staffID int64) ([]studentpresence.SupervisionBlocker, error)
	}
}

// ListActiveSupervisionBlockers returns the rows the owner listed, converted,
// together with the owner's error.
func (b CaregiverSupervisionBlockers) ListActiveSupervisionBlockers(ctx context.Context, staffID int64) ([]usersModels.BlockerSupervision, error) {
	rows, err := b.Records.ListActiveSupervisionBlockers(ctx, staffID)
	if rows == nil {
		return nil, err
	}
	result := make([]usersModels.BlockerSupervision, len(rows))
	for i, row := range rows {
		result[i] = usersModels.BlockerSupervision{ID: row.ID, GroupID: row.GroupID, GroupName: row.GroupName, StartDate: row.StartDate}
	}
	return result, err
}

// CaregiverPlannedSupervisionBlockers serves People Directory's caregiver
// capability the planned activity supervisions (Timetable & Activities) a
// staff member holds, as its own blocker rows (#3754).
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
