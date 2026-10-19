package owners

import (
	"context"
	"maps"
	"slices"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// deliveryStudentGroups names the education group of each reported child
// from People Directory.
type deliveryStudentGroups struct {
	students interface {
		FindReadScopeByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.StudentState, error)
	}
}

func (g deliveryStudentGroups) StudentGroupIDs(ctx context.Context, studentIDs []int64) (map[int64]int64, error) {
	students, err := g.students.FindReadScopeByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	groups := make(map[int64]int64, len(students))
	for studentID, student := range students {
		if student != nil && student.GroupID != nil {
			groups[studentID] = *student.GroupID
		}
	}
	return groups, nil
}

// deliveryOnDutyStaff names today's staff on duty by Workforce's rule.
type deliveryOnDutyStaff struct {
	presence workforceCompose.WorkPresenceQuery
}

func (s deliveryOnDutyStaff) OnDutyStaffIDs(ctx context.Context) ([]int64, error) {
	presence, err := s.presence.GetTodayPresenceMap(ctx)
	if err != nil {
		return nil, err
	}
	return slices.Sorted(maps.Keys(workforce.OnDutyStaffIDs(presence))), nil
}
