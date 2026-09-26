package students

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// personRecordsFake is a func-field PersonRecords double for the handler unit
// tests; an unset function fails loudly instead of answering a zero value.
type personRecordsFake struct {
	listStudentsForDay func(context.Context, []int64, timezone.Date, timezone.Date) ([]peopleModule.StudentRecord, error)
}

var errPersonRecordsFakeUnset = errors.New("person records fake: method not configured")

func (personRecordsFake) CreatePerson(context.Context, peopleModule.CreatePerson) (peopleModule.Person, error) {
	return peopleModule.Person{}, errPersonRecordsFakeUnset
}

func (personRecordsFake) UpdatePerson(context.Context, peopleModule.UpdatePerson) (peopleModule.Person, error) {
	return peopleModule.Person{}, errPersonRecordsFakeUnset
}

func (personRecordsFake) DeletePerson(context.Context, int64) error { return errPersonRecordsFakeUnset }

func (personRecordsFake) AssignStudentTag(context.Context, int64, string) error {
	return errPersonRecordsFakeUnset
}

func (personRecordsFake) UnassignTag(context.Context, int64) error { return errPersonRecordsFakeUnset }

func (f personRecordsFake) ListStudentsForDay(ctx context.Context, groupIDs []int64, date, today timezone.Date) ([]peopleModule.StudentRecord, error) {
	if f.listStudentsForDay == nil {
		return nil, errPersonRecordsFakeUnset
	}
	return f.listStudentsForDay(ctx, groupIDs, date, today)
}

func (personRecordsFake) StaffIDForPerson(context.Context, int64) (int64, bool, error) {
	return 0, false, errPersonRecordsFakeUnset
}
