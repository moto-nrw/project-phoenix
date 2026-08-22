package schedule

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	educationModel "github.com/moto-nrw/project-phoenix/models/education"
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
	// GetWeeklySchedulesByStudentIDsForDate returns the full recurring arrival
	// plan applicable on date in one batch projection.
	GetWeeklySchedulesByStudentIDsForDate(ctx context.Context, studentIDs []int64, date timezone.Date) ([]*schedule.StudentArrivalSchedule, error)
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
	GetStudentArrivalDataForDate(ctx context.Context, studentID int64, date timezone.Date) (*StudentArrivalData, error)
	GetStudentArrivalDataForDateRange(ctx context.Context, studentID int64, from, to timezone.Date) (*StudentArrivalData, error)
	GetStudentsWithStoredArrivalSchedules(ctx context.Context, studentIDs []int64) (map[int64]bool, error)
	GetEffectiveArrivalTimeForDate(ctx context.Context, studentID int64, date timezone.Date) (*EffectiveArrivalTime, error)
	GetBulkEffectiveArrivalTimesForDate(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]*EffectiveArrivalTime, error)
	BulkUpsertArrivalSchedules(ctx context.Context, filter ArrivalScheduleBulkFilter, schedules []ArrivalScheduleInput, createdBy int64) (*BulkUpsertResult, error)
	GetClassArrivalTimes(ctx context.Context, schoolClass string) (*ClassArrivalTimes, error)
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

// ArrivalScheduleBulkFilter selects the complete student cohort for a bulk
// arrival-schedule update. Exactly one field must be set. GroupID refers to the
// child's education/OGS group (users.students.group_id), not an activity group.
type ArrivalScheduleBulkFilter struct {
	SchoolClass string
	GroupID     int64
	StudentIDs  []int64
	Authorize   func(context.Context, *users.Student) (bool, error)
}

var (
	ErrBulkStudentUnauthorized = errors.New("bulk student selection contains an unauthorized student")
	ErrBulkStudentNotFound     = errors.New("bulk student selection contains a missing student")
)

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

// ClassArrivalTimes is the Unterrichtsschluss a class carries, for the screen
// that maintains it (#2414).
type ClassArrivalTimes struct {
	SchoolClass string            `json:"school_class"`
	Times       map[string]string `json:"times"`
	UpdatedAt   *time.Time        `json:"updated_at,omitempty"`
}

const opBulkUpsertArrivalSchedules = "bulk upsert arrival schedules"

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
	baselines    ArrivalBaselineReader
	classTimes   educationModel.ClassArrivalTimeRepository
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
	return NewArrivalScheduleServiceWithBaselines(
		scheduleRepo, exceptionRepo, noteRepo, studentRepo, personRepo, nil, nil, db, logger,
	)
}

// NewArrivalScheduleServiceWithBaselines is the wiring the HTTP server uses:
// baselines resolves the regular arrival time from the class timetable and,
// in booking mode, the care days from the approved bookings (#2414). Passing
// nil keeps the pre-#2414 behaviour of reading stored rows only, which is what
// the CLI and older tests want.
func NewArrivalScheduleServiceWithBaselines(
	scheduleRepo schedule.StudentArrivalScheduleRepository,
	exceptionRepo schedule.StudentArrivalExceptionRepository,
	noteRepo schedule.StudentArrivalNoteRepository,
	studentRepo users.StudentRepository,
	personRepo users.PersonRepository,
	baselines ArrivalBaselineReader,
	classTimes educationModel.ClassArrivalTimeRepository,
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
		baselines:    baselines,
		classTimes:   classTimes,
		db:           db,
		logger:       logger,
	}
}

