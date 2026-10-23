package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type Service struct {
	store     ports.Store
	directory ports.Directory
	settings  ports.Settings
	calendar  ports.Calendar
	tx        ports.Transaction
	observe   ports.Observer
	now       func() time.Time
}

func New(store ports.Store, directory ports.Directory, settings ports.Settings, calendar ports.Calendar, tx ports.Transaction, observe ports.Observer, now func() time.Time) *Service {
	if store == nil || directory == nil || settings == nil || calendar == nil || tx == nil || observe == nil || now == nil {
		panic("meal plan application: all dependencies are required")
	}
	return &Service{store: store, directory: directory, settings: settings, calendar: calendar, tx: tx, observe: observe, now: now}
}

func (s *Service) RegistrationAvailable(ctx context.Context) (available bool, err error) {
	err = s.run(ctx, "registration_availability", func(txCtx context.Context) (domain.OperationStats, error) {
		if err = s.requireEnabled(txCtx); err != nil {
			return domain.OperationStats{}, err
		}
		available, err = s.settings.MealRegistrationEnabled(txCtx)
		return domain.OperationStats{}, err
	})
	return available, err
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

func (s *Service) requireEnabled(ctx context.Context) error {
	enabled, err := s.settings.MealPlanEnabled(ctx)
	if err != nil {
		return fmt.Errorf("meal plan: resolve availability: %w", err)
	}
	if !enabled {
		return mealplan.ErrDisabled
	}
	return nil
}

// closedDayMargin is the first calendar window around the requested dates.
// Windows touching a closed day grow in this increment until both edges are
// open, so a long closing period remains fully visible to deadline rules.
const closedDayMargin = 31

// requireRegistration resolves the cutoffs and the closed days the dates
// from..to and today need.
func (s *Service) requireRegistration(ctx context.Context, from, to domain.Date) (domain.Rules, mealplan.Cutoffs, error) {
	if err := s.requireEnabled(ctx); err != nil {
		return domain.Rules{}, mealplan.Cutoffs{}, err
	}
	enabled, err := s.settings.MealRegistrationEnabled(ctx)
	if err != nil {
		return domain.Rules{}, mealplan.Cutoffs{}, fmt.Errorf("meal plan: resolve registration availability: %w", err)
	}
	if !enabled {
		return domain.Rules{}, mealplan.Cutoffs{}, mealplan.ErrRegistrationDisabled
	}
	cutoffs, err := s.settings.MealParticipationCutoffs(ctx)
	if err != nil {
		return domain.Rules{}, mealplan.Cutoffs{}, fmt.Errorf("meal plan: resolve registration cutoff: %w", err)
	}
	if !validCutoffs(cutoffs) {
		return domain.Rules{}, mealplan.Cutoffs{}, mealplan.ErrInvalidParticipation
	}
	today := calendar.DateFromTime(s.now())
	days, err := s.nonWorkingDays(ctx, minDate(from, today).AddDays(-closedDayMargin), maxDate(to, today).AddDays(closedDayMargin))
	if err != nil {
		return domain.Rules{}, mealplan.Cutoffs{}, fmt.Errorf("meal plan: resolve closed days: %w", err)
	}
	closed := make(map[domain.Date]bool, len(days))
	for day, isClosed := range days {
		if isClosed {
			closed[domain.Date(day)] = true
		}
	}
	rules := domain.Rules{
		Registration: domain.Deadline{Time: cutoffs.CutoffTime, LeadDays: cutoffs.CutoffLeadDays, Weekday: domain.Weekday(cutoffs.CutoffWeekday), Closed: closed},
		Cancellation: domain.Deadline{Time: cutoffs.CancellationCutoffTime, LeadDays: cutoffs.CancellationCutoffLeadDays, Closed: closed},
	}
	return rules, cutoffs, nil
}

// nonWorkingDays reads the requested window and continues in either direction
// while its edge is closed. An open edge proves that no contiguous closure
// affecting a deadline or the next effective date continues beyond it.
func (s *Service) nonWorkingDays(ctx context.Context, from, to domain.Date) (map[string]bool, error) {
	days := make(map[string]bool)
	for {
		window, err := s.calendar.NonWorkingDayDates(ctx, from.String(), to.String())
		if err != nil {
			return nil, err
		}
		for day, closed := range window {
			if closed {
				days[day] = true
			}
		}

		expandBefore := days[from.String()]
		expandAfter := days[to.String()]
		if !expandBefore && !expandAfter {
			return days, nil
		}
		if expandBefore {
			from = from.AddDays(-closedDayMargin)
		}
		if expandAfter {
			to = to.AddDays(closedDayMargin)
		}
	}
}

func validCutoffs(cutoffs mealplan.Cutoffs) bool {
	if _, err := time.Parse("15:04", cutoffs.CutoffTime); err != nil || cutoffs.CutoffLeadDays < 0 {
		return false
	}
	if cutoffs.CutoffWeekday < 0 || cutoffs.CutoffWeekday > mealplan.Friday || cutoffs.CancellationCutoffLeadDays < 0 {
		return false
	}
	if cutoffs.CancellationCutoffTime == "" {
		return true
	}
	_, err := time.Parse("15:04", cutoffs.CancellationCutoffTime)
	return err == nil
}

func minDate(left, right domain.Date) domain.Date {
	if right.Before(left) {
		return right
	}
	return left
}

func maxDate(left, right domain.Date) domain.Date {
	if right.After(left) {
		return right
	}
	return left
}

func (s *Service) resolveParticipation(data domain.ParticipationData, from, to domain.Date, rules domain.Rules) domain.ParticipationPlan {
	now := s.now()
	overrides := make(map[domain.Date]bool, len(data.Overrides))
	for _, override := range data.Overrides {
		overrides[override.Date] = override.Participating
	}
	sickDays := effectiveSickDays(data.SickDays, rules)
	plan := domain.ParticipationPlan{Rules: rules, Days: make([]domain.ParticipationDay, 0, from.DaysUntil(to)+1)}
	for date := from; !date.After(to); date = date.AddDays(1) {
		if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			continue
		}
		participating, source := dayParticipation(data.Schedules, overrides, sickDays, date, rules)
		// The one change the day allows: cancelling when participating,
		// registering otherwise.
		changeable := rules.Registration.Changeable(now, date)
		if participating {
			changeable = rules.CanCancel(now, date)
		}
		plan.Days = append(plan.Days, domain.ParticipationDay{Date: date, Participating: participating, Source: source, Changeable: changeable})
	}
	editableSchedule := scheduleForDate(data.Schedules, rules.Registration.NextEffectiveDate(now))
	if editableSchedule != nil {
		plan.EffectiveFrom = editableSchedule.EffectiveFrom
		plan.Weekdays = append([]domain.Weekday(nil), editableSchedule.Weekdays...)
	}
	return plan
}

