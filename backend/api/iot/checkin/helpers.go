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
// This is the single source of truth for the fallback chain — both the checkin handler
// (which needs a parsed time.Time) and the IoT config endpoint (which needs the raw string
// for PyrePortal) must use this helper instead of reimplementing the chain.
func ResolveRawDailyCheckoutTime(ctx context.Context, settingsService configSvc.SettingsService) string {
	fallback := os.Getenv("STUDENT_DAILY_CHECKOUT_TIME")
	return configSvc.ResolveStringOrDefault(ctx, settingsService, configModel.KeyStudentDailyCheckoutTime, fallback, slog.Default())
}

// getStudentDailyCheckoutTime resolves the daily checkout time as a parsed *time.Time.
// Returns nil when no time is configured, meaning daily checkout is always available.
func (rs *Resource) getStudentDailyCheckoutTime(ctx context.Context) (*time.Time, error) {
	checkoutTimeStr := ResolveRawDailyCheckoutTime(ctx, rs.SettingsService)

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

// shouldUpgradeToDailyCheckout checks if a checkout should be upgraded to daily checkout.
// Encapsulates the complex condition to reduce cognitive complexity in deviceCheckin.
func (rs *Resource) shouldUpgradeToDailyCheckout(ctx context.Context, action string, student *users.Student, currentVisit *active.Visit) bool {
	if action != "checked_out" {
		return false
	}
	if student.GroupID == nil || currentVisit == nil || currentVisit.ActiveGroup == nil {
		return false
	}
	return rs.shouldShowDailyCheckoutWithGroup(ctx, student, currentVisit)
}

// isAfterCheckoutTimeGate checks whether the current time is past the applicable
// checkout time gate for this student. When per-student checkout is enabled and
// the student has a pickup time, uses pickup_time - delta. Otherwise falls back
// to the global daily checkout time.
func (rs *Resource) isAfterCheckoutTimeGate(ctx context.Context, student *users.Student) bool {
	// Check per-student mode (new setting, no env var fallback needed)
	perStudentEnabled := false
	if rs.SettingsService != nil {
		if val, err := rs.SettingsService.ResolveBool(ctx, configModel.KeyPerStudentCheckoutEnabled); err == nil {
			perStudentEnabled = val
		}
	}

	if perStudentEnabled && rs.PickupScheduleService != nil {
		now := timeNow()
		effectivePickup, err := rs.PickupScheduleService.GetEffectivePickupTimeForDate(ctx, student.ID, timezone.DateFromTime(now))
		if err == nil && effectivePickup != nil && effectivePickup.PickupTime != nil {
			// Student has a pickup time — use it with the delta
			delta := 15
			if rs.SettingsService != nil {
				if val, err := rs.SettingsService.ResolveInt(ctx, configModel.KeyPerStudentCheckoutDeltaMinutes); err == nil {
					delta = val
				}
			}

			todayPickup := time.Date(now.Year(), now.Month(), now.Day(),
				effectivePickup.PickupTime.Hour(), effectivePickup.PickupTime.Minute(),
				0, 0, now.Location())
			threshold := todayPickup.Add(-time.Duration(delta) * time.Minute)
			return now.After(threshold)
		}
		// No pickup time for this student — fall through to global check
	}

	// Global checkout time check (existing behavior)
	checkoutTime, err := rs.getStudentDailyCheckoutTime(ctx)
	if err != nil {
		return false
	}
	if checkoutTime == nil {
		return true // No time gate — always available
	}
	return timeNow().After(*checkoutTime)
}

// shouldShowDailyCheckoutWithGroup checks if daily checkout should be shown by verifying education group room
func (rs *Resource) shouldShowDailyCheckoutWithGroup(ctx context.Context, student *users.Student, currentVisit *active.Visit) bool {
	if student.GroupID == nil {
		return false
	}
	if currentVisit == nil || currentVisit.ActiveGroup == nil {
		return false
	}

	if !rs.isAfterCheckoutTimeGate(ctx, student) {
		return false
	}

	educationGroup, err := rs.EducationService.GetGroup(ctx, *student.GroupID)
	if err != nil || educationGroup == nil {
		return false
	}

	// If the group has no room assigned, daily checkout is available from any room
	if educationGroup.RoomID == nil {
		return true
	}

	return currentVisit.ActiveGroup.RoomID == *educationGroup.RoomID
}
