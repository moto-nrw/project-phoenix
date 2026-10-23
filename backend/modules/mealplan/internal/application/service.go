package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/ports"
)

type Service struct {
	store     ports.Store
	directory ports.Directory
	settings  ports.Settings
	tx        ports.Transaction
	observe   ports.Observer
	now       func() time.Time
}

func New(store ports.Store, directory ports.Directory, settings ports.Settings, tx ports.Transaction, observe ports.Observer, now func() time.Time) *Service {
	if store == nil || directory == nil || settings == nil || tx == nil || observe == nil || now == nil {
		panic("meal plan application: all dependencies are required")
	}
	return &Service{store: store, directory: directory, settings: settings, tx: tx, observe: observe, now: now}
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

func (s *Service) requireRegistration(ctx context.Context) (domain.Deadline, error) {
	if err := s.requireEnabled(ctx); err != nil {
		return domain.Deadline{}, err
	}
	enabled, err := s.settings.MealRegistrationEnabled(ctx)
	if err != nil {
		return domain.Deadline{}, fmt.Errorf("meal plan: resolve registration availability: %w", err)
	}
	if !enabled {
		return domain.Deadline{}, mealplan.ErrRegistrationDisabled
	}
	cutoff, leadDays, err := s.settings.MealRegistrationCutoff(ctx)
	if err != nil {
		return domain.Deadline{}, fmt.Errorf("meal plan: resolve registration cutoff: %w", err)
	}
	if _, err := time.Parse("15:04", cutoff); err != nil || leadDays < 0 {
		return domain.Deadline{}, mealplan.ErrInvalidParticipation
	}
	return domain.Deadline{Time: cutoff, LeadDays: leadDays}, nil
}

func (s *Service) resolveParticipation(data domain.ParticipationData, from, to domain.Date, deadline domain.Deadline) domain.ParticipationPlan {
	now := s.now()
	overrides := make(map[domain.Date]bool, len(data.Overrides))
	for _, override := range data.Overrides {
		overrides[override.Date] = override.Participating
	}
	sickDays := effectiveSickDays(data.SickDays, deadline)
	plan := domain.ParticipationPlan{Deadline: deadline, Days: make([]domain.ParticipationDay, 0, from.DaysUntil(to)+1)}
	for date := from; !date.After(to); date = date.AddDays(1) {
		if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
			continue
		}
		schedule := scheduleForDate(data.Schedules, date)
		participating := scheduleContains(schedule, date.Weekday())
		source := domain.ParticipationNone
		if participating {
			source = domain.ParticipationRegular
		}
		if value, exists := overrides[date]; exists {
			participating = value
			source = domain.ParticipationOverride
		}
		if sick, exists := sickDays[date]; exists && sickAtCutoff(sick.ReportedAt, sick.ClearedAt, deadline.At(date)) {
			participating = false
			source = domain.ParticipationSick
		}
		plan.Days = append(plan.Days, domain.ParticipationDay{Date: date, Participating: participating, Source: source, Changeable: deadline.Changeable(now, date)})
	}
	editableSchedule := scheduleForDate(data.Schedules, deadline.NextEffectiveDate(now))
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
		}
	}
}

func effectiveSickDays(days []domain.SickDay, deadline domain.Deadline) map[domain.Date]domain.SickDay {
	sickDays := make(map[domain.Date]domain.SickDay, len(days))
	for _, sick := range days {
		cutoff := deadline.At(sick.Date)
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
