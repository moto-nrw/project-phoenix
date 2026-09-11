package active

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

// Active-session lifecycle policy (Rule 12: Models Hold Data, Not Decisions).
//
// The active models (Group, Visit, GroupSupervisor, CombinedGroup) only hold
// their timestamp facts. The decisions built on top of those facts — "has the
// session timed out?", "is this supervision still active right now?", "stamp
// the end time" — live here in the service, where the clock is injected and the
// per-tenant inactivity-timeout policy is resolved from the settings registry.

// DefaultSessionInactivityTimeout is the fallback inactivity window applied to
// an active group session that has no per-session TimeoutMinutes set and no
// tenant override for KeySessionInactivityTimeoutMin. Mirrors the registry
// default so behavior is unchanged when settings are unavailable.
const DefaultSessionInactivityTimeout = 30 * time.Minute

// ResolveSessionTimeout returns the inactivity-timeout window for a group
// session. A per-session TimeoutMinutes (> 0) always wins; otherwise the value
// is resolved from the tenant's KeySessionInactivityTimeoutMin override, then
// the registry default, then the hardcoded DefaultSessionInactivityTimeout.
func (s *service) ResolveSessionTimeout(ctx context.Context, group *active.Group) time.Duration {
	if group != nil && group.TimeoutMinutes > 0 {
		return time.Duration(group.TimeoutMinutes) * time.Minute
	}
	return s.resolveDefaultSessionTimeout(ctx)
}

// resolveDefaultSessionTimeout reads the tenant-configurable default inactivity
// window from the settings registry. ResolveInt returns the tenant override when
// one exists and the registry default (30) otherwise, so no HasTenantOverride
// dance is needed — there is no env-var fallback to disambiguate against. The
// hardcoded constant only guards the resolver-unavailable / error / zero paths.
func (s *service) resolveDefaultSessionTimeout(ctx context.Context) time.Duration {
	if s.settings == nil {
		return DefaultSessionInactivityTimeout
	}

	minutes, err := s.settings.ResolveInt(ctx, configModel.KeySessionInactivityTimeoutMin)
	if err != nil {
		s.getLogger().Warn("session inactivity timeout resolve failed, using default",
			slog.String("key", configModel.KeySessionInactivityTimeoutMin),
			slog.String("error", err.Error()),
		)
		return DefaultSessionInactivityTimeout
	}
	if minutes <= 0 {
		return DefaultSessionInactivityTimeout
	}
	return time.Duration(minutes) * time.Minute
}

// SessionInactivityDuration reports how long the group session has been inactive
// relative to the supplied clock. The model no longer reads the wall clock.
func SessionInactivityDuration(group *active.Group, now time.Time) time.Duration {
	return now.Sub(group.LastActivity)
}

// IsSessionTimedOut decides whether an active group session has exceeded its
// inactivity timeout as of now. Ended sessions never count as timed out.
func (s *service) IsSessionTimedOut(ctx context.Context, group *active.Group, now time.Time) bool {
	if group == nil || !group.IsActive() {
		return false
	}
	return SessionInactivityDuration(group, now) >= s.ResolveSessionTimeout(ctx, group)
}

// SessionTimeUntilTimeout returns how long remains before the session times out
// (negative when it has already timed out), as of now.
func (s *service) SessionTimeUntilTimeout(ctx context.Context, group *active.Group, now time.Time) time.Duration {
	return s.ResolveSessionTimeout(ctx, group) - SessionInactivityDuration(group, now)
}

// IsSupervisorActive decides whether a supervision assignment has started and
// has not ended as of now. A nil EndDate means open-ended after its StartDate.
func IsSupervisorActive(supervisor *active.GroupSupervisor, now time.Time) bool {
	today := timezone.DateFromTime(now)
	return !supervisor.StartDate.After(today) &&
		(supervisor.EndDate == nil || today.Before(*supervisor.EndDate))
}

// IsCombinedGroupActive decides whether a combined group is still active as of
// now, using the same open-ended-until-EndTime rule as supervisions.
func IsCombinedGroupActive(group *active.CombinedGroup, now time.Time) bool {
	if group.EndTime == nil {
		return true
	}
	return now.Before(*group.EndTime)
}
