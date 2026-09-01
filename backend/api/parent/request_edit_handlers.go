package parent

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
)

// Guardian edit of an open request (#2267, stories 37-38). A guardian who
// picked the wrong day corrects the request instead of withdrawing it and
// filing a new one: the request keeps its id, the co-guardians it was shared
// with stay recipients, and the ledger records the change.
//
// Every body carries expected_version — the value the list emitted. An empty
// version still passes (older clients), a mismatching one is refused with 409
// change_request_stale so an edit can never overwrite a newer state blind.

// EditExcusedRequestBody is the wire shape of
// PUT /parent/me/children/{studentId}/excused-requests/{requestId}.
// The absence kind is not editable: sick and excused can be gated
// differently, so changing it is a new request.
type EditExcusedRequestBody struct {
	Dates           []string `json:"dates"`
	Note            string   `json:"note"`
	ExpectedVersion string   `json:"expected_version"`
}

// EditPickupChangeRequestBody is the wire shape of
// PUT /parent/me/children/{studentId}/pickup-change-requests/{requestId}.
type EditPickupChangeRequestBody struct {
	Date            string `json:"date"`
	PickupTime      string `json:"pickup_time"`
	Reason          string `json:"reason"`
	ExpectedVersion string `json:"expected_version"`
}

// EditCareScheduleRequestBody is the wire shape of
// PUT /parent/me/children/{studentId}/care-schedule/requests/{requestId}.
type EditCareScheduleRequestBody struct {
	Payload         map[string]any `json:"payload"`
	ExpectedVersion string         `json:"expected_version"`
}

// EditMasterDataRequestBody is the wire shape of
// PUT /parent/me/children/{studentId}/master-data/requests/{requestId}.
// Target and field stay fixed — they are properties of the request, not of
// the edit.
type EditMasterDataRequestBody struct {
	NewValue        json.RawMessage `json:"new_value"`
	ExpectedVersion string          `json:"expected_version"`
}

// EditOfferingChangeRequestBody is the wire shape of
// PUT /parent/me/children/{studentId}/care-offerings/requests/{requestId}.
type EditOfferingChangeRequestBody struct {
	OfferingChangeRequestBody
	ExpectedVersion string `json:"expected_version"`
}

// ParentRequestEventResponse is one line of a request's history for the
// "Geändert am …" hint in the parents portal.
type ParentRequestEventResponse struct {
	EventType string `json:"event_type"`
	CreatedAt string `json:"created_at"`
	Version   string `json:"version"`
}

// renderParentRequestError adds the shared stale code to the parent write
// error mapping. It stays a separate function so the big write-error switch
// keeps its shape.
// It also maps the shared reason_required sentinel, which the reason policy
// (operations.parent_request_reason_policy) can raise on any submit or edit.
func renderParentRequestError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, usersService.ErrParentRequestStale):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "change_request_stale"))
	case errors.Is(err, usersService.ErrParentRequestReasonRequired):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "reason_required"))
	default:
		renderParentWriteError(w, r, err)
	}
}

// parseCreateRecipients reads the recipient choice that travels WITH a request
// creation (#2267, finding 4). Sharing used to be a second call, so a dropped
// or refused follow-up left the request visible to nobody the family picked.
// A non-numeric or duplicate id is refused before anything is written, with
// its own code so the portal can say which field is wrong.
func parseCreateRecipients(w http.ResponseWriter, r *http.Request, raw []string) ([]int64, bool) {
	ids, err := parseRecipientGuardianProfileIDs(raw)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(
			errors.New("recipient_guardian_profile_ids must be distinct numeric guardian profile ids"),
			"invalid_recipients"))
		return nil, false
	}
	return ids, true
}

// parentRequestEditTarget is the child + request the caller addressed, parsed
// once for every edit handler.
type parentRequestEditTarget struct {
	accountID int64
	studentID int64
	requestID int64
}

func (rs *Resource) parentRequestEditTarget(w http.ResponseWriter, r *http.Request) (parentRequestEditTarget, bool) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return parentRequestEditTarget{}, false
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return parentRequestEditTarget{}, false
	}
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "requestId", "invalid request ID")
	if !ok {
		return parentRequestEditTarget{}, false
	}
	return parentRequestEditTarget{accountID: accountID, studentID: studentID, requestID: requestID}, true
}

// decodeEditBody reads a JSON body into target, answering with the same
// "invalid request body" every other parent write uses.
func decodeEditBody(w http.ResponseWriter, r *http.Request, target any) bool {
	if err := json.NewDecoder(r.Body).Decode(target); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return false
	}
	return true
}

func (rs *Resource) editExcusedRequest(w http.ResponseWriter, r *http.Request) {
	target, ok := rs.parentRequestEditTarget(w, r)
	if !ok {
		return
	}
	var body EditExcusedRequestBody
	if !decodeEditBody(w, r, &body) {
		return
	}
	dates, err := parseSickNoteDates(body.Dates)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	row, err := rs.ParentService.EditExcusedRequest(
		r.Context(), target.accountID, target.studentID, target.requestID, dates, body.Note, body.ExpectedVersion,
	)
	if err != nil {
		renderParentRequestError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toParentExcusedRequestResponse(row, target.accountID), "Request updated")
}

