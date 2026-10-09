package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	schoolStructure "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// groupSupervisionReads is the slice of the retained group repository the
// notification recipients read.
type groupSupervisionReads interface {
	ListStaffIDsByEducationGroupIDs(ctx context.Context, groupIDs []int64, on calendar.Date) ([]schoolStructure.StaffGroupID, error)
}

// notificationGroupSupervisors serves the staff recipient resolver's
// supervision port from School Structure's group reads.
type notificationGroupSupervisors struct{ groups groupSupervisionReads }

func (s notificationGroupSupervisors) ListStaffIDsByEducationGroupIDs(ctx context.Context, groupIDs []int64, on calendar.Date) ([]notifications.StaffGroupPair, error) {
	pairs, err := s.groups.ListStaffIDsByEducationGroupIDs(ctx, groupIDs, on)
	if err != nil {
		return nil, err
	}
	result := make([]notifications.StaffGroupPair, 0, len(pairs))
	for _, pair := range pairs {
		result = append(result, notifications.StaffGroupPair{StaffID: pair.StaffID, GroupID: pair.GroupID})
	}
	return result, nil
}
