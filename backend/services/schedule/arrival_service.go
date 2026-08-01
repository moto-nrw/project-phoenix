package schedule

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// ArrivalScheduleService defines operations for managing student arrival schedules.
// The domain-named contract remains stable for all existing consumers.
type ArrivalScheduleService interface {
	GetStudentArrivalSchedules(ctx context.Context, studentID int64) ([]*schedule.StudentArrivalSchedule, error)
	GetWeeklySchedulesByStudentIDsAndWeekday(ctx context.Context, studentIDs []int64, weekday int) ([]*schedule.StudentArrivalSchedule, error)
	GetStudentArrivalScheduleForWeekday(ctx context.Context, studentID int64, weekday int) (*schedule.StudentArrivalSchedule, error)
	UpsertStudentArrivalSchedule(ctx context.Context, scheduleData *schedule.StudentArrivalSchedule) error
	UpsertBulkStudentArrivalSchedules(ctx context.Context, studentID int64, schedules []*schedule.StudentArrivalSchedule) error
	DeleteStudentArrivalSchedule(ctx context.Context, scheduleID int64) error
	DeleteAllStudentArrivalSchedules(ctx context.Context, studentID int64) error

	GetStudentArrivalExceptionByID(ctx context.Context, exceptionID int64) (*schedule.StudentArrivalException, error)
	GetStudentArrivalExceptionForDate(ctx context.Context, studentID int64, date timezone.Date) (*schedule.StudentArrivalException, error)
	GetStudentArrivalExceptions(ctx context.Context, studentID int64) ([]*schedule.StudentArrivalException, error)
	GetUpcomingStudentArrivalExceptions(ctx context.Context, studentID int64) ([]*schedule.StudentArrivalException, error)
	CreateStudentArrivalException(ctx context.Context, exception *schedule.StudentArrivalException) error
	UpdateStudentArrivalException(ctx context.Context, exception *schedule.StudentArrivalException) error
	DeleteStudentArrivalException(ctx context.Context, exceptionID int64) error
	DeleteAllStudentArrivalExceptions(ctx context.Context, studentID int64) error
	CreateOrReclaimException(ctx context.Context, studentID int64, date timezone.Date, arrivalTime *time.Time, reason *string, staffID int64, resolveStaffID func() (int64, error)) (*schedule.StudentArrivalException, error)
	UpdateException(ctx context.Context, exceptionID, studentID int64, date timezone.Date, reason *string, arrivalTime *time.Time, clearArrivalTime bool, resolveStaffID func() (int64, error)) (*schedule.StudentArrivalException, error)

	GetStudentArrivalNoteByID(ctx context.Context, noteID int64) (*schedule.StudentArrivalNote, error)
	GetStudentArrivalNotes(ctx context.Context, studentID int64) ([]*schedule.StudentArrivalNote, error)
	GetStudentArrivalNotesForDate(ctx context.Context, studentID int64, date timezone.Date) ([]*schedule.StudentArrivalNote, error)
	CreateStudentArrivalNote(ctx context.Context, note *schedule.StudentArrivalNote) error
	UpdateStudentArrivalNote(ctx context.Context, note *schedule.StudentArrivalNote) error
	DeleteStudentArrivalNote(ctx context.Context, noteID int64) error
	DeleteAllStudentArrivalNotes(ctx context.Context, studentID int64) error

	GetStudentArrivalData(ctx context.Context, studentID int64) (*StudentArrivalData, error)
	GetEffectiveArrivalTimeForDate(ctx context.Context, studentID int64, date timezone.Date) (*EffectiveArrivalTime, error)
	GetBulkEffectiveArrivalTimesForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*EffectiveArrivalTime, error)
	BulkUpsertBySchoolClass(ctx context.Context, schoolClass string, schedules []ArrivalScheduleInput, createdBy int64) (*BulkUpsertResult, error)
}

type StudentArrivalData struct {
	Schedules  []*schedule.StudentArrivalSchedule  `json:"schedules"`
	Exceptions []*schedule.StudentArrivalException `json:"exceptions"`
	Notes      []*schedule.StudentArrivalNote      `json:"notes"`
}

type ArrivalNoteData struct {
	ID      int64  `json:"id"`
	Content string `json:"content"`
}

type EffectiveArrivalTime struct {
	Date        timezone.Date     `json:"date"`
	ArrivalTime *time.Time        `json:"arrival_time"`
	WeekdayName string            `json:"weekday_name"`
	IsException bool              `json:"is_exception"`
	Notes       string            `json:"notes,omitempty"`
	DayNotes    []ArrivalNoteData `json:"day_notes,omitempty"`
}

