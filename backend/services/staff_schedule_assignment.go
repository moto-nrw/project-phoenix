package services

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
)

// staffScheduleRecords is the Workforce employment query: the template
// binding and rotation anchor are Workforce's own facts (#2753), so the
// schedule resolution reads them from their owner in one statement.
type staffScheduleRecords interface {
	StaffEmployments(context.Context, []int64) (map[int64]workforce.StaffEmployment, error)
}

type staffScheduleAssignments struct{ source staffScheduleRecords }

func StaffScheduleAssignments(source staffScheduleRecords) timetracking.StaffScheduleQuery {
	return staffScheduleAssignments{source: source}
}

func (q staffScheduleAssignments) ScheduleAssignment(ctx context.Context, staffID int64) (*timetracking.StaffScheduleAssignment, error) {
	profiles, err := q.source.StaffEmployments(ctx, []int64{staffID})
	if err != nil {
		return nil, err
	}
	profile, ok := profiles[staffID]
	if !ok {
		return nil, fmt.Errorf("staff %d has no employment profile: %w", staffID, sql.ErrNoRows)
	}
	var anchor *timezone.Date
	if profile.RotationAnchorDate != "" {
		date := timezone.Date(profile.RotationAnchorDate)
		anchor = &date
	}
	return &timetracking.StaffScheduleAssignment{WorkTimeModelID: profile.WorkTimeModelID, RotationAnchorDate: anchor}, nil
}
