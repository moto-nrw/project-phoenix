package schedule

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// PickupScheduleService defines operations for managing student pickup schedules.
// The domain-named contract is intentionally stable because IoT, planning,
// reminders, exports, and messaging consume it directly.
type PickupScheduleService interface {
	GetStudentPickupSchedules(ctx context.Context, studentID int64) ([]*schedule.StudentPickupSchedule, error)
	GetWeeklySchedulesByStudentIDsAndWeekday(ctx context.Context, studentIDs []int64, weekday int) ([]*schedule.StudentPickupSchedule, error)
	// GetWeeklySchedulesByStudentIDs returns every weekday row for the given
	// students in one query (class-roster export, #2290).
	GetWeeklySchedulesByStudentIDs(ctx context.Context, studentIDs []int64) ([]*schedule.StudentPickupSchedule, error)
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
	UpdateException(ctx context.Context, exceptionID, studentID int64, date timezone.Date, reason *string, pickupTime *time.Time, clearPickupTime bool, resolveStaffID func() (int64, error)) (*schedule.StudentPickupException, error)

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
	BulkUpsertPickupSchedules(ctx context.Context, filter ArrivalScheduleBulkFilter, schedules []PickupScheduleInput, createdBy int64) (*BulkUpsertResult, error)
}

type PickupScheduleInput struct {
	Weekday    int    `json:"weekday"`
	PickupTime string `json:"pickup_time"`
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
	scheduleRepo schedule.StudentPickupScheduleRepository
	studentRepo  users.StudentRepository
	personRepo   users.PersonRepository
	db           *bun.DB
	logger       *slog.Logger
}

func newPickupScheduleService(
	scheduleRepo schedule.StudentPickupScheduleRepository,
	exceptionRepo schedule.StudentPickupExceptionRepository,
	noteRepo schedule.StudentPickupNoteRepository,
	db *bun.DB,
) *pickupScheduleService {
	return &pickupScheduleService{
		core: newEffectiveTimeCore(
			scheduleRepo,
			exceptionRepo,
			noteRepo,
			db,
			pickupTimeDomain{},
		),
		scheduleRepo: scheduleRepo,
		db:           db,
	}
}

func NewPickupScheduleServiceWithBulk(
	scheduleRepo schedule.StudentPickupScheduleRepository,
	exceptionRepo schedule.StudentPickupExceptionRepository,
	noteRepo schedule.StudentPickupNoteRepository,
	studentRepo users.StudentRepository,
	personRepo users.PersonRepository,
	db *bun.DB,
	logger *slog.Logger,
) PickupScheduleService {
	service := newPickupScheduleService(scheduleRepo, exceptionRepo, noteRepo, db)
	service.studentRepo = studentRepo
	service.personRepo = personRepo
	service.logger = logger
	return service
}

