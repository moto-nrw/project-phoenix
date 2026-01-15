package checkin

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
)

// getStudentDailyCheckoutTime parses the daily checkout time from environment variable
func getStudentDailyCheckoutTime() (time.Time, error) {
	checkoutTimeStr := os.Getenv("STUDENT_DAILY_CHECKOUT_TIME")
	if checkoutTimeStr == "" {
		checkoutTimeStr = "15:00" // Default to 3:00 PM
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
	if action != activeService.StatusCheckedOut {
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

	checkoutTime, err := getStudentDailyCheckoutTime()
	if err != nil || !time.Now().After(checkoutTime) {
		return false
	}

	educationGroup, err := rs.EducationService.GetGroup(ctx, *student.GroupID)
	if err != nil || educationGroup == nil || educationGroup.RoomID == nil {
		return false
	}

	return currentVisit.ActiveGroup.RoomID == *educationGroup.RoomID
}

// isPendingDailyCheckoutScenario checks if this scan should trigger a pending daily checkout
// (deferred checkout that waits for user confirmation before processing).
// This is called BEFORE processCheckout() to determine if we should return early.
func (rs *Resource) isPendingDailyCheckoutScenario(ctx context.Context, student *users.Student, currentVisit *active.Visit) bool {
	// Check prerequisites
	if student.GroupID == nil || currentVisit == nil || currentVisit.ActiveGroup == nil {
		return false
	}

	// Check if time has passed daily checkout threshold
	checkoutTime, err := getStudentDailyCheckoutTime()
	if err != nil || !time.Now().After(checkoutTime) {
		return false
	}

	// Check if student's room matches education group room
	educationGroup, err := rs.EducationService.GetGroup(ctx, *student.GroupID)
	if err != nil || educationGroup == nil || educationGroup.RoomID == nil {
		return false
	}

	return currentVisit.ActiveGroup.RoomID == *educationGroup.RoomID
}

// handlePendingDailyCheckoutResponse sends the pending daily checkout response and returns true if handled.
// This helper reduces cognitive complexity in deviceCheckin by extracting the response building logic.
func handlePendingDailyCheckoutResponse(w http.ResponseWriter, r *http.Request, student *users.Student, person *users.Person, currentVisit *active.Visit) {
	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"student_id":   student.ID,
			"student_name": person.FirstName + " " + person.LastName,
		}).Debug("CHECKIN: Pending daily checkout - awaiting confirmation")
	}

	// Get room name for response
	roomName := getRoomNameFromVisit(currentVisit)

	// Build and send pending response
	response := map[string]interface{}{
		"student_id":   student.ID,
		"student_name": person.FirstName + " " + person.LastName,
		"action":       "pending_daily_checkout",
		"visit_id":     currentVisit.ID,
		"room_name":    roomName,
		"processed_at": time.Now(),
		"message":      "Gehst du nach Hause?",
		"status":       "success",
	}
	sendCheckinResponse(w, r, response, "pending_daily_checkout")
}
