package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/ports"
)

var ErrDisabled = errors.New("meal plan is disabled")
var (
	ErrRegistrationDisabled = errors.New("meal registration is disabled")
	ErrInvalidCutoff        = errors.New("meal registration cutoff is invalid")
	ErrCutoffPassed         = errors.New("meal registration cutoff has passed")
	// ErrPastDay refuses the school a change of a day already over.
	ErrPastDay = errors.New("meal registration day is in the past")
	// ErrDayClosed refuses a change of a day the school is closed.
	ErrDayClosed = errors.New("meal registration day is closed")
	// ErrStudentNotFound: the school does not know the child.
	ErrStudentNotFound = errors.New("meal registration child not found")
	// ErrNotEnrolled: the child is not in care on the day.
	ErrNotEnrolled = errors.New("meal registration child not in care on the day")
	// ErrTooFarAhead refuses regular days that start too far ahead.
	ErrTooFarAhead = errors.New("meal registration starts too far ahead")
)

// maxScheduleLeadDays is how far ahead the school may let new regular days
// start: one year.
const maxScheduleLeadDays = 365

type Service struct {
	store     ports.Store
	directory ports.Directory
	calendar  ports.Calendar
	settings  ports.Settings
	tx        ports.Transaction
	observe   ports.Observer
	now       func() time.Time
}

func New(store ports.Store, directory ports.Directory, calendar ports.Calendar, settings ports.Settings, tx ports.Transaction, observe ports.Observer, now func() time.Time) *Service {
	if store == nil || directory == nil || calendar == nil || settings == nil || tx == nil || observe == nil || now == nil {
		panic("meal plan application: all dependencies are required")
	}
	return &Service{store: store, directory: directory, calendar: calendar, settings: settings, tx: tx, observe: observe, now: now}
}

func (s *Service) registrationAvailable(ctx context.Context) (available bool, err error) {
	err = s.run(ctx, "registration_availability", func(txCtx context.Context) (domain.OperationStats, error) {
		if err = s.requireEnabled(txCtx); err != nil {
			return domain.OperationStats{}, err
		}
		available, err = s.settings.MealRegistrationEnabled(txCtx)
		return domain.OperationStats{}, err
	})
	return available, err
}

func (s *Service) participationPlan(ctx context.Context, studentID int64, from, to domain.Date) (plan domain.ParticipationPlan, err error) {
	err = s.run(ctx, "read_participation", func(txCtx context.Context) (domain.OperationStats, error) {
		rules, rulesErr := s.requireRegistration(txCtx)
		if rulesErr != nil {
			return domain.OperationStats{}, rulesErr
		}
		var stats domain.OperationStats
		plan, stats, err = s.readParticipation(txCtx, studentID, from, to, rules)
		return stats, err
	})
	return plan, err
}

// replaceSchedule replaces the regular days. A parent's change
// takes effect on the first day whose order is not fixed yet; the school's
// (schoolFrom set) on the day it names, from today up to a year ahead
// (#3873).
func (s *Service) replaceSchedule(ctx context.Context, studentID, accountID int64, weekdays []domain.Weekday, schoolFrom domain.Date) (effectiveFrom domain.Date, err error) {
	err = s.run(ctx, "replace_participation_schedule", func(txCtx context.Context) (domain.OperationStats, error) {
		rules, rulesErr := s.requireRegistration(txCtx)
		if rulesErr != nil {
			return domain.OperationStats{}, rulesErr
		}
		var stats domain.OperationStats
		var fromErr error
		effectiveFrom, stats, fromErr = s.scheduleStart(txCtx, rules, studentID, schoolFrom)
		if fromErr != nil {
			return stats, fromErr
		}
		// A new pattern can change every day it reaches.
		today := domain.DateFromInstant(s.now())
		change := domain.RegistrationChange{StudentID: studentID, AccountID: accountID, BySchool: schoolFrom != "", Kind: domain.RegistrationSchedule, Date: effectiveFrom, Weekdays: weekdays}
		writeStats, err := s.settleWrite(txCtx, rules, today, today.AddDays(orderReachDays), changeAuthor{studentID: studentID, accountID: accountID}, func() (domain.OperationStats, error) {
			saveStats, saveErr := s.store.InsertParticipationSchedule(txCtx, studentID, accountID, effectiveFrom, weekdays)
			if saveErr != nil {
				return saveStats, saveErr
			}
			logStats, logErr := s.store.InsertRegistrationChanges(txCtx, []domain.RegistrationChange{change})
			return addStats(saveStats, logStats), logErr
		})
		return addStats(stats, writeStats), err
	})
	return effectiveFrom, err
}

