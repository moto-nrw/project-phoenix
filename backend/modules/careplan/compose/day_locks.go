package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/uptrace/bun"
)

type DayLocks interface {
	LockStudentAndExceptionDay(context.Context, int64, string) error
	LockExceptionDay(context.Context, int64, string) error
}

func NewDayLocks(db *bun.DB, student func(context.Context, int64) error, notFound error) (DayLocks, error) {
	if db == nil || student == nil || notFound == nil {
		return nil, errors.New("care plan day locks: database, student lock, and not-found error are required")
	}
	return postgres.DayLocks{Database: postgres.CarePlanDatabase(db), Student: student, NotFound: notFound}, nil
}
