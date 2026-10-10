package calendar

import (
	"context"
	"time"
)

// WeekendPlanSource reports whether the current tenant runs Saturday and
// Sunday on Friday's weekly plan (#3921). The composition root binds it over
// the tenant setting; it is consulted only for weekend dates, so a weekday
// never costs a settings read.
type WeekendPlanSource func(ctx context.Context) (bool, error)

type weekendPlanKey struct{}

// WithWeekendPlan attaches the source for the tenant setting to ctx. The
// HTTP root and the worker's tenant loop attach it; without it every weekend
// stays what it always was: no care day.
func WithWeekendPlan(ctx context.Context, source WeekendPlanSource) context.Context {
	if source == nil {
		return ctx
	}
	return context.WithValue(ctx, weekendPlanKey{}, source)
}

// ISOWeekday is the ISO weekday of d: 1 for Monday through 7 for Sunday.
func ISOWeekday(d Date) int {
	weekday := int(d.Weekday())
	if weekday == 0 {
		return 7
	}
	return weekday
}

// IsWeekend reports whether d is a Saturday or a Sunday.
func IsWeekend(d Date) bool {
	weekday := d.Weekday()
	return weekday == time.Saturday || weekday == time.Sunday
}

// WeekendFollowsFriday reports whether the tenant in ctx runs the weekend on
// Friday's plan. Without a bound source it is false; a failed resolution is
// returned so the caller decides, never silently read as a value.
func WeekendFollowsFriday(ctx context.Context) (bool, error) {
	source, _ := ctx.Value(weekendPlanKey{}).(WeekendPlanSource)
	if source == nil {
		return false, nil
	}
	return source(ctx)
}

// PlanWeekday is the ISO weekday whose weekly plan applies on d: d's own
// weekday, or Friday (5) for a weekend that follows Friday's plan. A weekday
// never consults the setting.
func PlanWeekday(ctx context.Context, d Date) (int, error) {
	weekday := ISOWeekday(d)
	if weekday <= 5 {
		return weekday, nil
	}
	follows, err := WeekendFollowsFriday(ctx)
	if err != nil {
		return weekday, err
	}
	if follows {
		return 5, nil
	}
	return weekday, nil
}

// IsCareWeekday reports whether d can be a care day at all: Monday to
// Friday, or a weekend that follows Friday's plan. A weekday never consults
// the setting.
func IsCareWeekday(ctx context.Context, d Date) (bool, error) {
	if !IsWeekend(d) {
		return true, nil
	}
	return WeekendFollowsFriday(ctx)
}
