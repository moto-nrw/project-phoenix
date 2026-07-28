package schedule

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/uptrace/bun"
)

// PickupScheduleService defines operations for managing student pickup schedules.
// The domain-named contract is intentionally stable because IoT, planning,
// reminders, exports, and messaging consume it directly.
type PickupScheduleService interface {
	GetStudentPickupSchedules(ctx context.Context, studentID int64) ([]*schedule.StudentPickupSchedule, error)
	GetWeeklySchedulesByStudentIDsAndWeekday(ctx context.Context, studentIDs []int64, weekday int) ([]*schedule.StudentPickupSchedule, error)
	GetStudentPickupScheduleForWeekday(ctx context.Context, studentID int64, weekday int) (*schedule.StudentPickupSchedule, error)
	UpsertStudentPickupSchedule(ctx context.Context, scheduleData *schedule.StudentPickupSchedule) error
	UpsertBulkStudentPickupSchedules(ctx context.Context, studentID int64, schedules []*schedule.StudentPickupSchedule) error
	DeleteStudentPickupSchedule(ctx context.Context, scheduleID int64) error
	DeleteAllStudentPickupSchedules(ctx context.Context, studentID int64) error

	GetStudentPickupExceptionByID(ctx context.Context, exceptionID int64) (*schedule.StudentPickupException, error)
	GetStudentPickupExceptionForDate(ctx context.Context, studentID int64, date timezone.Date) (*schedule.StudentPickupException, error)
	GetStudentPickupExceptions(ctx context.Context, studentID int64) ([]*schedule.StudentPickupException, error)
	GetUpcomingStudentPickupExceptions(ctx context.Context, studentID int64) ([]*schedule.StudentPickupException, error)
	CreateStudentPickupException(ctx context.Context, exception *schedule.StudentPickupException) error
	UpdateStudentPickupException(ctx context.Context, exception *schedule.StudentPickupException) error
	DeleteStudentPickupException(ctx context.Context, exceptionID int64) error
	DeleteAllStudentPickupExceptions(ctx context.Context, studentID int64) error
	CreateOrReclaimException(ctx context.Context, studentID int64, date timezone.Date, pickupTime *time.Time, reason *string, staffID int64, resolveStaffID func() (int64, error)) (*schedule.StudentPickupException, error)
	UpsertExceptions(ctx context.Context, studentID int64, dates []timezone.Date, pickupTime *time.Time, reason *string, staffID int64, resolveStaffID func() (int64, error)) ([]*schedule.StudentPickupException, error)
	UpdateException(ctx context.Context, exceptionID, studentID int64, date timezone.Date, reason *string, pickupTime *time.Time, resolveStaffID func() (int64, error)) (*schedule.StudentPickupException, error)

	GetStudentPickupNoteByID(ctx context.Context, noteID int64) (*schedule.StudentPickupNote, error)
	GetStudentPickupNotes(ctx context.Context, studentID int64) ([]*schedule.StudentPickupNote, error)
	GetStudentPickupNotesForDate(ctx context.Context, studentID int64, date timezone.Date) ([]*schedule.StudentPickupNote, error)
	CreateStudentPickupNote(ctx context.Context, note *schedule.StudentPickupNote) error
	UpdateStudentPickupNote(ctx context.Context, note *schedule.StudentPickupNote) error
	DeleteStudentPickupNote(ctx context.Context, noteID int64) error
	DeleteAllStudentPickupNotes(ctx context.Context, studentID int64) error

	GetStudentPickupData(ctx context.Context, studentID int64) (*StudentPickupData, error)
	GetEffectivePickupTimeForDate(ctx context.Context, studentID int64, date timezone.Date) (*EffectivePickupTime, error)
	GetBulkEffectivePickupTimesForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*EffectivePickupTime, error)
}

type StudentPickupData struct {
	Schedules  []*schedule.StudentPickupSchedule  `json:"schedules"`
	Exceptions []*schedule.StudentPickupException `json:"exceptions"`
	Notes      []*schedule.StudentPickupNote      `json:"notes"`
}

type NoteData struct {
	ID      int64  `json:"id"`
	Content string `json:"content"`
}

type EffectivePickupTime struct {
	Date        timezone.Date `json:"date"`
	PickupTime  *time.Time    `json:"pickup_time"`
	WeekdayName string        `json:"weekday_name"`
	IsException bool          `json:"is_exception"`
	Notes       string        `json:"notes,omitempty"`
	DayNotes    []NoteData    `json:"day_notes,omitempty"`
}

