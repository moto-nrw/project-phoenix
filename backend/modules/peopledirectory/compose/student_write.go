package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (e engine) CreateStudent(
	ctx context.Context,
	write peopledirectory.StudentWrite,
) (peopledirectory.StudentRecord, error) {
	record, err := e.students.CreateStudent(ctx, toApplicationStudentWrite(write))
	return peopledirectory.StudentRecord(record), mapError(err)
}

func (e engine) UpdateStudent(
	ctx context.Context,
	write peopledirectory.StudentWrite,
) (peopledirectory.StudentRecord, error) {
	record, err := e.students.UpdateStudent(ctx, toApplicationStudentWrite(write))
	return peopledirectory.StudentRecord(record), mapError(err)
}

func (e engine) DeleteStudentRecord(ctx context.Context, studentID int64) error {
	return mapError(e.students.DeleteStudent(ctx, studentID))
}

func (e engine) VerifyStudentStrandingBatch(ctx context.Context) error {
	return mapError(e.students.VerifyStrandingBatch(ctx))
}

func toApplicationStudentWrite(write peopledirectory.StudentWrite) application.StudentWrite {
	return application.StudentWrite{
		Record:        domain.StudentRecord(write.Record),
		Plan:          write.Plan,
		Baseline:      write.Baseline,
		CompanionNote: write.CompanionNote,
		NoteSupplied:  write.NoteSupplied,
	}
}

func (e engine) SetStudentStatus(ctx context.Context, studentID int64, status string) error {
	return mapError(e.students.SetStudentStatus(ctx, studentID, status))
}

func (e engine) TransitionStudentStatus(ctx context.Context, studentID int64, expected, next string) (bool, error) {
	moved, err := e.students.TransitionStudentStatus(ctx, studentID, expected, next)
	return moved, mapError(err)
}

func (e engine) SetStudentCareEnd(ctx context.Context, ids []int64, until string) (int64, error) {
	affected, err := e.students.SetStudentCareEnd(ctx, ids, until)
	return affected, mapError(err)
}

func (e engine) ReopenStudentCare(ctx context.Context, studentID int64, from, status string) error {
	return mapError(e.students.ReopenStudentCare(ctx, studentID, from, status))
}

func (e engine) ListStudentCareEnds(ctx context.Context, ids []int64) (map[int64]string, error) {
	bounds, err := e.students.ListStudentCareEnds(ctx, ids)
	return bounds, mapError(err)
}
