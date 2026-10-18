package application

import (
	"context"
	"time"
)

func (e Engine) CountStudentGuardianLinks(ctx context.Context, studentID, personID int64) (int, error) {
	count, err := e.students.CountGuardianLinks(ctx, studentID, personID)
	return count, MapError(err)
}

func (e Engine) DeleteLegacyGuardianLinks(ctx context.Context, personID int64) (int64, error) {
	rows, err := e.students.DeleteLegacyGuardianLinks(ctx, personID)
	return rows, MapError(err)
}

func (e Engine) DeleteStudent(ctx context.Context, id int64) (int64, error) {
	rows, err := e.students.Delete(ctx, id)
	return rows, MapError(err)
}

func (e Engine) AnonymizeDeletedStudentPerson(ctx context.Context, personID int64, updatedAt time.Time) (bool, error) {
	anonymized, err := e.service.AnonymizeIfUnchanged(ctx, personID, updatedAt)
	return anonymized, MapError(err)
}