func (s *pickupScheduleService) BulkUpsertPickupSchedules(
	ctx context.Context,
	filter ArrivalScheduleBulkFilter,
	inputs []PickupScheduleInput,
	createdBy int64,
) (*BulkUpsertResult, error) {
	if len(filter.StudentIDs) == 0 || filter.SchoolClass != "" || filter.GroupID != 0 {
		return nil, &ScheduleError{Op: "bulk upsert pickup schedules", Err: errors.New("student_ids is required")}
	}
	if len(filter.StudentIDs) > 500 {
		return nil, &ScheduleError{Op: "bulk upsert pickup schedules", Err: errors.New("student_ids cannot exceed 500 items")}
	}
	if len(inputs) == 0 {
		return nil, &ScheduleError{Op: "bulk upsert pickup schedules", Err: errors.New("schedules cannot be empty")}
	}
	if s.studentRepo == nil || s.scheduleRepo == nil || s.db == nil {
		return nil, &ScheduleError{Op: "bulk upsert pickup schedules", Err: errors.New("bulk pickup dependencies are not configured")}
	}

	parsed := make(map[int]time.Time, len(inputs))
	for _, input := range inputs {
		if input.Weekday < 1 || input.Weekday > 5 {
			return nil, &ScheduleError{Op: "bulk upsert pickup schedules", Err: fmt.Errorf("invalid weekday %d", input.Weekday)}
		}
		if _, duplicate := parsed[input.Weekday]; duplicate {
			return nil, &ScheduleError{Op: "bulk upsert pickup schedules", Err: fmt.Errorf("duplicate weekday %d", input.Weekday)}
		}
		value, err := time.Parse("2006-01-02 15:04", "2000-01-01 "+input.PickupTime)
		if err != nil {
			return nil, &ScheduleError{Op: "bulk upsert pickup schedules", Err: fmt.Errorf("invalid pickup_time %q for weekday %d: %w", input.PickupTime, input.Weekday, err)}
		}
		parsed[input.Weekday] = value
	}

	byID, err := s.studentRepo.FindByIDs(ctx, filter.StudentIDs)
	if err != nil {
		return nil, &ScheduleError{Op: "bulk upsert pickup schedules", Err: err}
	}
	students := make([]*users.Student, 0, len(filter.StudentIDs))
	for _, id := range filter.StudentIDs {
		student, ok := byID[id]
		if !ok || student.Status == users.StudentStatusAlumnus {
			return nil, &ScheduleError{Op: "bulk upsert pickup schedules", Err: fmt.Errorf("%w: student %d", ErrBulkStudentNotFound, id)}
		}
		students = append(students, student)
	}
	sort.Slice(students, func(i, j int) bool { return students[i].ID < students[j].ID })

	result := &BulkUpsertResult{AffectedStudentIDs: make([]int64, 0, len(students))}
	tenantID := tenant.FromContext(ctx)
	err = tenant.WithTenantTx(ctx, s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		locked := make([]*users.Student, 0, len(students))
		for _, selected := range students {
			fresh, lockErr := s.studentRepo.FindByIDForUpdate(txCtx, selected.ID)
			if errors.Is(lockErr, sql.ErrNoRows) {
				return fmt.Errorf("%w: student %d", ErrBulkStudentNotFound, selected.ID)
			}
			if lockErr != nil {
				return fmt.Errorf("lock student %d: %w", selected.ID, lockErr)
			}
			if fresh.Status == users.StudentStatusAlumnus {
				return fmt.Errorf("%w: student %d", ErrBulkStudentNotFound, selected.ID)
			}
			if filter.Authorize != nil {
				// Production denials return (false, err); treat either signal as
				// unauthorized so handlers map to 403 instead of 500.
				allowed, authorizeErr := filter.Authorize(txCtx, fresh)
				if authorizeErr != nil || !allowed {
					return fmt.Errorf("%w: student %d", ErrBulkStudentUnauthorized, fresh.ID)
				}
			}
			locked = append(locked, fresh)
		}
		for _, student := range locked {
			existing, findErr := s.scheduleRepo.FindByStudentID(txCtx, student.ID)
			if findErr != nil {
				return findErr
			}
			existingByWeekday := make(map[int]*schedule.StudentPickupSchedule, len(existing))
			for _, row := range existing {
				existingByWeekday[row.Weekday] = row
			}
			for _, input := range inputs {
				row := &schedule.StudentPickupSchedule{
					StudentID:  student.ID,
					Weekday:    input.Weekday,
					PickupTime: parsed[input.Weekday],
					CreatedBy:  createdBy,
				}
				if previous := existingByWeekday[input.Weekday]; previous != nil {
					row.Notes = previous.Notes
					if previous.PickupTime.Format("15:04") != row.PickupTime.Format("15:04") {
						studentName, nameErr := s.getPickupStudentName(txCtx, student)
						if nameErr != nil {
							return nameErr
						}
						result.OverwrittenStudents = append(result.OverwrittenStudents, OverwriteWarning{
							StudentID: student.ID, StudentName: studentName, Weekday: input.Weekday,
							WeekdayName:  schedule.WeekdayNames[input.Weekday],
							PreviousTime: previous.PickupTime.Format("15:04"), NewTime: row.PickupTime.Format("15:04"),
						})
					}
				}
				row.SetTenantID(tenantID)
				if upsertErr := s.scheduleRepo.UpsertSchedule(txCtx, row); upsertErr != nil {
					return upsertErr
				}
			}
			result.AffectedStudentIDs = append(result.AffectedStudentIDs, student.ID)
		}
		return nil
	})
	if err != nil {
		return nil, &ScheduleError{Op: "bulk upsert pickup schedules", Err: err}
	}
	result.StudentsAffected = len(result.AffectedStudentIDs)
	if s.logger != nil {
		s.logger.Info(
			"bulk upsert pickup schedules",
			"students_affected", result.StudentsAffected,
			"weekdays_set", len(inputs),
			"overwrites", len(result.OverwrittenStudents),
		)
	}
	return result, nil
}

