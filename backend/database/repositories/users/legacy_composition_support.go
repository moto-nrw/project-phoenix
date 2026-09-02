package users

import (
	"context"
	"database/sql"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The helpers below serve the legacy composition seam
// (database/repositories), which assembles the staff, teacher and guest
// repositories over the School Membership owner. That package composes
// owners; it does not carry the transaction runtime, the calendar-date type
// or the permission matcher, so the small pieces of those it needs live here,
// next to the repositories that already depend on them.

// NotFoundError builds the not-found shape every legacy repository returns, so
// callers keep classifying with errors.Is(err, sql.ErrNoRows) and
// modelBase.IsNoRows(err) alike.
func NotFoundError(op string) error {
	return &modelBase.DatabaseError{Op: op, Err: base.TranslateNotFound(sql.ErrNoRows)}
}

// WrapError wraps err in the legacy repository error shape, preserving the
// chain so errors.Is on the original sentinel still works.
func WrapError(op string, err error) error {
	if err == nil {
		return nil
	}
	return &modelBase.DatabaseError{Op: op, Err: err}
}

// IsNotFound reports whether err is a repository not-found error.
func IsNotFound(err error) bool { return modelBase.IsNoRows(err) }

// CalendarDateString renders an optional calendar date as YYYY-MM-DD, empty
// when unset.
func CalendarDateString(value *timezone.Date) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.String()
}

// ParseCalendarDate parses a YYYY-MM-DD value; an empty or malformed value
// yields nil, which is how the legacy models spell "no date".
func ParseCalendarDate(value string) *timezone.Date {
	if value == "" {
		return nil
	}
	parsed, err := timezone.ParseDate(value)
	if err != nil {
		return nil
	}
	return &parsed
}

// TodayCalendarDate is today in Berlin as YYYY-MM-DD.
func TodayCalendarDate() string { return timezone.TodayDate().String() }

// TenantIDFromContext returns the tenant the caller acts for, 0 when none.
func TenantIDFromContext(ctx context.Context) int64 { return tenant.FromContext(ctx) }

// CalendarOwnPermission is the permission a staff member needs to use the
// calendar for themselves.
const CalendarOwnPermission = permissions.CalendarOwn

// HasEffectivePermission applies the same wildcard-aware matcher route
// authorization uses, so a composition cannot accidentally decide by exact
// string match what the routes decide by pattern.
func HasEffectivePermission(name string, granted []string) bool {
	return authorize.HasPermission(name, granted)
}