// projectedWeeklySchedules flattens the projected week into the sorted row
// list the weekly readers expect.
func (s *arrivalScheduleService) projectedWeeklySchedules(
	ctx context.Context,
	studentIDs []int64,
	date timezone.Date,
) ([]*schedule.StudentArrivalSchedule, error) {
	week, err := s.projectedWeek(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	rows := make([]*schedule.StudentArrivalSchedule, 0, len(studentIDs)*schedule.WeekdayFriday)
	for _, studentID := range uniquePositiveStudentIDs(studentIDs) {
		for _, row := range week[studentID] {
			rows = append(rows, row)
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].StudentID != rows[j].StudentID {
			return rows[i].StudentID < rows[j].StudentID
		}
		return rows[i].Weekday < rows[j].Weekday
	})
	return rows, nil
}

// projectedWeekRange resolves each weekday against its own calendar date.
// Booking coverage can change within a week, so one day's projection cannot
// stand in for the whole weekly plan.
func (s *arrivalScheduleService) projectedWeekRange(
	ctx context.Context,
	studentIDs []int64,
	from, to timezone.Date,
) ([]*schedule.StudentArrivalSchedule, error) {
	if s.baselines == nil {
		return s.projectedWeeklySchedules(ctx, studentIDs, from)
	}
	projection, err := s.baselines.Project(ctx, studentIDs, from, to)
	if err != nil {
		return nil, &ScheduleError{Op: "project arrival schedule week", Err: err}
	}
	rows := make([]*schedule.StudentArrivalSchedule, 0, len(studentIDs)*schedule.WeekdayFriday)
	for _, studentID := range uniquePositiveStudentIDs(studentIDs) {
		for date := from; !date.After(to); date = date.AddDays(1) {
			if row := projection.ForDate(studentID, date); row != nil {
				rows = append(rows, row)
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].StudentID != rows[j].StudentID {
			return rows[i].StudentID < rows[j].StudentID
		}
		return rows[i].Weekday < rows[j].Weekday
	})
	return rows, nil
}

func weekStart(date timezone.Date) timezone.Date {
	return date.AddDays(-(int(date.Weekday()) + 6) % 7)
}

// projectedWeek returns the recurring rows in force on a date, class times
// resolved. Without a baseline reader it falls back to the stored rows.
func (s *arrivalScheduleService) projectedWeek(
	ctx context.Context,
	studentIDs []int64,
	date timezone.Date,
) (map[int64]ArrivalWeek, error) {
	out := make(map[int64]ArrivalWeek, len(studentIDs))
	if s.baselines == nil {
		for _, studentID := range studentIDs {
			rows, err := s.core.Schedules(ctx, studentID)
			if err != nil {
				return nil, err
			}
			week := make(ArrivalWeek, len(rows))
			for _, row := range rows {
				week[row.Weekday] = row
			}
			out[studentID] = week
		}
		return out, nil
	}
	projection, err := s.baselines.Project(ctx, studentIDs, date, date)
	if err != nil {
		return nil, &ScheduleError{Op: "project arrival schedules", Err: err}
	}
	for _, studentID := range studentIDs {
		out[studentID] = projection.WeeklyForDate(studentID, date)
	}
	return out, nil
}

func (s *arrivalScheduleService) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

func (s *arrivalScheduleService) GetStudentArrivalSchedules(
	ctx context.Context,
	studentID int64,
) ([]*schedule.StudentArrivalSchedule, error) {
	from := weekStart(timezone.TodayDate())
	return s.projectedWeekRange(ctx, []int64{studentID}, from, from.AddDays(4))
}