// scheduleStart is the first day of new regular days: for a parent the
// first day whose order is not fixed, for the school the day it names.
func (s *Service) scheduleStart(ctx context.Context, rules domain.OrderingRules, studentID int64, schoolFrom domain.Date) (domain.Date, domain.OperationStats, error) {
	now := s.now()
	today := domain.DateFromInstant(now)
	switch {
	case schoolFrom == "" && !rules.ParentCanChangeSchedule(now):
		return "", domain.OperationStats{}, ErrCutoffPassed
	case schoolFrom == "":
		return rules.NextEffectiveDate(now), domain.OperationStats{}, nil
	case schoolFrom.Before(today):
		return "", domain.OperationStats{}, ErrPastDay
	case schoolFrom.After(today.AddDays(maxScheduleLeadDays)):
		return "", domain.OperationStats{}, ErrTooFarAhead
	}
	names, stats, err := s.directory.FindStudentNames(ctx, []int64{studentID})
	if _, known := names[studentID]; err == nil && !known {
		err = ErrStudentNotFound
	}
	return schoolFrom, stats, err
}

// requireChangeableDay checks that a day can change at all: the school is
// open, and for the school's change the child is in care that day.
func (s *Service) requireChangeableDay(ctx context.Context, studentID int64, date domain.Date, bySchool bool) (domain.OperationStats, error) {
	days, stats, err := s.calendar.Days(ctx, date, date)
	if err != nil {
		return stats, err
	}
	if days.Closed[date] {
		return stats, ErrDayClosed
	}
	if !bySchool {
		return stats, nil
	}
	enrollmentStats, err := s.requireInCare(ctx, studentID, date)
	return addStats(stats, enrollmentStats), err
}

// requireInCare checks that the child is in care on date, and tells a child
// the school does not know from one whose care has not begun or has ended.
func (s *Service) requireInCare(ctx context.Context, studentID int64, date domain.Date) (domain.OperationStats, error) {
	candidates, stats, err := s.directory.FindDailyCandidates(ctx, date)
	if err != nil {
		return stats, err
	}
	for _, candidate := range candidates {
		if candidate.StudentID == studentID {
			return stats, nil
		}
	}
	names, nameStats, err := s.directory.FindStudentNames(ctx, []int64{studentID})
	stats = addStats(stats, nameStats)
	if _, known := names[studentID]; err == nil && !known {
		return stats, ErrStudentNotFound
	}
	if err != nil {
		return stats, err
	}
	return stats, ErrNotEnrolled
}

// mayChangeDay checks a day change: the school changes any day from today
// on, a parent until the deadline of its direction.
func mayChangeDay(rules domain.OrderingRules, now time.Time, date domain.Date, participating, bySchool bool) error {
	switch {
	case bySchool && date.Before(domain.DateFromInstant(now)):
		return ErrPastDay
	case !bySchool && !parentMayChange(rules, now, date, participating):
		return ErrCutoffPassed
	default:
		return nil
	}
}

// setParticipationDay registers (participating) or cancels the day: for a
// parent each until its own deadline, for the school at any time.
func (s *Service) setParticipationDay(ctx context.Context, studentID, accountID int64, date domain.Date, participating, bySchool bool) error {
	return s.run(ctx, "set_participation_day", func(txCtx context.Context) (domain.OperationStats, error) {
		rules, rulesErr := s.requireRegistration(txCtx)
		if rulesErr != nil {
			return domain.OperationStats{}, rulesErr
		}
		if err := mayChangeDay(rules, s.now(), date, participating, bySchool); err != nil {
			return domain.OperationStats{}, err
		}
		stats, err := s.requireChangeableDay(txCtx, studentID, date, bySchool)
		if err != nil {
			return stats, err
		}
		change := domain.RegistrationChange{StudentID: studentID, AccountID: accountID, BySchool: bySchool, Kind: domain.RegistrationDay, Date: date, Participating: &participating}
		writeStats, err := s.settleWrite(txCtx, rules, date, date, changeAuthor{studentID: studentID, accountID: accountID}, func() (domain.OperationStats, error) {
			saveStats, saveErr := s.store.UpsertParticipationOverride(txCtx, studentID, accountID, date, participating)
			if saveErr != nil {
				return saveStats, saveErr
			}
			logStats, logErr := s.store.InsertRegistrationChanges(txCtx, []domain.RegistrationChange{change})
			return addStats(saveStats, logStats), logErr
		})
		return addStats(stats, writeStats), err
	})
}

