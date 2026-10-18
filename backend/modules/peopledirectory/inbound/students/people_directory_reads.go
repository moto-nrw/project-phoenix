package students

import (
	context "context"
	errors "errors"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	tenant "github.com/moto-nrw/project-phoenix/tenant"
)

// findStudent reads one child, alumni included.
func (rs *Resource) findStudent(ctx context.Context, id int64) (*peopleModule.StudentState, error) {
	record, err := rs.PeopleDirectory.FindStudentRecord(ctx, id)
	if err != nil {
		return nil, err
	}
	return peopleModule.StudentStateFromRecord(record), nil
}

// lockStudent reads one child and holds its row lock for the caller's
// transaction.
func (rs *Resource) lockStudent(ctx context.Context, id int64) (*peopleModule.StudentState, error) {
	record, err := rs.PeopleDirectory.FindStudentRecordForMutation(ctx, id)
	if err != nil {
		return nil, err
	}
	return peopleModule.StudentStateFromRecord(record), nil
}

// findPerson reads one person of the tenant.
func (rs *Resource) findPerson(ctx context.Context, id int64) (*peopleModule.Person, error) {
	person, err := rs.PeopleDirectory.FindPerson(ctx, id)
	if err != nil {
		return nil, err
	}
	return &person, nil
}

// errPhotoNoTenant refuses a photo route reached without a tenant context.
// Every other outcome is the owner's.
var errPhotoNoTenant = errors.New("no tenant context")

func requirePhotoTenant(ctx context.Context) error {
	if tenant.FromContext(ctx) <= 0 {
		return errPhotoNoTenant
	}
	return nil
}
