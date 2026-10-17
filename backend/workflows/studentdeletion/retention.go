package studentdeletion

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The frame of the school's window after the last care day (ADR 0048): the
// setting validates it, and the retention refuses a value outside it rather
// than deleting on a wrong period.
const (
	MinRetentionDays = 30
	MaxRetentionDays = 1095
)

// ErrIsolationFailed marks a failure of the savepoint around one child. The
// tenant transaction can no longer be trusted, so the run stops.
var ErrIsolationFailed = errors.New("student retention: isolating one child failed")

// RetentionDependencies are the ports of the automatic deletion. Every one
// runs in the school's tenant transaction of the nightly cleanup.
type RetentionDependencies struct {
	Workflow *Workflow
	// Window resolves the school's window in days after the last care day.
	Window func(context.Context) (int, error)
	// Tenant is the school of the tenant transaction in ctx.
	Tenant func(context.Context) int64
	// Isolate runs one child's deletion in a savepoint, so a refused child
	// rolls back alone. A savepoint failure wraps ErrIsolationFailed.
	Isolate func(context.Context, func(context.Context) error) error
	Logger  *slog.Logger
}

// Retention deletes, in the nightly cleanup of a school, every child whose
// last care day lies more than the school's window back (#3962). It takes
// the same owner commands as a manual deletion with the reason
// retention_expired, attributed to no account.
//
// A child counts when its care ended (status inactive) or it graduated
// (status alumnus) and users.students.enrolled_until is older than the
// window. A child taken back into care has no past last care day: resuming
// care clears it, re-enrolling moves it. A graduate without a recorded last
// care day stays for the manual purge of the Abgänge list.
type Retention struct{ deps RetentionDependencies }

// RetentionResult reports one run: the deleted children, the refused ones
// the next run tries again, and the window applied.
type RetentionResult struct {
	Deleted       int
	Refused       int
	RetentionDays int
}

func NewRetention(deps RetentionDependencies) (*Retention, error) {
	if deps.Workflow == nil || deps.Window == nil || deps.Tenant == nil || deps.Isolate == nil {
		return nil, errors.New("student retention: all dependencies are required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &Retention{deps: deps}, nil
}

// DeleteExpiredStudents deletes the children of the school in ctx past the
// window. A child the deletion refuses (a linked child would lose its way
// home, a row is busy) is skipped and logged; it stays for the next run.
func (r *Retention) DeleteExpiredStudents(ctx context.Context) (RetentionResult, error) {
	days, err := r.deps.Window(ctx)
	if err != nil {
		return RetentionResult{}, fmt.Errorf("student retention: resolve window: %w", err)
	}
	if days < MinRetentionDays || days > MaxRetentionDays {
		return RetentionResult{}, fmt.Errorf("student retention: window of %d days is outside %d to %d", days, MinRetentionDays, MaxRetentionDays)
	}
	tenantID := r.deps.Tenant(ctx)
	if tenantID <= 0 {
		return RetentionResult{}, ErrUnauthorized
	}
	lastExpiredDay, err := expiredLastCareDay(r.deps.Workflow.deps.Today(), days)
	if err != nil {
		return RetentionResult{}, err
	}
	candidates, err := r.candidates(ctx, lastExpiredDay)
	if err != nil {
		return RetentionResult{}, err
	}
	result := RetentionResult{RetentionDays: days}
	actor := Actor{TenantID: tenantID}
	for _, studentID := range candidates {
		err := r.deps.Isolate(ctx, func(childCtx context.Context) error {
			return r.deleteExpired(childCtx, actor, studentID, lastExpiredDay)
		})
		switch {
		case err == nil:
			result.Deleted++
		case errors.Is(err, ErrIsolationFailed) || ctx.Err() != nil:
			return RetentionResult{}, err
		case isRetentionRefusal(err):
			result.Refused++
			r.deps.Logger.Warn("student retention skipped a child",
				"tenant_id", tenantID,
				"student_id", studentID,
				"error", err.Error(),
			)
		default:
			return RetentionResult{}, fmt.Errorf("student retention: delete child %d: %w", studentID, err)
		}
	}
	return result, nil
}

// isRetentionRefusal identifies expected conflicts between candidate selection
// and the locked deletion. They leave that child for a later run; every other
// error aborts the step so the scheduler can retry it.
func isRetentionRefusal(err error) bool {
	return errors.Is(err, ErrStudentNotFound) ||
		errors.Is(err, ErrRetentionNotEnded) ||
		errors.Is(err, ErrPreviewChanged) ||
		errors.Is(err, ErrCompanionWouldLoseDeparture) ||
		errors.Is(err, ErrCompanionLockBusy)
}

// expiredLastCareDay is the latest last care day past a window of days: a
// child whose care ended on it has been gone for more than days days.
func expiredLastCareDay(today string, days int) (string, error) {
	day, err := calendar.ParseDate(today)
	if err != nil {
		return "", fmt.Errorf("student retention: today: %w", err)
	}
	return day.AddDays(-(days + 1)).String(), nil
}

func (r *Retention) candidates(ctx context.Context, lastExpiredDay string) ([]int64, error) {
	var ids []int64
	for _, status := range []string{peopledirectory.StudentStatusInactive, peopledirectory.StudentStatusAlumnus} {
		records, err := r.deps.Workflow.deps.Directory.ListStudentRecordsDueForStatus(ctx, status, peopledirectory.StudentBoundCareEnd, lastExpiredDay)
		if err != nil {
			return nil, fmt.Errorf("student retention: list %s children: %w", status, err)
		}
		for _, record := range records {
			ids = append(ids, record.ID)
		}
	}
	return ascending(ids), nil
}

// deleteExpired locks the child like a manual deletion, re-checks under the
// lock that its care still ended on or before lastExpiredDay, and deletes it.
func (r *Retention) deleteExpired(ctx context.Context, actor Actor, studentID int64, lastExpiredDay string) error {
	w := r.deps.Workflow
	preview, err := w.lockedSnapshot(ctx, studentID)
	if err != nil {
		return err
	}
	student, err := w.deps.Directory.ReadEnrollmentStudent(ctx, studentID, "")
	if err != nil {
		return err
	}
	ended := student.Status == peopledirectory.StudentStatusInactive || student.Status == peopledirectory.StudentStatusAlumnus
	if !ended || student.EnrolledUntil == "" || student.EnrolledUntil > lastExpiredDay {
		return ErrRetentionNotEnded
	}
	// A graduate's deletion counts its rows like the purge of the Abgänge
	// list: the child and its person.
	primaryRows := 1
	if preview.Alumnus {
		primaryRows = 2
	}
	_, err = w.mutate(ctx, actor, preview, ReasonRetentionExpired, primaryRows)
	return err
}