// setParticipationWeek registers or cancels the remaining weekdays of the
// week for a parent: only the days that would change, so cancelling never
// touches a day the child does not eat. A day whose deadline has passed stays
// as it is and is returned as locked. Nothing left to change but a locked day
// is a passed deadline; nothing to change at all is no error.
func (s *Service) setParticipationWeek(ctx context.Context, studentID, accountID int64, monday domain.Date, participating bool) (changed, locked []domain.Date, err error) {
	err = s.run(ctx, "set_participation_week", func(txCtx context.Context) (domain.OperationStats, error) {
		rules, rulesErr := s.requireRegistration(txCtx)
		if rulesErr != nil {
			return domain.OperationStats{}, rulesErr
		}
		plan, stats, readErr := s.readParticipation(txCtx, studentID, monday, monday.AddDays(4), rules)
		if readErr != nil {
			return stats, readErr
		}
		changed, locked = weekChanges(rules, s.now(), plan.Days, participating)
		if len(changed) == 0 && len(locked) > 0 {
			return stats, ErrCutoffPassed
		}
		entries := make([]domain.RegistrationChange, 0, len(changed))
		for _, date := range changed {
			entries = append(entries, domain.RegistrationChange{StudentID: studentID, AccountID: accountID, Kind: domain.RegistrationDay, Date: date, Participating: &participating})
		}
		writeStats, err := s.settleWrite(txCtx, rules, monday, monday.AddDays(4), changeAuthor{studentID: studentID, accountID: accountID}, func() (domain.OperationStats, error) {
			saveStats, saveErr := s.store.UpsertParticipationOverrides(txCtx, studentID, accountID, changed, participating)
			if saveErr != nil {
				return saveStats, saveErr
			}
			logStats, logErr := s.store.InsertRegistrationChanges(txCtx, entries)
			return addStats(saveStats, logStats), logErr
		})
		return addStats(stats, writeStats), err
	})
	return changed, locked, err
}

// weekChanges sorts the days of a week change: the days that would change
// and are still open, and those whose deadline has passed. Closed and past
// days and days already as wanted stay out.
func weekChanges(rules domain.OrderingRules, now time.Time, days []domain.ParticipationDay, participating bool) (changed, locked []domain.Date) {
	today := domain.DateFromInstant(now)
	for _, day := range days {
		switch {
		case day.Source == domain.ParticipationClosed, day.Registered == participating, day.Date.Before(today):
		case parentMayChange(rules, now, day.Date, participating):
			changed = append(changed, day.Date)
		default:
			locked = append(locked, day.Date)
		}
	}
	return changed, locked
}

// readParticipation resolves the child's days in [from, to] from its
// registration and the school calendar.
func (s *Service) readParticipation(ctx context.Context, studentID int64, from, to domain.Date, rules domain.OrderingRules) (domain.ParticipationPlan, domain.OperationStats, error) {
	data, stats, err := s.store.FindParticipation(ctx, studentID, from, to)
	if err != nil {
		return domain.ParticipationPlan{}, stats, err
	}
	days, calendarStats, err := s.calendar.Days(ctx, from, to)
	stats = addStats(stats, calendarStats)
	if err != nil {
		return domain.ParticipationPlan{}, stats, err
	}
	return s.resolveParticipation(data, from, to, rules, days), stats, nil
}

// clearParticipationDay drops the day's exception, so the weekly pattern
// applies again. Whether that registers or cancels the day depends on the
// pattern, and for a parent the matching deadline decides. Nobody changes a
// day the school is closed, and the school no day already over.
func (s *Service) clearParticipationDay(ctx context.Context, studentID, accountID int64, date domain.Date, bySchool bool) error {
	return s.run(ctx, "clear_participation_day", func(txCtx context.Context) (domain.OperationStats, error) {
		rules, rulesErr := s.requireRegistration(txCtx)
		if rulesErr != nil {
			return domain.OperationStats{}, rulesErr
		}
		stats, err := s.requireClearableDay(txCtx, rules, studentID, date, bySchool)
		if err != nil {
			return stats, err
		}
		change := domain.RegistrationChange{StudentID: studentID, AccountID: accountID, BySchool: bySchool, Kind: domain.RegistrationRegular, Date: date}
		writeStats, err := s.settleWrite(txCtx, rules, date, date, changeAuthor{studentID: studentID, accountID: accountID}, func() (domain.OperationStats, error) {
			deleteStats, deleteErr := s.store.DeleteParticipationOverride(txCtx, studentID, date)
			if deleteErr != nil || deleteStats.Rows == 0 {
				return deleteStats, deleteErr
			}
			logStats, logErr := s.store.InsertRegistrationChanges(txCtx, []domain.RegistrationChange{change})
			return addStats(deleteStats, logStats), logErr
		})
		return addStats(stats, writeStats), err
	})
}

