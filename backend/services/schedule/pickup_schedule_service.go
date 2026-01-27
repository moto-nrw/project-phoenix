package schedule

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

// PickupScheduleService defines operations for managing student pickup schedules
type PickupScheduleService interface {
	// Schedule operations
	GetStudentPickupSchedules(ctx context.Context, studentID int64) ([]*schedule.StudentPickupSchedule, error)
	GetStudentPickupScheduleForWeekday(ctx context.Context, studentID int64, weekday int) (*schedule.StudentPickupSchedule, error)
	UpsertStudentPickupSchedule(ctx context.Context, scheduleData *schedule.StudentPickupSchedule) error
	UpsertBulkStudentPickupSchedules(ctx context.Context, studentID int64, schedules []*schedule.StudentPickupSchedule) error
	DeleteStudentPickupSchedule(ctx context.Context, scheduleID int64) error
	DeleteAllStudentPickupSchedules(ctx context.Context, studentID int64) error

	// Exception operations
	GetStudentPickupExceptionByID(ctx context.Context, exceptionID int64) (*schedule.StudentPickupException, error)
	GetStudentPickupExceptions(ctx context.Context, studentID int64) ([]*schedule.StudentPickupException, error)
	GetUpcomingStudentPickupExceptions(ctx context.Context, studentID int64) ([]*schedule.StudentPickupException, error)
	CreateStudentPickupException(ctx context.Context, exception *schedule.StudentPickupException) error
	UpdateStudentPickupException(ctx context.Context, exception *schedule.StudentPickupException) error
	DeleteStudentPickupException(ctx context.Context, exceptionID int64) error
	DeleteAllStudentPickupExceptions(ctx context.Context, studentID int64) error

	// Computed operations
	GetStudentPickupData(ctx context.Context, studentID int64) (*StudentPickupData, error)
	GetEffectivePickupTimeForDate(ctx context.Context, studentID int64, date time.Time) (*EffectivePickupTime, error)
	GetBulkEffectivePickupTimesForDate(ctx context.Context, studentIDs []int64, date time.Time) (map[int64]*EffectivePickupTime, error)
}

// StudentPickupData contains combined pickup schedule and exception data
type StudentPickupData struct {
	Schedules  []*schedule.StudentPickupSchedule  `json:"schedules"`
	Exceptions []*schedule.StudentPickupException `json:"exceptions"`
}

// EffectivePickupTime represents the pickup time for a specific date
type EffectivePickupTime struct {
	Date        time.Time  `json:"date"`
	PickupTime  *time.Time `json:"pickup_time"`
	WeekdayName string     `json:"weekday_name"`
	IsException bool       `json:"is_exception"`
	Reason      string     `json:"reason,omitempty"`
	Notes       string     `json:"notes,omitempty"`
}

// Operation names for ScheduleError.
const opCreateStudentPickupException = "create student pickup exception"

// pickupScheduleService implements PickupScheduleService
type pickupScheduleService struct {
	scheduleRepo  schedule.StudentPickupScheduleRepository
	exceptionRepo schedule.StudentPickupExceptionRepository
	db            *bun.DB
	txHandler     *base.TxHandler
}

// NewPickupScheduleService creates a new pickup schedule service
func NewPickupScheduleService(
	scheduleRepo schedule.StudentPickupScheduleRepository,
	exceptionRepo schedule.StudentPickupExceptionRepository,
	db *bun.DB,
) PickupScheduleService {
	return &pickupScheduleService{
		scheduleRepo:  scheduleRepo,
		exceptionRepo: exceptionRepo,
		db:            db,
		txHandler:     base.NewTxHandler(db),
	}
}

// Schedule operations

// GetStudentPickupSchedules returns all pickup schedules for a student
func (s *pickupScheduleService) GetStudentPickupSchedules(ctx context.Context, studentID int64) ([]*schedule.StudentPickupSchedule, error) {
	schedules, err := s.scheduleRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ScheduleError{Op: "get student pickup schedules", Err: err}
	}
	return schedules, nil
}

