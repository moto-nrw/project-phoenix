package common

import (
	"context"
	"net/http"

	configSvc "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// WeekendSettings is the one read the weekend plan needs from the Settings
// Platform.
type WeekendSettings interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
}

// WithWeekendPlan lets the request read whether its school's weekend follows
// Friday's plan (#3921). The source runs only for weekend dates and inside the
// request's tenant context, so a weekday never reads it and the request cache
// serves repeats.
func WithWeekendPlan(r *http.Request, settings WeekendSettings) *http.Request {
	if settings == nil {
		return r
	}
	return r.WithContext(calendar.WithWeekendPlan(r.Context(), func(ctx context.Context) (bool, error) {
		return settings.ResolveBool(ctx, configSvc.KeyWeekendFollowsFriday)
	}))
}