type pickupScheduleService struct {
	core *effectiveTimeCore[
		*schedule.StudentPickupSchedule,
		*schedule.StudentPickupException,
		*schedule.StudentPickupNote,
		pickupTimeDomain,
	]
}

func NewPickupScheduleService(
	scheduleRepo schedule.StudentPickupScheduleRepository,
	exceptionRepo schedule.StudentPickupExceptionRepository,
	noteRepo schedule.StudentPickupNoteRepository,
	db *bun.DB,
) PickupScheduleService {
	return &pickupScheduleService{
		core: newEffectiveTimeCore(
			scheduleRepo,
			exceptionRepo,
			noteRepo,
			db,
			pickupTimeDomain{},
		),
	}
}

func (s *pickupScheduleService) GetStudentPickupSchedules(
	ctx context.Context,
	studentID int64,
) ([]*schedule.StudentPickupSchedule, error) {
	return s.core.Schedules(ctx, studentID)
}

func (s *pickupScheduleService) GetWeeklySchedulesByStudentIDsAndWeekday(
	ctx context.Context,
	studentIDs []int64,
	weekday int,
) ([]*schedule.StudentPickupSchedule, error) {
	return s.core.WeeklySchedulesByStudentIDsAndWeekday(ctx, studentIDs, weekday)
}

func (s *pickupScheduleService) GetStudentPickupScheduleForWeekday(
	ctx context.Context,
	studentID int64,
	weekday int,
) (*schedule.StudentPickupSchedule, error) {
	return s.core.ScheduleForWeekday(ctx, studentID, weekday)
}

func (s *pickupScheduleService) UpsertStudentPickupSchedule(
	ctx context.Context,
	row *schedule.StudentPickupSchedule,
) error {
	return s.core.UpsertSchedule(ctx, row)
}

func (s *pickupScheduleService) UpsertBulkStudentPickupSchedules(
	ctx context.Context,
	studentID int64,
	rows []*schedule.StudentPickupSchedule,
) error {
	return s.core.UpsertBulkSchedules(ctx, studentID, rows)
}

func (s *pickupScheduleService) DeleteStudentPickupSchedule(
	ctx context.Context,
	scheduleID int64,
) error {
	return s.core.DeleteSchedule(ctx, scheduleID)
}

func (s *pickupScheduleService) DeleteAllStudentPickupSchedules(
	ctx context.Context,
	studentID int64,
) error {
	return s.core.DeleteAllSchedules(ctx, studentID)
}

func (s *pickupScheduleService) GetStudentPickupExceptionByID(
	ctx context.Context,
	exceptionID int64,
) (*schedule.StudentPickupException, error) {
	return s.core.ExceptionByID(ctx, exceptionID)
}

func (s *pickupScheduleService) GetStudentPickupExceptionForDate(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
) (*schedule.StudentPickupException, error) {
	return s.core.ExceptionForDate(ctx, studentID, date)
}

func (s *pickupScheduleService) GetStudentPickupExceptions(
	ctx context.Context,
	studentID int64,
) ([]*schedule.StudentPickupException, error) {
	return s.core.Exceptions(ctx, studentID)
}

func (s *pickupScheduleService) GetUpcomingStudentPickupExceptions(
	ctx context.Context,
	studentID int64,
) ([]*schedule.StudentPickupException, error) {
	return s.core.UpcomingExceptions(ctx, studentID)
}

func (s *pickupScheduleService) CreateStudentPickupException(
	ctx context.Context,
	row *schedule.StudentPickupException,
) error {
	return s.core.CreateException(ctx, row)
}

func (s *pickupScheduleService) UpdateStudentPickupException(
	ctx context.Context,
	row *schedule.StudentPickupException,
) error {
	return s.core.UpdateExceptionRow(ctx, row)
}

func (s *pickupScheduleService) CreateOrReclaimException(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
	pickupTime *time.Time,
	reason *string,
	staffID int64,
	resolveStaffID func() (int64, error),
) (*schedule.StudentPickupException, error) {
	return s.core.CreateOrReclaimException(
		ctx,
		studentID,
		date,
		pickupTime,
		reason,
		staffID,
		resolveStaffID,
	)
}

