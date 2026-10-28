package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

type Service struct {
	store   ports.Store
	tx      ports.Transaction
	observe func(ports.Observation)
	// The session attendance rules resolve their participants through the
	// Timetable owner and the care plan; a composition that does not serve
	// those rules leaves the ports unbound and the rules report it.
	roster   ports.PlannedRoster
	carePlan ports.CarePlanDirectory
	careDays ports.CareDayLocker
	// now is the clock the day-bound reads take the school day from.
	now func() time.Time
}

// New builds the service. The attendance rule ports may be nil in a
// composition that serves no attendance rules; the rules that need one then
// fail with a configuration error. now is the clock of the day-bound reads.
func New(store ports.Store, tx ports.Transaction, observe func(ports.Observation), rules ports.AttendanceRulePorts, now func() time.Time) *Service {
	if store == nil || tx == nil || observe == nil || now == nil {
		panic("student presence: all application dependencies are required")
	}
	return &Service{store: store, tx: tx, observe: observe, roster: rules.Roster, carePlan: rules.CarePlan, careDays: rules.CareDays, now: now}
}

func (s *Service) run(operation string, fn func() (ports.Stats, error)) error {
	started := time.Now()
	stats, err := fn()
	s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	return err
}

func validIDs(ids []int64) bool {
	for _, id := range ids {
		if id <= 0 {
			return false
		}
	}
	return true
}

func (s *Service) ListOpenPresence(ctx context.Context, ids []int64) (result []int64, err error) {
	err = s.runWrite(ctx, "list_open_presence", func(ctx context.Context) (ports.Stats, error) {
		if !validIDs(ids) {
			return ports.Stats{}, errors.New("student presence: invalid student ID")
		}
		var stats ports.Stats
		result, stats, err = s.store.ListOpenPresence(ctx, ids)
		return stats, err
	})
	return result, err
}

// LockOpenVisits holds the current group's visit locks until the surrounding
// workflow commits. It requires an existing tenant transaction.
func (s *Service) LockOpenVisits(ctx context.Context, groupID int64) error {
	return s.run("lock_open_visits", func() (ports.Stats, error) {
		if groupID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid active group ID")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		return s.store.LockOpenVisits(ctx, groupID)
	})
}

// RestoreVisits reopens exactly the closed visits in a completion snapshot.
// A mismatch fails the surrounding recovery transaction.
func (s *Service) RestoreVisits(ctx context.Context, ids []int64) error {
	return s.run("restore_visits", func() (ports.Stats, error) {
		if !validIDs(ids) {
			return ports.Stats{}, errors.New("student presence: invalid visit ID")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		return s.store.RestoreVisits(ctx, ids)
	})
}

// CloseOpenPresence closes all recorded attendance and visits for a care exit.
// It joins the caller's transaction so roster and care-plan writes remain atomic.
func (s *Service) CloseOpenPresence(ctx context.Context, ids []int64, at time.Time) (int64, error) {
	var result int64
	err := s.run("close_open_presence", func() (ports.Stats, error) {
		if !validIDs(ids) || at.IsZero() {
			return ports.Stats{}, errors.New("student presence: invalid presence close")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		stats, err := s.store.CloseOpenPresence(ctx, ids, at)
		if err == nil {
			result = stats.Rows
		}
		return stats, err
	})
	return result, err
}

// LatestPresenceDate returns the last attendance or visit day as YYYY-MM-DD.
// Visit instants are interpreted in the school's Europe/Berlin calendar.
func (s *Service) LatestPresenceDate(ctx context.Context, studentID int64) (result *string, err error) {
	err = s.runWrite(ctx, "latest_presence_date", func(ctx context.Context) (ports.Stats, error) {
		if studentID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid student ID")
		}
		var stats ports.Stats
		result, stats, err = s.store.LatestPresenceDate(ctx, studentID)
		return stats, err
	})
	return result, err
}

// CountAttendanceRecords counts the student's attendance rows plus scheduled
// checkouts; the deletion preview reports both as attendance records.
func (s *Service) CountAttendanceRecords(ctx context.Context, studentID int64) (result int, err error) {
	err = s.runWrite(ctx, "count_attendance_records", func(ctx context.Context) (ports.Stats, error) {
		if studentID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid student ID")
		}
		var stats ports.Stats
		result, stats, err = s.store.CountAttendanceRecords(ctx, studentID)
		return stats, err
	})
	return result, err
}

// CountStudentVisitsForDeletion counts every visit of the child, including
// holiday-care visits hosted by another tenant that the home tenant's RLS
// hides. The permanent-deletion preview reports them because the cascade
// removes them with the child.
func (s *Service) CountStudentVisitsForDeletion(ctx context.Context, studentID int64) (result int, err error) {
	err = s.runWrite(ctx, "count_student_visits_for_deletion", func(ctx context.Context) (ports.Stats, error) {
		if studentID <= 0 {
			return ports.Stats{}, errors.New("student presence: invalid student ID")
		}
		var stats ports.Stats
		result, stats, err = s.store.CountStudentVisitsForDeletion(ctx, studentID)
		return stats, err
	})
	return result, err
}

func (s *Service) LockOpenPresence(ctx context.Context, ids []int64) error {
	return s.run("lock_open_presence", func() (ports.Stats, error) {
		if !validIDs(ids) {
			return ports.Stats{}, errors.New("student presence: invalid student ID")
		}
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		return s.store.LockOpenPresence(ctx, ids)
	})
}

// LockGroupSupervisions takes a SHARE ROW EXCLUSIVE table lock on the live
// group supervision rows for the caller's transaction, so a caregiver
// capability re-check cannot race a concurrent supervision write. Callers
// serializing staff lifecycle changes take it alongside the other binding
// owners' locks.
func (s *Service) LockGroupSupervisions(ctx context.Context) error {
	return s.run("lock_group_supervisions", func() (ports.Stats, error) {
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		return s.store.LockGroupSupervisions(ctx)
	})
}

func (s *Service) runQuery(ctx context.Context, forUpdate bool, operation string, fn func(context.Context) (ports.Stats, error)) error {
	if forUpdate {
		return s.run(operation, func() (ports.Stats, error) { return fn(ctx) })
	}
	return s.runWrite(ctx, operation, fn)
}

// listResult keeps the owner's list contract: a failed read carries no rows
// and a successful one returns a non-nil slice.
func listResult[T any](rows []T, err error) ([]T, error) {
	if err != nil {
		return nil, err
	}
	if rows == nil {
		return []T{}, nil
	}
	return rows, nil
}
