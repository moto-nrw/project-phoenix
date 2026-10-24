package postgres

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/uptrace/bun"
)

func LockExcusalStudentDay(ctx context.Context, db *bun.DB, id int64, date string) error {
	err := careplanning.LockStudentAndExceptionDay(ctx, db, id, date)
	if errors.Is(err, careplanning.ErrStudentNotFound) {
		return careplan.ErrPartialAbsenceNotFound
	}
	return err
}
