package application

import (
	"context"
	"errors"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// StudentDirectoryRows supplies directory operations directly from the native services.
type StudentDirectoryRows struct {
	directory *StudentService
	photos    *StudentPhotoService
}

func NewStudentDirectoryRows(directory *StudentService, photos *StudentPhotoService) *StudentDirectoryRows {
	return &StudentDirectoryRows{directory: directory, photos: photos}
}

func (d *StudentDirectoryRows) ListStudents(
	ctx context.Context,
	filter peopleModule.StudentDirectoryFilter,
) ([]*peopleModule.StudentState, error) {
	filter, err := peopleModule.NormalizeStudentDirectoryFilter(filter)
	if err != nil {
		return nil, err
	}
	records, err := d.directory.ListDirectory(ctx, toDomainStudentDirectoryFilter(filter))
	if err != nil {
		return nil, MapError(err)
	}
	if len(records) == 0 {
		return nil, nil
	}
	result := make([]*peopleModule.StudentState, 0, len(records))
	for _, record := range records {
		result = append(result, peopleModule.StudentStateFromRecord(peopleModule.StudentRecord(record)))
	}
	return result, nil
}

func (d *StudentDirectoryRows) CountStudents(ctx context.Context, filter peopleModule.StudentDirectoryFilter) (int, error) {
	filter, err := peopleModule.NormalizeStudentDirectoryFilter(filter)
	if err != nil {
		return 0, err
	}
	filter.Page, filter.PageSize = 0, 0
	total, err := d.directory.CountDirectory(ctx, toDomainStudentDirectoryFilter(filter))
	return total, MapError(err)
}

func (d *StudentDirectoryRows) ListStudentIDs(ctx context.Context) ([]int64, error) {
	ids, err := d.directory.ListDirectoryIDs(ctx)
	return ids, MapError(err)
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

func (d *StudentDirectoryRows) GetStudentsByID(ctx context.Context, ids []int64) ([]*peopleModule.StudentState, error) {
	ids = peopleModule.PositiveIDs(ids)
	if len(ids) == 0 {
		return []*peopleModule.StudentState{}, nil
	}
	records, err := d.directory.ListRecordsByIDs(ctx, ids)
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]*peopleModule.StudentState, 0, len(records))
	for _, record := range records {
		result = append(result, peopleModule.StudentStateFromRecord(peopleModule.StudentRecord(record)))
	}
	return result, nil
}

func (d *StudentDirectoryRows) LockStudent(ctx context.Context, id int64) (*peopleModule.StudentState, error) {
	if id <= 0 {
		return nil, peopleModule.ErrStudentRowMissing
	}
	record, err := d.directory.FindRecord(ctx, id, "UPDATE")
	if err != nil {
		return nil, translateStudentDirectoryError(MapError(err))
	}
	return peopleModule.StudentStateFromRecord(peopleModule.StudentRecord(record)), nil
}

func (d *StudentDirectoryRows) LockPhotoFeature(ctx context.Context) error {
	return mapPhotoError(d.photos.LockFeature(ctx))
}

func (d *StudentDirectoryRows) LockClassWritesShared(ctx context.Context) error {
	return MapError(d.directory.LockEnrollmentClassWrites(ctx))
}
