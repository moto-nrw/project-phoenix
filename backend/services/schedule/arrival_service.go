package schedule

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// ArrivalScheduleService defines operations for managing student arrival schedules
type ArrivalScheduleService interface {
	// Schedule operations
	GetStudentArrivalSchedules(ctx context.Context, studentID int64) ([]*schedule.StudentArrivalSchedule, error)
	GetStudentArrivalScheduleForWeekday(ctx context.Context, studentID int64, weekday int) (*schedule.StudentArrivalSchedule, error)
	UpsertStudentArrivalSchedule(ctx context.Context, scheduleData *schedule.StudentArrivalSchedule) error
	UpsertBulkStudentArrivalSchedules(ctx context.Context, studentID int64, schedules []*schedule.StudentArrivalSchedule) error
	DeleteStudentArrivalSchedule(ctx context.Context, scheduleID int64) error
	DeleteAllStudentArrivalSchedules(ctx context.Context, studentID int64) error

	// Exception operations
	GetStudentArrivalExceptionByID(ctx context.Context, exceptionID int64) (*schedule.StudentArrivalException, error)
	GetStudentArrivalExceptions(ctx context.Context, studentID int64) ([]*schedule.StudentArrivalException, error)
	GetUpcomingStudentArrivalExceptions(ctx context.Context, studentID int64) ([]*schedule.StudentArrivalException, error)
	CreateStudentArrivalException(ctx context.Context, exception *schedule.StudentArrivalException) error
	UpdateStudentArrivalException(ctx context.Context, exception *schedule.StudentArrivalException) error
	DeleteStudentArrivalException(ctx context.Context, exceptionID int64) error
	DeleteAllStudentArrivalExceptions(ctx context.Context, studentID int64) error

	// Note operations
	GetStudentArrivalNoteByID(ctx context.Context, noteID int64) (*schedule.StudentArrivalNote, error)
	GetStudentArrivalNotes(ctx context.Context, studentID int64) ([]*schedule.StudentArrivalNote, error)
	GetStudentArrivalNotesForDate(ctx context.Context, studentID int64, date time.Time) ([]*schedule.StudentArrivalNote, error)
	CreateStudentArrivalNote(ctx context.Context, note *schedule.StudentArrivalNote) error
	UpdateStudentArrivalNote(ctx context.Context, note *schedule.StudentArrivalNote) error
	DeleteStudentArrivalNote(ctx context.Context, noteID int64) error
	DeleteAllStudentArrivalNotes(ctx context.Context, studentID int64) error

	// Computed operations
	GetStudentArrivalData(ctx context.Context, studentID int64) (*StudentArrivalData, error)
	GetEffectiveArrivalTimeForDate(ctx context.Context, studentID int64, date time.Time) (*EffectiveArrivalTime, error)
	GetBulkEffectiveArrivalTimesForDate(ctx context.Context, studentIDs []int64, date time.Time) (map[int64]*EffectiveArrivalTime, error)

	// Bulk class operation
	BulkUpsertBySchoolClass(ctx context.Context, schoolClass string, schedules []ArrivalScheduleInput, createdBy int64) (*BulkUpsertResult, error)
}

// StudentArrivalData contains combined arrival schedule and exception data
type StudentArrivalData struct {
	Schedules  []*schedule.StudentArrivalSchedule  `json:"schedules"`
	Exceptions []*schedule.StudentArrivalException `json:"exceptions"`
	Notes      []*schedule.StudentArrivalNote      `json:"notes"`
}

// ArrivalNoteData represents a single note in the effective arrival time response
type ArrivalNoteData struct {
	ID      int64  `json:"id"`
	Content string `json:"content"`
}

// EffectiveArrivalTime represents the arrival time for a specific date
type EffectiveArrivalTime struct {
	Date        time.Time         `json:"date"`
	ArrivalTime *time.Time        `json:"arrival_time"`
	WeekdayName string            `json:"weekday_name"`
	IsException bool              `json:"is_exception"`
	Notes       string            `json:"notes,omitempty"`
	DayNotes    []ArrivalNoteData `json:"day_notes,omitempty"`
}

// ArrivalScheduleInput represents input for a single weekday's arrival schedule
type ArrivalScheduleInput struct {
	Weekday     int    `json:"weekday"`
	ArrivalTime string `json:"arrival_time"` // HH:MM
}

