package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
)

type DayLocks struct {
	Database Database
	Student  func(context.Context, int64) error
	NotFound error
}

func (l DayLocks) LockStudentAndExceptionDay(ctx context.Context, id int64, date string) error {
	if id <= 0 {
		return errors.New("student id is required")
	}
	if _, _, err := l.Database(ctx); err != nil {
		return fmt.Errorf("lock student for care exception day: %w", err)
	}
	if err := l.Student(ctx, id); err != nil {
		if errors.Is(err, l.NotFound) {
			err = careplanning.ErrStudentNotFound
		}
		return fmt.Errorf("lock student for care exception day: %w", err)
	}
	return l.LockExceptionDay(ctx, id, date)
}
func (l DayLocks) LockExceptionDay(ctx context.Context, id int64, date string) error {
	db, tenantID, err := l.Database(ctx)
	if err != nil {
		return fmt.Errorf("lock care exception day: %w", err)
	}
	if err := LockExceptionDay(ctx, db, tenantID, id, date); err != nil {
		return fmt.Errorf("lock care exception day: %w", err)
	}
	return nil
}
