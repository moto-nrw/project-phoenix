package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/uptrace/bun"
)

// EngineLocks binds the service's existing student and day locks to its database.
type EngineLocks struct{ DB *bun.DB }

func (r EngineLocks) LockStudentAndExceptionDay(ctx context.Context, studentID int64, date string) error {
	return LockEngineStudentDay(ctx, r.DB, studentID, date)
}

func (r EngineLocks) LockExceptionDay(ctx context.Context, studentID int64, date string) error {
	return careplanning.LockExceptionDay(ctx, r.DB, studentID, date)
}

func (r EngineLocks) LockStudent(ctx context.Context, studentID int64) error {
	return careplanning.LockStudent(ctx, r.DB, studentID)
}
