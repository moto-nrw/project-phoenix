package owners

import (
	"context"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

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
func (b CaregiverSupervisionBlockers) ListActiveSupervisionBlockers(ctx context.Context, staffID int64) ([]peopleValues.BlockerSupervision, error) {
	rows, err := b.Records.ListActiveSupervisionBlockers(ctx, staffID)
	if rows == nil {
		return nil, err
	}
	result := make([]peopleValues.BlockerSupervision, len(rows))
	for i, row := range rows {
		result[i] = peopleValues.BlockerSupervision{ID: row.ID, GroupID: row.GroupID, GroupName: row.GroupName, StartDate: row.StartDate}
	}
	return result, err
}
