package timetracking

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func ReadScheduleAssignment(ctx context.Context, staffID int64, source interface {
	StaffEmployments(context.Context, []int64) (map[int64]workforce.StaffEmployment, error)
}, notFound error) (*StaffScheduleAssignment, error) {
	profiles, err := source.StaffEmployments(ctx, []int64{staffID})
	if err != nil {
		return nil, err
	}
	profile, ok := profiles[staffID]
	if !ok {
		return nil, fmt.Errorf("staff %d has no employment profile: %w", staffID, notFound)
	}
	var anchor *timezone.Date
	if profile.RotationAnchorDate != "" {
		date := timezone.Date(profile.RotationAnchorDate)
		anchor = &date
	}
	return &StaffScheduleAssignment{WorkTimeModelID: profile.WorkTimeModelID, RotationAnchorDate: anchor}, nil
}