// BulkUpsertResult contains the result of a bulk class upsert operation
type BulkUpsertResult struct {
	StudentsAffected    int                `json:"students_affected"`
	OverwrittenStudents []OverwriteWarning `json:"overwritten_students,omitempty"`
}

// OverwriteWarning warns that a student had an existing individual schedule that was overwritten
type OverwriteWarning struct {
	StudentID    int64  `json:"student_id"`
	StudentName  string `json:"student_name"`
	Weekday      int    `json:"weekday"`
	WeekdayName  string `json:"weekday_name"`
	PreviousTime string `json:"previous_time"`
	NewTime      string `json:"new_time"`
}

// Operation names for ScheduleError.
const (
	opCreateStudentArrivalException     = "create student arrival exception"
	opUpdateStudentArrivalException     = "update student arrival exception"
	opUpsertBulkStudentArrivalSchedules = "upsert bulk student arrival schedules"
	opGetStudentArrivalData             = "get student arrival data"
	opGetEffectiveArrivalTime           = "get effective arrival time"
	opGetBulkEffectiveArrivalTimes      = "get bulk effective arrival times"
	opBulkUpsertBySchoolClass           = "bulk upsert arrival schedules by school class"
)

// arrivalScheduleService implements ArrivalScheduleService
type arrivalScheduleService struct {
	scheduleRepo  schedule.StudentArrivalScheduleRepository
	exceptionRepo schedule.StudentArrivalExceptionRepository
	noteRepo      schedule.StudentArrivalNoteRepository
	studentRepo   users.StudentRepository
	personRepo    users.PersonRepository
	db            *bun.DB
	logger        *slog.Logger
}

// NewArrivalScheduleService creates a new arrival schedule service
func NewArrivalScheduleService(
	scheduleRepo schedule.StudentArrivalScheduleRepository,
	exceptionRepo schedule.StudentArrivalExceptionRepository,
	noteRepo schedule.StudentArrivalNoteRepository,
	studentRepo users.StudentRepository,
	personRepo users.PersonRepository,
	db *bun.DB,
	logger *slog.Logger,
) ArrivalScheduleService {
	return &arrivalScheduleService{
		scheduleRepo:  scheduleRepo,
		exceptionRepo: exceptionRepo,
		noteRepo:      noteRepo,
		studentRepo:   studentRepo,
		personRepo:    personRepo,
		db:            db,
		logger:        logger,
	}
}

func (s *arrivalScheduleService) getLogger() *slog.Logger {
	if s.logger != nil {
		return s.logger
	}
	return slog.Default()
}

// Schedule operations

// GetStudentArrivalSchedules returns all arrival schedules for a student
func (s *arrivalScheduleService) GetStudentArrivalSchedules(ctx context.Context, studentID int64) ([]*schedule.StudentArrivalSchedule, error) {
	schedules, err := s.scheduleRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ScheduleError{Op: "get student arrival schedules", Err: err}
	}
	return schedules, nil
}

// GetStudentArrivalScheduleForWeekday returns the arrival schedule for a specific weekday
func (s *arrivalScheduleService) GetStudentArrivalScheduleForWeekday(ctx context.Context, studentID int64, weekday int) (*schedule.StudentArrivalSchedule, error) {
	if weekday < schedule.WeekdayMonday || weekday > schedule.WeekdayFriday {
		return nil, &ScheduleError{Op: "get student arrival schedule for weekday", Err: errors.New("invalid weekday")}
	}

	arrivalSchedule, err := s.scheduleRepo.FindByStudentIDAndWeekday(ctx, studentID, weekday)
	if err != nil {
		return nil, &ScheduleError{Op: "get student arrival schedule for weekday", Err: err}
	}
	return arrivalSchedule, nil
}

// UpsertStudentArrivalSchedule creates or updates an arrival schedule
func (s *arrivalScheduleService) UpsertStudentArrivalSchedule(ctx context.Context, scheduleData *schedule.StudentArrivalSchedule) error {
	if err := scheduleData.Validate(); err != nil {
		return &ScheduleError{Op: "upsert student arrival schedule", Err: err}
	}

	if err := s.scheduleRepo.UpsertSchedule(ctx, scheduleData); err != nil {
		return &ScheduleError{Op: "upsert student arrival schedule", Err: err}
	}
	return nil
}

