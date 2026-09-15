package compose

import (
	"context"
	"time"
)

func (e engine) CountStudentGuardianLinks(ctx context.Context, studentID, personID int64) (int, error) {
	count, err := e.students.CountGuardianLinks(ctx, studentID, personID)
	return count, mapError(err)
}

func (e engine) DeleteLegacyGuardianLinks(ctx context.Context, personID int64) (int64, error) {
	rows, err := e.students.DeleteLegacyGuardianLinks(ctx, personID)
	return rows, mapError(err)
}

func (e engine) DeleteStudent(ctx context.Context, id int64) (int64, error) {
	rows, err := e.students.Delete(ctx, id)
	return rows, mapError(err)
}

func (e engine) AnonymizeDeletedStudentPerson(ctx context.Context, personID int64, updatedAt time.Time) (bool, error) {
	anonymized, err := e.service.AnonymizeIfUnchanged(ctx, personID, updatedAt)
	return anonymized, mapError(err)
}
