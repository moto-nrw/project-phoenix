package studentpresencehttp

import (
	json "encoding/json"
	errors "errors"
	fmt "fmt"
	http "net/http"
	time "time"

	common "github.com/moto-nrw/project-phoenix/api/common"
	studentpresence "github.com/moto-nrw/project-phoenix/modules/studentpresence"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	tenant "github.com/moto-nrw/project-phoenix/tenant"
)

// Bounds of one presence history request. The demo seeder sends a school's
// days in chunks below them; a full chunk stays well under the body limit.
const (
	maxDemoPresenceHistoryDays  = 1000
	maxDemoPresenceHistoryBytes = 1 << 20
)

// DemoPresenceHistoryRequest is the body of POST
// /operator/schools/{id}/demo-presence-history.
type DemoPresenceHistoryRequest struct {
	Days []DemoPresenceHistoryDay `json:"days"`
}

// DemoPresenceHistoryDay is one finished care day of one child.
type DemoPresenceHistoryDay struct {
	StudentID int64                     `json:"student_id"`
	Date      calendar.Date             `json:"date"`
	DeviceID  int64                     `json:"device_id"`
	CheckIn   time.Time                 `json:"check_in"`
	CheckOut  time.Time                 `json:"check_out"`
	Visit     *DemoPresenceHistoryVisit `json:"visit,omitempty"`
}

// DemoPresenceHistoryVisit is the room visit of a day inside its session.
type DemoPresenceHistoryVisit struct {
	ActivityGroupID int64     `json:"activity_group_id"`
	RoomID          int64     `json:"room_id"`
	SessionStart    time.Time `json:"session_start"`
	SessionEnd      time.Time `json:"session_end"`
	Entry           time.Time `json:"entry"`
	Exit            time.Time `json:"exit"`
}

// DemoPresenceHistoryResponse reports how many days the request recorded.
type DemoPresenceHistoryResponse struct {
	Written int `json:"written"`
}

// DemoPresenceHistoryHandler lets the public demo's seeder record the past
// care days of one demo school (ADR 0127). The root mounts it on the operator
// routes under APP_ENV=demo only; the school in the path is the tenant the
// days are written in, in one transaction of that school.
func DemoPresenceHistoryHandler(history studentpresence.PresenceHistory) http.HandlerFunc {
	if history == nil {
		panic("student presence http: the demo presence history needs the presence history writer")
	}
	return func(w http.ResponseWriter, r *http.Request) {
		schoolID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid school ID")
		if !ok {
			return
		}
		days, err := decodeDemoPresenceHistory(w, r)
		if err != nil {
			renderError(w, r, common.OperatorInvalidRequest(err))
			return
		}
		written, err := history.RecordPresenceHistory(tenant.WithTenantID(r.Context(), schoolID), days)
		if errors.Is(err, studentpresence.ErrPresenceHistoryInvalid) {
			renderError(w, r, common.OperatorInvalidRequest(err))
			return
		}
		if err != nil {
			renderError(w, r, common.ErrorInternalServerWrap("record presence history", err))
			return
		}
		common.Respond(w, r, http.StatusOK, DemoPresenceHistoryResponse{Written: written}, "Presence history recorded")
	}
}

func decodeDemoPresenceHistory(w http.ResponseWriter, r *http.Request) ([]studentpresence.PastPresenceDay, error) {
	var body DemoPresenceHistoryRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxDemoPresenceHistoryBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		return nil, fmt.Errorf("invalid presence history: %w", err)
	}
	if len(body.Days) > maxDemoPresenceHistoryDays {
		return nil, fmt.Errorf("presence history holds %d days, at most %d per request", len(body.Days), maxDemoPresenceHistoryDays)
	}
	days := make([]studentpresence.PastPresenceDay, 0, len(body.Days))
	for _, day := range body.Days {
		past := studentpresence.PastPresenceDay{StudentID: day.StudentID, Date: day.Date, DeviceID: day.DeviceID, CheckIn: day.CheckIn, CheckOut: day.CheckOut}
		if visit := day.Visit; visit != nil {
			past.Visit = &studentpresence.PastVisit{
				ActivityGroupID: visit.ActivityGroupID, RoomID: visit.RoomID,
				SessionStart: visit.SessionStart, SessionEnd: visit.SessionEnd,
				Entry: visit.Entry, Exit: visit.Exit,
			}
		}
		days = append(days, past)
	}
	return days, nil
}
