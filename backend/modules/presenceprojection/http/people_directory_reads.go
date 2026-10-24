package presenceprojectionhttp

import (
	context "context"

	common "github.com/moto-nrw/project-phoenix/api/common"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// findStudent reads one child, alumni included.
func (rs *Resource) findStudent(ctx context.Context, id int64) (*peopleModule.StudentState, error) {
	record, err := rs.PeopleDirectory.FindStudentRecord(ctx, id)
	if err != nil {
		return nil, err
	}
	return peopleModule.StudentStateFromRecord(record), nil
}

// studentsByIDs reads the given children, alumni included, keyed by id. A
// missing child is absent from the map.
func (rs *Resource) studentsByIDs(ctx context.Context, ids []int64) (map[int64]*peopleModule.StudentState, error) {
	if len(ids) == 0 {
		return map[int64]*peopleModule.StudentState{}, nil
	}
	records, err := rs.PeopleDirectory.ListStudentRecordsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	return peopleModule.StudentStatesByID(records), nil
}

// findPerson reads one person of the tenant.
func (rs *Resource) findPerson(ctx context.Context, id int64) (*peopleModule.Person, error) {
	person, err := rs.PeopleDirectory.FindPerson(ctx, id)
	if err != nil {
		return nil, err
	}
	return &person, nil
}

// personsByIDs reads the given persons keyed by id; a missing person is
// absent from the map.
func (rs *Resource) personsByIDs(ctx context.Context, ids []int64) (map[int64]*peopleModule.Person, error) {
	result := make(map[int64]*peopleModule.Person, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	persons, err := rs.PeopleDirectory.ListPersonsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range persons {
		result[persons[i].ID] = &persons[i]
	}
	return result, nil
}

// hasFullAccessToStudent is the snapshot access decision for a child the
// caller holds; a child that could not be loaded stays redacted.
func hasFullAccessToStudent(access *common.StudentAccessContext, student *peopleModule.StudentState) bool {
	return student != nil && access.HasFullAccess()
}

// studentsByGroups reads the children of the given groups in scope.
func (rs *Resource) studentsByGroups(ctx context.Context, groupIDs []int64, scope peopleModule.StudentScope) ([]*peopleModule.StudentState, error) {
	if len(groupIDs) == 0 && scope == peopleModule.StudentScopeAll {
		return []*peopleModule.StudentState{}, nil
	}
	records, err := rs.PeopleDirectory.ListStudentRecordsByGroup(ctx, groupIDs, scope)
	if err != nil {
		return nil, err
	}
	return peopleModule.StudentStatesFromRecords(records), nil
}
