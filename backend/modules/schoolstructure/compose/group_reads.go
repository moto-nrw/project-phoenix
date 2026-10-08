package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/education"
)

// Group reads adapt the private store directly. Lifecycle orchestration stays
// in the application; a second query-only service layer is not needed.
type groupReads struct{ groupRepo GroupRecords }

func (s *groupReads) GetGroup(ctx context.Context, id int64) (*education.Group, error) {
	group, err := s.groupRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &EducationError{Op: "GetGroup", Err: ErrGroupNotFound}
	}
	return group, nil
}

func (s *groupReads) GetGroupsByIDs(ctx context.Context, ids []int64) (map[int64]*education.Group, error) {
	if len(ids) == 0 {
		return make(map[int64]*education.Group), nil
	}

	groups, err := s.groupRepo.FindByIDs(ctx, ids)
	if err != nil {
		return nil, &EducationError{Op: "GetGroupsByIDs", Err: err}
	}

	return groups, nil
}

func (s *groupReads) ListGroups(ctx context.Context, query *education.GroupListQuery) ([]*education.Group, error) {
	groups, err := s.groupRepo.ListWithRooms(ctx, query)
	if err != nil {
		return nil, &EducationError{Op: "ListGroups", Err: err}
	}
	return groups, nil
}

func (s *groupReads) CountGroups(ctx context.Context, query *education.GroupListQuery) (int, error) {
	count, err := s.groupRepo.CountGroups(ctx, query)
	if err != nil {
		return 0, &EducationError{Op: "CountGroups", Err: err}
	}
	return count, nil
}

func (s *groupReads) FindGroupWithRoom(ctx context.Context, groupID int64) (*education.Group, error) {
	group, err := s.groupRepo.FindWithRoom(ctx, groupID)
	if err != nil {
		return nil, &EducationError{Op: "FindGroupWithRoom", Err: ErrGroupNotFound}
	}
	return group, nil
}

func (s *groupReads) GetGroupsWithRoomsByIDs(ctx context.Context, ids []int64) (map[int64]*education.Group, error) {
	groups, err := s.groupRepo.FindByIDsWithRooms(ctx, ids)
	if err != nil {
		return nil, &EducationError{Op: "GetGroupsWithRoomsByIDs", Err: err}
	}
	return groups, nil
}
