package checkin

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/users"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
)

// timeNow is a package-local clock hook that tests may override to pin the
// "current time" for deterministic assertions. Production code uses time.Now.
var timeNow = time.Now

// ResolveRawDailyCheckoutTime resolves the raw daily checkout time string.
// Fallback chain: tenant DB override → STUDENT_DAILY_CHECKOUT_TIME env var → empty string.
// Returns empty string when no time is configured, meaning daily checkout is always available.
//
// This is the single source of truth for the fallback chain — both the checkin flow
// (which needs a parsed time.Time) and the IoT config endpoint (which needs the raw string
// for PyrePortal) must use this helper instead of reimplementing the chain.
func ResolveRawDailyCheckoutTime(ctx context.Context, settingsService configSvc.SettingsService) string {
	fallback := os.Getenv("STUDENT_DAILY_CHECKOUT_TIME")
	return configSvc.ResolveStringOrDefault(ctx, settingsService, configModel.KeyStudentDailyCheckoutTime, fallback, slog.Default())
}

// studentDailyCheckoutTime resolves the daily checkout time as a parsed *time.Time.
// Returns nil when no time is configured, meaning daily checkout is always available.
func (s *CheckinService) studentDailyCheckoutTime(ctx context.Context) (*time.Time, error) {
	checkoutTimeStr := ResolveRawDailyCheckoutTime(ctx, s.settings)

	// No time configured — daily checkout is always available
	if checkoutTimeStr == "" {
		return nil, nil
	}

	// Parse time in HH:MM format
	parts := strings.Split(checkoutTimeStr, ":")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid checkout time format: %s", checkoutTimeStr)
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return nil, fmt.Errorf("invalid hour in checkout time: %s", checkoutTimeStr)
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return nil, fmt.Errorf("invalid minute in checkout time: %s", checkoutTimeStr)
	}

	now := timeNow()
	checkoutTime := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	return &checkoutTime, nil
}

// ShouldUpgradeToDailyCheckout checks if a checkout should be upgraded to daily checkout.
func (s *CheckinService) ShouldUpgradeToDailyCheckout(ctx context.Context, action string, student *users.Student, currentVisit *active.Visit) bool {
	if action != "checked_out" {
		return false
	}
	if student.GroupID == nil || currentVisit == nil || currentVisit.ActiveGroup == nil {
		return false
	}
	return s.ShouldShowDailyCheckoutWithGroup(ctx, student, currentVisit)
}

// IsAfterCheckoutTimeGate checks whether the current time is past the applicable
// checkout time gate for this student. When per-student checkout is enabled and
// the student has a pickup time, uses pickup_time - delta. Otherwise falls back
// to the global daily checkout time.
func (s *CheckinService) IsAfterCheckoutTimeGate(ctx context.Context, student *users.Student) bool {
	if gate, ok := s.perStudentCheckoutGate(ctx, student); ok {
		return gate
	}
	return s.isAfterGlobalCheckoutTime(ctx)
}

// perStudentCheckoutGate evaluates the per-student checkout window. The second
// return is false when per-student checkout does not apply (disabled, no pickup
// service, or the student has no pickup time), signalling the caller to fall
// back to the global gate.
func (s *CheckinService) perStudentCheckoutGate(ctx context.Context, student *users.Student) (bool, bool) {
	if !s.perStudentCheckoutEnabled(ctx) || s.pickup == nil {
		return false, false
	}

	now := timeNow()
	effectivePickup, err := s.pickup.GetEffectivePickupTimeForDate(ctx, student.ID, timezone.DateFromTime(now))
	if err != nil || effectivePickup == nil || effectivePickup.PickupTime == nil {
		return false, false
	}

	delta := s.perStudentCheckoutDelta(ctx)
	todayPickup := time.Date(now.Year(), now.Month(), now.Day(),
		effectivePickup.PickupTime.Hour(), effectivePickup.PickupTime.Minute(),
		0, 0, now.Location())
	threshold := todayPickup.Add(-time.Duration(delta) * time.Minute)
	return now.After(threshold), true
}

// perStudentCheckoutEnabled reports whether per-student checkout is enabled.
func (s *CheckinService) perStudentCheckoutEnabled(ctx context.Context) bool {
	if s.settings == nil {
		return false
	}
	if val, err := s.settings.ResolveBool(ctx, configModel.KeyPerStudentCheckoutEnabled); err == nil {
		return val
	}
	return false
}

// perStudentCheckoutDelta resolves the per-student checkout delta in minutes,
// defaulting to 15 when unset or unreadable.
func (s *CheckinService) perStudentCheckoutDelta(ctx context.Context) int {
	delta := 15
	if s.settings != nil {
		if val, err := s.settings.ResolveInt(ctx, configModel.KeyPerStudentCheckoutDeltaMinutes); err == nil {
			delta = val
		}
	}
	return delta
}

// isAfterGlobalCheckoutTime evaluates the global daily checkout time gate. A nil
// configured time means there is no gate and checkout is always available.
func (s *CheckinService) isAfterGlobalCheckoutTime(ctx context.Context) bool {
	checkoutTime, err := s.studentDailyCheckoutTime(ctx)
	if err != nil {
		return false
	}
	if checkoutTime == nil {
		return true
	}
	return timeNow().After(*checkoutTime)
}

// ShouldShowDailyCheckoutWithGroup checks if daily checkout should be shown by verifying education group room.
func (s *CheckinService) ShouldShowDailyCheckoutWithGroup(ctx context.Context, student *users.Student, currentVisit *active.Visit) bool {
	if student.GroupID == nil {
		return false
	}
	if currentVisit == nil || currentVisit.ActiveGroup == nil {
		return false
	}

	if !s.IsAfterCheckoutTimeGate(ctx, student) {
		return false
	}

	educationGroup, err := s.education.GetGroup(ctx, *student.GroupID)
	if err != nil || educationGroup == nil {
		return false
	}

	// If the group has no room assigned, daily checkout is available from any room
	if educationGroup.RoomID == nil {
		return true
	}

	return currentVisit.ActiveGroup.RoomID == *educationGroup.RoomID
}