func (s *arrivalScheduleService) GetWeeklySchedulesByStudentIDsAndWeekday(
	ctx context.Context,
	studentIDs []int64,
	weekday int,
) ([]*schedule.StudentArrivalSchedule, error) {
	if weekday < schedule.WeekdayMonday || weekday > schedule.WeekdayFriday {
		return nil, nil
	}
	date := weekStart(timezone.TodayDate()).AddDays(weekday - schedule.WeekdayMonday)
	rows, err := s.projectedWeeklySchedules(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	filtered := make([]*schedule.StudentArrivalSchedule, 0, len(rows))
	for _, row := range rows {
		if row.Weekday == weekday {
			filtered = append(filtered, row)
		}
	}
	return filtered, nil
}

func (s *arrivalScheduleService) GetWeeklySchedulesByStudentIDsForDate(
	ctx context.Context,
	studentIDs []int64,
	date timezone.Date,
) ([]*schedule.StudentArrivalSchedule, error) {
	return s.projectedWeeklySchedules(ctx, studentIDs, date)
}

func (s *arrivalScheduleService) GetStudentArrivalScheduleForWeekday(
	ctx context.Context,
	studentID int64,
	weekday int,
) (*schedule.StudentArrivalSchedule, error) {
	// The weekday validation stays with the core so an invalid weekday keeps
	// erroring rather than reading as "no care that day".
	stored, err := s.core.ScheduleForWeekday(ctx, studentID, weekday)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return stored, err
	}
	if s.baselines == nil {
		return stored, err
	}
	date := weekStart(timezone.TodayDate()).AddDays(weekday - schedule.WeekdayMonday)
	week, err := s.projectedWeek(ctx, []int64{studentID}, date)
	if err != nil {
		return nil, err
	}
	return week[studentID][weekday], nil
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
	if err := s.collapseIntoClassTime(ctx, studentID, rows); err != nil {
		return err
	}
	rows, err := s.preserveInactiveBookingRows(ctx, studentID, rows)
	if err != nil {
		return err
	}
	return s.core.UpsertBulkSchedules(ctx, studentID, rows)
}

// preserveInactiveBookingRows keeps omitted manual rows while bookings define
// the active care days. Disabling booking authority later must restore the
// school's complete weekly plan, including rows currently ignored by bookings.
func (s *arrivalScheduleService) preserveInactiveBookingRows(
	ctx context.Context,
	studentID int64,
	rows []*schedule.StudentArrivalSchedule,
) ([]*schedule.StudentArrivalSchedule, error) {
	if s.baselines == nil {
		return rows, nil
	}
	today := timezone.TodayDate()
	projection, err := s.baselines.Project(ctx, []int64{studentID}, today, today)
	if err != nil {
		return nil, &ScheduleError{Op: "preserve inactive booking arrival rows", Err: err}
	}
	if projection == nil || !projection.BookingsAuthoritative {
		return rows, nil
	}
	stored, err := s.scheduleRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &ScheduleError{Op: "preserve inactive booking arrival rows", Err: err}
	}
	incoming := make(map[int]bool, len(rows))
	merged := append(make([]*schedule.StudentArrivalSchedule, 0, len(rows)+len(stored)), rows...)
	for _, row := range rows {
		if row != nil {
			incoming[row.Weekday] = true
		}
	}
	for _, row := range stored {
		if row != nil && !incoming[row.Weekday] {
			preserved := *row
			if !preserved.ExpectedArrival.IsZero() {
				preserved.ExpectedArrival = timezone.WallClock(preserved.ExpectedArrival)
			}
			merged = append(merged, &preserved)
		}
	}
	return merged, nil
}

