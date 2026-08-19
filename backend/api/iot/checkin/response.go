package checkin

import (
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/models/users"
	checkinSvc "github.com/moto-nrw/project-phoenix/services/iot/checkin"
)

// buildCheckinResponse builds the final checkin response map from the service
// result. Response building stays in the handler layer (issue #575 B8).
func buildCheckinResponse(student *users.Student, result *checkinSvc.CheckinResult, now time.Time) map[string]interface{} {
	studentName := student.Person.FirstName + " " + student.Person.LastName

	response := map[string]interface{}{
		"student_id":               student.ID,
		"student_name":             studentName,
		"action":                   result.Action,
		"visit_id":                 result.VisitID,
		"room_name":                result.RoomName,
		"processed_at":             now,
		"message":                  result.GreetingMsg,
		"status":                   "success",
		"daily_checkout_available": result.DailyCheckoutAvailable,
	}

	if result.Action == "transferred" && result.PreviousRoomName != "" {
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

// sendCheckinResponse sends the final response.
func sendCheckinResponse(w http.ResponseWriter, r *http.Request, response map[string]interface{}, action string) {
	common.Respond(w, r, http.StatusOK, response, "Student "+action+" successfully")
}
