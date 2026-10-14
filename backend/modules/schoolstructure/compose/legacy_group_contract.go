package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// LegacyGroupRepository is the retained contract of education.groups the
// legacy composition hands to its consumers (#2742, #3556), served by
// GroupRepository. It goes with the last consumer that uses it.
type LegacyGroupRepository interface {
	Create(ctx context.Context, group *Group) error
	FindByID(ctx context.Context, id any) (*Group, error)
	Update(ctx context.Context, group *Group) error
	Delete(ctx context.Context, id any) error
	List(ctx context.Context, filters map[string]any) ([]*Group, error)
	FindByIDForUpdate(ctx context.Context, id any) (*Group, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*Group, error)
	// FindByIDsWithRooms is the bulk sibling of FindWithRoom (#2094 review).
	FindByIDsWithRooms(ctx context.Context, ids []int64) (map[int64]*Group, error)
	// Exists reports whether a group with the given ID exists in the current
	// tenant (issue #584).
	Exists(ctx context.Context, id int64) (bool, error)
	// ListWithRooms returns groups and their optional room.
	ListWithRooms(ctx context.Context, query *GroupListQuery) ([]*Group, error)
	// CountGroups counts the groups the overview filters select.
	CountGroups(ctx context.Context, query *GroupListQuery) (int, error)
	FindByName(ctx context.Context, name string) (*Group, error)
	FindByTeacher(ctx context.Context, teacherID int64) ([]*Group, error)
	FindWithRoom(ctx context.Context, groupID int64) (*Group, error)
	// ListStaffIDsByEducationGroupIDs answers who supervises these groups on
	// the given day. It mirrors the caller context's group chain from the
	// group side, so the two must agree on the join shape.
	ListStaffIDsByEducationGroupIDs(ctx context.Context, groupIDs []int64, on calendar.Date) ([]StaffGroupID, error)
}

var _ LegacyGroupRepository = (*GroupRepository)(nil)

// GroupNames resolves education group names by id for the enrollment class
// roster, which prints a child's group next to its name (#3563). A group id
// without a row is absent from the result.
type GroupNames struct {
	groups interface {
		FindByIDs(ctx context.Context, ids []int64) (map[int64]*Group, error)
	}
}

// NewGroupNames binds the lookup to the retained group repository.
func NewGroupNames(groups LegacyGroupRepository) GroupNames {
	return GroupNames{groups: groups}
}

// GroupNamesByID returns the names of the groups in one query.
func (g GroupNames) GroupNamesByID(ctx context.Context, groupIDs []int64) (map[int64]string, error) {
	groups, err := g.groups.FindByIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(groups))
	for id, group := range groups {
		if group != nil {
			names[id] = group.Name
		}
	}
	return names, nil
}
