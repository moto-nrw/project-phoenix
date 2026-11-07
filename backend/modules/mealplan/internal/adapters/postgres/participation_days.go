package postgres

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/uptrace/bun"
)

// FindParticipationDays reads the same children's registration across a date
// set. Each date keeps its own effective schedule and absence cutoff.
func (s *Store) FindParticipationDays(ctx context.Context, studentIDs []int64, cutoffs map[domain.Date]time.Time) (map[domain.Date]map[int64]domain.DailyParticipation, domain.OperationStats, error) {
	states := make(map[domain.Date]map[int64]domain.DailyParticipation, len(cutoffs))
	if len(studentIDs) == 0 || len(cutoffs) == 0 {
		return states, domain.OperationStats{}, nil
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	dates := slices.Sorted(maps.Keys(cutoffs))
	var stats domain.OperationStats
	var schedules []participationScheduleRow
	err = timedQuery(&stats, func() error {
		return db.NewSelect().Model(&schedules).
			ModelTableExpr(`schedule.meal_participation_schedules AS "meal_participation_schedule"`).
			Where(`"meal_participation_schedule".tenant_id = ?`, tenantID).
			Where(`"meal_participation_schedule".student_id IN (?)`, bun.List(studentIDs)).
			Where(`"meal_participation_schedule".effective_from <= ?`, dates[len(dates)-1]).
			OrderExpr(`"meal_participation_schedule".student_id ASC, "meal_participation_schedule".effective_from ASC`).
			Scan(ctx)
	})
	if err != nil {
		return nil, stats, fmt.Errorf("meal plan postgres: find date-set schedules: %w", err)
	}
	stats.Rows += int64(len(schedules))
	for _, date := range dates {
		daily := make(map[int64]domain.DailyParticipation, len(studentIDs))
		for _, id := range studentIDs {
			daily[id] = domain.DailyParticipation{}
		}
		for _, schedule := range schedules {
			if !schedule.EffectiveFrom.After(date) {
				daily[schedule.StudentID] = domain.DailyParticipation{Regular: scheduleIncludes(schedule, date.Weekday())}
			}
		}
		states[date] = daily
	}
	if err := loadParticipationDayOverrides(ctx, db, tenantID, studentIDs, dates, states, &stats); err != nil {
		return nil, stats, err
	}
	err = loadParticipationDayAbsences(ctx, db, tenantID, studentIDs, dates, cutoffs, states, &stats)
	return states, stats, err
}

func loadParticipationDayOverrides(ctx context.Context, db bun.IDB, tenantID int64, studentIDs []int64, dates []domain.Date, states map[domain.Date]map[int64]domain.DailyParticipation, stats *domain.OperationStats) error {
	var overrides []participationOverrideRow
	err := timedQuery(stats, func() error {
		return db.NewSelect().Model(&overrides).
			ModelTableExpr(`schedule.meal_participation_overrides AS "meal_participation_override"`).
			Where(`"meal_participation_override".tenant_id = ?`, tenantID).
			Where(`"meal_participation_override".student_id IN (?)`, bun.List(studentIDs)).
			Where(`"meal_participation_override".date IN (?)`, bun.List(dates)).
			Scan(ctx)
	})
	if err != nil {
		return fmt.Errorf("meal plan postgres: find date-set overrides: %w", err)
	}
	stats.Rows += int64(len(overrides))
	for _, override := range overrides {
		daily := states[override.Date]
		state := daily[override.StudentID]
		participating := override.Participating
		state.Override = &participating
		daily[override.StudentID] = state
	}
	return nil
}

func loadParticipationDayAbsences(ctx context.Context, db bun.IDB, tenantID int64, studentIDs []int64, dates []domain.Date, cutoffs map[domain.Date]time.Time, states map[domain.Date]map[int64]domain.DailyParticipation, stats *domain.OperationStats) error {
	var latest time.Time
	for _, cutoff := range cutoffs {
		if cutoff.After(latest) {
			latest = cutoff
		}
	}
	var sickDays []sickDayRow
	err := timedQuery(stats, func() error {
		return absenceHistory(db, &sickDays, tenantID).
			Where(`"sickness_history".student_id IN (?)`, bun.List(studentIDs)).
			Where(`"sickness_history".date IN (?)`, bun.List(dates)).
			Where(`"sickness_history".changed_at <= ?`, latest).
			OrderExpr(`"sickness_history".student_id ASC, "sickness_history".changed_at ASC, "sickness_history".id ASC`).
			Scan(ctx)
	})
	if err != nil {
		return fmt.Errorf("meal plan postgres: find date-set sickness history: %w", err)
	}
	stats.Rows += int64(len(sickDays))
	for _, sickDay := range sickDays {
		if sickDay.ChangedAt.After(cutoffs[sickDay.Date]) {
			continue
		}
		daily := states[sickDay.Date]
		state := daily[sickDay.StudentID]
		state.Absences = append(state.Absences, sickDay.absence())
		daily[sickDay.StudentID] = state
	}
	return nil
}