func (s *pickupScheduleService) UpsertExceptions(
	ctx context.Context,
	studentID int64,
	dates []timezone.Date,
	pickupTime *time.Time,
	reason *string,
	staffID int64,
	resolveStaffID func() (int64, error),
) ([]*schedule.StudentPickupException, error) {
	return s.core.UpsertExceptions(
		ctx,
		studentID,
		dates,
		pickupTime,
		reason,
		staffID,
		resolveStaffID,
	)
}

func (s *pickupScheduleService) UpdateException(
	ctx context.Context,
	exceptionID int64,
	studentID int64,
	date timezone.Date,
	reason *string,
	pickupTime *time.Time,
	resolveStaffID func() (int64, error),
) (*schedule.StudentPickupException, error) {
	return s.core.UpdateException(
		ctx,
		exceptionID,
		studentID,
		date,
		reason,
		pickupTime,
		resolveStaffID,
	)
}

func (s *pickupScheduleService) DeleteStudentPickupException(
	ctx context.Context,
	exceptionID int64,
) error {
	return s.core.DeleteException(ctx, exceptionID)
}

func (s *pickupScheduleService) DeleteAllStudentPickupExceptions(
	ctx context.Context,
	studentID int64,
) error {
	return s.core.DeleteAllExceptions(ctx, studentID)
}

func (s *pickupScheduleService) GetStudentPickupNoteByID(
	ctx context.Context,
	noteID int64,
) (*schedule.StudentPickupNote, error) {
	return s.core.NoteByID(ctx, noteID)
}

func (s *pickupScheduleService) GetStudentPickupNotes(
	ctx context.Context,
	studentID int64,
) ([]*schedule.StudentPickupNote, error) {
	return s.core.Notes(ctx, studentID)
}

func (s *pickupScheduleService) GetStudentPickupNotesForDate(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
) ([]*schedule.StudentPickupNote, error) {
	return s.core.NotesForDate(ctx, studentID, date)
}

func (s *pickupScheduleService) CreateStudentPickupNote(
	ctx context.Context,
	row *schedule.StudentPickupNote,
) error {
	return s.core.CreateNote(ctx, row)
}

func (s *pickupScheduleService) UpdateStudentPickupNote(
	ctx context.Context,
	row *schedule.StudentPickupNote,
) error {
	return s.core.UpdateNote(ctx, row)
}

func (s *pickupScheduleService) DeleteStudentPickupNote(
	ctx context.Context,
	noteID int64,
) error {
	return s.core.DeleteNote(ctx, noteID)
}

func (s *pickupScheduleService) DeleteAllStudentPickupNotes(
	ctx context.Context,
	studentID int64,
) error {
	return s.core.DeleteAllNotes(ctx, studentID)
}

func (s *pickupScheduleService) GetStudentPickupData(
	ctx context.Context,
	studentID int64,
) (*StudentPickupData, error) {
	data, err := s.core.Data(ctx, studentID)
	if err != nil {
		return nil, err
	}
	return &StudentPickupData{
		Schedules:  data.Schedules,
		Exceptions: data.Exceptions,
		Notes:      data.Notes,
	}, nil
}

func (s *pickupScheduleService) GetEffectivePickupTimeForDate(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
) (*EffectivePickupTime, error) {
	result, err := s.core.EffectiveTimeForDate(ctx, studentID, date)
	if err != nil {
		return nil, err
	}
	return pickupEffectiveTime(result), nil
}

func (s *pickupScheduleService) GetBulkEffectivePickupTimesForDate(
	ctx context.Context,
	studentIDs []int64,
	date timezone.Date,
) (map[int64]*EffectivePickupTime, error) {
	results, err := s.core.BulkEffectiveTimesForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	mapped := make(map[int64]*EffectivePickupTime, len(results))
	for studentID, result := range results {
		mapped[studentID] = pickupEffectiveTime(result)
	}
	return mapped, nil
}

func pickupEffectiveTime(result *effectiveTimeResult) *EffectivePickupTime {
	mapped := &EffectivePickupTime{
		Date:        result.Date,
		PickupTime:  result.Time,
		WeekdayName: result.WeekdayName,
		IsException: result.IsException,
		Notes:       result.Notes,
	}
	for _, note := range result.DayNotes {
		mapped.DayNotes = append(mapped.DayNotes, NoteData(note))
	}
	return mapped
}