// GetStudentPickupScheduleForWeekday returns the pickup schedule for a specific weekday
func (s *pickupScheduleService) GetStudentPickupScheduleForWeekday(ctx context.Context, studentID int64, weekday int) (*schedule.StudentPickupSchedule, error) {
	if weekday < schedule.WeekdayMonday || weekday > schedule.WeekdayFriday {
		return nil, &ScheduleError{Op: "get student pickup schedule for weekday", Err: errors.New("invalid weekday")}
	}

	pickupSchedule, err := s.scheduleRepo.FindByStudentIDAndWeekday(ctx, studentID, weekday)
	if err != nil {
		return nil, &ScheduleError{Op: "get student pickup schedule for weekday", Err: err}
	}
	return pickupSchedule, nil
}

// UpsertStudentPickupSchedule creates or updates a pickup schedule
func (s *pickupScheduleService) UpsertStudentPickupSchedule(ctx context.Context, scheduleData *schedule.StudentPickupSchedule) error {
	if err := scheduleData.Validate(); err != nil {
		return &ScheduleError{Op: "upsert student pickup schedule", Err: err}
	}

	if err := s.scheduleRepo.UpsertSchedule(ctx, scheduleData); err != nil {
		return &ScheduleError{Op: "upsert student pickup schedule", Err: err}
	}
	return nil
}

// UpsertBulkStudentPickupSchedules replaces all pickup schedules for a student.
// This deletes existing schedules and inserts the new ones in a transaction,
// ensuring that cleared weekdays are properly removed.
func (s *pickupScheduleService) UpsertBulkStudentPickupSchedules(ctx context.Context, studentID int64, schedules []*schedule.StudentPickupSchedule) error {
	return s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Delete all existing schedules for this student first
		_, err := tx.NewDelete().
			Model((*schedule.StudentPickupSchedule)(nil)).
			ModelTableExpr("schedule.student_pickup_schedules").
			Where("student_id = ?", studentID).
			Exec(ctx)
		if err != nil {
			return &ScheduleError{Op: "upsert bulk student pickup schedules", Err: fmt.Errorf("failed to delete existing schedules: %w", err)}
		}

		// Insert new schedules
		for _, sched := range schedules {
			sched.StudentID = studentID
			if err := sched.Validate(); err != nil {
				return &ScheduleError{Op: "upsert bulk student pickup schedules", Err: fmt.Errorf("invalid schedule for weekday %d: %w", sched.Weekday, err)}
			}

			_, err := tx.NewInsert().
				Model(sched).
				ModelTableExpr("schedule.student_pickup_schedules").
				Returning("id").
				Exec(ctx)
			if err != nil {
				return &ScheduleError{Op: "upsert bulk student pickup schedules", Err: err}
			}
		}
		return nil
	})
}

// DeleteStudentPickupSchedule deletes a pickup schedule by ID
func (s *pickupScheduleService) DeleteStudentPickupSchedule(ctx context.Context, scheduleID int64) error {
	if err := s.scheduleRepo.Delete(ctx, scheduleID); err != nil {
		return &ScheduleError{Op: "delete student pickup schedule", Err: err}
	}
	return nil
}

// DeleteAllStudentPickupSchedules deletes all pickup schedules for a student
func (s *pickupScheduleService) DeleteAllStudentPickupSchedules(ctx context.Context, studentID int64) error {
	if err := s.scheduleRepo.DeleteByStudentID(ctx, studentID); err != nil {
		return &ScheduleError{Op: "delete all student pickup schedules", Err: err}
	}
	return nil
}

// Exception operations

// GetStudentPickupExceptionByID returns a pickup exception by its ID
func (s *pickupScheduleService) GetStudentPickupExceptionByID(ctx context.Context, exceptionID int64) (*schedule.StudentPickupException, error) {
	exception, err := s.exceptionRepo.FindByID(ctx, exceptionID)
	if err != nil {
		return nil, &ScheduleError{Op: "get student pickup exception by id", Err: err}
	}
	return exception, nil
}

