package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"

	"github.com/moto-ogs/project-phoenix/internal/careplanning"
)

type DayLocks struct {
	Database Database
	Student  func(context.Context, int64) error
	NotFound error
	Students []func(context.Context, []int64) (int, error)
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

// LockRosterDays holds all student rows before taking their care-day locks,
// both in stable order, with two statements independent of batch size.
func (l DayLocks) LockRosterDays(ctx context.Context, days map[int64][]string) error {
	db, tenantID, err := l.Database(ctx)
	if err != nil {
		return err
	}
	ids, rows, err := rosterDayKeys(tenantID, days)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	if len(l.Students) != 1 || l.Students[0] == nil {
		return errors.New("roster day locks: student batch lock is required")
	}
	count, err := l.Students[0](ctx, ids)
	if err != nil {
		return err
	}
	if count != len(ids) {
		return careplanning.ErrStudentNotFound
	}
	payload, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	_, err = db.NewRaw(`WITH ordered AS MATERIALIZED (
  SELECT * FROM jsonb_to_recordset(?::jsonb) AS x(student_id bigint, date text, key text) ORDER BY student_id,date
 ) SELECT pg_advisory_xact_lock(hashtext(key)) FROM ordered`, string(payload)).Exec(ctx)
	return err
}

type rosterDayKey struct {
	StudentID int64  `json:"student_id"`
	Date      string `json:"date"`
	Key       string `json:"key"`
}

func rosterDayKeys(tenantID int64, days map[int64][]string) ([]int64, []rosterDayKey, error) {
	ids := make([]int64, 0, len(days))
	rows := []rosterDayKey{}
	for id, dates := range days {
		if id <= 0 {
			return nil, nil, errors.New("student id is required")
		}
		ids = append(ids, id)
		for _, date := range dates {
			if _, err := calendar.ParseDate(date); err != nil {
				return nil, nil, err
			}
			rows = append(rows, rosterDayKey{id, date, fmt.Sprintf("care-exception-day:%d:%d:%s", tenantID, id, date)})
		}
	}
	slices.Sort(ids)
	return ids, rows, nil
}
