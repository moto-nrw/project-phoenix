package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/uptrace/bun"
)

func LockEngineStudentDay(ctx context.Context, db *bun.DB, studentID int64, date string) error {
	if _, _, err := CarePlanDatabase(db)(ctx); err != nil {
		return err
	}
	return careplanning.LockStudentAndExceptionDay(ctx, db, studentID, date)
}