// dayParticipation resolves one day: the fixed days, a day override, a
// sick note before the cancellation cutoff and a closed day, in that order.
func dayParticipation(schedules []domain.ParticipationSchedule, overrides map[domain.Date]bool, sickDays map[domain.Date]domain.SickDay, date domain.Date, rules domain.Rules) (bool, domain.ParticipationSource) {
	if rules.Closed(date) {
		return false, domain.ParticipationClosed
	}
	participating := scheduleContains(scheduleForDate(schedules, date), date.Weekday())
	source := domain.ParticipationNone
	if participating {
		source = domain.ParticipationRegular
	}
	if value, exists := overrides[date]; exists {
		participating = value
		source = domain.ParticipationOverride
	}
	if sick, exists := sickDays[date]; exists && sickAtCutoff(sick.ReportedAt, sick.ClearedAt, rules.CancellationAt(date)) {
		participating = false
		source = domain.ParticipationSick
	}
	return participating, source
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

func sickAtCutoff(reportedAt time.Time, clearedAt *time.Time, cutoff time.Time) bool {
	return !reportedAt.After(cutoff) && (clearedAt == nil || clearedAt.After(cutoff))
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

func appendDailyParticipants(list *mealplan.DailyList, candidates []mealplan.ParticipantCandidate, participation map[int64]domain.DailyParticipation, cutoff time.Time) {
	for _, candidate := range candidates {
		state := participation[candidate.StudentID]
		participating := state.Regular
		if state.Override != nil {
			participating = *state.Override
		}
		if state.SickReportedAt != nil && sickAtCutoff(*state.SickReportedAt, state.SickClearedAt, cutoff) {
			participating = false
		}
		if participating {
			list.Participants = append(list.Participants, mealplan.DailyParticipant(candidate))
		} else {
			list.Available = append(list.Available, mealplan.DailyParticipant(candidate))
		}
	}
}

func effectiveSickDays(days []domain.SickDay, rules domain.Rules) map[domain.Date]domain.SickDay {
	sickDays := make(map[domain.Date]domain.SickDay, len(days))
	for _, sick := range days {
		cutoff := rules.CancellationAt(sick.Date)
		if sick.ChangedAt.After(cutoff) {
			continue
		}
		current, exists := sickDays[sick.Date]
		if !exists || sick.ChangedAt.After(current.ChangedAt) || (sick.ChangedAt.Equal(current.ChangedAt) && sick.ID > current.ID) {
			sickDays[sick.Date] = sick
		}
	}
	return sickDays
}
