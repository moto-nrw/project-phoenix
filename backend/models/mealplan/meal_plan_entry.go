// Package mealplan holds the per-day meal plan (Essensplan) domain entity.
// The plan is school-wide: one optional entry per tenant per calendar day,
// maintained by staff and read by parents in the parents portal. The backing
// table lives in the schedule schema (schedule.meal_plan_entries).
package mealplan

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// MealPlanEntry is one dish served on a calendar day for one school. A day can
// have several dishes (Menü 1 / Menü 2 / vegetarisch), ordered by Position.
type MealPlanEntry struct {
	base.Model `bun:"schema:schedule,table:meal_plan_entries"`
	base.TenantModel
	Date timezone.Date `bun:"date,notnull,type:date" json:"date"`
	// Position orders the dishes within a day (0-based).
	Position int    `bun:"position,notnull" json:"position"`
	Dish     string `bun:"dish,notnull" json:"dish"`
	// Note is an optional free-text hint shown beneath the dish (e.g.
	// "vegetarisch", "ohne Schwein"). Deliberately not an allergen feature.
	Note *string `bun:"note" json:"note,omitempty"`
}

// MealPlanEntryRepository is the data-access boundary for meal plan entries.
type MealPlanEntryRepository interface {
	// FindByDateRange returns all dish entries with start <= date <= end for
	// the current tenant, ordered by date then position ascending.
	FindByDateRange(ctx context.Context, start, end timezone.Date) ([]*MealPlanEntry, error)
	// ReplaceDay atomically replaces all dishes for the given date with the
	// supplied entries (positions assigned by the caller). An empty slice
	// clears the day. Runs in the ambient tenant transaction.
	ReplaceDay(ctx context.Context, date timezone.Date, entries []*MealPlanEntry) error
	// DeleteByDate removes all dishes for the given date in the current tenant.
	DeleteByDate(ctx context.Context, date timezone.Date) error
}