// collapseIntoClassTime drops an own time that is identical to what the class
// already supplies (ADR 0005): storing it would create a deviation that is not
// one, and the child would then stop following the class when it changes.
//
// It reads the class timetable directly rather than through the projection:
// the projection only yields a class row where a care day already exists, and
// the very first write of a week has none yet.
func (s *arrivalScheduleService) collapseIntoClassTime(
	ctx context.Context,
	studentID int64,
	rows []*schedule.StudentArrivalSchedule,
) error {
	if s.classTimes == nil || len(rows) == 0 {
		return nil
	}
	student, err := s.studentRepo.FindByID(ctx, studentID)
	if err != nil {
		return fmt.Errorf("load student before checking class arrival time: %w", err)
	}
	if student == nil || strings.TrimSpace(student.SchoolClass) == "" {
		return nil
	}
	classRows, err := s.classTimes.FindByClasses(ctx, []string{student.SchoolClass})
	if err != nil {
		return fmt.Errorf("load class arrival time before saving weekly plan: %w", err)
	}
	if len(classRows) == 0 || classRows[0] == nil {
		return nil
	}
	for _, row := range rows {
		if row == nil || row.ExpectedArrival.IsZero() {
			continue
		}
		classTime, ok := classRows[0].TimeForWeekday(row.Weekday)
		if !ok {
			continue
		}
		if row.ExpectedArrival.Format("15:04") == classTime.Format("15:04") {
			row.ExpectedArrival = time.Time{}
		}
	}
	return nil
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
	from := weekStart(timezone.TodayDate())
	return s.GetStudentArrivalDataForDateRange(ctx, studentID, from, from.AddDays(4))
}

func (s *arrivalScheduleService) GetStudentArrivalDataForDate(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
) (*StudentArrivalData, error) {
	data, err := s.core.Data(ctx, studentID)
	if err != nil {
		return nil, err
	}
	// The weekly plan the detail screen renders is the projected one, so the
	// class time shows up where a child inherits it (#2414).
	projected, err := s.projectedWeeklySchedules(ctx, []int64{studentID}, date)
	if err != nil {
		return nil, err
	}
	data.Schedules = projected
	return &StudentArrivalData{
		Schedules:  data.Schedules,
		Exceptions: data.Exceptions,
		Notes:      data.Notes,
	}, nil
}

// GetStudentArrivalDataForDateRange returns one row for each care day in the
// requested range. Booking coverage is date-dependent, so a weekly editor
// must not reuse Monday's care plan for Tuesday through Friday.
func (s *arrivalScheduleService) GetStudentArrivalDataForDateRange(
	ctx context.Context,
	studentID int64,
	from, to timezone.Date,
) (*StudentArrivalData, error) {
	data, err := s.core.Data(ctx, studentID)
	if err != nil {
		return nil, err
	}
	if to.Before(from) {
		return nil, &ScheduleError{Op: "project arrival schedule range", Err: errors.New("arrival schedule range ends before it starts")}
	}
	if s.baselines == nil {
		return s.GetStudentArrivalDataForDate(ctx, studentID, from)
	}
	projection, err := s.baselines.Project(ctx, []int64{studentID}, from, to)
	if err != nil {
		return nil, &ScheduleError{Op: "project arrival schedule range", Err: err}
	}
	byWeekday := make(map[int]*schedule.StudentArrivalSchedule, schedule.WeekdayFriday)
	for date := from; !date.After(to); date = date.AddDays(1) {
		if row := projection.ForDate(studentID, date); row != nil {
			byWeekday[row.Weekday] = row
		}
	}
	schedules := make([]*schedule.StudentArrivalSchedule, 0, len(byWeekday))
	for weekday := schedule.WeekdayMonday; weekday <= schedule.WeekdayFriday; weekday++ {
		if row := byWeekday[weekday]; row != nil {
			schedules = append(schedules, row)
		}
	}
	return &StudentArrivalData{
		Schedules:  schedules,
		Exceptions: data.Exceptions,
		Notes:      data.Notes,
	}, nil
}

// GetStudentsWithStoredArrivalSchedules identifies children with stored weekly
// rows, including care-day markers that inherit their class time. A bulk write
// replaces either kind of row.
func (s *arrivalScheduleService) GetStudentsWithStoredArrivalSchedules(
	ctx context.Context,
	studentIDs []int64,
) (map[int64]bool, error) {
	rows, err := s.scheduleRepo.FindByStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, &ScheduleError{Op: "list stored arrival schedules", Err: err}
	}
	hasSchedules := make(map[int64]bool, len(rows))
	for _, row := range rows {
		hasSchedules[row.StudentID] = true
	}
	return hasSchedules, nil
}

