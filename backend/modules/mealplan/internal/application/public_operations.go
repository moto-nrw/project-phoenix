package application

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (s *Service) Week(ctx context.Context, weekStart mealplan.Date) ([]mealplan.Entry, error) {
	date, err := calendar.ParseDate(string(weekStart))
	if err != nil {
		return nil, mealplan.ErrInvalidMealDate
	}
	monday := date.StartOfISOWeek()
	var entries []domain.Entry

	err = s.run(ctx, "read_week", func(txCtx context.Context) (domain.OperationStats, error) {
		if err := s.requireEnabled(txCtx); err != nil {
			return domain.OperationStats{}, err
		}
		var stats domain.OperationStats
		entries, stats, err = s.store.FindWeek(txCtx, monday, monday.AddDays(4))
		return stats, err
	})
	if err != nil {
		return nil, err
	}
	result := make([]mealplan.Entry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, mealplan.Entry{Date: mealplan.Date(entry.Date.String()), Position: entry.Position, Dish: entry.Dish, Note: entry.Note})
	}
	return result, nil
}

func (s *Service) ReplaceDay(ctx context.Context, command mealplan.ReplaceDay) error {
	date, err := calendar.ParseDate(string(command.Date))
	if err != nil {
		return mealplan.ErrInvalidMealDate
	}
	if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
		return mealplan.ErrInvalidMealDate
	}

	dishes := make([]mealplan.Dish, 0, len(command.Dishes))
	for _, input := range command.Dishes {
		dish := strings.TrimSpace(input.Dish)
		if dish == "" {
			continue
		}
		dishes = append(dishes, mealplan.Dish{Dish: dish, Note: trimOptional(input.Note)})
	}
	values := dishes

	return s.run(ctx, "replace_day", func(txCtx context.Context) (domain.OperationStats, error) {
		if err := s.requireEnabled(txCtx); err != nil {
			return domain.OperationStats{}, err
		}
		return s.store.ReplaceDay(txCtx, date, values)
	})
}

func (s *Service) ClearDay(ctx context.Context, date mealplan.Date) error {
	parsed, err := calendar.ParseDate(string(date))
	if err != nil {
		return mealplan.ErrInvalidMealDate
	}

	return s.run(ctx, "clear_day", func(txCtx context.Context) (domain.OperationStats, error) {
		if err := s.requireEnabled(txCtx); err != nil {
			return domain.OperationStats{}, err
		}
		return s.store.ClearDay(txCtx, parsed)
	})
}

func (s *Service) Participation(ctx context.Context, studentID int64, from, to mealplan.Date) (mealplan.ParticipationPlan, error) {
	start, startErr := calendar.ParseDate(string(from))
	end, endErr := calendar.ParseDate(string(to))
	if studentID <= 0 || startErr != nil || endErr != nil || end.Before(start) || start.DaysUntil(end) > 31 {
		return mealplan.ParticipationPlan{}, mealplan.ErrInvalidParticipation
	}
	var plan domain.ParticipationPlan
	var cutoffs mealplan.Cutoffs
	err := s.run(ctx, "read_participation", func(txCtx context.Context) (domain.OperationStats, error) {
		rules, resolved, cutoffErr := s.requireRegistration(txCtx, start, end)
		if cutoffErr != nil {
			return domain.OperationStats{}, cutoffErr
		}
		cutoffs = resolved
		data, stats, findErr := s.store.FindParticipation(txCtx, studentID, start, end)
		if findErr != nil {
			return stats, findErr
		}
		plan = s.resolveParticipation(data, start, end, rules)
		return stats, nil
	})
	if err != nil {
		return mealplan.ParticipationPlan{}, err
	}
	return publicParticipationPlan(plan, cutoffs), nil
}

