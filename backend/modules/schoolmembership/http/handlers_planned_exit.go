package staff

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// PlannedExitRequest enters or moves the last working day of a staff member
// (#4809). The day is a calendar date, YYYY-MM-DD.
type PlannedExitRequest struct {
	LastWorkingDay string `json:"last_working_day"`
}

// PlannedExitResponse is the planned exit as stored.
type PlannedExitResponse struct {
	LastWorkingDay calendar.Date `json:"last_working_day"`
}

func (rs *Resource) planExit(w http.ResponseWriter, r *http.Request) {
	id, ok := rs.parseStaffID(w, r)
	if !ok {
		return
	}
	var request PlannedExitRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.LastWorkingDay == "" {
		rs.failure(w, r, FailureInvalidRequest, errors.New("last_working_day is required (YYYY-MM-DD)"), "invalid_request")
		return
	}
	day, err := calendar.ParseDate(request.LastWorkingDay)
	if err != nil {
		rs.failure(w, r, FailureInvalidRequest, errors.New("last_working_day is required (YYYY-MM-DD)"), "invalid_request")
		return
	}
	day, err = rs.runtime.PlanExit(r.Context(), id, day, rs.runtime.CurrentUsername(r.Context()))
	if err != nil {
		rs.runtime.ExitFailure(w, r, err)
		return
	}
	rs.respond(w, r, http.StatusOK, PlannedExitResponse{LastWorkingDay: day}, "Austritt gespeichert")
}

func (rs *Resource) withdrawExit(w http.ResponseWriter, r *http.Request) {
	id, ok := rs.parseStaffID(w, r)
	if !ok {
		return
	}
	if err := rs.runtime.WithdrawExit(r.Context(), id, rs.runtime.CurrentUsername(r.Context())); err != nil {
		rs.runtime.ExitFailure(w, r, err)
		return
	}
	rs.respond(w, r, http.StatusOK, nil, "Austritt zurückgenommen")
}