func (s *arrivalScheduleService) GetEffectiveArrivalTimeForDate(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
) (*EffectiveArrivalTime, error) {
	if s.baselines == nil {
		result, err := s.core.EffectiveTimeForDate(ctx, studentID, date)
		if err != nil {
			return nil, err
		}
		return arrivalEffectiveTime(result), nil
	}
	projection, err := s.baselines.Project(ctx, []int64{studentID}, date, date)
	if err != nil {
		return nil, &ScheduleError{Op: "get effective arrival time", Err: err}
	}
	result, err := s.core.EffectiveTimeForDateWithSchedule(ctx, studentID, date, projection.ForDate(studentID, date))
	if err != nil {
		return nil, err
	}
	return arrivalEffectiveTime(result), nil
}

// bulkEffectiveResults resolves the recurring rows through the projection
// before the shared exception/note merge runs, mirroring the pickup path.
func (s *arrivalScheduleService) bulkEffectiveResults(
	ctx context.Context,
	studentIDs []int64,
	date timezone.Date,
) (map[int64]*effectiveTimeResult, error) {
	if s.baselines == nil {
		return s.core.BulkEffectiveTimesForDate(ctx, studentIDs, date)
	}
	projection, err := s.baselines.Project(ctx, studentIDs, date, date)
	if err != nil {
		return nil, &ScheduleError{Op: "get bulk effective arrival times", Err: err}
	}
	schedules := make(map[int64]*schedule.StudentArrivalSchedule, len(studentIDs))
	for _, studentID := range studentIDs {
		if row := projection.ForDate(studentID, date); row != nil {
			schedules[studentID] = row
		}
	}
	return s.core.BulkEffectiveTimesForDateWithSchedules(ctx, studentIDs, date, schedules)
}

