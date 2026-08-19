package students

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	absenceService "github.com/moto-nrw/project-phoenix/services/absence"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

// History pages default to a screenful and cap the page size against abusive
// limits; the frontend loads more via the cursor.
const (
	historyDefaultLimit = 25
	historyMaxLimit     = 100
)

// historyCursorPayload is the opaque wire form of a history cursor:
// base64url-encoded JSON of the keyset position. Opaque so clients cannot
// build one by hand and the encoding may change.
type historyCursorPayload struct {
	UpdatedAt time.Time `json:"u"`
	ID        int64     `json:"i"`
}

func encodeHistoryCursor(next *userService.HistoryCursor) string {
	if next == nil {
		return ""
	}
	raw, err := json.Marshal(historyCursorPayload{UpdatedAt: next.UpdatedAt, ID: next.ID})
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

var errInvalidHistoryQuery = errors.New("invalid history query")

// parseHistoryQuery reads the shared ?cursor= and ?limit= params of the four
// history endpoints. A zero beforeUpdatedAt means "first page".
func parseHistoryQuery(r *http.Request) (beforeUpdatedAt time.Time, beforeID int64, limit int, err error) {
	limit = historyDefaultLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		n, convErr := strconv.Atoi(raw)
		if convErr != nil || n <= 0 {
			return time.Time{}, 0, 0, errInvalidHistoryQuery
		}
		limit = min(n, historyMaxLimit)
	}
	if raw := r.URL.Query().Get("cursor"); raw != "" {
		decoded, decErr := base64.RawURLEncoding.DecodeString(raw)
		if decErr != nil {
			return time.Time{}, 0, 0, errInvalidHistoryQuery
		}
		var payload historyCursorPayload
		if jsonErr := json.Unmarshal(decoded, &payload); jsonErr != nil || payload.UpdatedAt.IsZero() || payload.ID <= 0 {
			return time.Time{}, 0, 0, errInvalidHistoryQuery
		}
		beforeUpdatedAt = payload.UpdatedAt
		beforeID = payload.ID
	}
	return beforeUpdatedAt, beforeID, limit, nil
}

// historyPageResponse is the cursor envelope every history endpoint returns.
type historyPageResponse[T any] struct {
	Items []T `json:"items"`
	// NextCursor is absent on the last page.
	NextCursor string `json:"next_cursor,omitempty"`
}

// historyDecidedAt is the display "decided at" instant: reviewed_at when a
// reviewer stamped the decision, otherwise updated_at (withdrawn and
// auto-applied rows carry no reviewed_at, but every decide path stamps
// updated_at in the same UPDATE).
func historyDecidedAt(reviewedAt *time.Time, updatedAt time.Time) time.Time {
	if reviewedAt != nil {
		return *reviewedAt
	}
	return updatedAt
}

// MasterDataChangeRequestHistoryResponse extends the queue projection with the
// decision facts for the staff history.
type MasterDataChangeRequestHistoryResponse struct {
	MasterDataChangeRequestResponse
	DecidedAt     time.Time `json:"decided_at"`
	DecidedByName string    `json:"decided_by_name,omitempty"`
	ReviewReason  *string   `json:"review_reason,omitempty"`
}

// toMasterDataHistoryResponse maps one decided Stammdaten request (shared by
// the per-type history route and the aggregated list, #2432).
func toMasterDataHistoryResponse(item *userService.MasterDataHistoryItem) MasterDataChangeRequestHistoryResponse {
	return MasterDataChangeRequestHistoryResponse{
		MasterDataChangeRequestResponse: toMasterDataChangeRequestResponse(&userService.MasterDataReviewItem{
			Request:   item.Request,
			FirstName: item.FirstName,
			LastName:  item.LastName,
		}),
		DecidedAt:     historyDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		DecidedByName: item.ReviewerName,
		ReviewReason:  item.Request.ReviewReason,
	}
}

