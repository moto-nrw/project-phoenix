package services

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

type sseGroupQuery interface {
	QueryLiveGroups(context.Context, studentpresence.LiveGroupFilter) ([]studentpresence.LiveGroup, error)
	QueryGroupSupervisions(context.Context, studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error)
}

// ssePresence reads the room sessions a live-update client subscribes to.
type ssePresence struct {
	groups sseGroupQuery
}

func (s ssePresence) GetStaffActiveGroupIDs(ctx context.Context, staffID int64) ([]int64, error) {
	return activeStaffGroupIDs(ctx, s.groups, staffID)
}

func activeStaffGroupIDs(ctx context.Context, presence interface {
	QueryGroupSupervisions(context.Context, studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error)
}, staffID int64) ([]int64, error) {
	day := timezone.TodayDate().String()
	rows, err := presence.QueryGroupSupervisions(ctx, studentpresence.GroupSupervisionFilter{StaffID: &staffID, ActiveOn: &day})
	if err != nil {
		return nil, &studentpresence.OperationError{Op: "GetStaffActiveSupervisions", Err: studentpresence.ErrDatabaseOperation}
	}
	// ActiveOn is the owner's rule: started on or before the day, not ended by it.
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.GroupID)
	}
	return ids, nil
}

func (s ssePresence) ListSSEGroups(ctx context.Context) ([]studentpresence.LiveGroup, error) {
	rows, err := s.groups.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{})
	if err != nil {
		return nil, &studentpresence.OperationError{Op: "ListActiveGroups", Err: fmt.Errorf("list failed: %w", err)}
	}
	return rows, nil
}