func (s *arrivalScheduleService) GetBulkEffectiveArrivalTimesForDate(
	ctx context.Context,
	studentIDs []int64,
	date timezone.Date,
) (map[int64]*EffectiveArrivalTime, error) {
	results, err := s.bulkEffectiveResults(ctx, studentIDs, date)
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

func (s *arrivalScheduleService) BulkUpsertArrivalSchedules(
	ctx context.Context,
	filter ArrivalScheduleBulkFilter,
	schedules []ArrivalScheduleInput,
	createdBy int64,
) (*BulkUpsertResult, error) {
	schoolClass := strings.TrimSpace(filter.SchoolClass)
	hasSchoolClass := schoolClass != ""
	hasGroup := filter.GroupID != 0
	hasStudents := len(filter.StudentIDs) > 0
	selectorCount := 0
	if hasSchoolClass {
		selectorCount++
	}
	if hasGroup {
		selectorCount++
	}
	if hasStudents {
		selectorCount++
	}
	if selectorCount != 1 {
		return nil, &ScheduleError{
			Op:  opBulkUpsertArrivalSchedules,
			Err: errors.New("exactly one bulk filter is required: school_class, group_id, or student_ids"),
		}
	}
	if filter.GroupID < 0 {
		return nil, &ScheduleError{
			Op:  opBulkUpsertArrivalSchedules,
			Err: errors.New("group_id must be positive"),
		}
	}
	if len(schedules) == 0 {
		return nil, &ScheduleError{
			Op:  opBulkUpsertArrivalSchedules,
			Err: errors.New("schedules cannot be empty"),
		}
	}
	if len(filter.StudentIDs) > 500 {
		return nil, &ScheduleError{Op: opBulkUpsertArrivalSchedules, Err: errors.New("student_ids cannot exceed 500 items")}
	}

	// A whole school class means the class timetable, not N per-child rows
	// (#2414, ADR 0005). The other two filters stay per-child: a group is not
	// a class, and an explicit selection is a deliberate deviation.
	if hasSchoolClass && s.classTimes != nil {
		return s.upsertClassArrivalTimes(ctx, schoolClass, schedules, createdBy, filter.Authorize)
	}

	var (
		students    []*users.Student
		filterType  string
		filterValue string
		err         error
	)
	if hasSchoolClass {
		filterType = "school_class"
		filterValue = schoolClass
		students, err = s.studentRepo.FindBySchoolClass(ctx, schoolClass)
	} else if hasGroup {
		filterType = "group_id"
		filterValue = strconv.FormatInt(filter.GroupID, 10)
		students, err = s.studentRepo.FindByGroupID(ctx, filter.GroupID)
	} else {
		filterType = "student_ids"
		filterValue = strconv.Itoa(len(filter.StudentIDs))
		byID, findErr := s.studentRepo.FindByIDs(ctx, filter.StudentIDs)
		err = findErr
		if err == nil {
			students = make([]*users.Student, 0, len(filter.StudentIDs))
			for _, id := range filter.StudentIDs {
				student, ok := byID[id]
				if !ok || student.Status == users.StudentStatusAlumnus {
					return nil, &ScheduleError{Op: opBulkUpsertArrivalSchedules, Err: fmt.Errorf("%w: student %d", ErrBulkStudentNotFound, id)}
				}
				students = append(students, student)
			}
		}
	}
	if err != nil {
		return nil, &ScheduleError{
			Op:  opBulkUpsertArrivalSchedules,
			Err: fmt.Errorf("failed to find students for %s %s: %w", filterType, filterValue, err),
		}
	}
	if len(students) == 0 {
		return &BulkUpsertResult{StudentsAffected: 0}, nil
	}

	tenantID := tenant.FromContext(ctx)
	warnings := make([]OverwriteWarning, 0)
	parsedTimes := make(map[int]time.Time, len(schedules))
	for _, input := range schedules {
		if input.Weekday < 1 || input.Weekday > 5 {
			return nil, &ScheduleError{Op: opBulkUpsertArrivalSchedules, Err: fmt.Errorf("invalid weekday %d", input.Weekday)}
		}
		if _, duplicate := parsedTimes[input.Weekday]; duplicate {
			return nil, &ScheduleError{Op: opBulkUpsertArrivalSchedules, Err: fmt.Errorf("duplicate weekday %d", input.Weekday)}
		}
		if strings.TrimSpace(input.ArrivalTime) == "" {
			return nil, &ScheduleError{Op: opBulkUpsertArrivalSchedules, Err: fmt.Errorf("expected_arrival is required for weekday %d unless school_class is selected", input.Weekday)}
		}
		value, err := time.Parse("2006-01-02 15:04", "2000-01-01 "+input.ArrivalTime)
		if err != nil {
			return nil, &ScheduleError{
				Op: opBulkUpsertArrivalSchedules,
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
		sort.Slice(students, func(i, j int) bool { return students[i].ID < students[j].ID })
		lockedStudents := make([]*users.Student, 0, len(students))
		for _, selected := range students {
			fresh, lockErr := s.studentRepo.FindByIDForUpdate(txCtx, selected.ID)
			if lockErr != nil {
				if errors.Is(lockErr, sql.ErrNoRows) {
					return fmt.Errorf("%w: student %d", ErrBulkStudentNotFound, selected.ID)
				}
				return fmt.Errorf("lock selected student %d: %w", selected.ID, lockErr)
			}
			if fresh.Status == users.StudentStatusAlumnus {
				return fmt.Errorf("%w: student %d", ErrBulkStudentNotFound, fresh.ID)
			}
			if hasSchoolClass && !strings.EqualFold(strings.TrimSpace(fresh.SchoolClass), schoolClass) {
				return fmt.Errorf("%w: student %d left school class", ErrBulkStudentNotFound, fresh.ID)
			}
			if hasGroup && (fresh.GroupID == nil || *fresh.GroupID != filter.GroupID) {
				return fmt.Errorf("%w: student %d left group", ErrBulkStudentNotFound, fresh.ID)
			}
			if filter.Authorize != nil {
				// Production denials return (false, err); treat either signal as
				// unauthorized so handlers map to 403 instead of 500.
				allowed, authorizeErr := filter.Authorize(txCtx, fresh)
				if authorizeErr != nil || !allowed {
					return fmt.Errorf("%w: student %d", ErrBulkStudentUnauthorized, fresh.ID)
				}
			}
			lockedStudents = append(lockedStudents, fresh)
		}
		students = lockedStudents

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
				if existingRow := existingByWeekday[input.Weekday]; existingRow != nil {
					row.Notes = existingRow.Notes
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
		return nil, &ScheduleError{Op: opBulkUpsertArrivalSchedules, Err: err}
	}

	s.getLogger().Info(
		"bulk upsert arrival schedules",
		slog.String("filter_type", filterType),
		slog.String("filter_value", filterValue),
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

// upsertClassArrivalTimes writes the Unterrichtsschluss of one class. Weekdays
// the request does not mention keep whatever the class had, mirroring the
// "empty fields stay unchanged" contract the bulk screen has always had.
// Existing child rows are deliberately untouched: an own time remains the
// higher-priority deviation until a person resets it explicitly (ADR 0005).
func (s *arrivalScheduleService) upsertClassArrivalTimes(
	ctx context.Context,
	schoolClass string,
	schedules []ArrivalScheduleInput,
	updatedBy int64,
	authorize func(context.Context, *users.Student) (bool, error),
) (*BulkUpsertResult, error) {
	touched := make(map[int]string, len(schedules))
	for _, input := range schedules {
		if input.Weekday < schedule.WeekdayMonday || input.Weekday > schedule.WeekdayFriday {
			return nil, &ScheduleError{Op: opBulkUpsertArrivalSchedules, Err: fmt.Errorf("invalid weekday %d", input.Weekday)}
		}
		if _, duplicate := touched[input.Weekday]; duplicate {
			return nil, &ScheduleError{Op: opBulkUpsertArrivalSchedules, Err: fmt.Errorf("duplicate weekday %d", input.Weekday)}
		}
		touched[input.Weekday] = strings.TrimSpace(input.ArrivalTime)
	}

	students, err := s.studentRepo.FindBySchoolClass(ctx, schoolClass)
	if err != nil {
		return nil, &ScheduleError{
			Op:  opBulkUpsertArrivalSchedules,
			Err: fmt.Errorf("failed to find students for school class %s: %w", schoolClass, err),
		}
	}

	result := &BulkUpsertResult{OverwrittenStudents: make([]OverwriteWarning, 0)}
	tenantID := tenant.FromContext(ctx)
	err = tenant.WithTenantTx(ctx, s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		// A class row affects every matched child, so its authorization boundary
		// is the complete class, not merely the caller's selected filter.
		sort.Slice(students, func(i, j int) bool { return students[i].ID < students[j].ID })
		lockedStudents := make([]*users.Student, 0, len(students))
		for _, selected := range students {
			fresh, lockErr := s.studentRepo.FindByIDForUpdate(txCtx, selected.ID)
			if lockErr != nil {
				if errors.Is(lockErr, sql.ErrNoRows) {
					return fmt.Errorf("%w: student %d", ErrBulkStudentNotFound, selected.ID)
				}
				return fmt.Errorf("lock selected student %d: %w", selected.ID, lockErr)
			}
			if fresh.Status == users.StudentStatusAlumnus || !strings.EqualFold(strings.TrimSpace(fresh.SchoolClass), schoolClass) {
				return fmt.Errorf("%w: student %d", ErrBulkStudentNotFound, fresh.ID)
			}
			if authorize != nil {
				allowed, authorizeErr := authorize(txCtx, fresh)
				if authorizeErr != nil || !allowed {
					return fmt.Errorf("%w: student %d", ErrBulkStudentUnauthorized, fresh.ID)
				}
			}
			lockedStudents = append(lockedStudents, fresh)
		}
		students = lockedStudents

		// The row may not exist yet, so SELECT ... FOR UPDATE cannot serialize
		// all read-modify-write cases. The repository's transaction advisory lock
		// covers concurrent first inserts as well.
		if lockErr := s.classTimes.LockClass(txCtx, schoolClass); lockErr != nil {
			return fmt.Errorf("lock class arrival times for %s: %w", schoolClass, lockErr)
		}
		row, mergeErr := s.mergedClassRow(txCtx, schoolClass, touched, updatedBy)
		if mergeErr != nil {
			return mergeErr
		}
		if upsertErr := s.classTimes.Upsert(txCtx, row); upsertErr != nil {
			return fmt.Errorf("upsert class arrival times for %s: %w", schoolClass, upsertErr)
		}
		return nil
	})
	if err != nil {
		return nil, &ScheduleError{Op: opBulkUpsertArrivalSchedules, Err: err}
	}
	result.StudentsAffected = len(students)
	for _, student := range students {
		result.AffectedStudentIDs = append(result.AffectedStudentIDs, student.ID)
	}
	return result, nil
}

// mergedClassRow folds the touched weekdays into whatever the class already
// carries. An empty time clears that weekday.
func (s *arrivalScheduleService) mergedClassRow(
	ctx context.Context,
	schoolClass string,
	touched map[int]string,
	updatedBy int64,
) (*educationModel.ClassArrivalTime, error) {
	existing, err := s.classTimes.FindByClasses(ctx, []string{schoolClass})
	if err != nil {
		return nil, fmt.Errorf("load class arrival times for %s: %w", schoolClass, err)
	}
	row := &educationModel.ClassArrivalTime{SchoolClass: schoolClass, ArrivalTimes: map[string]string{}}
	if len(existing) > 0 && existing[0] != nil {
		row = existing[0]
		row.SchoolClass = schoolClass
		if row.ArrivalTimes == nil {
			row.ArrivalTimes = map[string]string{}
		}
	}
	for weekday, hhmm := range touched {
		day, ok := educationModel.ISOWeekdayToCanonicalDay(weekday)
		if !ok {
			continue
		}
		if hhmm == "" {
			delete(row.ArrivalTimes, day)
			continue
		}
		row.ArrivalTimes[day] = hhmm
	}
	if updatedBy > 0 {
		row.UpdatedBy = &updatedBy
	}
	normalized, err := educationModel.NormalizeClassArrivalTimes(row.ArrivalTimes)
	if err != nil {
		return nil, err
	}
	row.ArrivalTimes = normalized
	if err := row.Validate(); err != nil {
		return nil, err
	}
	return row, nil
}

// GetClassArrivalTimes returns what a class currently carries, so the
// maintenance screen shows the present state instead of empty fields, and can
// name when it was last touched (ADR 0005, "Bekannte Grenze").
func (s *arrivalScheduleService) GetClassArrivalTimes(
	ctx context.Context,
	schoolClass string,
) (*ClassArrivalTimes, error) {
	class := strings.TrimSpace(schoolClass)
	result := &ClassArrivalTimes{SchoolClass: class, Times: map[string]string{}}
	if class == "" || s.classTimes == nil {
		return result, nil
	}
	rows, err := s.classTimes.FindByClasses(ctx, []string{class})
	if err != nil {
		return nil, &ScheduleError{Op: "get class arrival times", Err: err}
	}
	if len(rows) == 0 || rows[0] == nil {
		return result, nil
	}
	result.SchoolClass = rows[0].SchoolClass
	for day, hhmm := range rows[0].ArrivalTimes {
		result.Times[day] = hhmm
	}
	if !rows[0].UpdatedAt.IsZero() {
		updated := rows[0].UpdatedAt
		result.UpdatedAt = &updated
	}
	return result, nil
}
