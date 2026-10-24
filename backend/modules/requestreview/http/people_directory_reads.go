package requestreviewhttp

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
