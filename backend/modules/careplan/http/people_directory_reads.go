package careplanhttp

import (
	context "context"
	errors "errors"

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

// lockStudent reads one child and holds its row lock for the caller's
// transaction.
func (rs *Resource) lockStudent(ctx context.Context, id int64) (*peopleModule.StudentState, error) {
	record, err := rs.PeopleDirectory.FindStudentRecordForMutation(ctx, id)
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

// saveStudent persists the child through the owner's write command and
// reflects the stored row back onto it.
func (rs *Resource) saveStudent(ctx context.Context, student *peopleModule.StudentState, create bool) error {
	if student == nil {
		return errors.New("student cannot be nil")
	}
	var (
		stored peopleModule.StudentRecord
		err    error
	)
	if create {
		stored, err = rs.PeopleDirectory.CreateStudent(ctx, student.DirectoryWrite())
	} else {
		stored, err = rs.PeopleDirectory.UpdateStudent(ctx, student.DirectoryWrite())
	}
	if err != nil {
		return peopleModule.TranslateStudentWriteError(err)
	}
	student.ApplyDirectoryRecord(stored)
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

// hasFullAccessToStudent is the snapshot access decision for a child the
// caller holds; a child that could not be loaded stays redacted.
func hasFullAccessToStudent(access *common.StudentAccessContext, student *peopleModule.StudentState) bool {
	return student != nil && access.HasFullAccess()
}

// applyPhotoConsent reconciles a requested photo consent on the locked row
// the caller holds; the caller persists it with its own write.
func (rs *Resource) applyPhotoConsent(ctx context.Context, requested *bool, fresh *peopleModule.StudentState) error {
	if fresh == nil {
		return nil
	}
	updated, err := rs.StudentPhotos.ApplyStudentPhotoConsent(ctx, peopleModule.StudentPhotoState{
		StudentID:           fresh.ID,
		PhotoPath:           fresh.PhotoPath,
		PhotoConsentGivenAt: fresh.PhotoConsentGivenAt,
		PhotoConsentGivenBy: fresh.PhotoConsentGivenBy,
	}, requested)
	if err != nil {
		return err
	}
	fresh.PhotoPath = updated.PhotoPath
	fresh.PhotoConsentGivenAt = updated.PhotoConsentGivenAt
	fresh.PhotoConsentGivenBy = updated.PhotoConsentGivenBy
	return nil
}