// listMasterDataChangeRequestHistory returns the tenant's decided parent
// Stammdaten change requests, newest decision first, cursor-paginated.
func (rs *Resource) listMasterDataChangeRequestHistory(w http.ResponseWriter, r *http.Request) {
	if rs.MasterDataReviewService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("master data review service not configured")))
		return
	}
	before, beforeID, limit, err := parseHistoryQuery(r)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	items, next, err := rs.MasterDataReviewService.ListHistory(r.Context(), before, beforeID, limit)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	out := historyPageResponse[MasterDataChangeRequestHistoryResponse]{
		Items:      make([]MasterDataChangeRequestHistoryResponse, 0, len(items)),
		NextCursor: encodeHistoryCursor(next),
	}
	for _, item := range items {
		out.Items = append(out.Items, toMasterDataHistoryResponse(item))
	}
	common.Respond(w, r, http.StatusOK, out, "Change request history retrieved")
}

// CareRequestHistoryResponse is one decided care-schedule request. It never
// carries a recomputed live diff — current data has moved on since the
// decision. Diff replays the frozen decision snapshot (ADR 0002, #2430);
// rows without one (withdrawn, pre-snapshot) fall back to the payload-derived
// requested summary (each entry's old side is empty).
type CareRequestHistoryResponse struct {
	ID             string                    `json:"id"`
	StudentID      string                    `json:"student_id"`
	FirstName      string                    `json:"first_name"`
	LastName       string                    `json:"last_name"`
	Status         string                    `json:"status"`
	RequestKind    string                    `json:"request_kind"`
	Requested      []CareRequestDiffResponse `json:"requested"`
	Diff           []CareRequestDiffResponse `json:"diff,omitempty"`
	DecisionReason *string                   `json:"decision_reason,omitempty"`
	CreatedAt      time.Time                 `json:"created_at"`
	DecidedAt      time.Time                 `json:"decided_at"`
	DecidedByName  string                    `json:"decided_by_name,omitempty"`
}

// toCareRequestHistoryResponse maps one decided care-schedule request (shared
// by the per-type history route and the aggregated list, #2432).
func toCareRequestHistoryResponse(item *scheduleService.CareRequestHistoryItem) CareRequestHistoryResponse {
	req := item.Request
	requested := toCareRequestDiffResponses(item.Requested)
	var diff []CareRequestDiffResponse
	if len(item.Diff) > 0 {
		diff = toCareRequestDiffResponses(item.Diff)
	}
	return CareRequestHistoryResponse{
		ID:             strconv.FormatInt(req.ID, 10),
		StudentID:      strconv.FormatInt(req.StudentID, 10),
		FirstName:      item.FirstName,
		LastName:       item.LastName,
		Status:         req.Status,
		RequestKind:    req.RequestKind,
		Requested:      requested,
		Diff:           diff,
		DecisionReason: req.DecisionReason,
		CreatedAt:      req.CreatedAt,
		DecidedAt:      historyDecidedAt(req.ReviewedAt, req.UpdatedAt),
		DecidedByName:  item.ReviewerName,
	}
}

// listCareScheduleChangeRequestHistory returns the tenant's decided
// care-schedule change requests, newest decision first, cursor-paginated.
func (rs *Resource) listCareScheduleChangeRequestHistory(w http.ResponseWriter, r *http.Request) {
	if rs.CareRequestService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("care request service not configured")))
		return
	}
	before, beforeID, limit, err := parseHistoryQuery(r)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	items, next, err := rs.CareRequestService.ListHistory(r.Context(), before, beforeID, limit)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	out := historyPageResponse[CareRequestHistoryResponse]{
		Items:      make([]CareRequestHistoryResponse, 0, len(items)),
		NextCursor: encodeHistoryCursor(next),
	}
	for _, item := range items {
		out.Items = append(out.Items, toCareRequestHistoryResponse(item))
	}
	common.Respond(w, r, http.StatusOK, out, "Care schedule change request history retrieved")
}

// OfferingRequestHistoryResponse extends the queue projection with the
// decision facts. Its diff comes from the frozen decision snapshot.
type OfferingRequestHistoryResponse struct {
	OfferingRequestResponse
	DecidedAt     time.Time                          `json:"decided_at"`
	DecidedByName string                             `json:"decided_by_name,omitempty"`
	Requested     []OfferingRequestRequestedResponse `json:"requested,omitempty"`
}

