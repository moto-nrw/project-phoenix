package application

import (
	"context"
	"errors"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// This file is the translation seam of the staff student directory (#3349):
// People Directory owns the filtered, paginated read over users.students, and
// the retained handlers still carry users.Student rows. It holds no filter
// rules of its own — which columns a caller may narrow by is the owner's
// decision, expressed by peopledirectory.StudentDirectoryFilter.

// StudentDirectoryCapability is the owner surface this seam translates to.
type StudentDirectoryCapability interface {
	peopleModule.StudentDirectoryQuery
	peopleModule.StudentDirectoryCommand
	// LockEnrollmentClassWrites is the owner's shared class-writes gate; the
	// retained update path takes it for the same reason the enrollment flows do.
	LockEnrollmentClassWrites(ctx context.Context) error
}

// StudentDirectory adapts the owner's directory read to the retained
// model-typed contract (StudentDirectoryAccess and StudentLocker in
// modules/peopledirectory/compose, satisfied structurally).
type StudentDirectory struct{ directory StudentDirectoryCapability }

// NewStudentDirectory adapts the owner's directory read to the retained
// model-typed contract.
func NewStudentDirectory(directory StudentDirectoryCapability) *StudentDirectory {
	return &StudentDirectory{directory: directory}
}

func (d *StudentDirectory) ListStudents(
	ctx context.Context,
	filter peopleModule.StudentDirectoryFilter,
) ([]*peopleModule.StudentState, error) {
	records, err := d.directory.ListStudentDirectory(ctx, filter)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	result := make([]*peopleModule.StudentState, 0, len(records))
	for _, record := range records {
		result = append(result, peopleModule.StudentStateFromRecord(record))
	}
	return result, nil
}

func (d *StudentDirectory) CountStudents(ctx context.Context, filter peopleModule.StudentDirectoryFilter) (int, error) {
	return d.directory.CountStudentDirectory(ctx, filter)
}

func (d *StudentDirectory) ListStudentIDs(ctx context.Context) ([]int64, error) {
	return d.directory.ListStudentDirectoryIDs(ctx)
}

// translateStudentDirectoryError restates a child the owner would not hand
// over in the vocabulary the retained contract uses. A missing row and an
// unusable id collapse to the same answer on purpose: both mean "no such child
// of this tenant", which is what the retained repository reported for either,
// and what the handlers above render as 404 rather than 500.
func translateStudentDirectoryError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, peopleModule.ErrStudentNotFound) || errors.Is(err, peopleModule.ErrInvalidStudent) {
		return peopleModule.ErrStudentRowMissing
	}
	return err
}

func (d *StudentDirectory) GetStudentsByID(ctx context.Context, ids []int64) ([]*peopleModule.StudentState, error) {
	records, err := d.directory.ListStudentRecordsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]*peopleModule.StudentState, 0, len(records))
	for _, record := range records {
		result = append(result, peopleModule.StudentStateFromRecord(record))
	}
	return result, nil
}

func (d *StudentDirectory) LockStudent(ctx context.Context, id int64) (*peopleModule.StudentState, error) {
	record, err := d.directory.FindStudentRecordForMutation(ctx, id)
	if err != nil {
		return nil, translateStudentDirectoryError(err)
	}
	return peopleModule.StudentStateFromRecord(record), nil
}

func (d *StudentDirectory) LockPhotoFeature(ctx context.Context) error {
	return d.directory.LockStudentPhotoFeature(ctx)
}

func (d *StudentDirectory) LockClassWritesShared(ctx context.Context) error {
	return d.directory.LockEnrollmentClassWrites(ctx)
}
