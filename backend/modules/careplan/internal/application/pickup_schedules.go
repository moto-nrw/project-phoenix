package application

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type pickupScheduleService struct {
	*effectiveTimeCore[*careplan.PickupSchedule, *careplan.PickupException, *careplan.PickupNote, PickupTimeDomain]
	scheduleRepo ports.PickupScheduleRepository
	autoExcusal  careplan.PickupAutoExcusal
	baselines    careplan.PickupBaselineReader
	tx           ports.PickupScheduleTransaction
	students     ports.PickupBulkStudents
	logger       *slog.Logger
}

func NewPickupSchedules(schedules ports.PickupScheduleRepository, exceptions ports.EffectiveExceptionRepository[*careplan.PickupException],
	notes ports.EffectiveNoteRepository[*careplan.PickupNote], tx ports.PickupScheduleTransaction,
	baselines careplan.PickupBaselineReader, auto careplan.PickupAutoExcusal,
	students ports.PickupBulkStudents, logger *slog.Logger,
) careplan.PickupScheduleService {
	return &pickupScheduleService{
		effectiveTimeCore: newEffectiveTimeCore(schedules, exceptions, notes, tx, PickupTimeDomain{}),
		scheduleRepo:      schedules, autoExcusal: auto, baselines: baselines, tx: tx,
		students: students, logger: logger,
	}
}

func uniquePickupStudents(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (s *pickupScheduleService) GetStudentPickupSchedules(
	ctx context.Context,
	studentID int64,
) ([]*careplan.PickupSchedule, error) {
	rows, err := s.projectedWeeklySchedules(ctx, []int64{studentID}, calendar.TodayDate())
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *pickupScheduleService) HasBookedOfferingPickupForWeekday(ctx context.Context, studentID int64, weekday int) (bool, error) {
	return s.baselines.HasBookedOfferingPickupForWeekday(ctx, studentID, weekday)
}

func (s *pickupScheduleService) GetWeeklySchedulesByStudentIDsAndWeekday(
	ctx context.Context,
	studentIDs []int64,
	weekday int,
) ([]*careplan.PickupSchedule, error) {
	if weekday < 1 || weekday > 5 {
		return nil, &careplan.ScheduleError{Op: "get student pickup schedules for weekday", Err: errors.New("invalid weekday")}
	}
	rows, err := s.projectedWeeklySchedules(ctx, studentIDs, calendar.TodayDate())
	if err != nil {
		return nil, err
	}
	out := make([]*careplan.PickupSchedule, 0, len(rows))
	for _, row := range rows {
		if row.Weekday == weekday {
			out = append(out, row)
		}
	}
	return out, nil
}

func (s *pickupScheduleService) GetWeeklySchedulesByStudentIDs(
	ctx context.Context,
	studentIDs []int64,
) ([]*careplan.PickupSchedule, error) {
	return s.projectedWeeklySchedules(ctx, studentIDs, calendar.TodayDate())
}

func (s *pickupScheduleService) GetWeeklySchedulesByStudentIDsForDate(
	ctx context.Context,
	studentIDs []int64,
	date calendar.Date,
) ([]*careplan.PickupSchedule, error) {
	return s.projectedWeeklySchedules(ctx, studentIDs, date)
}

func (s *pickupScheduleService) GetStudentPickupScheduleForWeekday(
	ctx context.Context,
	studentID int64,
	weekday int,
) (*careplan.PickupSchedule, error) {
	if weekday < 1 || weekday > 5 {
		return nil, &careplan.ScheduleError{Op: "get student pickup schedule for weekday", Err: errors.New("invalid weekday")}
	}
	rows, err := s.projectedWeeklySchedules(ctx, []int64{studentID}, calendar.TodayDate())
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.Weekday == weekday {
			return row, nil
		}
	}
	return nil, nil
}

