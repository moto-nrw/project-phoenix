package parent

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
)

// careExceptionTimeLayout is the wall-clock layout parents send (HH:MM).
const careExceptionTimeLayout = "15:04"

// careExceptionTimeRefDate anchors a parsed wall-clock time to the same
// reference date the staff schedule handlers use, so the value round-trips
// through the TIME column unchanged.
const careExceptionTimeRefDate = "2000-01-01"

// parseCareExceptionTime turns an optional "HH:MM" string into the reference-
// date-anchored instant stored in the TIME column. A nil/empty input yields a
// nil time meaning "this leg is not being changed".
func parseCareExceptionTime(raw *string) (*time.Time, error) {
	if raw == nil {
		return nil, nil
	}
	if *raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(careExceptionTimeLayout+" 2006-01-02", *raw+" "+careExceptionTimeRefDate)
	if err != nil {
		return nil, errors.New("invalid time format, expected HH:MM")
	}
	return &parsed, nil
}

// formatCareExceptionTime renders a stored wall-clock instant back to "HH:MM".
func formatCareExceptionTime(t *time.Time) *string {
	if t == nil {
		return nil
	}
	formatted := t.Format(careExceptionTimeLayout)
	return &formatted
}

// SubmitCareExceptionRequest is the wire shape for POST
// /parent/me/children/{studentId}/care-exception. Parent requests require a
// pickup time and a short explanation for the school team. ArrivalTime is
// retained only to reject outdated clients explicitly.
type SubmitCareExceptionRequest struct {
	Date        string  `json:"date"`         // YYYY-MM-DD
	PickupTime  *string `json:"pickup_time"`  // HH:MM, required
	ArrivalTime *string `json:"arrival_time"` // Unsupported for parents
	Reason      string  `json:"reason"`
}

// CareExceptionResponse is one day's merged pickup/arrival override, projected
// for the parent UI: stringified times, date as YYYY-MM-DD.
type CareExceptionResponse struct {
	Date         string    `json:"date"`
	PickupTime   *string   `json:"pickup_time,omitempty"`
	ArrivalTime  *string   `json:"arrival_time,omitempty"`
	Reason       *string   `json:"reason,omitempty"`
	Source       string    `json:"source"`
	PickupSource string    `json:"pickup_source"`
	UpdatedAt    time.Time `json:"updated_at"`
	// PickupAbsent marks a staff "not coming today" pickup exception (a pickup
	// row with no time). The parent tile resolves it as an absence instead of
	// falling back to the base-plan pickup. Omitted (false) for ordinary rows,
	// so existing consumers are unaffected (#1725 review).
	PickupAbsent bool `json:"pickup_absent,omitempty"`
	// ArrivalAbsent is the arrival-leg counterpart: an arrival exception with no
	// expected time ("not coming today"). Either leg being absent resolves the
	// tile to an absence, so a guardian is never told to expect a pickup for a
	// child who is not coming. Omitted (false) for ordinary rows (#1725 review).
	ArrivalAbsent bool `json:"arrival_absent,omitempty"`
}