// UpsertBulkStudentArrivalSchedules replaces all arrival schedules for a student.
// This deletes existing schedules and inserts the new ones atomically,
// ensuring that cleared weekdays are properly removed.
func (s *arrivalScheduleService) UpsertBulkStudentArrivalSchedules(ctx context.Context, studentID int64, schedules []*schedule.StudentArrivalSchedule) error {
	// Use transaction from context if available (handler's WithTenantTx), otherwise fall back to db
	var db bun.IDB = s.db
	if tx, ok := base.TxFromContext(ctx); ok && tx != nil {
		db = tx
	}

	// Delete all existing schedules for this student first
	_, err := db.NewDelete().
		Model((*schedule.StudentArrivalSchedule)(nil)).
		ModelTableExpr("schedule.student_arrival_schedules").
		Where("student_id = ?", studentID).
		Exec(ctx)
	if err != nil {
		return &ScheduleError{Op: opUpsertBulkStudentArrivalSchedules, Err: fmt.Errorf("failed to delete existing schedules: %w", err)}
	}

	// Insert new schedules
	for _, sched := range schedules {
		sched.StudentID = studentID
		if err := sched.Validate(); err != nil {
			return &ScheduleError{Op: opUpsertBulkStudentArrivalSchedules, Err: fmt.Errorf("invalid schedule for weekday %d: %w", sched.Weekday, err)}
		}
		sched.SetTenantID(tenant.FromContext(ctx))

		_, err := db.NewInsert().
			Model(sched).
			ModelTableExpr("schedule.student_arrival_schedules").
			Returning("id").
			Exec(ctx)
		if err != nil {
			return &ScheduleError{Op: opUpsertBulkStudentArrivalSchedules, Err: err}
		}
	}
	return nil
}

// DeleteStudentArrivalSchedule deletes an arrival schedule by ID
func (s *arrivalScheduleService) DeleteStudentArrivalSchedule(ctx context.Context, scheduleID int64) error {
	if err := s.scheduleRepo.Delete(ctx, scheduleID); err != nil {
		return &ScheduleError{Op: "delete student arrival schedule", Err: err}
	}
	return nil
}

// DeleteAllStudentArrivalSchedules deletes all arrival schedules for a student
func (s *arrivalScheduleService) DeleteAllStudentArrivalSchedules(ctx context.Context, studentID int64) error {
	if err := s.scheduleRepo.DeleteByStudentID(ctx, studentID); err != nil {
		return &ScheduleError{Op: "delete all student arrival schedules", Err: err}
	}
	return nil
}

// Exception operations

// GetStudentArrivalExceptionByID returns an arrival exception by its ID
func (s *arrivalScheduleService) GetStudentArrivalExceptionByID(ctx context.Context, exceptionID int64) (*schedule.StudentArrivalException, error) {
	exception, err := s.exceptionRepo.FindByID(ctx, exceptionID)
	if err != nil {
		return nil, &ScheduleError{Op: "get student arrival exception by id", Err: err}
	}
	return exception, nil
}

// GetStudentArrivalExceptions returns all arrival exceptions for a student
func (s *arrivalScheduleService) GetStudentArrivalExceptions(ctx context.Context, studentID int64) ([]*schedule.StudentArrivalException, error) {
	exceptions, err := s.exceptionRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ScheduleError{Op: "get student arrival exceptions", Err: err}
	}
	return exceptions, nil
}

// GetUpcomingStudentArrivalExceptions returns upcoming arrival exceptions for a student
func (s *arrivalScheduleService) GetUpcomingStudentArrivalExceptions(ctx context.Context, studentID int64) ([]*schedule.StudentArrivalException, error) {
	exceptions, err := s.exceptionRepo.FindUpcomingByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ScheduleError{Op: "get upcoming student arrival exceptions", Err: err}
	}
	return exceptions, nil
}

// CreateStudentArrivalException creates a new arrival exception
func (s *arrivalScheduleService) CreateStudentArrivalException(ctx context.Context, exception *schedule.StudentArrivalException) error {
	if err := exception.Validate(); err != nil {
		return &ScheduleError{Op: opCreateStudentArrivalException, Err: err}
	}

	// Check for existing exception on the same date
	existing, err := s.exceptionRepo.FindByStudentIDAndDate(ctx, exception.StudentID, exception.ExceptionDate)
	if err != nil {
		return &ScheduleError{Op: opCreateStudentArrivalException, Err: err}
	}
	if existing != nil {
		return &ScheduleError{Op: opCreateStudentArrivalException, Err: errors.New("exception already exists for this date")}
	}

	exception.SetTenantID(tenant.FromContext(ctx))
	if err := s.exceptionRepo.Create(ctx, exception); err != nil {
		return &ScheduleError{Op: opCreateStudentArrivalException, Err: err}
	}
	return nil
}

