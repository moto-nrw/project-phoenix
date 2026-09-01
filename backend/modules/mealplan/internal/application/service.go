package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/ports"
)

var ErrDisabled = errors.New("meal plan is disabled")
var (
	ErrRegistrationDisabled = errors.New("meal registration is disabled")
	ErrInvalidCutoff        = errors.New("meal registration cutoff is invalid")
	ErrCutoffPassed         = errors.New("meal registration cutoff has passed")
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

func (s *Service) Participation(ctx context.Context, studentID int64, from, to domain.Date) (plan domain.ParticipationPlan, err error) {
	err = s.run(ctx, "read_participation", func(txCtx context.Context) (domain.OperationStats, error) {
		cutoffTime, cutoffErr := s.requireRegistration(txCtx)
		if cutoffErr != nil {
			return domain.OperationStats{}, cutoffErr
		}
		data, stats, findErr := s.store.FindParticipation(txCtx, studentID, from, to)
		if findErr != nil {
			return stats, findErr
		}
		plan = s.resolveParticipation(data, from, to, cutoffTime)
		return stats, nil
	})
	return plan, err
}

func (s *Service) ReplaceParticipationSchedule(ctx context.Context, studentID, accountID int64, weekdays []domain.Weekday) (effectiveFrom domain.Date, err error) {
	err = s.run(ctx, "replace_participation_schedule", func(txCtx context.Context) (domain.OperationStats, error) {
		cutoffTime, cutoffErr := s.requireRegistration(txCtx)
		if cutoffErr != nil {
			return domain.OperationStats{}, cutoffErr
		}
		effectiveFrom = domain.NextEffectiveDate(s.now(), cutoffTime)
		return s.store.InsertParticipationSchedule(txCtx, studentID, accountID, effectiveFrom, weekdays)
	})
	return effectiveFrom, err
}

func (s *Service) SetParticipationDay(ctx context.Context, studentID, accountID int64, date domain.Date, participating bool) error {
	return s.run(ctx, "set_participation_day", func(txCtx context.Context) (domain.OperationStats, error) {
		cutoffTime, cutoffErr := s.requireRegistration(txCtx)
		if cutoffErr != nil {
			return domain.OperationStats{}, cutoffErr
		}
		if !domain.Changeable(s.now(), date, cutoffTime) {
			return domain.OperationStats{}, ErrCutoffPassed
		}
		return s.store.UpsertParticipationOverride(txCtx, studentID, accountID, date, participating)
	})
}

func (s *Service) ClearParticipationDay(ctx context.Context, studentID, _ int64, date domain.Date) error {
	return s.run(ctx, "clear_participation_day", func(txCtx context.Context) (domain.OperationStats, error) {
		cutoffTime, cutoffErr := s.requireRegistration(txCtx)
		if cutoffErr != nil {
			return domain.OperationStats{}, cutoffErr
		}
		if !domain.Changeable(s.now(), date, cutoffTime) {
			return domain.OperationStats{}, ErrCutoffPassed
		}
		return s.store.DeleteParticipationOverride(txCtx, studentID, date)
	})
}

func (s *Service) DailyParticipants(ctx context.Context, date domain.Date) (list domain.DailyList, err error) {
	err = s.run(ctx, "read_daily_participants", func(txCtx context.Context) (domain.OperationStats, error) {
		cutoffTime, cutoffErr := s.requireRegistration(txCtx)
		if cutoffErr != nil {
			return domain.OperationStats{}, cutoffErr
		}
		cutoff := domain.CutoffAt(date, cutoffTime)
		candidates, stats, findErr := s.directory.FindDailyCandidates(txCtx, date)
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
		list = domain.DailyList{Date: date, CutoffTime: cutoffTime, Participants: make([]domain.DailyParticipant, 0, len(candidates))}
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
				list.Participants = append(list.Participants, domain.DailyParticipant{
					StudentID: candidate.StudentID, FirstName: candidate.FirstName, LastName: candidate.LastName, SchoolClass: candidate.SchoolClass,
				})
			}
		}
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

func (s *Service) Week(ctx context.Context, monday domain.Date) (entries []domain.Entry, err error) {
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

func (s *Service) Replace(ctx context.Context, date domain.Date, dishes []domain.Dish) error {
	return s.run(ctx, "replace_day", func(txCtx context.Context) (domain.OperationStats, error) {
		if err := s.requireEnabled(txCtx); err != nil {
			return domain.OperationStats{}, err
		}
		return s.store.ReplaceDay(txCtx, date, dishes)
	})
}

func (s *Service) Clear(ctx context.Context, date domain.Date) error {
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

func (s *Service) requireRegistration(ctx context.Context) (string, error) {
	if err := s.requireEnabled(ctx); err != nil {
		return "", err
	}
	enabled, err := s.settings.MealRegistrationEnabled(ctx)
	if err != nil {
		return "", fmt.Errorf("meal plan: resolve registration availability: %w", err)
	}
	if !enabled {
		return "", ErrRegistrationDisabled
	}
	cutoff, err := s.settings.MealRegistrationCutoff(ctx)
	if err != nil {
		return "", fmt.Errorf("meal plan: resolve registration cutoff: %w", err)
	}
	if _, err := time.Parse("15:04", cutoff); err != nil {
		return "", fmt.Errorf("%w: %q", ErrInvalidCutoff, cutoff)
	}
	return cutoff, nil
}

func (s *Service) resolveParticipation(data domain.ParticipationData, from, to domain.Date, cutoffTime string) domain.ParticipationPlan {
	overrides := make(map[domain.Date]bool, len(data.Overrides))
	for _, override := range data.Overrides {
		overrides[override.Date] = override.Participating
	}
	sickDays := make(map[domain.Date]domain.SickDay, len(data.SickDays))
	for _, sick := range data.SickDays {
		cutoff := domain.CutoffAt(sick.Date, cutoffTime)
		if sick.ChangedAt.After(cutoff) {
			continue
		}
		current, exists := sickDays[sick.Date]
		if !exists || sick.ChangedAt.After(current.ChangedAt) || (sick.ChangedAt.Equal(current.ChangedAt) && sick.ID > current.ID) {
			sickDays[sick.Date] = sick
		}
	}
	plan := domain.ParticipationPlan{CutoffTime: cutoffTime, Days: make([]domain.ParticipationDay, 0, from.DaysUntil(to)+1)}
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
		if sick, exists := sickDays[date]; exists && sickAtCutoff(sick.ReportedAt, sick.ClearedAt, domain.CutoffAt(date, cutoffTime)) {
			participating = false
			source = domain.ParticipationSick
		}
		plan.Days = append(plan.Days, domain.ParticipationDay{Date: date, Participating: participating, Source: source, Changeable: domain.Changeable(s.now(), date, cutoffTime)})
	}
	if len(data.Schedules) > 0 {
		latest := data.Schedules[len(data.Schedules)-1]
		plan.EffectiveFrom = latest.EffectiveFrom
		plan.Weekdays = append([]domain.Weekday(nil), latest.Weekdays...)
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
