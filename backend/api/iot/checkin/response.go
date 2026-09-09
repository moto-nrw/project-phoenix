package checkin

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

// buildCheckinResponse renders a detailed-mode scan. The key set is the
// PyrePortal contract: visit_id is always present (null without a visit),
// previous_room only for a transfer, active_students and pickup_time only
// when known.
func buildCheckinResponse(result *devicescan.ScanResult) map[string]any {
	response := map[string]any{
		"student_id":               result.PersonID,
		"student_name":             result.PersonName,
		"action":                   result.Action,
		"visit_id":                 result.VisitID,
		"room_name":                result.RoomName,
		"processed_at":             result.ProcessedAt,
		"message":                  result.Message,
		"status":                   "success",
		"daily_checkout_available": result.DailyCheckoutAvailable,
		"feedback_enabled":         result.FeedbackEnabled,
	}
	if result.Action == devicescan.ScanActionTransferred && result.PreviousRoomName != "" {
		response["previous_room"] = result.PreviousRoomName
	}
	if result.ActiveStudents != nil {
		response["active_students"] = *result.ActiveStudents
	}
	if result.PickupTime != nil {
		response["pickup_time"] = *result.PickupTime
	}
	return response
}

// buildBinaryCheckinResponse renders a binary-mode toggle in the detailed
// shape: room_name empty, visit_id omitted, both checkout flags false.
func buildBinaryCheckinResponse(result *devicescan.ScanResult) map[string]any {
	return map[string]any{
		"student_id":               result.PersonID,
		"student_name":             result.PersonName,
		"action":                   result.Action,
		"room_name":                "",
		"processed_at":             result.ProcessedAt,
		"message":                  result.Message,
		"status":                   "success",
		"daily_checkout_available": false,
		"feedback_enabled":         false,
	}
}

// buildSupervisorResponse renders a staff card that joined the session.
func buildSupervisorResponse(result *devicescan.ScanResult) map[string]any {
	return map[string]any{
		"student_id":   result.PersonID,
		"student_name": result.PersonName,
		"action":       result.Action,
		"room_name":    result.RoomName,
		"processed_at": result.ProcessedAt,
		"message":      result.Message,
		"status":       "success",
	}
}

// sendCheckinResponse sends the scan envelope.
func sendCheckinResponse(w http.ResponseWriter, r *http.Request, runtime Runtime, response map[string]any, action string) {
	runtime.Success(w, r, http.StatusOK, response, "Student "+action+" successfully")
}