type PickupChangeRequestResponse struct {
	ID             string     `json:"id"`
	Date           string     `json:"date"`
	PickupTime     string     `json:"pickup_time"`
	PreviousPickup *string    `json:"previous_pickup_time,omitempty"`
	Reason         string     `json:"reason"`
	Status         string     `json:"status"`
	DecisionReason *string    `json:"decision_reason,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ReviewedAt     *time.Time `json:"reviewed_at,omitempty"`
	IsSelf         bool       `json:"is_self"`
}

func toPickupChangeRequestResponse(req *scheduleModels.CareScheduleChangeRequest, accountID int64) (PickupChangeRequestResponse, error) {
	date, dateOK := req.Payload["date"].(string)
	pickup, pickupOK := req.Payload["pickup_time"].(string)
	reason, reasonOK := req.Payload["reason"].(string)
	if !dateOK || !pickupOK || !reasonOK {
		return PickupChangeRequestResponse{}, errors.New("invalid pickup change request payload")
	}
	var previousPickup *string
	if value, ok := req.Payload["previous_pickup_time"].(string); ok && value != "" {
		previousPickup = &value
	}
	return PickupChangeRequestResponse{
		ID: strconv.FormatInt(req.ID, 10), Date: date, PickupTime: pickup,
		PreviousPickup: previousPickup, Reason: reason, Status: req.Status, DecisionReason: req.DecisionReason,
		CreatedAt: req.CreatedAt, ReviewedAt: req.ReviewedAt, IsSelf: req.SubmittedBy == accountID,
	}, nil
}

func toCareExceptionResponse(c *parentService.CareException) CareExceptionResponse {
	return CareExceptionResponse{
		Date:          c.Date.String(),
		PickupTime:    formatCareExceptionTime(c.PickupTime),
		ArrivalTime:   formatCareExceptionTime(c.ArrivalTime),
		Reason:        c.Reason,
		Source:        c.Source,
		PickupSource:  c.PickupSource,
		UpdatedAt:     c.UpdatedAt,
		PickupAbsent:  c.PickupAbsent,
		ArrivalAbsent: c.ArrivalAbsent,
	}
}

// submitCareException sets a one-day pickup override for the parent's child.
// The service verifies the relationship-level pickup permission.
func (rs *Resource) submitCareException(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}

	var req SubmitCareExceptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}

	date, err := timezone.ParseDate(req.Date)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid date, expected YYYY-MM-DD")))
		return
	}
	pickup, err := parseCareExceptionTime(req.PickupTime)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if req.ArrivalTime != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("parents cannot change arrival times")))
		return
	}

	if pickup == nil {
		renderParentWriteError(w, r, parentService.ErrNoCareException)
		return
	}
	result, err := rs.ParentService.SubmitPickupChangeRequest(r.Context(), accountID, studentID, date, *pickup, req.Reason)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	response, err := toPickupChangeRequestResponse(result, accountID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusCreated, response, "Pickup change request submitted")
}

func (rs *Resource) listPickupChangeRequests(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	rows, err := rs.ParentService.ListPickupChangeRequests(r.Context(), accountID, studentID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	out := make([]PickupChangeRequestResponse, 0, len(rows))
	for _, row := range rows {
		response, convertErr := toPickupChangeRequestResponse(row, accountID)
		if convertErr != nil {
			common.RenderError(w, r, common.ErrorInternalServer(convertErr))
			return
		}
		out = append(out, response)
	}
	common.Respond(w, r, http.StatusOK, out, "Pickup change requests retrieved")
}

func (rs *Resource) withdrawPickupChangeRequest(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "requestId", "invalid request id")
	if !ok {
		return
	}
	result, err := rs.ParentService.WithdrawPickupChangeRequest(r.Context(), accountID, studentID, requestID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	response, err := toPickupChangeRequestResponse(result, accountID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, response, "Pickup change request withdrawn")
}

// listCareExceptions returns the child's pickup/arrival overrides in the
// requested range (defaults to today .. +2 months).
func (rs *Resource) listCareExceptions(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}

	from, to, err := parseSickDayRange(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	rows, err := rs.ParentService.ListCareExceptions(r.Context(), accountID, studentID, from, to)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}

	out := make([]CareExceptionResponse, 0, len(rows))
	for _, c := range rows {
		out = append(out, toCareExceptionResponse(c))
	}
	common.Respond(w, r, http.StatusOK, out, "Care exceptions retrieved")
}

// deleteCareException removes the parent-authored override for a single date,
// reverting the day to the standard weekly plan. The date is a required query
// parameter.
func (rs *Resource) deleteCareException(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}

	date, err := timezone.ParseDate(r.URL.Query().Get("date"))
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid date, expected YYYY-MM-DD")))
		return
	}

	if err := rs.ParentService.DeleteCareException(r.Context(), accountID, studentID, date); err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Care exception removed")
}