func (rs *Resource) editPickupChangeRequest(w http.ResponseWriter, r *http.Request) {
	target, ok := rs.parentRequestEditTarget(w, r)
	if !ok {
		return
	}
	var body EditPickupChangeRequestBody
	if !decodeEditBody(w, r, &body) {
		return
	}
	date, err := timezone.ParseDate(strings.TrimSpace(body.Date))
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid date, expected YYYY-MM-DD")))
		return
	}
	pickup, err := parseCareExceptionTime(&body.PickupTime)
	if err != nil || pickup == nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid pickup_time, expected HH:MM")))
		return
	}
	row, err := rs.ParentService.EditPickupChangeRequest(
		r.Context(), target.accountID, target.studentID, target.requestID,
		date, *pickup, body.Reason, body.ExpectedVersion,
	)
	if err != nil {
		renderParentRequestError(w, r, err)
		return
	}
	response, err := toPickupChangeRequestResponse(row, target.accountID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, response, "Request updated")
}

func (rs *Resource) editCareScheduleRequest(w http.ResponseWriter, r *http.Request) {
	target, ok := rs.parentRequestEditTarget(w, r)
	if !ok {
		return
	}
	var body EditCareScheduleRequestBody
	if !decodeEditBody(w, r, &body) {
		return
	}
	view, err := rs.ParentService.EditCareScheduleRequest(
		r.Context(), target.accountID, target.studentID, target.requestID, body.Payload, body.ExpectedVersion,
	)
	if err != nil {
		renderParentRequestError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toCareScheduleResponse(view), "Request updated")
}

func (rs *Resource) editMasterDataRequest(w http.ResponseWriter, r *http.Request) {
	target, ok := rs.parentRequestEditTarget(w, r)
	if !ok {
		return
	}
	var body EditMasterDataRequestBody
	if !decodeEditBody(w, r, &body) {
		return
	}
	if len(body.NewValue) == 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("new_value is required")))
		return
	}
	row, err := rs.ParentService.EditMasterDataRequest(
		r.Context(), target.accountID, target.studentID, target.requestID, body.NewValue, body.ExpectedVersion,
	)
	if err != nil {
		renderParentRequestError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK,
		toMasterDataChangeResponses([]*usersModels.StudentDataChangeRequest{row}, target.accountID)[0], "Request updated")
}

func (rs *Resource) editOfferingChangeRequest(w http.ResponseWriter, r *http.Request) {
	target, ok := rs.parentRequestEditTarget(w, r)
	if !ok {
		return
	}
	var body EditOfferingChangeRequestBody
	if !decodeEditBody(w, r, &body) {
		return
	}
	effectiveFrom, err := timezone.ParseDate(strings.TrimSpace(body.EffectiveFrom))
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(
			errors.New("effective_from must be a date in YYYY-MM-DD form"), "offering_change_invalid"))
		return
	}
	selections := make([]enrollmentService.OfferingChangeSelection, 0, len(body.Offerings))
	for _, entry := range body.Offerings {
		offeringID, convErr := strconv.ParseInt(strings.TrimSpace(entry.OfferingID), 10, 64)
		if convErr != nil || offeringID <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequestWithCode(
				errors.New("offering_id must be a numeric id"), "offering_change_invalid"))
			return
		}
		selections = append(selections, enrollmentService.OfferingChangeSelection{
			OfferingID:   offeringID,
			SelectedDays: entry.SelectedDays,
		})
	}
	view, err := rs.ParentService.EditOfferingChangeRequest(
		r.Context(), target.accountID, target.studentID, target.requestID,
		selections, effectiveFrom, body.Note, body.CompleteWithdrawalConfirmed, body.ExpectedVersion,
	)
	if err != nil {
		renderParentRequestError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toCareOfferingsResponse(view), "Request updated")
}

// listRequestEvents returns one request's history to the guardian who
// submitted it. A co-guardian the request was shared with gets the same
// not-found a stranger does: the pill tells them what happened, the edit trail
// belongs to the author.
func (rs *Resource) listRequestEvents(w http.ResponseWriter, r *http.Request) {
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
	requestType := strings.TrimSpace(chi.URLParam(r, "requestType"))
	rows, err := rs.ParentService.ListRequestEvents(r.Context(), accountID, studentID, requestType, requestID)
	if err != nil {
		renderRequestSharingError(w, r, err)
		return
	}
	out := make([]ParentRequestEventResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, ParentRequestEventResponse{
			EventType: row.EventType,
			CreatedAt: row.CreatedAt.UTC().Format(time.RFC3339),
			Version:   row.Version,
		})
	}
	common.Respond(w, r, http.StatusOK, out, "Request history retrieved")
}