// requireClearableDay checks that returning the day to the regular days is
// allowed: the school changes no day already over, a parent only until the
// deadline of the direction the day then changes in.
func (s *Service) requireClearableDay(ctx context.Context, rules domain.OrderingRules, studentID int64, date domain.Date, bySchool bool) (domain.OperationStats, error) {
	if bySchool && date.Before(domain.DateFromInstant(s.now())) {
		return domain.OperationStats{}, ErrPastDay
	}
	plan, stats, err := s.readParticipation(ctx, studentID, date, date, rules)
	if err != nil {
		return stats, err
	}
	for _, day := range plan.Days {
		if day.Registered == day.Regular || day.Source == domain.ParticipationClosed {
			continue
		}
		if err := mayChangeDay(rules, s.now(), date, day.Regular, bySchool); err != nil {
			return stats, err
		}
	}
	dayStats, err := s.requireChangeableDay(ctx, studentID, date, bySchool)
	return addStats(stats, dayStats), err
}

// parentMayChange reports whether a parent may still register (participating)
// or cancel the day.
func parentMayChange(rules domain.OrderingRules, now time.Time, date domain.Date, participating bool) bool {
	if participating {
		return rules.ParentCanRegister(now, date)
	}
	return rules.ParentCanCancel(now, date)
}

func (s *Service) dailyParticipants(ctx context.Context, date domain.Date) (list domain.DailyList, err error) {
	err = s.run(ctx, "read_daily_participants", func(txCtx context.Context) (domain.OperationStats, error) {
		rules, rulesErr := s.requireRegistration(txCtx)
		if rulesErr != nil {
			return domain.OperationStats{}, rulesErr
		}
		deadline := rules.FreezeAt(date)
		list = domain.DailyList{Date: date, CutoffTime: deadline.In(calendarZone).Format("15:04"), Deadline: deadline, Participants: []domain.DailyParticipant{}}
		days, stats, calendarErr := s.calendar.Days(txCtx, date, date)
		list.Closed = days.Closed[date]
		if calendarErr != nil || list.Closed {
			return stats, calendarErr
		}
		cutoff := rules.AbsenceCutoff(date)
		candidates, candidateStats, findErr := s.directory.FindDailyCandidates(txCtx, date)
		stats = addStats(stats, candidateStats)
		if findErr != nil {
			return stats, findErr
		}
		studentIDs := make([]int64, 0, len(candidates))
		for _, candidate := range candidates {
			studentIDs = append(studentIDs, candidate.StudentID)
		}
		participation, participationStats, findErr := s.store.FindDailyParticipation(txCtx, studentIDs, date, cutoff)
		stats = addStats(stats, participationStats)
		if findErr != nil {
			return stats, findErr
		}
		for _, candidate := range candidates {
			state := participation[candidate.StudentID]
			day := dayState{holiday: days.Holiday[date], scheduled: state.Regular, override: state.Override, absences: state.Absences}
			list.Add(domain.DailyParticipant(candidate), resolveDay(day, cutoff).participating)
		}
		domain.SortDailyParticipants(list.Participants)
		domain.SortDailyParticipants(list.Others)
		return stats, nil
	})
	return list, err
}

func addStats(left, right domain.OperationStats) domain.OperationStats {
	return domain.OperationStats{
		Queries:           left.Queries + right.Queries,
		Rows:              left.Rows + right.Rows,
		StatementDuration: left.StatementDuration + right.StatementDuration,
	}
}

func (s *Service) Available(ctx context.Context) (available bool, err error) {
	err = s.run(ctx, "availability", func(txCtx context.Context) (domain.OperationStats, error) {
		available, err = s.settings.MealPlanEnabled(txCtx)
		return domain.OperationStats{}, err
	})
	return available, err
}

func (s *Service) weekEntries(ctx context.Context, monday domain.Date) (entries []domain.Entry, err error) {
	err = s.run(ctx, "read_week", func(txCtx context.Context) (domain.OperationStats, error) {
		if err := s.requireEnabled(txCtx); err != nil {
			return domain.OperationStats{}, err
		}
		var stats domain.OperationStats
		entries, stats, err = s.store.FindWeek(txCtx, monday, monday.AddDays(4))
		return stats, err
	})
	return entries, err
}

func (s *Service) replace(ctx context.Context, date domain.Date, dishes []mealplan.Dish) error {
	return s.run(ctx, "replace_day", func(txCtx context.Context) (domain.OperationStats, error) {
		if err := s.requireEnabled(txCtx); err != nil {
			return domain.OperationStats{}, err
		}
		return s.store.ReplaceDay(txCtx, date, dishes)
	})
}

