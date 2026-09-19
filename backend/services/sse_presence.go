package services

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/usercontext"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeService "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/services/active"
)

type sseGroupQuery interface {
	QueryLiveGroups(context.Context, studentpresence.LiveGroupFilter) ([]studentpresence.LiveGroup, error)
	QueryGroupSupervisions(context.Context, studentpresence.GroupSupervisionFilter) ([]studentpresence.GroupSupervision, error)
}

type ssePresence struct {
	groups sseGroupQuery
}

func NewSSEPresence(groups sseGroupQuery) usercontext.SSEPresence {
	return ssePresence{groups: groups}
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
		return nil, &activeService.ActiveError{Op: "GetStaffActiveSupervisions", Err: activeService.ErrDatabaseOperation}
	}
	ids := make([]int64, 0, len(rows))
	today := timezone.TodayDate()
	for _, row := range rows {
		start, err := timezone.ParseDate(row.StartDate)
		if err != nil {
			return nil, &activeService.ActiveError{Op: "GetStaffActiveSupervisions", Err: activeService.ErrDatabaseOperation}
		}
		if start.After(today) {
			continue
		}
		if row.EndDate != nil {
			end, err := timezone.ParseDate(*row.EndDate)
			if err != nil {
				return nil, &activeService.ActiveError{Op: "GetStaffActiveSupervisions", Err: activeService.ErrDatabaseOperation}
			}
			if !today.Before(end) {
				continue
			}
		}
		ids = append(ids, row.GroupID)
	}
	return ids, nil
}

func (s ssePresence) ListSSEGroups(ctx context.Context) ([]studentpresence.LiveGroup, error) {
	rows, err := s.groups.QueryLiveGroups(ctx, studentpresence.LiveGroupFilter{})
	if err != nil {
		return nil, &activeService.ActiveError{Op: "ListActiveGroups", Err: fmt.Errorf("list failed: %w", err)}
	}
	return rows, nil
}