func (s *pickupScheduleService) getPickupStudentName(ctx context.Context, student *users.Student) (string, error) {
	if s.personRepo == nil {
		return "", errors.New("person repository is not configured")
	}
	person, err := s.personRepo.FindByID(ctx, student.PersonID)
	if err != nil {
		return "", fmt.Errorf("find person for student %d: %w", student.ID, err)
	}
	if person == nil {
		return "", fmt.Errorf("person for student %d is missing", student.ID)
	}
	return person.FirstName + " " + person.LastName, nil
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

func (s *pickupScheduleService) GetWeeklySchedulesByStudentIDs(
	ctx context.Context,
	studentIDs []int64,
) ([]*schedule.StudentPickupSchedule, error) {
	return s.core.WeeklySchedulesByStudentIDs(ctx, studentIDs)
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
	if err := s.preserveOfferingProvenance(ctx, studentID, rows); err != nil {
		return err
	}
	return s.core.UpsertBulkSchedules(ctx, studentID, rows)
}

// preserveOfferingProvenance keeps the Angebots-Gehzeit ownership of weekly
// rows a wholesale replacement does not actually change (#2290): a day whose
// incoming time equals the existing care_offering-sourced row keeps
// source/care_offering_id; a changed time flips to staff ownership (the
// incoming rows' zero-value source).
func (s *pickupScheduleService) preserveOfferingProvenance(
	ctx context.Context,
	studentID int64,
	rows []*schedule.StudentPickupSchedule,
) error {
	if len(rows) == 0 {
		return nil
	}
	existing, err := s.core.Schedules(ctx, studentID)
	if err != nil {
		return err
	}
	sourcedByWeekday := make(map[int]*schedule.StudentPickupSchedule)
	for _, row := range existing {
		if row.Source == schedule.PickupScheduleSourceCareOffering {
			sourcedByWeekday[row.Weekday] = row
		}
	}
	for _, row := range rows {
		if row.Source != "" && row.Source != schedule.PickupScheduleSourceStaff {
			continue
		}
		previous := sourcedByWeekday[row.Weekday]
		if previous == nil {
			continue
		}
		if previous.PickupTime.Format("15:04") == row.PickupTime.Format("15:04") {
			row.Source = schedule.PickupScheduleSourceCareOffering
			row.CareOfferingID = previous.CareOfferingID
		}
	}
	return nil
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

func (s *pickupScheduleService) UpdateException(
	ctx context.Context,
	exceptionID int64,
	studentID int64,
	date timezone.Date,
	reason *string,
	pickupTime *time.Time,
	clearPickupTime bool,
	resolveStaffID func() (int64, error),
) (*schedule.StudentPickupException, error) {
	return s.core.UpdateException(
		ctx,
		exceptionID,
		studentID,
		date,
		reason,
		pickupTime,
		clearPickupTime,
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