func (s *Service) clear(ctx context.Context, date domain.Date) error {
	return s.run(ctx, "clear_day", func(txCtx context.Context) (domain.OperationStats, error) {
		if err := s.requireEnabled(txCtx); err != nil {
			return domain.OperationStats{}, err
		}
		return s.store.ClearDay(txCtx, date)
	})
}

func (s *Service) requireEnabled(ctx context.Context) error {
	enabled, err := s.settings.MealPlanEnabled(ctx)
	if err != nil {
		return fmt.Errorf("meal plan: resolve availability: %w", err)
	}
	if !enabled {
		return ErrDisabled
	}
	return nil
}

func (s *Service) requireRegistration(ctx context.Context) (domain.OrderingRules, error) {
	if err := s.requireEnabled(ctx); err != nil {
		return domain.OrderingRules{}, err
	}
	enabled, err := s.settings.MealRegistrationEnabled(ctx)
	if err != nil {
		return domain.OrderingRules{}, fmt.Errorf("meal plan: resolve registration availability: %w", err)
	}
	if !enabled {
		return domain.OrderingRules{}, ErrRegistrationDisabled
	}
	settings, err := s.settings.MealOrderingSettings(ctx)
	if err != nil {
		return domain.OrderingRules{}, fmt.Errorf("meal plan: resolve ordering settings: %w", err)
	}
	return orderingRules(settings)
}

func (s *Service) resolveParticipation(data domain.ParticipationData, from, to domain.Date, rules domain.OrderingRules, days domain.CalendarDays) domain.ParticipationPlan {
	now := s.now()
	overrides := make(map[domain.Date]bool, len(data.Overrides))
	for _, override := range data.Overrides {
		overrides[override.Date] = override.Participating
	}
	absences := make(map[domain.Date][]domain.SickDay, len(data.SickDays))
	for _, absence := range data.SickDays {
		absences[absence.Date] = append(absences[absence.Date], absence)
	}
	plan := domain.ParticipationPlan{
		CutoffTime: rules.Registration.Clock, Model: rules.Model, ScheduleChangeable: rules.ParentCanChangeSchedule(now), Sick: rules.Sick,
		Days: make([]domain.ParticipationDay, 0, from.DaysUntil(to)+1),
	}
	for date := from; !date.After(to); date = date.AddDays(1) {
		if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			continue
		}
		state := dayState{
			closed: days.Closed[date], holiday: days.Holiday[date],
			scheduled: scheduleContains(scheduleForDate(data.Schedules, date), date.Weekday()), absences: absences[date],
		}
		if value, exists := overrides[date]; exists {
			state.override = &value
		}
		result := resolveDay(state, rules.AbsenceCutoff(date))
		day := domain.ParticipationDay{
			Date: date, Participating: result.participating, Registered: result.registered, Regular: result.regular, Source: result.source,
			RegisterUntil: rules.ParentRegistrationUntil(date), CancelUntil: rules.CancellationAt(date),
		}
		if !state.closed {
			day.CanRegister, day.CanCancel = rules.ParentCanRegister(now, date), rules.ParentCanCancel(now, date)
		}
		plan.Days = append(plan.Days, day)
	}
	editableSchedule := scheduleForDate(data.Schedules, rules.NextEffectiveDate(now))
	if editableSchedule != nil {
		plan.EffectiveFrom = editableSchedule.EffectiveFrom
		plan.Weekdays = append([]domain.Weekday(nil), editableSchedule.Weekdays...)
	}
	return plan
}

func scheduleForDate(schedules []domain.ParticipationSchedule, date domain.Date) *domain.ParticipationSchedule {
	var current *domain.ParticipationSchedule
	for index := range schedules {
		if schedules[index].EffectiveFrom.After(date) {
			break
		}
		current = &schedules[index]
	}
	return current
}

func scheduleContains(schedule *domain.ParticipationSchedule, weekday time.Weekday) bool {
	if schedule == nil {
		return false
	}
	for _, candidate := range schedule.Weekdays {
		if int(candidate) == int(weekday) {
			return true
		}
	}
	return false
}

func (s *Service) run(ctx context.Context, operation string, fn func(context.Context) (domain.OperationStats, error)) (err error) {
	started := time.Now()
	var stats domain.OperationStats
	defer func() {
		s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	}()
	err = s.tx.Run(ctx, func(txCtx context.Context) error {
		stats, err = fn(txCtx)
		return err
	})
	if err != nil {
		stats.Rows = 0
	}
	return err
}
