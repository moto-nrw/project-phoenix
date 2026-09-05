package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/uptrace/bun"
)

type DayLocks interface {
	LockStudentAndExceptionDay(context.Context, int64, string) error
	LockExceptionDay(context.Context, int64, string) error
}

type dayLocks struct {
	database postgres.Database
	student  func(context.Context, int64) error
	notFound error
}

func NewDayLocks(db *bun.DB, student func(context.Context, int64) error, notFound error) (DayLocks, error) {
	if db == nil || student == nil || notFound == nil {
		return nil, errors.New("care plan day locks: database, student lock, and not-found error are required")
	}
	return dayLocks{database: carePlanDatabase(db), student: student, notFound: notFound}, nil
}

func (l dayLocks) LockStudentAndExceptionDay(ctx context.Context, id int64, date string) error {
	if id <= 0 {
		return errors.New("student id is required")
	}
	if err := l.student(ctx, id); err != nil {
		if errors.Is(err, l.notFound) {
			err = careplanning.ErrStudentNotFound
		}
		return fmt.Errorf("lock student for care exception day: %w", err)
	}
	return l.LockExceptionDay(ctx, id, date)
}

func (l dayLocks) LockExceptionDay(ctx context.Context, id int64, date string) error {
	db, tenantID, err := l.database(ctx)
	if err != nil {
		return fmt.Errorf("lock care exception day: %w", err)
	}
	if err := postgres.LockExceptionDay(ctx, db, tenantID, id, date); err != nil {
		return fmt.Errorf("lock care exception day: %w", err)
	}
	return nil
}
