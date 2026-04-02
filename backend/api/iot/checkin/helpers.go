package checkin

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// getStudentDailyCheckoutTime resolves the daily checkout time.
// Fallback chain: tenant DB override → STUDENT_DAILY_CHECKOUT_TIME env var → "15:00".
func (rs *Resource) getStudentDailyCheckoutTime(ctx context.Context) (time.Time, error) {
	checkoutTimeStr := ""

	// Try tenant DB override first (only if an explicit override exists)
	if rs.SettingsService != nil {
		if has, err := rs.SettingsService.HasTenantOverride(ctx, configModel.KeyStudentDailyCheckoutTime); err != nil {
			slog.Warn("settings override check failed, falling back to env var",
				slog.String("key", configModel.KeyStudentDailyCheckoutTime),
				slog.String("error", err.Error()),
			)
		} else if has {
			if val, err := rs.SettingsService.ResolveString(ctx, configModel.KeyStudentDailyCheckoutTime); err == nil && val != "" {
				checkoutTimeStr = val
			}
		}
	}

	// Fall back to env var
	if checkoutTimeStr == "" {
		checkoutTimeStr = os.Getenv("STUDENT_DAILY_CHECKOUT_TIME")
	}

	// Fall back to default
	if checkoutTimeStr == "" {
		checkoutTimeStr = "15:00"
	}

	// Parse time in HH:MM format
	parts := strings.Split(checkoutTimeStr, ":")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid checkout time format: %s", checkoutTimeStr)
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return time.Time{}, fmt.Errorf("invalid hour in checkout time: %s", checkoutTimeStr)
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return time.Time{}, fmt.Errorf("invalid minute in checkout time: %s", checkoutTimeStr)
	}

	now := time.Now()
	checkoutTime := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	return checkoutTime, nil
}

// getRoomNameFromVisit extracts the room name from a visit's active group if available.
func getRoomNameFromVisit(visit *active.Visit) string {
	if visit != nil && visit.ActiveGroup != nil && visit.ActiveGroup.Room != nil {
		return visit.ActiveGroup.Room.Name
	}
	return ""
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

// shouldShowDailyCheckoutWithGroup checks if daily checkout should be shown by verifying education group room
func (rs *Resource) shouldShowDailyCheckoutWithGroup(ctx context.Context, student *users.Student, currentVisit *active.Visit) bool {
	if student.GroupID == nil {
		return false
	}
	if currentVisit == nil || currentVisit.ActiveGroup == nil {
		return false
	}

	checkoutTime, err := rs.getStudentDailyCheckoutTime(ctx)
	if err != nil || !time.Now().After(checkoutTime) {
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
