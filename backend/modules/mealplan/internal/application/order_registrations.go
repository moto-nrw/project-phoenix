package application

import (
	"context"
	"maps"
	"slices"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/mealplan/internal/domain"
)

// prepareOrderRegistrations replaces the per-day participation reads in the
// existing read-local registrations with ID/date-set loads. Writes keep their
// daily reads under locks.
func prepareOrderRegistrations(seen registrations, rules domain.OrderingRules, days domain.CalendarDays, from, to domain.Date, now time.Time) {
	first := reconciliationStart(rules, domain.DateFromInstant(now))
	for date := from; !date.After(to); date = date.AddDays(1) {
		if date.Before(first) || date.Weekday() == time.Saturday || date.Weekday() == time.Sunday || days.Closed[date] {
			continue
		}
		seen[date] = dayRegistration{until: rules.AbsenceCutoff(date)}
	}
}

func (s *Service) orderCandidates(ctx context.Context, date domain.Date, seen registrations) ([]domain.DailyCandidate, domain.OperationStats, error) {
	prepared, exists := seen[date]
	if !exists || len(seen) == 1 {
		return s.directory.FindDailyCandidates(ctx, date)
	}
	if prepared.roster != nil {
		return prepared.roster, domain.OperationStats{}, nil
	}
	dates := make([]domain.Date, 0, len(seen))
	for day := range seen {
		dates = append(dates, day)
	}
	slices.Sort(dates)
	loaded, stats, err := s.directory.FindCandidatesOnDates(ctx, dates)
	if err != nil {
		// A later child's health decode failure must not preempt this day's
		// participation error. Retry only the original point read.
		daily, dailyStats, dailyErr := s.directory.FindDailyCandidates(ctx, date)
		return daily, addStats(stats, dailyStats), dailyErr
	}
	for day, candidates := range loaded {
		registration := seen[day]
		registration.roster = candidates
		seen[day] = registration
	}
	return seen[date].roster, stats, nil
}

func (s *Service) orderParticipation(ctx context.Context, ids []int64, date domain.Date, until time.Time, seen registrations) (map[int64]domain.DailyParticipation, domain.OperationStats, error) {
	prepared, exists := seen[date]
	if !exists || len(seen) == 1 || prepared.until.Before(until) {
		return s.store.FindDailyParticipation(ctx, ids, date, until)
	}
	requested := slices.Clone(ids)
	for _, registration := range seen {
		for _, candidate := range registration.roster {
			requested = append(requested, candidate.StudentID)
		}
	}
	slices.Sort(requested)
	requested = slices.Compact(requested)
	missing := make([]int64, 0, len(requested))
	for _, id := range requested {
		if _, known := prepared.states[id]; !known {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return prepared.states, domain.OperationStats{}, nil
	}
	cutoffs := make(map[domain.Date]time.Time, len(seen))
	for day, registration := range seen {
		cutoffs[day] = registration.until
	}
	loaded, stats, err := s.store.FindParticipationDays(ctx, missing, cutoffs)
	if err != nil {
		// A later date's batch failure must not preempt this day's result. Retry
		// the original daily read; the next roster is still reached only after it.
		daily, dailyStats, dailyErr := s.store.FindDailyParticipation(ctx, ids, date, until)
		return daily, addStats(stats, dailyStats), dailyErr
	}
	for day, states := range loaded {
		registration := seen[day]
		if registration.states == nil {
			registration.states = states
		} else {
			maps.Copy(registration.states, states)
		}
		seen[day] = registration
	}
	return seen[date].states, stats, nil
}
