package careexit

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ResumptionStudent contains only the membership facts needed to reopen care.
type ResumptionStudent struct {
	Status        string
	EnrolledUntil *calendar.Date
}

// ResumptionStudents locks the school-scoped membership before checking its
// current state. ResumeCare retains School Membership's conditional guard.
type ResumptionStudents interface {
	LockResumptionStudent(context.Context, int64) (ResumptionStudent, bool, error)
	ResumeCare(context.Context, int64, calendar.Date, string, calendar.Date) (bool, error)
}

// ResumptionRecords changes only Care Plan's exit and removal ledger.
type ResumptionRecords interface {
	FindCareExits(context.Context, []int64) (map[int64]careplan.CareExit, error)
	DeleteCareExits(context.Context, []int64) error
	DiscardCareExitRemovals(context.Context, []int64) error
}

// ResumptionTrail records the care-end change inside the same unit of work.
type ResumptionTrail interface {
	RecordCareEndChange(context.Context, int64, *calendar.Date, *calendar.Date, int64) error
}

// Resumption coordinates School Membership, Care Plan and the change trail.
// It never restores planning: the school reviews the returning child's plan.
type Resumption struct {
	students ResumptionStudents
	records  ResumptionRecords
	trail    ResumptionTrail
	unit     func(context.Context, func(context.Context) error) error
	today    func() calendar.Date
	logger   *slog.Logger
}

// NewResumption binds the workflow's required owner commands and outer unit.
func NewResumption(students ResumptionStudents, records ResumptionRecords, trail ResumptionTrail,
	unit func(context.Context, func(context.Context) error) error, today func() calendar.Date, logger *slog.Logger,
) *Resumption {
	if students == nil || records == nil || trail == nil || unit == nil {
		panic("care resumption: students, records, trail and unit of work are required")
	}
	if today == nil {
		today = calendar.TodayDate
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Resumption{students: students, records: records, trail: trail, unit: unit, today: today, logger: logger}
}

// Resume validates before opening the unit, then re-reads under the student
// lock. The reader takes the shared class-writes gate before the student
// locks; exit/ledger writes and audit follow. No recurrence or booking gate
// is added because no bookings, roster or weekly plan is restored.
func (s *Resumption) Resume(ctx context.Context, input careplan.CareResumeInput) error {
	if input.StudentID <= 0 {
		return careplan.ErrCareExitNoStudents
	}
	if !input.Checked {
		return careplan.ErrCareResumeNotChecked
	}
	today := s.today()
	if input.NewStart.Before(today) {
		return careplan.ErrCareResumeStartInPast
	}
	if err := s.unit(ctx, func(txCtx context.Context) error {
		return s.resumeCare(txCtx, input, today)
	}); err != nil {
		return err
	}
	s.logger.Info("care resumed",
		slog.Int64("student_id", input.StudentID),
		slog.String("new_start", input.NewStart.String()),
		slog.Int64("actor_account_id", input.ActorAccountID),
	)
	return nil
}

func (s *Resumption) resumeCare(ctx context.Context, input careplan.CareResumeInput, today calendar.Date) error {
	student, found, err := s.students.LockResumptionStudent(ctx, input.StudentID)
	if err != nil {
		return err
	}
	if !found {
		return careplan.ErrCareResumeUnknownStudent
	}
	if student.Status == careplan.StudentStatusAlumnus {
		return careplan.ErrCareResumeAlumnus
	}
	if student.EnrolledUntil == nil || !today.After(*student.EnrolledUntil) {
		return careplan.ErrCareResumeNotEnded
	}
	exits, err := s.records.FindCareExits(ctx, []int64{input.StudentID})
	if err != nil {
		return fmt.Errorf("care lifecycle: find care exits: %w", err)
	}
	if _, recorded := exits[input.StudentID]; !recorded {
		return careplan.ErrCareResumeMissing
	}
	status := careplan.StudentStatusPending
	if !input.NewStart.After(today) {
		status = careplan.StudentStatusActive
	}
	resumed, err := s.students.ResumeCare(ctx, input.StudentID, input.NewStart, status, today)
	if err != nil {
		return err
	}
	if !resumed {
		return careplan.ErrCareResumeNotEnded
	}
	if err := s.records.DeleteCareExits(ctx, []int64{input.StudentID}); err != nil {
		return fmt.Errorf("care lifecycle: delete care exits: %w", err)
	}
	if err := s.records.DiscardCareExitRemovals(ctx, []int64{input.StudentID}); err != nil {
		return fmt.Errorf("care lifecycle: discard care exit removals: %w", err)
	}
	return s.trail.RecordCareEndChange(ctx, input.StudentID, student.EnrolledUntil, nil, input.ActorAccountID)
}