// UpdateStudentArrivalException updates an existing arrival exception
func (s *arrivalScheduleService) UpdateStudentArrivalException(ctx context.Context, exception *schedule.StudentArrivalException) error {
	if err := exception.Validate(); err != nil {
		return &ScheduleError{Op: opUpdateStudentArrivalException, Err: err}
	}

	// Check if changing date would conflict with another exception
	existing, err := s.exceptionRepo.FindByStudentIDAndDate(ctx, exception.StudentID, exception.ExceptionDate)
	if err != nil {
		return &ScheduleError{Op: opUpdateStudentArrivalException, Err: err}
	}
	if existing != nil && existing.ID != exception.ID {
		return &ScheduleError{Op: opUpdateStudentArrivalException, Err: errors.New("exception already exists for this date")}
	}

	if err := s.exceptionRepo.Update(ctx, exception); err != nil {
		return &ScheduleError{Op: opUpdateStudentArrivalException, Err: err}
	}
	return nil
}

// DeleteStudentArrivalException deletes an arrival exception by ID
func (s *arrivalScheduleService) DeleteStudentArrivalException(ctx context.Context, exceptionID int64) error {
	if err := s.exceptionRepo.Delete(ctx, exceptionID); err != nil {
		return &ScheduleError{Op: "delete student arrival exception", Err: err}
	}
	return nil
}

// DeleteAllStudentArrivalExceptions deletes all arrival exceptions for a student
func (s *arrivalScheduleService) DeleteAllStudentArrivalExceptions(ctx context.Context, studentID int64) error {
	if err := s.exceptionRepo.DeleteByStudentID(ctx, studentID); err != nil {
		return &ScheduleError{Op: "delete all student arrival exceptions", Err: err}
	}
	return nil
}

// Note operations

// GetStudentArrivalNoteByID returns an arrival note by its ID
func (s *arrivalScheduleService) GetStudentArrivalNoteByID(ctx context.Context, noteID int64) (*schedule.StudentArrivalNote, error) {
	note, err := s.noteRepo.FindByID(ctx, noteID)
	if err != nil {
		return nil, &ScheduleError{Op: "get student arrival note by id", Err: err}
	}
	return note, nil
}

// GetStudentArrivalNotes returns all arrival notes for a student
func (s *arrivalScheduleService) GetStudentArrivalNotes(ctx context.Context, studentID int64) ([]*schedule.StudentArrivalNote, error) {
	notes, err := s.noteRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ScheduleError{Op: "get student arrival notes", Err: err}
	}
	return notes, nil
}

// GetStudentArrivalNotesForDate returns arrival notes for a student on a specific date
func (s *arrivalScheduleService) GetStudentArrivalNotesForDate(ctx context.Context, studentID int64, date time.Time) ([]*schedule.StudentArrivalNote, error) {
	notes, err := s.noteRepo.FindByStudentIDAndDate(ctx, studentID, date)
	if err != nil {
		return nil, &ScheduleError{Op: "get student arrival notes for date", Err: err}
	}
	return notes, nil
}

// CreateStudentArrivalNote creates a new arrival note
func (s *arrivalScheduleService) CreateStudentArrivalNote(ctx context.Context, note *schedule.StudentArrivalNote) error {
	if err := note.Validate(); err != nil {
		return &ScheduleError{Op: "create student arrival note", Err: err}
	}

	note.SetTenantID(tenant.FromContext(ctx))
	if err := s.noteRepo.Create(ctx, note); err != nil {
		return &ScheduleError{Op: "create student arrival note", Err: err}
	}
	return nil
}

// UpdateStudentArrivalNote updates an existing arrival note
func (s *arrivalScheduleService) UpdateStudentArrivalNote(ctx context.Context, note *schedule.StudentArrivalNote) error {
	if err := note.Validate(); err != nil {
		return &ScheduleError{Op: "update student arrival note", Err: err}
	}

	if err := s.noteRepo.Update(ctx, note); err != nil {
		return &ScheduleError{Op: "update student arrival note", Err: err}
	}
	return nil
}