func (s *Service) ReplaceParticipationSchedule(ctx context.Context, command mealplan.ReplaceParticipationSchedule) (mealplan.Date, error) {
	if command.StudentID <= 0 || command.GuardianAccountID <= 0 {
		return "", mealplan.ErrInvalidParticipation
	}
	weekdays, ok := normalizeWeekdays(command.Weekdays)
	if !ok {
		return "", mealplan.ErrInvalidParticipation
	}
	values := make([]domain.Weekday, 0, len(weekdays))
	for _, day := range weekdays {
		values = append(values, domain.Weekday(day))
	}
	var effectiveFrom domain.Date
	err := s.run(ctx, "replace_participation_schedule", func(txCtx context.Context) (domain.OperationStats, error) {
		today := calendar.DateFromTime(s.now())
		rules, _, cutoffErr := s.requireRegistration(txCtx, today, today)
		if cutoffErr != nil {
			return domain.OperationStats{}, cutoffErr
		}
		effectiveFrom = rules.Registration.NextEffectiveDate(s.now())
		return s.store.InsertParticipationSchedule(txCtx, command.StudentID, command.GuardianAccountID, effectiveFrom, values)
	})
	return mealplan.Date(effectiveFrom.String()), err
}

func (s *Service) SetParticipationForDay(ctx context.Context, command mealplan.SetParticipationDay) error {
	date, err := participationDate(command.Date)
	if command.StudentID <= 0 || command.GuardianAccountID <= 0 || err != nil {
		return mealplan.ErrInvalidParticipation
	}

	return s.run(ctx, "set_participation_day", func(txCtx context.Context) (domain.OperationStats, error) {
		rules, _, cutoffErr := s.requireRegistration(txCtx, date, date)
		if cutoffErr != nil {
			return domain.OperationStats{}, cutoffErr
		}
		stats, err := s.requireChangeOpen(txCtx, rules, command.StudentID, date, &command.Participating)
		if err != nil {
			return stats, err
		}
		writeStats, err := s.store.UpsertParticipationOverride(txCtx, command.StudentID, command.GuardianAccountID, date, command.Participating)
		return addStats(stats, writeStats), err
	})
}

func (s *Service) ClearParticipationForDay(ctx context.Context, command mealplan.SetParticipationDay) error {
	date, err := participationDate(command.Date)
	if command.StudentID <= 0 || command.GuardianAccountID <= 0 || err != nil {
		return mealplan.ErrInvalidParticipation
	}

	return s.run(ctx, "clear_participation_day", func(txCtx context.Context) (domain.OperationStats, error) {
		rules, _, cutoffErr := s.requireRegistration(txCtx, date, date)
		if cutoffErr != nil {
			return domain.OperationStats{}, cutoffErr
		}
		stats, err := s.requireChangeOpen(txCtx, rules, command.StudentID, date, nil)
		if err != nil {
			return stats, err
		}
		writeStats, err := s.store.DeleteParticipationOverride(txCtx, command.StudentID, date)
		return addStats(stats, writeStats), err
	})
}

func (s *Service) DailyList(ctx context.Context, date mealplan.Date) (mealplan.DailyList, error) {
	parsed, err := participationDate(date)
	if err != nil {
		return mealplan.DailyList{}, mealplan.ErrInvalidParticipation
	}
	var list mealplan.DailyList

	err = s.run(ctx, "read_daily_participants", func(txCtx context.Context) (domain.OperationStats, error) {
		rules, cutoffs, cutoffErr := s.requireRegistration(txCtx, parsed, parsed)
		if cutoffErr != nil {
			return domain.OperationStats{}, cutoffErr
		}
		var stats domain.OperationStats
		list, stats, err = s.dailyList(txCtx, parsed, rules, cutoffs)
		return stats, err
	})
	if err != nil {
		return mealplan.DailyList{}, err
	}
	return list, nil
}

