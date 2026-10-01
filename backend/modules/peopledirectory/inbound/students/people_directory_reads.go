package students

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/api/common"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The child and person reads the routes run on the People Directory
// capability, in the shapes the handlers carry.

// findStudent reads one child, alumni included.
func (rs *Resource) findStudent(ctx context.Context, id int64) (*Student, error) {
	record, err := rs.PeopleDirectory.FindStudentRecord(ctx, id)
	if err != nil {
		return nil, err
	}
	return studentFromRecord(record), nil
}

// lockStudent reads one child and holds its row lock for the caller's
// transaction.
func (rs *Resource) lockStudent(ctx context.Context, id int64) (*Student, error) {
	record, err := rs.PeopleDirectory.FindStudentRecordForMutation(ctx, id)
	if err != nil {
		return nil, err
	}
	return studentFromRecord(record), nil
}

// studentsByIDs reads the given children, alumni included, keyed by id. A
// missing child is absent from the map.
func (rs *Resource) studentsByIDs(ctx context.Context, ids []int64) (map[int64]*Student, error) {
	if len(ids) == 0 {
		return map[int64]*Student{}, nil
	}
	records, err := rs.PeopleDirectory.ListStudentRecordsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	return studentsByID(records), nil
}

// saveStudent persists the child through the owner's write command and
// reflects the stored row back onto it.
func (rs *Resource) saveStudent(ctx context.Context, student *Student, create bool) error {
	if student == nil {
		return errors.New("student cannot be nil")
	}
	var (
		stored peopleModule.StudentRecord
		err    error
	)
	if create {
		stored, err = rs.PeopleDirectory.CreateStudent(ctx, student.write())
	} else {
		stored, err = rs.PeopleDirectory.UpdateStudent(ctx, student.write())
	}
	if err != nil {
		return translateStudentWriteError(err)
	}
	student.applyRecord(stored)
	return nil
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

// errPhotoNoTenant refuses a photo route reached without a tenant context.
// Every other outcome is the owner's.
var errPhotoNoTenant = errors.New("no tenant context")

func requirePhotoTenant(ctx context.Context) error {
	if tenant.FromContext(ctx) <= 0 {
		return errPhotoNoTenant
	}
	return nil
}

// hasFullAccessToStudent is the snapshot access decision for a child the
// caller holds; a child that could not be loaded stays redacted.
func hasFullAccessToStudent(access *common.StudentAccessContext, student *Student) bool {
	return student != nil && access.HasFullAccess()
}

// studentsByGroups reads the children of the given groups in scope.
func (rs *Resource) studentsByGroups(ctx context.Context, groupIDs []int64, scope peopleModule.StudentScope) ([]*Student, error) {
	if len(groupIDs) == 0 && scope == peopleModule.StudentScopeAll {
		return []*Student{}, nil
	}
	records, err := rs.PeopleDirectory.ListStudentRecordsByGroup(ctx, groupIDs, scope)
	if err != nil {
		return nil, err
	}
	return studentsFromRecords(records), nil
}

// applyPhotoConsent reconciles a requested photo consent on the locked row
// the caller holds; the caller persists it with its own write.
func (rs *Resource) applyPhotoConsent(ctx context.Context, requested *bool, fresh *Student) {
	if fresh == nil {
		return
	}
	updated := rs.StudentPhotos.ApplyStudentPhotoConsent(ctx, peopleModule.StudentPhotoState{
		StudentID:           fresh.ID,
		PhotoPath:           fresh.PhotoPath,
		PhotoConsentGivenAt: fresh.PhotoConsentGivenAt,
		PhotoConsentGivenBy: fresh.PhotoConsentGivenBy,
	}, requested)
	fresh.PhotoPath = updated.PhotoPath
	fresh.PhotoConsentGivenAt = updated.PhotoConsentGivenAt
	fresh.PhotoConsentGivenBy = updated.PhotoConsentGivenBy
}
