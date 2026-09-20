package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

const bulkPickupOperation = "bulk upsert pickup schedules"

func (s *pickupScheduleService) BulkUpsertPickupSchedules(ctx context.Context, filter careplan.PickupBulkFilter, inputs []careplan.PickupScheduleInput, createdBy int64) (*careplan.BulkUpsertResult, error) {
	parsed, err := parseBulkPickups(filter.StudentIDs, inputs)
	if err != nil {
		return nil, &careplan.ScheduleError{Op: bulkPickupOperation, Err: err}
	}
	if s.students == nil {
		return nil, &careplan.ScheduleError{Op: bulkPickupOperation, Err: errors.New("bulk pickup dependencies are not configured")}
	}
	students, err := s.selectPickupStudents(ctx, filter.StudentIDs)
	if err != nil {
		return nil, &careplan.ScheduleError{Op: bulkPickupOperation, Err: err}
	}
	result := &careplan.BulkUpsertResult{AffectedStudentIDs: make([]int64, 0, len(students))}
	err = s.tx.WithinTenant(ctx, func(txCtx context.Context) error {
		locked, lockErr := s.lockPickupStudents(txCtx, students, filter.Authorize)
		if lockErr != nil {
			return lockErr
		}
		for _, student := range locked {
			if err := s.patchPickupWeek(txCtx, student, inputs, parsed, createdBy, result); err != nil {
				return err
			}
			result.AffectedStudentIDs = append(result.AffectedStudentIDs, student.ID)
		}
		return nil
	})
	if err != nil {
		return nil, &careplan.ScheduleError{Op: bulkPickupOperation, Err: err}
	}
	result.StudentsAffected = len(result.AffectedStudentIDs)
	if s.logger != nil {
		s.logger.Info("bulk upsert pickup schedules",
			"students_affected", result.StudentsAffected,
			"weekdays_set", len(inputs),
			"overwrites", len(result.OverwrittenStudents))
	}
	return result, nil
}

func parseBulkPickups(ids []int64, inputs []careplan.PickupScheduleInput) (map[int]time.Time, error) {
	if len(ids) == 0 {
		return nil, errors.New("student_ids is required")
	}
	if len(ids) > 500 {
		return nil, errors.New("student_ids cannot exceed 500 items")
	}
	if len(inputs) == 0 {
		return nil, errors.New("schedules cannot be empty")
	}
	parsed := make(map[int]time.Time, len(inputs))
	for _, input := range inputs {
		if input.Weekday < 1 || input.Weekday > 5 {
			return nil, fmt.Errorf("invalid weekday %d", input.Weekday)
		}
		if _, duplicate := parsed[input.Weekday]; duplicate {
			return nil, fmt.Errorf("duplicate weekday %d", input.Weekday)
		}
		value, err := time.Parse("2006-01-02 15:04", "2000-01-01 "+input.PickupTime)
		if err != nil {
			return nil, fmt.Errorf("invalid pickup_time %q for weekday %d: %w", input.PickupTime, input.Weekday, err)
		}
		parsed[input.Weekday] = value
	}
	return parsed, nil
}

func (s *pickupScheduleService) selectPickupStudents(ctx context.Context, ids []int64) ([]ports.PickupBulkStudent, error) {
	byID, err := s.students.FindByIDs(ctx, ids, calendar.TodayDate())
	if err != nil {
		return nil, err
	}
	students := make([]ports.PickupBulkStudent, 0, len(ids))
	for _, id := range ids {
		student, ok := byID[id]
		if !ok || !student.Eligible {
			return nil, fmt.Errorf("%w: student %d", careplan.ErrBulkStudentNotFound, id)
		}
		students = append(students, student)
	}
	sort.Slice(students, func(i, j int) bool { return students[i].ID < students[j].ID })
	return students, nil
}

func (s *pickupScheduleService) lockPickupStudents(ctx context.Context, students []ports.PickupBulkStudent, authorize func(context.Context, careplan.ScheduleStudent) (bool, error)) ([]ports.PickupBulkStudent, error) {
	locked := make([]ports.PickupBulkStudent, 0, len(students))
	for _, selected := range students {
		fresh, err := s.students.LockByID(ctx, selected.ID, calendar.TodayDate())
		if errors.Is(err, careplan.ErrBulkStudentNotFound) || err == nil && !fresh.Eligible {
			return nil, fmt.Errorf("%w: student %d", careplan.ErrBulkStudentNotFound, selected.ID)
		}
		if err != nil {
			return nil, fmt.Errorf("lock student %d: %w", selected.ID, err)
		}
		if authorize != nil {
			allowed, err := authorize(ctx, fresh.ScheduleStudent)
			if err != nil || !allowed {
				return nil, fmt.Errorf("%w: student %d", careplan.ErrBulkStudentUnauthorized, fresh.ID)
			}
		}
		locked = append(locked, fresh)
	}
	return locked, nil
}

func (s *pickupScheduleService) patchPickupWeek(ctx context.Context, student ports.PickupBulkStudent, inputs []careplan.PickupScheduleInput, parsed map[int]time.Time, createdBy int64, result *careplan.BulkUpsertResult) error {
	var before careplan.WeeklyPickupSnapshot
	if s.autoExcusal != nil {
		var err error
		before, err = s.autoExcusal.SnapshotWeeklyPickups(ctx, student.ID, calendar.TodayDate())
		if err != nil {
			return err
		}
	}
	existing, err := s.scheduleRepo.FindByStudentID(ctx, student.ID)
	if err != nil {
		return err
	}
	byWeekday := make(map[int]*careplan.PickupSchedule, len(existing))
	for _, row := range existing {
		byWeekday[row.Weekday] = row
	}
	for _, input := range inputs {
		row := &careplan.PickupSchedule{StudentID: student.ID, Weekday: input.Weekday, PickupTime: parsed[input.Weekday], CreatedBy: createdBy}
		if err := s.preservePickupNotes(ctx, student, row, byWeekday[input.Weekday], result); err != nil {
			return err
		}
		row.SetTenantID(s.tx.TenantID(ctx))
		if err := s.scheduleRepo.UpsertSchedule(ctx, row); err != nil {
			return err
		}
	}
	// Every selected student is locked before writes and per-day excusal locks.
	if s.autoExcusal != nil {
		if err := s.autoExcusal.ResyncFutureExceptions(ctx, student.ID); err != nil {
			return err
		}
		return s.autoExcusal.RecordWeeklyPickupChanges(ctx, student.ID, calendar.TodayDate(), before)
	}
	return nil
}

func (s *pickupScheduleService) preservePickupNotes(ctx context.Context, student ports.PickupBulkStudent, row, previous *careplan.PickupSchedule, result *careplan.BulkUpsertResult) error {
	if previous == nil {
		return nil
	}
	row.Notes = previous.Notes
	if previous.PickupTime.Format("15:04") == row.PickupTime.Format("15:04") {
		return nil
	}
	name, err := s.students.Name(ctx, student)
	if err != nil {
		return err
	}
	weekdays := [...]string{"", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag"}
	result.OverwrittenStudents = append(result.OverwrittenStudents, careplan.OverwriteWarning{
		StudentID: student.ID, StudentName: name, Weekday: row.Weekday, WeekdayName: weekdays[row.Weekday],
		PreviousTime: previous.PickupTime.Format("15:04"), NewTime: row.PickupTime.Format("15:04"),
	})
	return nil
}