// OfferingRequestRequestedResponse is a payload-derived recap for history
// rows without a frozen decision snapshot, such as withdrawn requests.
type OfferingRequestRequestedResponse struct {
	OfferingID string `json:"offering_id"`
	Label      string `json:"label"`
	New        string `json:"new"`
}

// toOfferingRequestHistoryResponse maps one decided offering request (shared
// by the per-type history route and the aggregated list, #2432).
func toOfferingRequestHistoryResponse(item *enrollmentService.OfferingChangeHistoryItem) OfferingRequestHistoryResponse {
	requested := make([]OfferingRequestRequestedResponse, 0, len(item.Requested))
	for _, entry := range item.Requested {
		requested = append(requested, OfferingRequestRequestedResponse{
			OfferingID: strconv.FormatInt(entry.OfferingID, 10),
			Label:      entry.Name,
			New:        germanOfferingDiffLabel("booked", entry.Days),
		})
	}
	return OfferingRequestHistoryResponse{
		OfferingRequestResponse: toOfferingRequestResponse(&enrollmentService.OfferingChangeView{
			Request:     item.Request,
			StudentName: item.StudentName,
			Diff:        item.Diff,
		}),
		DecidedAt:     historyDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		DecidedByName: item.ReviewerName,
		Requested:     requested,
	}
}

// listOfferingChangeRequestHistory returns the tenant's decided offering
// change requests, newest decision first, cursor-paginated.
func (rs *Resource) listOfferingChangeRequestHistory(w http.ResponseWriter, r *http.Request) {
	if rs.OfferingChangeService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("offering change service not configured")))
		return
	}
	before, beforeID, limit, err := parseHistoryQuery(r)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	items, next, err := rs.OfferingChangeService.ListHistory(r.Context(), before, beforeID, limit)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	out := historyPageResponse[OfferingRequestHistoryResponse]{
		Items:      make([]OfferingRequestHistoryResponse, 0, len(items)),
		NextCursor: encodeHistoryCursor(next),
	}
	for _, item := range items {
		out.Items = append(out.Items, toOfferingRequestHistoryResponse(item))
	}
	common.Respond(w, r, http.StatusOK, out, "Offering change request history retrieved")
}

// StaffExcusedRequestHistoryResponse extends the queue projection with the
// decision facts for the staff history.
type StaffExcusedRequestHistoryResponse struct {
	StaffExcusedRequestResponse
	DecidedAt     time.Time `json:"decided_at"`
	DecidedByName string    `json:"decided_by_name,omitempty"`
}

// toStaffExcusedHistoryResponse maps one decided excused-absence request
// (shared by the per-type history route and the aggregated list, #2432).
func toStaffExcusedHistoryResponse(item *absenceService.ExcusedRequestHistoryItem) StaffExcusedRequestHistoryResponse {
	return StaffExcusedRequestHistoryResponse{
		StaffExcusedRequestResponse: toStaffExcusedRequestResponse(&absenceService.ExcusedRequestReviewItem{
			Request:   item.Request,
			FirstName: item.FirstName,
			LastName:  item.LastName,
		}),
		DecidedAt:     historyDecidedAt(item.Request.ReviewedAt, item.Request.UpdatedAt),
		DecidedByName: item.ReviewerName,
	}
}

// listExcusedAbsenceRequestHistory returns the tenant's decided
// excused-absence requests, newest decision first, cursor-paginated.
func (rs *Resource) listExcusedAbsenceRequestHistory(w http.ResponseWriter, r *http.Request) {
	if rs.ExcusedRequestService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("excused request service not configured")))
		return
	}
	before, beforeID, limit, err := parseHistoryQuery(r)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	items, next, err := rs.ExcusedRequestService.ListHistory(r.Context(), before, beforeID, limit)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}
	out := historyPageResponse[StaffExcusedRequestHistoryResponse]{
		Items:      make([]StaffExcusedRequestHistoryResponse, 0, len(items)),
		NextCursor: encodeHistoryCursor(next),
	}
	for _, item := range items {
		out.Items = append(out.Items, toStaffExcusedHistoryResponse(item))
	}
	common.Respond(w, r, http.StatusOK, out, "Excused absence request history retrieved")
}