func (s *pickupScheduleService) projectedWeeklySchedules(
	ctx context.Context,
	studentIDs []int64,
	date calendar.Date,
) ([]*careplan.PickupSchedule, error) {
	projection, err := s.baselines.Project(ctx, studentIDs, date, date)
	if err != nil {
		return nil, &careplan.ScheduleError{Op: "project student pickup schedules", Err: err}
	}
	rows := make([]*careplan.PickupSchedule, 0, len(studentIDs)*5)
	for _, studentID := range uniquePickupStudents(studentIDs) {
		for _, row := range projection.WeeklyForDate(studentID, date) {
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

func (s *pickupScheduleService) UpsertStudentPickupSchedule(
	ctx context.Context,
	row *careplan.PickupSchedule,
) error {
	if s.autoExcusal == nil {
		return s.upsertSchedule(ctx, row)
	}
	return s.withWeeklyResync(ctx, row.StudentID, func(txCtx context.Context) error {
		return s.upsertSchedule(txCtx, row)
	})
}

func (s *pickupScheduleService) UpsertBulkStudentPickupSchedules(
	ctx context.Context,
	studentID int64,
	rows []*careplan.PickupSchedule,
) error {
	return s.UpsertBulkStudentPickupSchedulesForDate(ctx, studentID, calendar.TodayDate(), rows)
}

func (s *pickupScheduleService) UpsertBulkStudentPickupSchedulesForDate(
	ctx context.Context,
	studentID int64,
	date calendar.Date,
	rows []*careplan.PickupSchedule,
) error {
	return s.tx.WithinTenant(ctx, func(txCtx context.Context) error {
		if err := s.tx.LockStudent(txCtx, studentID); err != nil {
			return err
		}
		manualRows, err := s.manualRowsForReplacement(txCtx, studentID, date, rows)
		if err != nil {
			return err
		}
		if s.autoExcusal == nil {
			return s.upsertBulkSchedules(txCtx, studentID, manualRows)
		}
		before, err := s.autoExcusal.SnapshotWeeklyPickups(txCtx, studentID, date)
		if err != nil {
			return err
		}
		if err := s.upsertBulkSchedules(txCtx, studentID, manualRows); err != nil {
			return err
		}
		if err := s.autoExcusal.ResyncFutureExceptions(txCtx, studentID); err != nil {
			return err
		}
		return s.autoExcusal.RecordWeeklyPickupChanges(txCtx, studentID, date, before)
	})
}

// withWeeklyResync runs a weekly-baseline write in a tenant transaction and
// re-derives the student's auto excusals afterwards (#2360 review): a changed
// or removed weekday Gehzeit re-qualifies or releases the block absences of
// every future day exception. Lock order is student row FIRST, then the
// weekly rows, then the per-day care locks inside the resync — the same
// student-first order all care-day writers use, so a weekly writer cannot
// deadlock against a concurrent exception writer.
func (s *pickupScheduleService) withWeeklyResync(
	ctx context.Context,
	studentID int64,
	write func(txCtx context.Context) error,
) error {
	return s.tx.WithinTenant(ctx, func(txCtx context.Context) error {
		if err := s.tx.LockStudent(txCtx, studentID); err != nil {
			return err
		}
		today := calendar.TodayDate()
		before, err := s.autoExcusal.SnapshotWeeklyPickups(txCtx, studentID, today)
		if err != nil {
			return err
		}
		if err := write(txCtx); err != nil {
			return err
		}
		if err := s.autoExcusal.ResyncFutureExceptions(txCtx, studentID); err != nil {
			return err
		}
		return s.autoExcusal.RecordWeeklyPickupChanges(txCtx, studentID, today, before)
	})
}

// manualRowsForReplacement strips booking-derived values from a wholesale
// replacement. An unchanged projected offering value remains a projection;
// an existing or newly changed staff value remains a stored override.
func (s *pickupScheduleService) manualRowsForReplacement(
	ctx context.Context,
	studentID int64,
	date calendar.Date,
	rows []*careplan.PickupSchedule,
) ([]*careplan.PickupSchedule, error) {
	existing, err := s.scheduleRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, &careplan.ScheduleError{Op: "prepare pickup schedule replacement", Err: err}
	}
	weekStart := date.AddDays(1 - domain.ISOWeekday(date))
	projection, err := s.baselines.Project(ctx, []int64{studentID}, weekStart, weekStart.AddDays(4))
	if err != nil {
		return nil, &careplan.ScheduleError{Op: "prepare pickup schedule replacement", Err: err}
	}
	staffByWeekday := staffPickupRowsByWeekday(existing)
	manual := make([]*careplan.PickupSchedule, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		row.Source = careplan.ScheduleSourceStaff
		row.CareOfferingID = nil
		row.PickupTime = calendar.NormalizeWallClock(row.PickupTime)
		rowDate := weekStart.AddDays(row.Weekday - 1)
		if !projection.AllowsPickupForDate(studentID, rowDate) {
			continue
		}
		if shouldStoreManualPickup(row, staffByWeekday[row.Weekday], projection.OfferingForDate(studentID, rowDate)) {
			manual = append(manual, row)
		}
	}
	return preserveInactivePickupRows(studentID, weekStart, manual, staffByWeekday, projection), nil
}

func preserveInactivePickupRows(
	studentID int64,
	weekStart calendar.Date,
	manual []*careplan.PickupSchedule,
	staffByWeekday map[int]*careplan.PickupSchedule,
	projection *careplan.PickupBaselineProjection,
) []*careplan.PickupSchedule {
	for weekday, row := range staffByWeekday {
		rowDate := weekStart.AddDays(weekday - 1)
		if !projection.AllowsPickupForDate(studentID, rowDate) && !containsPickupWeekday(manual, weekday) {
			manual = append(manual, pickupRowForRewrite(row))
		}
	}
	return manual
}

func pickupRowForRewrite(row *careplan.PickupSchedule) *careplan.PickupSchedule {
	return &careplan.PickupSchedule{
		StudentID:      row.StudentID,
		Weekday:        row.Weekday,
		PickupTime:     calendar.NormalizeWallClock(row.PickupTime),
		Notes:          row.Notes,
		CreatedBy:      row.CreatedBy,
		Source:         row.Source,
		CareOfferingID: row.CareOfferingID,
	}
}

func staffPickupRowsByWeekday(rows []*careplan.PickupSchedule) map[int]*careplan.PickupSchedule {
	out := make(map[int]*careplan.PickupSchedule)
	for _, row := range rows {
		if row != nil && row.Source != careplan.ScheduleSourceCareOffering {
			out[row.Weekday] = row
		}
	}
	return out
}

func shouldStoreManualPickup(
	row, previous, projected *careplan.PickupSchedule,
) bool {
	if previous != nil && samePickupMinute(previous, row) && equalOptionalPickupNotes(previous.Notes, row.Notes) {
		return true
	}
	return projected == nil || !samePickupMinute(projected, row) || !emptyOptionalPickupNotes(row.Notes)
}

func samePickupMinute(left, right *careplan.PickupSchedule) bool {
	return left.PickupTime.Format("15:04") == right.PickupTime.Format("15:04")
}

func equalOptionalPickupNotes(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func emptyOptionalPickupNotes(notes *string) bool {
	return notes == nil || strings.TrimSpace(*notes) == ""
}

func (s *pickupScheduleService) DeleteStudentPickupSchedule(
	ctx context.Context,
	scheduleID int64,
) error {
	if scheduleID <= 0 {
		return &careplan.ScheduleError{Op: "delete student pickup schedule", Err: errors.New("schedule id must be positive")}
	}
	if s.autoExcusal == nil {
		return s.deleteSchedule(ctx, scheduleID)
	}
	row, err := s.scheduleRepo.FindByID(ctx, scheduleID)
	if err != nil && !s.tx.IsNotFound(err) {
		return &careplan.ScheduleError{Op: "delete student pickup schedule", Err: err}
	}
	if row == nil {
		return s.deleteSchedule(ctx, scheduleID)
	}
	return s.withWeeklyResync(ctx, row.StudentID, func(txCtx context.Context) error {
		return s.deleteSchedule(txCtx, scheduleID)
	})
}

func (s *pickupScheduleService) DeleteAllStudentPickupSchedules(
	ctx context.Context,
	studentID int64,
) error {
	if s.autoExcusal == nil {
		return s.deleteAllSchedules(ctx, studentID)
	}
	return s.withWeeklyResync(ctx, studentID, func(txCtx context.Context) error {
		return s.deleteAllSchedules(txCtx, studentID)
	})
}

func (s *pickupScheduleService) GetStudentPickupNoteByID(
	ctx context.Context,
	noteID int64,
) (*careplan.PickupNote, error) {
	return s.noteByID(ctx, noteID)
}

func (s *pickupScheduleService) GetStudentPickupNotes(
	ctx context.Context,
	studentID int64,
) ([]*careplan.PickupNote, error) {
	return s.loadNotes(ctx, studentID)
}

func (s *pickupScheduleService) GetStudentPickupNotesForDate(
	ctx context.Context,
	studentID int64,
	date calendar.Date,
) ([]*careplan.PickupNote, error) {
	return s.notesForDate(ctx, studentID, date)
}

func (s *pickupScheduleService) CreateStudentPickupNote(
	ctx context.Context,
	row *careplan.PickupNote,
) error {
	return s.createNote(ctx, row)
}

func (s *pickupScheduleService) UpdateStudentPickupNote(
	ctx context.Context,
	row *careplan.PickupNote,
) error {
	return s.updateNote(ctx, row)
}

func (s *pickupScheduleService) DeleteStudentPickupNote(
	ctx context.Context,
	noteID int64,
) error {
	return s.deleteNote(ctx, noteID)
}

func (s *pickupScheduleService) DeleteAllStudentPickupNotes(
	ctx context.Context,
	studentID int64,
) error {
	return s.deleteAllNotes(ctx, studentID)
}

func (s *pickupScheduleService) GetStudentPickupData(
	ctx context.Context,
	studentID int64,
) (*careplan.StudentPickupData, error) {
	coreData, err := s.data(ctx, studentID)
	if err != nil {
		return nil, err
	}
	data := &careplan.StudentPickupData{Exceptions: coreData.Exceptions, Notes: coreData.Notes}
	schedules, err := s.GetStudentPickupSchedules(ctx, studentID)
	if err != nil {
		return nil, err
	}
	return &careplan.StudentPickupData{
		Schedules:  schedules,
		Exceptions: data.Exceptions,
		Notes:      data.Notes,
	}, nil
}

func (s *pickupScheduleService) GetStudentPickupDataForRange(
	ctx context.Context,
	studentID int64,
	from, to calendar.Date,
) (*careplan.StudentPickupData, error) {
	coreData, err := s.data(ctx, studentID)
	if err != nil {
		return nil, err
	}
	data := &careplan.StudentPickupData{Exceptions: coreData.Exceptions, Notes: coreData.Notes}
	projection, err := s.baselines.Project(ctx, []int64{studentID}, from, to)
	if err != nil {
		return nil, &careplan.ScheduleError{Op: "get dated student pickup data", Err: err}
	}
	for date := from; !date.After(to); date = date.AddDays(1) {
		schedule := projection.ForDate(studentID, date)
		if schedule != nil && !containsPickupWeekday(data.Schedules, schedule.Weekday) {
			data.Schedules = append(data.Schedules, schedule)
		}
		data.EffectiveSchedules = append(data.EffectiveSchedules, careplan.DatedPickupSchedule{
			Date:             date,
			Schedule:         schedule,
			OfferingSchedule: projection.OfferingForDate(studentID, date),
		})
	}
	return data, nil
}

func containsPickupWeekday(schedules []*careplan.PickupSchedule, weekday int) bool {
	for _, row := range schedules {
		if row != nil && row.Weekday == weekday {
			return true
		}
	}
	return false
}

func (s *pickupScheduleService) GetEffectivePickupTimeForDate(
	ctx context.Context,
	studentID int64,
	date calendar.Date,
) (*careplan.EffectivePickupTime, error) {
	results, err := s.GetBulkEffectivePickupTimesForDate(ctx, []int64{studentID}, date)
	if err != nil {
		return nil, err
	}
	return results[studentID], nil
}

func (s *pickupScheduleService) GetBulkEffectivePickupTimesForDate(
	ctx context.Context,
	studentIDs []int64,
	date calendar.Date,
) (map[int64]*careplan.EffectivePickupTime, error) {
	projection, err := s.baselines.Project(ctx, studentIDs, date, date)
	if err != nil {
		return nil, &careplan.ScheduleError{Op: "get bulk effective pickup times", Err: err}
	}
	schedules := make(map[int64]*careplan.PickupSchedule, len(studentIDs))
	for _, studentID := range studentIDs {
		if !projection.AllowsPickupForDate(studentID, date) {
			continue
		}
		if row := projection.ForDate(studentID, date); row != nil {
			schedules[studentID] = row
		}
	}
	results, err := s.bulkEffectiveTimesForDateWithSchedules(ctx, studentIDs, date, schedules)
	if err != nil {
		return nil, err
	}
	mapped := make(map[int64]*careplan.EffectivePickupTime, len(results))
	for studentID, result := range results {
		if !projection.AllowsPickupForDate(studentID, date) {
			// Outside a booked care day only the day's notes survive (#3369):
			// the child stays not expected, the note still reaches the tile.
			if len(result.DayNotes) > 0 {
				mapped[studentID] = pickupEffectiveTime(&domain.EffectiveTimeResult{Date: result.Date, WeekdayName: result.WeekdayName, DayNotes: result.DayNotes})
			}
			continue
		}
		mapped[studentID] = pickupEffectiveTime(result)
	}
	return mapped, nil
}

func pickupEffectiveTime(result *domain.EffectiveTimeResult) *careplan.EffectivePickupTime {
	mapped := &careplan.EffectivePickupTime{
		Date:              result.Date,
		PickupTime:        result.Time,
		WeekdayName:       result.WeekdayName,
		IsException:       result.IsException,
		Notes:             result.Notes,
		RegularPickupTime: result.RegularTime,
		ChangedAt:         result.ChangedAt,
	}
	for _, note := range result.DayNotes {
		mapped.DayNotes = append(mapped.DayNotes, careplan.NoteData(note))
	}
	return mapped
}