// WeeklyList is the kitchen list of each school day in the week of
// weekStart, read in one transaction with one deadline.
func (s *Service) WeeklyList(ctx context.Context, weekStart mealplan.Date) (mealplan.WeeklyList, error) {
	date, err := calendar.ParseDate(string(weekStart))
	if err != nil {
		return mealplan.WeeklyList{}, mealplan.ErrInvalidParticipation
	}
	monday := date.StartOfISOWeek()
	var week mealplan.WeeklyList

	err = s.run(ctx, "read_weekly_participants", func(txCtx context.Context) (domain.OperationStats, error) {
		rules, cutoffs, cutoffErr := s.requireRegistration(txCtx, monday, monday.AddDays(4))
		if cutoffErr != nil {
			return domain.OperationStats{}, cutoffErr
		}
		week = mealplan.WeeklyList{Cutoffs: cutoffs, WeekStart: mealplan.Date(monday.String()), Days: make([]mealplan.DailyList, 0, 5)}
		var stats domain.OperationStats
		for offset := range 5 {
			list, dayStats, dayErr := s.dailyList(txCtx, monday.AddDays(offset), rules, cutoffs)
			stats = addStats(stats, dayStats)
			if dayErr != nil {
				return stats, dayErr
			}
			week.Days = append(week.Days, list)
		}
		return stats, nil
	})
	if err != nil {
		return mealplan.WeeklyList{}, err
	}
	return week, nil
}

func (s *Service) dailyList(ctx context.Context, date domain.Date, rules domain.Rules, cutoffs mealplan.Cutoffs) (mealplan.DailyList, domain.OperationStats, error) {
	if rules.Closed(date) {
		return mealplan.DailyList{Cutoffs: cutoffs, Date: mealplan.Date(date.String()), Closed: true, Participants: []mealplan.DailyParticipant{}, Available: []mealplan.DailyParticipant{}}, domain.OperationStats{}, nil
	}
	cutoff := rules.CancellationAt(date)
	candidates, stats, err := s.directory.FindDailyCandidates(ctx, date)
	if err != nil {
		return mealplan.DailyList{}, stats, err
	}
	studentIDs := make([]int64, 0, len(candidates))
	for _, candidate := range candidates {
		studentIDs = append(studentIDs, candidate.StudentID)
	}
	participation, participationStats, err := s.store.FindDailyParticipation(ctx, studentIDs, date, cutoff)
	stats = addStats(stats, participationStats)
	if err != nil {
		return mealplan.DailyList{}, stats, err
	}
	list := mealplan.DailyList{Cutoffs: cutoffs, Date: mealplan.Date(date.String()), Participants: make([]mealplan.DailyParticipant, 0, len(candidates)), Available: make([]mealplan.DailyParticipant, 0)}
	appendDailyParticipants(&list, candidates, participation, cutoff)
	mealplan.SortDailyParticipants(list.Participants)
	mealplan.SortDailyParticipants(list.Available)
	return list, stats, nil
}

func participationDate(value mealplan.Date) (calendar.Date, error) {
	date, err := calendar.ParseDate(string(value))
	if err != nil || date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
		return "", mealplan.ErrInvalidParticipation
	}
	return date, nil
}

func normalizeWeekdays(values []mealplan.Weekday) ([]mealplan.Weekday, bool) {
	seen := make(map[mealplan.Weekday]struct{}, len(values))
	result := make([]mealplan.Weekday, 0, len(values))
	for _, weekday := range values {
		if weekday < mealplan.Monday || weekday > mealplan.Friday {
			return nil, false
		}
		if _, exists := seen[weekday]; exists {
			continue
		}
		seen[weekday] = struct{}{}
		result = append(result, weekday)
	}
	slices.Sort(result)
	return result, true
}

func trimOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func publicParticipationPlan(plan domain.ParticipationPlan, cutoffs mealplan.Cutoffs) mealplan.ParticipationPlan {
	result := mealplan.ParticipationPlan{Cutoffs: cutoffs, EffectiveFrom: mealplan.Date(plan.EffectiveFrom.String()), Weekdays: make([]mealplan.Weekday, 0, len(plan.Weekdays)), Days: make([]mealplan.ParticipationDay, 0, len(plan.Days))}
	for _, weekday := range plan.Weekdays {
		result.Weekdays = append(result.Weekdays, mealplan.Weekday(weekday))
	}
	for _, day := range plan.Days {
		result.Days = append(result.Days, mealplan.ParticipationDay{Date: mealplan.Date(day.Date.String()), Participating: day.Participating, Source: mealplan.ParticipationSource(day.Source), Changeable: day.Changeable})
	}
	return result
}