// GetStudentPickupExceptions returns all pickup exceptions for a student
func (s *pickupScheduleService) GetStudentPickupExceptions(ctx context.Context, studentID int64) ([]*schedule.StudentPickupException, error) {
	exceptions, err := s.exceptionRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ScheduleError{Op: "get student pickup exceptions", Err: err}
	}
	return exceptions, nil
}

// GetUpcomingStudentPickupExceptions returns upcoming pickup exceptions for a student
func (s *pickupScheduleService) GetUpcomingStudentPickupExceptions(ctx context.Context, studentID int64) ([]*schedule.StudentPickupException, error) {
	exceptions, err := s.exceptionRepo.FindUpcomingByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ScheduleError{Op: "get upcoming student pickup exceptions", Err: err}
	}
	return exceptions, nil
}

// CreateStudentPickupException creates a new pickup exception
func (s *pickupScheduleService) CreateStudentPickupException(ctx context.Context, exception *schedule.StudentPickupException) error {
	if err := exception.Validate(); err != nil {
		return &ScheduleError{Op: opCreateStudentPickupException, Err: err}
	}

	// Check for existing exception on the same date
	existing, err := s.exceptionRepo.FindByStudentIDAndDate(ctx, exception.StudentID, exception.ExceptionDate)
	if err != nil {
		return &ScheduleError{Op: opCreateStudentPickupException, Err: err}
	}
	if existing != nil {
		return &ScheduleError{Op: opCreateStudentPickupException, Err: errors.New("exception already exists for this date")}
	}

	if err := s.exceptionRepo.Create(ctx, exception); err != nil {
		return &ScheduleError{Op: opCreateStudentPickupException, Err: err}
	}
	return nil
}

// UpdateStudentPickupException updates an existing pickup exception
func (s *pickupScheduleService) UpdateStudentPickupException(ctx context.Context, exception *schedule.StudentPickupException) error {
	if err := exception.Validate(); err != nil {
		return &ScheduleError{Op: "update student pickup exception", Err: err}
	}

	if err := s.exceptionRepo.Update(ctx, exception); err != nil {
		return &ScheduleError{Op: "update student pickup exception", Err: err}
	}
	return nil
}

// DeleteStudentPickupException deletes a pickup exception by ID
func (s *pickupScheduleService) DeleteStudentPickupException(ctx context.Context, exceptionID int64) error {
	if err := s.exceptionRepo.Delete(ctx, exceptionID); err != nil {
		return &ScheduleError{Op: "delete student pickup exception", Err: err}
	}
	return nil
}

// DeleteAllStudentPickupExceptions deletes all pickup exceptions for a student
func (s *pickupScheduleService) DeleteAllStudentPickupExceptions(ctx context.Context, studentID int64) error {
	if err := s.exceptionRepo.DeleteByStudentID(ctx, studentID); err != nil {
		return &ScheduleError{Op: "delete all student pickup exceptions", Err: err}
	}
	return nil
}

// Computed operations

// GetStudentPickupData returns combined schedule and exception data for a student.
// Returns all exceptions (not just upcoming) to support week view navigation to past weeks.
func (s *pickupScheduleService) GetStudentPickupData(ctx context.Context, studentID int64) (*StudentPickupData, error) {
	schedules, err := s.scheduleRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ScheduleError{Op: "get student pickup data", Err: err}
	}

	exceptions, err := s.exceptionRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ScheduleError{Op: "get student pickup data", Err: err}
	}

	return &StudentPickupData{
		Schedules:  schedules,
		Exceptions: exceptions,
	}, nil
}