type ArrivalScheduleInput struct {
	Weekday     int    `json:"weekday"`
	ArrivalTime string `json:"expected_arrival"`
}

type BulkUpsertResult struct {
	StudentsAffected    int                `json:"students_affected"`
	OverwrittenStudents []OverwriteWarning `json:"overwritten_students,omitempty"`
	AffectedStudentIDs  []int64            `json:"-"`
}

type OverwriteWarning struct {
	StudentID    int64  `json:"student_id"`
	StudentName  string `json:"student_name"`
	Weekday      int    `json:"weekday"`
	WeekdayName  string `json:"weekday_name"`
	PreviousTime string `json:"previous_time"`
	NewTime      string `json:"new_time"`
}

const opBulkUpsertBySchoolClass = "bulk upsert arrival schedules by school class"

type arrivalScheduleService struct {
	core *effectiveTimeCore[
		*schedule.StudentArrivalSchedule,
		*schedule.StudentArrivalException,
		*schedule.StudentArrivalNote,
		arrivalTimeDomain,
	]
	scheduleRepo schedule.StudentArrivalScheduleRepository
	studentRepo  users.StudentRepository
	personRepo   users.PersonRepository
	db           *bun.DB
	logger       *slog.Logger
}

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
		core: newEffectiveTimeCore(
			scheduleRepo,
			exceptionRepo,
			noteRepo,
			db,
			arrivalTimeDomain{},
		),
		scheduleRepo: scheduleRepo,
		studentRepo:  studentRepo,
		personRepo:   personRepo,
		db:           db,
		logger:       logger,
	}
}

func (s *arrivalScheduleService) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

func (s *arrivalScheduleService) GetStudentArrivalSchedules(
	ctx context.Context,
	studentID int64,
) ([]*schedule.StudentArrivalSchedule, error) {
	return s.core.Schedules(ctx, studentID)
}

func (s *arrivalScheduleService) GetWeeklySchedulesByStudentIDsAndWeekday(
	ctx context.Context,
	studentIDs []int64,
	weekday int,
) ([]*schedule.StudentArrivalSchedule, error) {
	return s.core.WeeklySchedulesByStudentIDsAndWeekday(ctx, studentIDs, weekday)
}

func (s *arrivalScheduleService) GetStudentArrivalScheduleForWeekday(
	ctx context.Context,
	studentID int64,
	weekday int,
) (*schedule.StudentArrivalSchedule, error) {
	return s.core.ScheduleForWeekday(ctx, studentID, weekday)
}

func (s *arrivalScheduleService) UpsertStudentArrivalSchedule(
	ctx context.Context,
	row *schedule.StudentArrivalSchedule,
) error {
	return s.core.UpsertSchedule(ctx, row)
}

func (s *arrivalScheduleService) UpsertBulkStudentArrivalSchedules(
	ctx context.Context,
	studentID int64,
	rows []*schedule.StudentArrivalSchedule,
) error {
	return s.core.UpsertBulkSchedules(ctx, studentID, rows)
}

func (s *arrivalScheduleService) DeleteStudentArrivalSchedule(
	ctx context.Context,
	scheduleID int64,
) error {
	return s.core.DeleteSchedule(ctx, scheduleID)
}

func (s *arrivalScheduleService) DeleteAllStudentArrivalSchedules(
	ctx context.Context,
	studentID int64,
) error {
	return s.core.DeleteAllSchedules(ctx, studentID)
}

func (s *arrivalScheduleService) GetStudentArrivalExceptionByID(
	ctx context.Context,
	exceptionID int64,
) (*schedule.StudentArrivalException, error) {
	return s.core.ExceptionByID(ctx, exceptionID)
}

func (s *arrivalScheduleService) GetStudentArrivalExceptionForDate(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
) (*schedule.StudentArrivalException, error) {
	return s.core.ExceptionForDate(ctx, studentID, date)
}

func (s *arrivalScheduleService) GetStudentArrivalExceptions(
	ctx context.Context,
	studentID int64,
) ([]*schedule.StudentArrivalException, error) {
	return s.core.Exceptions(ctx, studentID)
}

func (s *arrivalScheduleService) GetUpcomingStudentArrivalExceptions(
	ctx context.Context,
	studentID int64,
) ([]*schedule.StudentArrivalException, error) {
	return s.core.UpcomingExceptions(ctx, studentID)
}

func (s *arrivalScheduleService) CreateStudentArrivalException(
	ctx context.Context,
	row *schedule.StudentArrivalException,
) error {
	return s.core.CreateException(ctx, row)
}

