// Package mealplan holds the business logic for the per-day meal plan
// (Essensplan): staff maintain one dish + optional note per calendar day and
// parents read it. Authorization, tenant scoping and the feature toggle are
// applied by the callers (staff handler / parent service); this service owns
// validation and week-range math.
package mealplan

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/mealplan"
)

// ErrInvalidMealDate means a write targeted a date the meal plan does not cover
// (a Saturday or Sunday). The plan is a Monday-Friday work week: GetWeek only
// ever reads Mon-Fri and neither the staff nor the parent UI has a weekend
// column, so a persisted weekend row would be invisible. Handlers map this to a
// 400 rather than a 500.
var ErrInvalidMealDate = errors.New("meal plan covers weekdays only (Monday-Friday)")

// isWeekend reports whether d falls on a Saturday or Sunday.
func isWeekend(d timezone.Date) bool {
	wd := d.Weekday()
	return wd == time.Saturday || wd == time.Sunday
}

// DishInput is one dish supplied for a day, in display order.
type DishInput struct {
	Dish string
	Note *string
}

// Service is the meal plan business-logic boundary for the staff side. It
// expects the tenant to already be present in the context (tenant middleware).
type Service interface {
	// GetWeek returns the dish entries for the Monday-Friday week containing
	// weekStart, ordered by date then position. Days without a dish are absent.
	GetWeek(ctx context.Context, weekStart timezone.Date) ([]*mealplan.MealPlanEntry, error)
	// SetDay replaces all dishes for one day. Blank dishes are dropped; an
	// all-blank/empty list clears the day. Weekend dates are rejected with
	// ErrInvalidMealDate — the plan is a Monday-Friday work week.
	SetDay(ctx context.Context, date timezone.Date, dishes []DishInput) error
	// Delete removes all dishes for one day. Deleting a missing day is a no-op.
	Delete(ctx context.Context, date timezone.Date) error
}

type service struct {
	repo mealplan.MealPlanEntryRepository
}

// NewService wires the meal plan service.
func NewService(repo mealplan.MealPlanEntryRepository) Service {
	return &service{repo: repo}
}

// WeekRange returns the Monday and Friday bounding the work week that contains
// weekStart. It is exported so the parent service can reuse the exact same
// range when reading a child's school plan. The result spans Monday..Friday;
// the meal plan deliberately ignores the weekend.
func WeekRange(weekStart timezone.Date) (monday, friday timezone.Date) {
	// time.Weekday: Sunday=0 .. Saturday=6. Shift so Monday=0.
	offset := (int(weekStart.Weekday()) + 6) % 7
	monday = weekStart.AddDays(-offset)
	friday = monday.AddDays(4)
	return monday, friday
}

func (s *service) GetWeek(ctx context.Context, weekStart timezone.Date) ([]*mealplan.MealPlanEntry, error) {
	monday, friday := WeekRange(weekStart)
	return s.repo.FindByDateRange(ctx, monday, friday)
}

func (s *service) SetDay(ctx context.Context, date timezone.Date, dishes []DishInput) error {
	if date.IsZero() {
		return errors.New("date is required")
	}
	// Reject weekend dates before persisting: a Saturday/Sunday row would never
	// be read back (GetWeek normalizes to Mon-Fri) and has no UI column, so it
	// would silently vanish. See ErrInvalidMealDate.
	if isWeekend(date) {
		return ErrInvalidMealDate
	}

	entries := make([]*mealplan.MealPlanEntry, 0, len(dishes))
	pos := 0
	for _, d := range dishes {
		dish := strings.TrimSpace(d.Dish)
		if dish == "" {
			// Skip blank rows so an empty add-row never persists.
			continue
		}
		entries = append(entries, &mealplan.MealPlanEntry{
			Date:     date,
			Position: pos,
			Dish:     dish,
			Note:     strutil.TrimPtrToNil(d.Note),
		})
		pos++
	}
	return s.repo.ReplaceDay(ctx, date, entries)
}

func (s *service) Delete(ctx context.Context, date timezone.Date) error {
	if date.IsZero() {
		return errors.New("date is required")
	}
	return s.repo.DeleteByDate(ctx, date)
}
