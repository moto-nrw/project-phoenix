package parent

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// CareScheduleResponse is the parent-facing weekly care plan: per weekday the
// current arrival/pickup wall-clock times (HH:MM, empty = not set) and the
// allowed departure-mode keys, plus the child's pending change request (nil
// when none) and whether the calling guardian may request a change.
type CareScheduleResponse struct {
	Weekdays            []CareScheduleWeekdayResponse           `json:"weekdays"`
	PendingRequest      *PendingCareRequestResponse             `json:"pending_request,omitempty"`
	CanRequest          bool                                    `json:"can_request"`
	RequestCapabilities CareScheduleRequestCapabilitiesResponse `json:"request_capabilities"`
	// TodayAbsent is true when the child has any active scheduled absence today
	// (sick / excused / class trip, any source). Parent-safe boolean only — the
	// "Heute → Abholung" tile uses it so it never shows a pickup time for a child
	// the school has recorded as off today. Only populated by the GET read view.
	TodayAbsent bool `json:"today_absent"`
	// TodayDate is the Berlin calendar day (YYYY-MM-DD) TodayAbsent was resolved
	// against — the server's "today" at request time. The client binds its
	// cached absence signal to this date instead of the browser's request-start
	// day so a response crossing Berlin midnight can't stamp the new day's
	// today_absent onto yesterday's pickup tile (#1725 review). Empty on the
	// write views (POST/withdraw), which don't populate TodayAbsent.
	TodayDate string `json:"today_date,omitempty"`
}

type CareScheduleRequestCapabilitiesResponse struct {
	Arrival       bool `json:"arrival"`
	Pickup        bool `json:"pickup"`
	DepartureMode bool `json:"departure_mode"`
}

// CareScheduleWeekdayResponse is one weekday (1=Mon..5=Fri) of the plan.
type CareScheduleWeekdayResponse struct {
	Weekday int      `json:"weekday"`
	Status  string   `json:"status"`
	Arrival string   `json:"arrival,omitempty"`
	Pickup  string   `json:"pickup,omitempty"`
	Modes   []string `json:"modes"`
}

// PendingCareRequestResponse is the child's open change request on the read
// view, with the same "current → requested" diff staff see.
type PendingCareRequestResponse struct {
	ID              string                    `json:"id"`
	CreatedAt       string                    `json:"created_at"`
	Diff            []CareRequestDiffResponse `json:"diff"`
	SubmittedBySelf bool                      `json:"submitted_by_self"`
}

// CareRequestDiffResponse mirrors the messaging diff wire shape so the
// frontend's RequestDiffPanel renders it unchanged.
type CareRequestDiffResponse struct {
	Label    string   `json:"label"`
	Old      string   `json:"old"`
	New      string   `json:"new"`
	Weekday  int      `json:"weekday,omitempty"`
	CareKind string   `json:"care_kind,omitempty"`
	OldModes []string `json:"old_modes,omitempty"`
	NewMode  string   `json:"new_mode,omitempty"`
}

// CareScheduleRequestBody is the wire shape for POST .../care-schedule/requests.
type CareScheduleRequestBody struct {
	Payload map[string]any `json:"payload"`
}

func toCareRequestDiffResponses(entries []scheduleService.RequestDiffEntry) []CareRequestDiffResponse {
	out := make([]CareRequestDiffResponse, 0, len(entries))
	for _, e := range entries {
		out = append(out, CareRequestDiffResponse{
			Label:    e.Label,
			Old:      e.Old,
			New:      e.New,
			Weekday:  e.Weekday,
			CareKind: e.CareKind,
			OldModes: e.OldModes,
			NewMode:  e.NewMode,
		})
	}
	return out
}

func toCareScheduleResponse(v *parentService.ChildCareSchedule) CareScheduleResponse {
	resp := CareScheduleResponse{
		Weekdays:   make([]CareScheduleWeekdayResponse, 0, len(v.Weekdays)),
		CanRequest: v.CanRequest,
		RequestCapabilities: CareScheduleRequestCapabilitiesResponse{
			Arrival:       v.RequestCapabilities.Arrival,
			Pickup:        v.RequestCapabilities.Pickup,
			DepartureMode: v.RequestCapabilities.DepartureMode,
		},
		TodayAbsent: v.TodayAbsent,
	}
	// Only the read view resolves today_absent, so only it carries a resolved
	// date; leave the write views' today_date empty (omitempty) rather than
	// emitting the zero Date's "0000-00-00" (#1725 review).
	if !v.TodayDate.IsZero() {
		resp.TodayDate = v.TodayDate.String()
	}
	for _, wd := range v.Weekdays {
		resp.Weekdays = append(resp.Weekdays, CareScheduleWeekdayResponse{
			Weekday: wd.Weekday,
			Status:  string(wd.Status),
			Arrival: wd.Arrival,
			Pickup:  wd.Pickup,
			Modes:   wd.Modes,
		})
	}
	if v.PendingRequest != nil {
		resp.PendingRequest = &PendingCareRequestResponse{
			ID:              strconv.FormatInt(v.PendingRequest.ID, 10),
			CreatedAt:       v.PendingRequest.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			Diff:            toCareRequestDiffResponses(v.PendingRequest.Diff),
			SubmittedBySelf: v.PendingRequest.SubmittedBySelf,
		}
	}
	return resp
}

// getChildCareSchedule returns the child's permanent weekly care plan.
func (rs *Resource) getChildCareSchedule(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	view, err := rs.ParentService.GetChildCareSchedule(r.Context(), accountID, studentID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toCareScheduleResponse(view), "Care schedule retrieved")
}

// createCareScheduleRequest stores a pending weekly-plan change request.
func (rs *Resource) createCareScheduleRequest(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	var body CareScheduleRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}
	view, err := rs.ParentService.CreateCareScheduleRequest(r.Context(), accountID, studentID, body.Payload)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toCareScheduleResponse(view), "Care schedule request submitted")
}

// withdrawCareScheduleRequest flips the caller's own pending request to
// withdrawn.
func (rs *Resource) withdrawCareScheduleRequest(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "requestId", "invalid request ID")
	if !ok {
		return
	}
	view, svcErr := rs.ParentService.WithdrawCareScheduleRequest(r.Context(), accountID, studentID, requestID)
	if svcErr != nil {
		renderParentWriteError(w, r, svcErr)
		return
	}
	common.Respond(w, r, http.StatusOK, toCareScheduleResponse(view), "Care schedule request withdrawn")
}