func (s *arrivalScheduleService) UpdateStudentArrivalException(
	ctx context.Context,
	row *schedule.StudentArrivalException,
) error {
	return s.core.UpdateExceptionRow(ctx, row)
}

func (s *arrivalScheduleService) CreateOrReclaimException(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
	arrivalTime *time.Time,
	reason *string,
	staffID int64,
	resolveStaffID func() (int64, error),
) (*schedule.StudentArrivalException, error) {
	return s.core.CreateOrReclaimException(
		ctx,
		studentID,
		date,
		arrivalTime,
		reason,
		staffID,
		resolveStaffID,
	)
}

func (s *arrivalScheduleService) UpdateException(
	ctx context.Context,
	exceptionID int64,
	studentID int64,
	date timezone.Date,
	reason *string,
	arrivalTime *time.Time,
	clearArrivalTime bool,
	resolveStaffID func() (int64, error),
) (*schedule.StudentArrivalException, error) {
	return s.core.UpdateException(
		ctx,
		exceptionID,
		studentID,
		date,
		reason,
		arrivalTime,
		clearArrivalTime,
		resolveStaffID,
	)
}

func (s *arrivalScheduleService) DeleteStudentArrivalException(
	ctx context.Context,
	exceptionID int64,
) error {
	return s.core.DeleteException(ctx, exceptionID)
}

func (s *arrivalScheduleService) DeleteAllStudentArrivalExceptions(
	ctx context.Context,
	studentID int64,
) error {
	return s.core.DeleteAllExceptions(ctx, studentID)
}

func (s *arrivalScheduleService) GetStudentArrivalNoteByID(
	ctx context.Context,
	noteID int64,
) (*schedule.StudentArrivalNote, error) {
	return s.core.NoteByID(ctx, noteID)
}

func (s *arrivalScheduleService) GetStudentArrivalNotes(
	ctx context.Context,
	studentID int64,
) ([]*schedule.StudentArrivalNote, error) {
	return s.core.Notes(ctx, studentID)
}

func (s *arrivalScheduleService) GetStudentArrivalNotesForDate(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
) ([]*schedule.StudentArrivalNote, error) {
	return s.core.NotesForDate(ctx, studentID, date)
}

func (s *arrivalScheduleService) CreateStudentArrivalNote(
	ctx context.Context,
	row *schedule.StudentArrivalNote,
) error {
	return s.core.CreateNote(ctx, row)
}

func (s *arrivalScheduleService) UpdateStudentArrivalNote(
	ctx context.Context,
	row *schedule.StudentArrivalNote,
) error {
	return s.core.UpdateNote(ctx, row)
}

func (s *arrivalScheduleService) DeleteStudentArrivalNote(
	ctx context.Context,
	noteID int64,
) error {
	return s.core.DeleteNote(ctx, noteID)
}

func (s *arrivalScheduleService) DeleteAllStudentArrivalNotes(
	ctx context.Context,
	studentID int64,
) error {
	return s.core.DeleteAllNotes(ctx, studentID)
}

func (s *arrivalScheduleService) GetStudentArrivalData(
	ctx context.Context,
	studentID int64,
) (*StudentArrivalData, error) {
	data, err := s.core.Data(ctx, studentID)
	if err != nil {
		return nil, err
	}
	return &StudentArrivalData{
		Schedules:  data.Schedules,
		Exceptions: data.Exceptions,
		Notes:      data.Notes,
	}, nil
}

func (s *arrivalScheduleService) GetEffectiveArrivalTimeForDate(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
) (*EffectiveArrivalTime, error) {
	result, err := s.core.EffectiveTimeForDate(ctx, studentID, date)
	if err != nil {
		return nil, err
	}
	return arrivalEffectiveTime(result), nil
}

func (s *arrivalScheduleService) GetBulkEffectiveArrivalTimesForDate(
	ctx context.Context,
	studentIDs []int64,
	date timezone.Date,
) (map[int64]*EffectiveArrivalTime, error) {
	results, err := s.core.BulkEffectiveTimesForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	mapped := make(map[int64]*EffectiveArrivalTime, len(results))
	for studentID, result := range results {
		mapped[studentID] = arrivalEffectiveTime(result)
	}
	return mapped, nil
}

func arrivalEffectiveTime(result *effectiveTimeResult) *EffectiveArrivalTime {
	mapped := &EffectiveArrivalTime{
		Date:        result.Date,
		ArrivalTime: result.Time,
		WeekdayName: result.WeekdayName,
		IsException: result.IsException,
		Notes:       result.Notes,
	}
	for _, note := range result.DayNotes {
		mapped.DayNotes = append(mapped.DayNotes, ArrivalNoteData(note))
	}
	return mapped
}

