package application

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type arrivalScheduleService struct {
	careplan.ClassArrivalExceptions
	*effectiveTimeCore[*careplan.ArrivalSchedule, *careplan.ArrivalException, *careplan.ArrivalNote, ArrivalTimeDomain]
	scheduleRepo ports.ArrivalScheduleRepository
	baselines    careplan.ArrivalBaselineReader
	rules        ports.ArrivalScheduleRules
	tx           ports.EffectiveTimeTransaction
	students     ports.ArrivalBulkStudents
	classes      ports.ArrivalClassPlans
	logger       *slog.Logger
}

func NewArrivalSchedules(schedules ports.ArrivalScheduleRepository, exceptions ports.EffectiveExceptionRepository[*careplan.ArrivalException],
	notes ports.EffectiveNoteRepository[*careplan.ArrivalNote], baselines careplan.ArrivalBaselineReader, rules ports.ArrivalScheduleRules, tx ports.EffectiveTimeTransaction, students ports.ArrivalBulkStudents, classes ports.ArrivalClassPlans, classExceptions careplan.ClassArrivalExceptions, logger *slog.Logger) careplan.ArrivalScheduleService {
	if classExceptions == nil {
		classExceptions = NewClassArrivalExceptions(nil, nil)
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &arrivalScheduleService{effectiveTimeCore: newEffectiveTimeCore(schedules, exceptions, notes, tx, ArrivalTimeDomain{}),
		scheduleRepo: schedules, baselines: baselines, rules: rules, tx: tx, students: students, classes: classes, ClassArrivalExceptions: classExceptions, logger: logger}
}

// projectedWeeklySchedules flattens the projected week into the sorted row
// list the weekly readers expect.
func (s *arrivalScheduleService) projectedWeeklySchedules(
	ctx context.Context,
	studentIDs []int64,
	date timezone.Date,
) ([]*careplan.ArrivalSchedule, error) {
	week, err := s.projectedWeek(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	rows := make([]*careplan.ArrivalSchedule, 0, len(studentIDs)*5)
	for _, studentID := range uniquePickupStudents(studentIDs) {
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
) ([]*careplan.ArrivalSchedule, error) {
	if s.baselines == nil {
		return s.projectedWeeklySchedules(ctx, studentIDs, from)
	}
	projection, err := s.baselines.Project(ctx, studentIDs, from, to)
	if err != nil {
		return nil, &careplan.ScheduleError{Op: "project arrival schedule week", Err: err}
	}
	rows := make([]*careplan.ArrivalSchedule, 0, len(studentIDs)*5)
	for _, studentID := range uniquePickupStudents(studentIDs) {
		for date := from; !date.After(to); date = date.AddDays(1) {
			if row := projection.WeeklyForDate(studentID, date)[domain.ISOWeekday(date)]; row != nil {
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
) (map[int64]careplan.ArrivalWeek, error) {
	out := make(map[int64]careplan.ArrivalWeek, len(studentIDs))
	if s.baselines == nil {
		for _, studentID := range studentIDs {
			rows, err := s.loadSchedules(ctx, studentID)
			if err != nil {
				return nil, err
			}
			week := make(careplan.ArrivalWeek, len(rows))
			for _, row := range rows {
				week[row.Weekday] = row
			}
			out[studentID] = week
		}
		return out, nil
	}
	projection, err := s.baselines.Project(ctx, studentIDs, date, date)
	if err != nil {
		return nil, &careplan.ScheduleError{Op: "project arrival schedules", Err: err}
	}
	for _, studentID := range studentIDs {
		out[studentID] = projection.WeeklyForDate(studentID, date)
	}
	return out, nil
}

func (s *arrivalScheduleService) GetStudentArrivalSchedules(
	ctx context.Context,
	studentID int64,
) ([]*careplan.ArrivalSchedule, error) {
	from := weekStart(timezone.TodayDate())
	return s.projectedWeekRange(ctx, []int64{studentID}, from, from.AddDays(4))
}

func (s *arrivalScheduleService) GetWeeklySchedulesByStudentIDsAndWeekday(
	ctx context.Context,
	studentIDs []int64,
	weekday int,
) ([]*careplan.ArrivalSchedule, error) {
	if weekday < 1 || weekday > 5 {
		return nil, nil
	}
	date := weekStart(timezone.TodayDate()).AddDays(weekday - 1)
	rows, err := s.projectedWeeklySchedules(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	filtered := make([]*careplan.ArrivalSchedule, 0, len(rows))
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
) ([]*careplan.ArrivalSchedule, error) {
	return s.projectedWeeklySchedules(ctx, studentIDs, date)
}

func (s *arrivalScheduleService) GetStudentArrivalScheduleForWeekday(
	ctx context.Context,
	studentID int64,
	weekday int,
) (*careplan.ArrivalSchedule, error) {
	// The weekday validation stays with the core so an invalid weekday keeps
	// erroring rather than reading as "no care that day".
	stored, err := s.scheduleForWeekday(ctx, studentID, weekday)
	if err != nil && !s.tx.IsNotFound(err) {
		return stored, err
	}
	if s.baselines == nil {
		return stored, err
	}
	date := weekStart(timezone.TodayDate()).AddDays(weekday - 1)
	week, err := s.projectedWeek(ctx, []int64{studentID}, date)
	if err != nil {
		return nil, err
	}
	return week[studentID][weekday], nil
}

func (s *arrivalScheduleService) UpsertStudentArrivalSchedule(
	ctx context.Context,
	row *careplan.ArrivalSchedule,
) error {
	return s.tx.WithinTenant(ctx, func(txCtx context.Context) error {
		if err := s.lockArrivalStudent(txCtx, row.StudentID); err != nil {
			return err
		}
		return s.upsertSchedule(txCtx, row)
	})
}

func (s *arrivalScheduleService) UpsertBulkStudentArrivalSchedules(
	ctx context.Context,
	studentID int64,
	rows []*careplan.ArrivalSchedule,
) error {
	return s.tx.WithinTenant(ctx, func(txCtx context.Context) error {
		if err := s.lockArrivalStudent(txCtx, studentID); err != nil {
			return err
		}
		if err := s.collapseIntoClassTime(txCtx, studentID, rows); err != nil {
			return err
		}
		preserved, err := s.preserveInactiveBookingRows(txCtx, studentID, rows)
		if err != nil {
			return err
		}
		return s.upsertBulkSchedules(txCtx, studentID, preserved)
	})
}

func (s *arrivalScheduleService) lockArrivalStudent(ctx context.Context, studentID int64) error {
	if s.rules == nil {
		return &careplan.ScheduleError{Op: "lock arrival schedule student", Err: errors.New("student repository is not configured")}
	}
	if err := s.rules.LockStudent(ctx, studentID); err != nil {
		return &careplan.ScheduleError{Op: "lock arrival schedule student", Err: err}
	}
	return nil
}

// preserveInactiveBookingRows keeps omitted manual rows while bookings define
// the active care days. Disabling booking authority later must restore the
// school's complete weekly plan, including rows currently ignored by bookings.
func (s *arrivalScheduleService) preserveInactiveBookingRows(
	ctx context.Context,
	studentID int64,
	rows []*careplan.ArrivalSchedule,
) ([]*careplan.ArrivalSchedule, error) {
	if s.baselines == nil {
		return rows, nil
	}
	today := timezone.TodayDate()
	projection, err := s.baselines.Project(ctx, []int64{studentID}, today, today)
	if err != nil {
		return nil, &careplan.ScheduleError{Op: "preserve inactive booking arrival rows", Err: err}
	}
	if projection == nil || !projection.BookingsAuthoritative {
		return rows, nil
	}
	stored, err := s.scheduleRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &careplan.ScheduleError{Op: "preserve inactive booking arrival rows", Err: err}
	}
	incoming := make(map[int]bool, len(rows))
	merged := append(make([]*careplan.ArrivalSchedule, 0, len(rows)+len(stored)), rows...)
	for _, row := range rows {
		if row != nil {
			incoming[row.Weekday] = true
		}
	}
	for _, row := range stored {
		if row != nil && !incoming[row.Weekday] {
			preserved := *row
			if !preserved.ExpectedArrival.IsZero() {
				preserved.ExpectedArrival = timezone.NormalizeWallClock(preserved.ExpectedArrival)
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
func (s *arrivalScheduleService) collapseIntoClassTime(ctx context.Context, studentID int64, rows []*careplan.ArrivalSchedule) error {
	if s.rules == nil || len(rows) == 0 {
		return nil
	}
	times, err := s.rules.ClassTimesForStudent(ctx, studentID)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row == nil || row.ExpectedArrival.IsZero() {
			continue
		}
		classTime, ok := times[row.Weekday]
		if ok && row.ExpectedArrival.Format("15:04") == classTime.Format("15:04") {
			row.ExpectedArrival = time.Time{}
		}
	}
	return nil
}

func (s *arrivalScheduleService) DeleteStudentArrivalSchedule(
	ctx context.Context,
	scheduleID int64,
) error {
	return s.tx.WithinTenant(ctx, func(txCtx context.Context) error {
		row, err := s.scheduleRepo.FindByID(txCtx, scheduleID)
		if s.tx.IsNotFound(err) || row == nil {
			return s.deleteSchedule(txCtx, scheduleID)
		}
		if err != nil {
			return &careplan.ScheduleError{Op: "find arrival schedule student", Err: err}
		}
		if err := s.lockArrivalStudent(txCtx, row.StudentID); err != nil {
			return err
		}
		return s.deleteSchedule(txCtx, scheduleID)
	})
}

func (s *arrivalScheduleService) DeleteAllStudentArrivalSchedules(
	ctx context.Context,
	studentID int64,
) error {
	return s.tx.WithinTenant(ctx, func(txCtx context.Context) error {
		if err := s.lockArrivalStudent(txCtx, studentID); err != nil {
			return err
		}
		return s.deleteAllSchedules(txCtx, studentID)
	})
}

func (s *arrivalScheduleService) GetStudentArrivalExceptionByID(
	ctx context.Context,
	exceptionID int64,
) (*careplan.ArrivalException, error) {
	return s.exceptionByID(ctx, exceptionID)
}

func (s *arrivalScheduleService) GetStudentArrivalExceptionForDate(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
) (*careplan.ArrivalException, error) {
	return s.exceptionForDate(ctx, studentID, date)
}

func (s *arrivalScheduleService) GetStudentArrivalExceptions(
	ctx context.Context,
	studentID int64,
) ([]*careplan.ArrivalException, error) {
	return s.loadExceptions(ctx, studentID)
}

func (s *arrivalScheduleService) GetUpcomingStudentArrivalExceptions(
	ctx context.Context,
	studentID int64,
) ([]*careplan.ArrivalException, error) {
	return s.upcomingExceptions(ctx, studentID)
}

func (s *arrivalScheduleService) CreateStudentArrivalException(
	ctx context.Context,
	row *careplan.ArrivalException,
) error {
	return s.createException(ctx, row)
}

func (s *arrivalScheduleService) UpdateStudentArrivalException(
	ctx context.Context,
	row *careplan.ArrivalException,
) error {
	return s.updateExceptionRow(ctx, row)
}

func (s *arrivalScheduleService) CreateOrReclaimException(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
	arrivalTime *time.Time,
	reason *string,
	staffID int64,
	resolveStaffID func() (int64, error),
) (*careplan.ArrivalException, error) {
	return s.createOrReclaimException(
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
) (*careplan.ArrivalException, error) {
	return s.updateException(
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
	exceptionID, studentID int64,
) error {
	return s.deleteException(ctx, exceptionID, studentID)
}

func (s *arrivalScheduleService) DeleteAllStudentArrivalExceptions(
	ctx context.Context,
	studentID int64,
) error {
	return s.deleteAllExceptions(ctx, studentID)
}

func (s *arrivalScheduleService) GetStudentArrivalNoteByID(
	ctx context.Context,
	noteID int64,
) (*careplan.ArrivalNote, error) {
	return s.noteByID(ctx, noteID)
}

func (s *arrivalScheduleService) GetStudentArrivalNotes(
	ctx context.Context,
	studentID int64,
) ([]*careplan.ArrivalNote, error) {
	return s.loadNotes(ctx, studentID)
}

func (s *arrivalScheduleService) GetStudentArrivalNotesForDate(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
) ([]*careplan.ArrivalNote, error) {
	return s.notesForDate(ctx, studentID, date)
}

func (s *arrivalScheduleService) CreateStudentArrivalNote(
	ctx context.Context,
	row *careplan.ArrivalNote,
) error {
	return s.createNote(ctx, row)
}

func (s *arrivalScheduleService) UpdateStudentArrivalNote(
	ctx context.Context,
	row *careplan.ArrivalNote,
) error {
	return s.updateNote(ctx, row)
}

func (s *arrivalScheduleService) DeleteStudentArrivalNote(
	ctx context.Context,
	noteID int64,
) error {
	return s.deleteNote(ctx, noteID)
}

func (s *arrivalScheduleService) DeleteAllStudentArrivalNotes(
	ctx context.Context,
	studentID int64,
) error {
	return s.deleteAllNotes(ctx, studentID)
}

func (s *arrivalScheduleService) GetStudentArrivalData(
	ctx context.Context,
	studentID int64,
) (*careplan.StudentArrivalData, error) {
	from := weekStart(timezone.TodayDate())
	return s.GetStudentArrivalDataForDateRange(ctx, studentID, from, from.AddDays(4))
}

func (s *arrivalScheduleService) GetStudentArrivalDataForDate(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
) (*careplan.StudentArrivalData, error) {
	data, err := s.data(ctx, studentID)
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
	return &careplan.StudentArrivalData{
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
) (*careplan.StudentArrivalData, error) {
	data, err := s.data(ctx, studentID)
	if err != nil {
		return nil, err
	}
	if to.Before(from) {
		return nil, &careplan.ScheduleError{Op: "project arrival schedule range", Err: errors.New("arrival schedule range ends before it starts")}
	}
	if s.baselines == nil {
		return s.GetStudentArrivalDataForDate(ctx, studentID, from)
	}
	projection, err := s.baselines.Project(ctx, []int64{studentID}, from, to)
	if err != nil {
		return nil, &careplan.ScheduleError{Op: "project arrival schedule range", Err: err}
	}
	byWeekday := make(map[int]*careplan.ArrivalSchedule, 5)
	for date := from; !date.After(to); date = date.AddDays(1) {
		weekday := domain.ISOWeekday(date)
		// A range may include the same weekday twice. The latest date wins, so
		// a booking that has ended cannot keep an earlier care-day marker alive.
		delete(byWeekday, weekday)
		if row := projection.WeeklyForDate(studentID, date)[weekday]; row != nil {
			byWeekday[weekday] = row
		}
	}
	schedules := make([]*careplan.ArrivalSchedule, 0, len(byWeekday))
	for weekday := 1; weekday <= 5; weekday++ {
		if row := byWeekday[weekday]; row != nil {
			schedules = append(schedules, row)
		}
	}
	return &careplan.StudentArrivalData{
		Schedules:  schedules,
		Exceptions: data.Exceptions,
		Notes:      data.Notes,
	}, nil
}

// GetStudentsWithStoredArrivalSchedules identifies children with a stored own
// arrival time. Care-day markers that inherit their class time are excluded.
func (s *arrivalScheduleService) GetStudentsWithStoredArrivalSchedules(
	ctx context.Context,
	studentIDs []int64,
) (map[int64]bool, error) {
	rows, err := s.scheduleRepo.FindByStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, &careplan.ScheduleError{Op: "list stored arrival schedules", Err: err}
	}
	hasSchedules := make(map[int64]bool, len(rows))
	for _, row := range rows {
		if !row.ExpectedArrival.IsZero() {
			hasSchedules[row.StudentID] = true
		}
	}
	return hasSchedules, nil
}

func (s *arrivalScheduleService) GetEffectiveArrivalTimeForDate(
	ctx context.Context,
	studentID int64,
	date timezone.Date,
) (*careplan.EffectiveArrivalTime, error) {
	if s.baselines == nil {
		result, err := s.effectiveTimeForDate(ctx, studentID, date)
		if err != nil {
			return nil, err
		}
		return arrivalEffectiveTime(result), nil
	}
	projection, err := s.baselines.Project(ctx, []int64{studentID}, date, date)
	if err != nil {
		return nil, &careplan.ScheduleError{Op: "get effective arrival time", Err: err}
	}
	scheduleRow := projection.ForDate(studentID, date)
	if projection.BookingsAuthoritative && scheduleRow == nil {
		return &careplan.EffectiveArrivalTime{
			Date:        date,
			WeekdayName: domain.WeekdayName(domain.ISOWeekday(date)),
		}, nil
	}
	result, err := s.effectiveTimeForDateWithSchedule(ctx, studentID, date, scheduleRow)
	if err != nil {
		return nil, err
	}
	mapped := arrivalEffectiveTime(result)
	attachClassException(mapped, scheduleRow)
	return mapped, nil
}

// bulkEffectiveResults resolves the recurring rows through the projection
// before the shared exception/note merge runs, mirroring the pickup path.
func (s *arrivalScheduleService) bulkEffectiveResults(
	ctx context.Context,
	studentIDs []int64,
	date timezone.Date,
) (map[int64]*domain.EffectiveTimeResult, map[int64]*careplan.ArrivalSchedule, error) {
	if s.baselines == nil {
		results, err := s.bulkEffectiveTimesForDate(ctx, studentIDs, date)
		return results, nil, err
	}
	projection, err := s.baselines.Project(ctx, studentIDs, date, date)
	if err != nil {
		return nil, nil, &careplan.ScheduleError{Op: "get bulk effective arrival times", Err: err}
	}
	schedules := make(map[int64]*careplan.ArrivalSchedule, len(studentIDs))
	for _, studentID := range studentIDs {
		if row := projection.ForDate(studentID, date); row != nil {
			schedules[studentID] = row
		}
	}
	results, err := s.bulkEffectiveTimesForDateWithSchedules(ctx, studentIDs, date, schedules)
	if err != nil {
		return nil, nil, err
	}
	for _, studentID := range studentIDs {
		if projection.BookingsAuthoritative && schedules[studentID] == nil {
			results[studentID] = &domain.EffectiveTimeResult{
				Date:        date,
				WeekdayName: domain.WeekdayName(domain.ISOWeekday(date)),
			}
		}
	}
	return results, schedules, nil
}

func (s *arrivalScheduleService) GetBulkEffectiveArrivalTimesForDate(
	ctx context.Context,
	studentIDs []int64,
	date timezone.Date,
) (map[int64]*careplan.EffectiveArrivalTime, error) {
	results, schedules, err := s.bulkEffectiveResults(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	mapped := make(map[int64]*careplan.EffectiveArrivalTime, len(results))
	for studentID, result := range results {
		effective := arrivalEffectiveTime(result)
		attachClassException(effective, schedules[studentID])
		mapped[studentID] = effective
	}
	return mapped, nil
}

func arrivalEffectiveTime(result *domain.EffectiveTimeResult) *careplan.EffectiveArrivalTime {
	mapped := &careplan.EffectiveArrivalTime{
		Date:        result.Date,
		ArrivalTime: result.Time,
		WeekdayName: result.WeekdayName,
		IsException: result.IsException,
		Notes:       result.Notes,
		ChangedAt:   result.ChangedAt,
	}
	for _, note := range result.DayNotes {
		mapped.DayNotes = append(mapped.DayNotes, careplan.ArrivalNoteData(note))
	}
	return mapped
}

// attachClassException copies the class-wide day exception behind a projected
// row onto the effective time (#2962). A per-child day exception overrides
// the class one, so nothing is attached then.
func attachClassException(effective *careplan.EffectiveArrivalTime, row *careplan.ArrivalSchedule) {
	if effective == nil || row == nil || effective.IsException || effective.ArrivalTime == nil {
		return
	}
	if row.Source != careplan.ScheduleSourceClassException {
		return
	}
	effective.ClassException = &careplan.ClassArrivalExceptionInfo{
		SchoolClass: row.SourceClass,
		ArrivalTime: effective.ArrivalTime.Format("15:04"),
		Label:       row.SourceLabel,
	}
}
