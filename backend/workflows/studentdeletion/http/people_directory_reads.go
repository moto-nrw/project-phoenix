package studentdeletionhttp

import (
	context "context"

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