func (s *arrivalScheduleService) BulkUpsertBySchoolClass(
	ctx context.Context,
	schoolClass string,
	schedules []ArrivalScheduleInput,
	createdBy int64,
) (*BulkUpsertResult, error) {
	if schoolClass == "" {
		return nil, &ScheduleError{
			Op:  opBulkUpsertBySchoolClass,
			Err: errors.New("school_class is required"),
		}
	}
	if len(schedules) == 0 {
		return nil, &ScheduleError{
			Op:  opBulkUpsertBySchoolClass,
			Err: errors.New("schedules cannot be empty"),
		}
	}

	students, err := s.studentRepo.FindBySchoolClass(ctx, schoolClass)
	if err != nil {
		return nil, &ScheduleError{
			Op:  opBulkUpsertBySchoolClass,
			Err: fmt.Errorf("failed to find students for class %s: %w", schoolClass, err),
		}
	}
	if len(students) == 0 {
		return &BulkUpsertResult{StudentsAffected: 0}, nil
	}

	tenantID := tenant.FromContext(ctx)
	warnings := make([]OverwriteWarning, 0)
	parsedTimes := make(map[int]time.Time, len(schedules))
	for _, input := range schedules {
		value, err := time.Parse("2006-01-02 15:04", "2000-01-01 "+input.ArrivalTime)
		if err != nil {
			return nil, &ScheduleError{
				Op: opBulkUpsertBySchoolClass,
				Err: fmt.Errorf(
					"invalid expected_arrival %q for weekday %d: %w",
					input.ArrivalTime,
					input.Weekday,
					err,
				),
			}
		}
		parsedTimes[input.Weekday] = value
	}

	err = tenant.WithTenantTx(ctx, s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		for _, student := range students {
			existing, err := s.scheduleRepo.FindByStudentID(txCtx, student.ID)
			if err != nil {
				return fmt.Errorf(
					"failed to fetch existing schedules for student %d: %w",
					student.ID,
					err,
				)
			}

			existingByWeekday := make(map[int]*schedule.StudentArrivalSchedule, len(existing))
			for _, row := range existing {
				existingByWeekday[row.Weekday] = row
			}

			for _, input := range schedules {
				arrivalTime := parsedTimes[input.Weekday]
				if existingRow, ok := existingByWeekday[input.Weekday]; ok &&
					existingRow.ExpectedArrival.Format("15:04") != arrivalTime.Format("15:04") {
					warnings = append(warnings, OverwriteWarning{
						StudentID:    student.ID,
						StudentName:  s.getStudentName(txCtx, student),
						Weekday:      input.Weekday,
						WeekdayName:  schedule.WeekdayNames[input.Weekday],
						PreviousTime: existingRow.ExpectedArrival.Format("15:04"),
						NewTime:      arrivalTime.Format("15:04"),
					})
				}

				row := &schedule.StudentArrivalSchedule{
					StudentID:       student.ID,
					Weekday:         input.Weekday,
					ExpectedArrival: arrivalTime,
					CreatedBy:       createdBy,
				}
				row.SetTenantID(tenantID)
				if err := s.scheduleRepo.UpsertSchedule(txCtx, row); err != nil {
					return fmt.Errorf(
						"failed to upsert schedule for student %d weekday %d: %w",
						student.ID,
						input.Weekday,
						err,
					)
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, &ScheduleError{Op: opBulkUpsertBySchoolClass, Err: err}
	}

	s.getLogger().Info(
		"bulk upsert arrival schedules by school class",
		slog.String("school_class", schoolClass),
		slog.Int("students_affected", len(students)),
		slog.Int("weekdays_set", len(schedules)),
		slog.Int("overwrites", len(warnings)),
	)

	affectedIDs := make([]int64, 0, len(students))
	for _, student := range students {
		affectedIDs = append(affectedIDs, student.ID)
	}
	return &BulkUpsertResult{
		StudentsAffected:    len(students),
		OverwrittenStudents: warnings,
		AffectedStudentIDs:  affectedIDs,
	}, nil
}

func (s *arrivalScheduleService) getStudentName(
	ctx context.Context,
	student *users.Student,
) string {
	if s.personRepo == nil {
		return fmt.Sprintf("Student %d", student.ID)
	}
	person, err := s.personRepo.FindByID(ctx, student.PersonID)
	if err != nil || person == nil {
		return fmt.Sprintf("Student %d", student.ID)
	}
	return person.FirstName + " " + person.LastName
}
