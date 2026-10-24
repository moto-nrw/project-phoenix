package parent

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"

	"github.com/moto-nrw/project-phoenix/api/common"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
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
// /parent/me/children/{studentId}/care-exception. Parent requests carry a
// pickup time, an Abholart for the day (#3945), or both, and a short
// explanation for the school team. ArrivalTime is retained only to reject
// outdated clients explicitly.
type SubmitCareExceptionRequest struct {
	Date          string  `json:"date"`           // YYYY-MM-DD
	PickupTime    *string `json:"pickup_time"`    // HH:MM
	DepartureMode *string `json:"departure_mode"` // alone | bus | pickup
	ArrivalTime   *string `json:"arrival_time"`   // Unsupported for parents
	Reason        string  `json:"reason"`
	// RecipientGuardianProfileIDs travel with the creation so the share is
	// written in the same transaction (#2267); empty shares with nobody.
	RecipientGuardianProfileIDs []string `json:"recipient_guardian_profile_ids"`
	// PickupPersonGuardianProfileID names a contact person who collects the
	// child that day; Signature is needed for a collector or walking home
	// alone (#4780).
	PickupPersonGuardianProfileID *string                      `json:"pickup_person_guardian_profile_id"`
	Signature                     *declarationSignatureRequest `json:"signature,omitempty"`
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
	// DepartureMode is the Abholart of the day the OGS set or approved
	// (#3945); omitted when the weekly plan applies.
	DepartureMode *string `json:"departure_mode,omitempty"`
}

// PickupChangeRequestResponse carries the stored ask. PickupTime and
// DepartureMode are null for a leg the request leaves unchanged (#3945).
type PickupChangeRequestResponse struct {
	ID                     string     `json:"id"`
	Date                   string     `json:"date"`
	PickupTime             *string    `json:"pickup_time"`
	PreviousPickup         *string    `json:"previous_pickup_time,omitempty"`
	DepartureMode          *string    `json:"departure_mode"`
	PreviousDepartureModes []string   `json:"previous_departure_modes,omitempty"`
	Reason                 string     `json:"reason"`
	Status                 string     `json:"status"`
	DecisionReason         *string    `json:"decision_reason,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	ReviewedAt             *time.Time `json:"reviewed_at,omitempty"`
	IsSelf                 bool       `json:"is_self"`
	// PickupPerson is the one-day collector (#4780); Signed says the request
	// carries a signature, whose proof the submitter can open.
	PickupPerson *string `json:"pickup_person,omitempty"`
	Signed       bool    `json:"signed"`
}

func toPickupChangeRequestResponse(req *careplan.CareScheduleChangeRequest, accountID int64) (PickupChangeRequestResponse, error) {
	var payload map[string]any
	if err := json.Unmarshal(req.Payload, &payload); err != nil {
		return PickupChangeRequestResponse{}, errors.New("invalid pickup change request payload")
	}
	// The reason may be blank under a school's reason policy; a missing key
	// still marks a malformed row.
	reason, reasonOK := payload["reason"].(string)
	terms := carerequests.StoredPickupTerms(req.Payload)
	if !reasonOK || terms == nil {
		return PickupChangeRequestResponse{}, errors.New("invalid pickup change request payload")
	}
	response := PickupChangeRequestResponse{
		ID: strconv.FormatInt(req.ID, 10), Date: terms.Date.String(),
		PreviousDepartureModes: terms.PreviousDepartureModes, Reason: reason, Status: req.Status, DecisionReason: req.DecisionReason,
		CreatedAt: req.CreatedAt, ReviewedAt: req.ReviewedAt, IsSelf: req.SubmittedBy == accountID,
	}
	if terms.PickupTime != "" {
		response.PickupTime = &terms.PickupTime
	}
	if terms.PreviousPickupTime != "" {
		response.PreviousPickup = &terms.PreviousPickupTime
	}
	if terms.DepartureMode != "" {
		response.DepartureMode = &terms.DepartureMode
	}
	if terms.PickupPersonName != "" {
		response.PickupPerson = &terms.PickupPersonName
	}
	response.Signed = carerequests.StoredSignature(req.Payload) != nil
	return response, nil
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
		DepartureMode: c.DepartureMode,
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

	departureMode := ""
	if req.DepartureMode != nil {
		departureMode = strings.TrimSpace(*req.DepartureMode)
	}
	personID, err := parsePickupPersonID(req.PickupPersonGuardianProfileID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if pickup == nil && departureMode == "" && personID == 0 {
		renderParentWriteError(w, r, parentService.ErrNoCareException)
		return
	}
	recipients, ok := parseCreateRecipients(w, r, req.RecipientGuardianProfileIDs)
	if !ok {
		return
	}
	var pickupTime time.Time
	if pickup != nil {
		pickupTime = *pickup
	}
	result, err := rs.ParentService.SubmitPickupChange(r.Context(), accountID, studentID, parentService.PickupChangeSubmission{
		Date: date, PickupTime: pickupTime, DepartureMode: departureMode, Reason: req.Reason,
		RecipientGuardianProfileIDs: recipients, PickupPersonID: personID, Signature: req.Signature.pickupInput(),
	})
	if err != nil {
		renderParentRequestError(w, r, err)
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
		response, convertErr := toPickupChangeRequestResponse(&row, accountID)
		if convertErr != nil {
			common.RenderError(w, r, common.ErrorInternalServer(convertErr))
			return
		}
		out = append(out, response)
	}
	common.Respond(w, r, http.StatusOK, out, "Pickup change requests retrieved")
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
