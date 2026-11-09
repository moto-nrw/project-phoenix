package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type ParticipationHistoryStore interface {
	InsertParticipationHistory(context.Context, []mealplan.PastMealDay) error
}

type ParticipationHistory struct {
	store ParticipationHistoryStore
	tx    interface {
		Run(context.Context, func(context.Context) error) error
	}
}

func NewParticipationHistory(store ParticipationHistoryStore, tx interface {
	Run(context.Context, func(context.Context) error) error
}) *ParticipationHistory {
	return &ParticipationHistory{store: store, tx: tx}
}

func (h *ParticipationHistory) RecordParticipationHistory(ctx context.Context, days []mealplan.PastMealDay) error {
	today := calendar.TodayDate()
	for _, day := range days {
		parsed, err := calendar.ParseDate(day.Date.String())
		if err != nil || day.StudentID <= 0 || !parsed.Before(today) || calendar.IsWeekend(parsed) {
			return fmt.Errorf("meal plan: invalid past meal day")
		}
	}
	if len(days) == 0 {
		return nil
	}
	return h.tx.Run(ctx, func(ctx context.Context) error { return h.store.InsertParticipationHistory(ctx, days) })
}