// GetEffectivePickupTimeForDate calculates the effective pickup time for a specific date
func (s *pickupScheduleService) GetEffectivePickupTimeForDate(ctx context.Context, studentID int64, date time.Time) (*EffectivePickupTime, error) {
	dateOnly := timezone.DateOf(date)
	weekday := int(dateOnly.Weekday())

	// Convert Go weekday (Sunday=0) to ISO weekday (Monday=1)
	if weekday == 0 {
		weekday = 7
	}

	result := &EffectivePickupTime{
		Date:        dateOnly,
		WeekdayName: schedule.WeekdayNames[weekday],
	}

	// Weekend check
	if weekday > schedule.WeekdayFriday {
		return result, nil
	}

	// Check for exception on this date first
	exception, err := s.exceptionRepo.FindByStudentIDAndDate(ctx, studentID, dateOnly)
	if err != nil {
		return nil, &ScheduleError{Op: "get effective pickup time", Err: err}
	}

	if exception != nil {
		result.IsException = true
		result.Reason = exception.Reason
		result.PickupTime = exception.PickupTime
		return result, nil
	}

	// Fall back to regular schedule
	sched, err := s.scheduleRepo.FindByStudentIDAndWeekday(ctx, studentID, weekday)
	if err != nil {
		return nil, &ScheduleError{Op: "get effective pickup time", Err: err}
	}

	if sched != nil {
		result.PickupTime = &sched.PickupTime
		if sched.Notes != nil {
			result.Notes = *sched.Notes
		}
	}

	return result, nil
}

// GetBulkEffectivePickupTimesForDate calculates effective pickup times for multiple students on a given date
// Uses bulk database queries for optimal performance (O(2) queries instead of O(N))
func (s *pickupScheduleService) GetBulkEffectivePickupTimesForDate(ctx context.Context, studentIDs []int64, date time.Time) (map[int64]*EffectivePickupTime, error) {
	if len(studentIDs) == 0 {
		return make(map[int64]*EffectivePickupTime), nil
	}

	dateOnly := timezone.DateOf(date)
	weekday := int(dateOnly.Weekday())

	// Convert Go weekday (Sunday=0) to ISO weekday (Monday=1)
	if weekday == 0 {
		weekday = 7
	}

	result := make(map[int64]*EffectivePickupTime, len(studentIDs))

	// Initialize results for all students
	for _, studentID := range studentIDs {
		result[studentID] = &EffectivePickupTime{
			Date:        dateOnly,
			WeekdayName: schedule.WeekdayNames[weekday],
		}
	}

	// Weekend check - all students have no pickup time
	if weekday > schedule.WeekdayFriday {
		return result, nil
	}

	// Bulk fetch all exceptions for the given date (single query)
	exceptions, err := s.exceptionRepo.FindByStudentIDsAndDate(ctx, studentIDs, dateOnly)
	if err != nil {
		return nil, &ScheduleError{Op: "get bulk effective pickup times", Err: err}
	}

	// Build exception map for O(1) lookup
	exceptionMap := make(map[int64]*schedule.StudentPickupException, len(exceptions))
	for _, exc := range exceptions {
		exceptionMap[exc.StudentID] = exc
	}

	// Bulk fetch all schedules for the given weekday (single query)
	schedules, err := s.scheduleRepo.FindByStudentIDsAndWeekday(ctx, studentIDs, weekday)
	if err != nil {
		return nil, &ScheduleError{Op: "get bulk effective pickup times", Err: err}
	}

	// Build schedule map for O(1) lookup
	scheduleMap := make(map[int64]*schedule.StudentPickupSchedule, len(schedules))
	for _, sched := range schedules {
		scheduleMap[sched.StudentID] = sched
	}

	// Merge results: exception takes precedence over schedule
	mergePickupResults(studentIDs, result, exceptionMap, scheduleMap)

	return result, nil
}

// mergePickupResults merges exception and schedule data into effective pickup times
func mergePickupResults(
	studentIDs []int64,
	result map[int64]*EffectivePickupTime,
	exceptionMap map[int64]*schedule.StudentPickupException,
	scheduleMap map[int64]*schedule.StudentPickupSchedule,
) {
	for _, studentID := range studentIDs {
		r := result[studentID]

		// Check for exception first (takes priority)
		if exc, ok := exceptionMap[studentID]; ok {
			r.IsException = true
			r.Reason = exc.Reason
			r.PickupTime = exc.PickupTime
			continue
		}

		// Fall back to regular schedule
		if sched, ok := scheduleMap[studentID]; ok {
			r.PickupTime = &sched.PickupTime
			if sched.Notes != nil {
				r.Notes = *sched.Notes
			}
		}
	}
}
