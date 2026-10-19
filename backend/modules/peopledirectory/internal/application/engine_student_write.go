package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (e Engine) CreateStudent(
	ctx context.Context,
	write peopledirectory.StudentWrite,
) (peopledirectory.StudentRecord, error) {
	record, err := e.students.CreateStudent(ctx, toApplicationStudentWrite(write))
	return peopledirectory.StudentRecord(record), MapError(err)
}

func (e Engine) UpdateStudent(
	ctx context.Context,
	write peopledirectory.StudentWrite,
) (peopledirectory.StudentRecord, error) {
	record, err := e.students.UpdateStudent(ctx, toApplicationStudentWrite(write))
	return peopledirectory.StudentRecord(record), MapError(err)
}

func (e Engine) DeleteStudentRecord(ctx context.Context, studentID int64) error {
	return MapError(e.students.DeleteStudent(ctx, studentID))
}

func (e Engine) VerifyStudentStrandingBatch(ctx context.Context) error {
	return MapError(e.students.VerifyStrandingBatch(ctx))
}

func toApplicationStudentWrite(write peopledirectory.StudentWrite) StudentWrite {
	return StudentWrite{
		Record:        domain.StudentRecord(write.Record),
		Plan:          write.Plan,
		Baseline:      write.Baseline,
		CompanionNote: write.CompanionNote,
		NoteSupplied:  write.NoteSupplied,
	}
}

func (e Engine) ListStudentCareEnds(ctx context.Context, ids []int64) (map[int64]string, error) {
	bounds, err := e.students.ListStudentCareEnds(ctx, ids)
	return bounds, MapError(err)
}