// DeleteStudentArrivalNote deletes an arrival note by ID
func (s *arrivalScheduleService) DeleteStudentArrivalNote(ctx context.Context, noteID int64) error {
	if err := s.noteRepo.Delete(ctx, noteID); err != nil {
		return &ScheduleError{Op: "delete student arrival note", Err: err}
	}
	return nil
}

// DeleteAllStudentArrivalNotes deletes all arrival notes for a student
func (s *arrivalScheduleService) DeleteAllStudentArrivalNotes(ctx context.Context, studentID int64) error {
	if err := s.noteRepo.DeleteByStudentID(ctx, studentID); err != nil {
		return &ScheduleError{Op: "delete all student arrival notes", Err: err}
	}
	return nil
}

// Computed operations

// GetStudentArrivalData returns combined schedule, exception, and note data for a student.
// Returns all exceptions and notes (not just upcoming) to support week view navigation to past weeks.
func (s *arrivalScheduleService) GetStudentArrivalData(ctx context.Context, studentID int64) (*StudentArrivalData, error) {
	schedules, err := s.scheduleRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ScheduleError{Op: opGetStudentArrivalData, Err: err}
	}

	exceptions, err := s.exceptionRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ScheduleError{Op: opGetStudentArrivalData, Err: err}
	}

	notes, err := s.noteRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ScheduleError{Op: opGetStudentArrivalData, Err: err}
	}

	return &StudentArrivalData{
		Schedules:  schedules,
		Exceptions: exceptions,
		Notes:      notes,
	}, nil
}

// GetEffectiveArrivalTimeForDate calculates the effective arrival time for a specific date
func (s *arrivalScheduleService) GetEffectiveArrivalTimeForDate(ctx context.Context, studentID int64, date time.Time) (*EffectiveArrivalTime, error) {
	dateOnly := timezone.DateOf(date)
	weekday := int(dateOnly.Weekday())

	// Convert Go weekday (Sunday=0) to ISO weekday (Monday=1)
	if weekday == 0 {
		weekday = 7
	}

	result := &EffectiveArrivalTime{
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
		return nil, &ScheduleError{Op: opGetEffectiveArrivalTime, Err: err}
	}

	if exception != nil {
		result.IsException = true
		result.ArrivalTime = exception.ExpectedArrival
		if reason := arrivalNoteOverride(exception.Reason); reason != "" {
			result.Notes = reason
		} else {
			sched, err := s.scheduleRepo.FindByStudentIDAndWeekday(ctx, studentID, weekday)
			if err != nil {
				return nil, &ScheduleError{Op: opGetEffectiveArrivalTime, Err: err}
			}
			result.Notes = arrivalScheduleNotes(sched)
		}
	} else {
		// Fall back to regular schedule
		sched, err := s.scheduleRepo.FindByStudentIDAndWeekday(ctx, studentID, weekday)
		if err != nil {
			return nil, &ScheduleError{Op: opGetEffectiveArrivalTime, Err: err}
		}

		if sched != nil {
			result.ArrivalTime = &sched.ExpectedArrival
			result.Notes = arrivalScheduleNotes(sched)
		}
	}

	// Load day notes
	dayNotes, err := s.noteRepo.FindByStudentIDAndDate(ctx, studentID, dateOnly)
	if err != nil {
		return nil, &ScheduleError{Op: opGetEffectiveArrivalTime, Err: err}
	}
	for _, n := range dayNotes {
		result.DayNotes = append(result.DayNotes, ArrivalNoteData{ID: n.ID, Content: n.Content})
	}

	return result, nil
}

// GetBulkEffectiveArrivalTimesForDate calculates effective arrival times for multiple students on a given date
// Uses bulk database queries for optimal performance (O(3) queries instead of O(N))
func (s *arrivalScheduleService) GetBulkEffectiveArrivalTimesForDate(ctx context.Context, studentIDs []int64, date time.Time) (map[int64]*EffectiveArrivalTime, error) {
	if len(studentIDs) == 0 {
		return make(map[int64]*EffectiveArrivalTime), nil
	}

	dateOnly := timezone.DateOf(date)
	weekday := int(dateOnly.Weekday())

	// Convert Go weekday (Sunday=0) to ISO weekday (Monday=1)
	if weekday == 0 {
		weekday = 7
	}

	result := make(map[int64]*EffectiveArrivalTime, len(studentIDs))

	// Initialize results for all students
	for _, studentID := range studentIDs {
		result[studentID] = &EffectiveArrivalTime{
			Date:        dateOnly,
			WeekdayName: schedule.WeekdayNames[weekday],
		}
	}

	// Weekend check - all students have no arrival time
	if weekday > schedule.WeekdayFriday {
		return result, nil
	}

	// Bulk fetch all exceptions for the given date (single query)
	exceptions, err := s.exceptionRepo.FindByStudentIDsAndDate(ctx, studentIDs, dateOnly)
	if err != nil {
		return nil, &ScheduleError{Op: opGetBulkEffectiveArrivalTimes, Err: err}
	}

	// Build exception map for O(1) lookup
	exceptionMap := make(map[int64]*schedule.StudentArrivalException, len(exceptions))
	for _, exc := range exceptions {
		exceptionMap[exc.StudentID] = exc
	}

	// Bulk fetch all schedules for the given weekday (single query)
	schedules, err := s.scheduleRepo.FindByStudentIDsAndWeekday(ctx, studentIDs, weekday)
	if err != nil {
		return nil, &ScheduleError{Op: opGetBulkEffectiveArrivalTimes, Err: err}
	}

	// Build schedule map for O(1) lookup
	scheduleMap := make(map[int64]*schedule.StudentArrivalSchedule, len(schedules))
	for _, sched := range schedules {
		scheduleMap[sched.StudentID] = sched
	}

	// Bulk fetch all notes for the given date (single query)
	notes, err := s.noteRepo.FindByStudentIDsAndDate(ctx, studentIDs, dateOnly)
	if err != nil {
		return nil, &ScheduleError{Op: opGetBulkEffectiveArrivalTimes, Err: err}
	}

	// Build notes map for grouping by student
	notesMap := make(map[int64][]*schedule.StudentArrivalNote)
	for _, n := range notes {
		notesMap[n.StudentID] = append(notesMap[n.StudentID], n)
	}

	// Merge results: exception takes precedence over schedule
	mergeArrivalResults(studentIDs, result, exceptionMap, scheduleMap, notesMap)

	return result, nil
}

// mergeArrivalResults merges exception, schedule, and note data into effective arrival times
func mergeArrivalResults(
	studentIDs []int64,
	result map[int64]*EffectiveArrivalTime,
	exceptionMap map[int64]*schedule.StudentArrivalException,
	scheduleMap map[int64]*schedule.StudentArrivalSchedule,
	notesMap map[int64][]*schedule.StudentArrivalNote,
) {
	for _, studentID := range studentIDs {
		r := result[studentID]
		sched := scheduleMap[studentID]

		// Check for exception first (takes priority)
		if exc, ok := exceptionMap[studentID]; ok {
			r.IsException = true
			r.ArrivalTime = exc.ExpectedArrival
			if reason := arrivalNoteOverride(exc.Reason); reason != "" {
				r.Notes = reason
			} else {
				r.Notes = arrivalScheduleNotes(sched)
			}
		} else if sched != nil {
			// Fall back to regular schedule
			r.ArrivalTime = &sched.ExpectedArrival
			r.Notes = arrivalScheduleNotes(sched)
		}

		// Attach day notes
		if dayNotes, ok := notesMap[studentID]; ok {
			for _, n := range dayNotes {
				r.DayNotes = append(r.DayNotes, ArrivalNoteData{ID: n.ID, Content: n.Content})
			}
		}
	}
}

// BulkUpsertBySchoolClass upserts arrival schedules for all students in a school class
func (s *arrivalScheduleService) BulkUpsertBySchoolClass(ctx context.Context, schoolClass string, schedules []ArrivalScheduleInput, createdBy int64) (*BulkUpsertResult, error) {
	if schoolClass == "" {
		return nil, &ScheduleError{Op: opBulkUpsertBySchoolClass, Err: errors.New("school_class is required")}
	}
	if len(schedules) == 0 {
		return nil, &ScheduleError{Op: opBulkUpsertBySchoolClass, Err: errors.New("schedules cannot be empty")}
	}

	// Query all students in this school class within the current tenant
	students, err := s.studentRepo.FindBySchoolClass(ctx, schoolClass)
	if err != nil {
		return nil, &ScheduleError{Op: opBulkUpsertBySchoolClass, Err: fmt.Errorf("failed to find students for class %s: %w", schoolClass, err)}
	}

	if len(students) == 0 {
		return &BulkUpsertResult{StudentsAffected: 0}, nil
	}

	tenantID := tenant.FromContext(ctx)
	warnings := make([]OverwriteWarning, 0)

	// Parse arrival times upfront
	parsedTimes := make(map[int]time.Time, len(schedules))
	for _, input := range schedules {
		t, err := time.Parse("2006-01-02 15:04", "2000-01-01 "+input.ArrivalTime)
		if err != nil {
			return nil, &ScheduleError{Op: opBulkUpsertBySchoolClass, Err: fmt.Errorf("invalid arrival_time %q for weekday %d: %w", input.ArrivalTime, input.Weekday, err)}
		}
		parsedTimes[input.Weekday] = t
	}

	// Wrap everything in a transaction
	if err := tenant.WithTenantTx(ctx, s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		for _, student := range students {
			// Fetch existing schedules for this student to detect overwrites
			existing, err := s.scheduleRepo.FindByStudentID(txCtx, student.ID)
			if err != nil {
				return fmt.Errorf("failed to fetch existing schedules for student %d: %w", student.ID, err)
			}

			// Build map of existing schedules by weekday for overwrite detection
			existingByWeekday := make(map[int]*schedule.StudentArrivalSchedule, len(existing))
			for _, ex := range existing {
				existingByWeekday[ex.Weekday] = ex
			}

			// Upsert each weekday
			for _, input := range schedules {
				arrivalTime := parsedTimes[input.Weekday]

				// Check for overwrite
				if ex, ok := existingByWeekday[input.Weekday]; ok {
					if ex.ExpectedArrival.Format("15:04") != arrivalTime.Format("15:04") {
						// Get student name for the warning
						studentName := s.getStudentName(txCtx, student)
						warnings = append(warnings, OverwriteWarning{
							StudentID:    student.ID,
							StudentName:  studentName,
							Weekday:      input.Weekday,
							WeekdayName:  schedule.WeekdayNames[input.Weekday],
							PreviousTime: ex.ExpectedArrival.Format("15:04"),
							NewTime:      arrivalTime.Format("15:04"),
						})
					}
				}

				sched := &schedule.StudentArrivalSchedule{
					StudentID:       student.ID,
					Weekday:         input.Weekday,
					ExpectedArrival: arrivalTime,
					CreatedBy:       createdBy,
				}
				sched.SetTenantID(tenantID)

				if err := s.scheduleRepo.UpsertSchedule(txCtx, sched); err != nil {
					return fmt.Errorf("failed to upsert schedule for student %d weekday %d: %w", student.ID, input.Weekday, err)
				}
			}
		}
		return nil
	}); err != nil {
		return nil, &ScheduleError{Op: opBulkUpsertBySchoolClass, Err: err}
	}

	s.getLogger().Info("bulk upsert arrival schedules by school class",
		slog.String("school_class", schoolClass),
		slog.Int("students_affected", len(students)),
		slog.Int("weekdays_set", len(schedules)),
		slog.Int("overwrites", len(warnings)),
	)

	return &BulkUpsertResult{
		StudentsAffected:    len(students),
		OverwrittenStudents: warnings,
	}, nil
}

// getStudentName resolves a student's display name from their person record.
// Returns a fallback string on any error so callers can always produce a warning.
func (s *arrivalScheduleService) getStudentName(ctx context.Context, student *users.Student) string {
	if s.personRepo == nil {
		return fmt.Sprintf("Student %d", student.ID)
	}
	person, err := s.personRepo.FindByID(ctx, student.PersonID)
	if err != nil || person == nil {
		return fmt.Sprintf("Student %d", student.ID)
	}
	return person.FirstName + " " + person.LastName
}

func arrivalNoteOverride(reason *string) string {
	if reason == nil {
		return ""
	}

	return strings.TrimSpace(*reason)
}

func arrivalScheduleNotes(sched *schedule.StudentArrivalSchedule) string {
	if sched == nil || sched.Notes == nil {
		return ""
	}

	return strings.TrimSpace(*sched.Notes)
}
