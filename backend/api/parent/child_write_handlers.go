package parent

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
)

const (
	// maxSickNoteDays caps a single submission so a parent can't report
	// hundreds of days in one request. A term-length range stays well
	// under this.
	maxSickNoteDays = 60
)

// --- Sick note ---

// SubmitSickNoteRequest is the wire shape for POST
// /parent/me/children/{studentId}/sick-note. Status is the absence kind the
// parent chose: "sick" (Krankmeldung) or "excused" (Termin/Abwesenheit). An
// empty status defaults to "sick" so older clients keep working. Reason is
// required for both absence kinds.
type SubmitSickNoteRequest struct {
	Dates  []string `json:"dates"`
	Reason string   `json:"reason"`
	Status string   `json:"status"`
}

// StatusDayResponse mirrors the staff status-day shape but is the
// parent-facing projection: stringified ids, date as YYYY-MM-DD.
type StatusDayResponse struct {
	ID         string    `json:"id"`
	StudentID  string    `json:"student_id"`
	Date       string    `json:"date"`
	Status     string    `json:"status"`
	ReportedAt time.Time `json:"reported_at"`
	Source     string    `json:"source"`
	Note       *string   `json:"note,omitempty"`
}

func toStatusDayResponse(d *activeModels.StudentStatusDay) StatusDayResponse {
	return StatusDayResponse{
		ID:         strconv.FormatInt(d.ID, 10),
		StudentID:  strconv.FormatInt(d.StudentID, 10),
		Date:       d.Date.String(),
		Status:     d.Status,
		ReportedAt: d.ReportedAt,
		Source:     d.Source,
		Note:       d.Note,
	}
}

// ParentExcusedRequestResponse is the parent-facing projection of a pending or
// recently-decided excused-absence approval request (#1845).
type ParentExcusedRequestResponse struct {
	ID             string     `json:"id"`
	StudentID      string     `json:"student_id"`
	Status         string     `json:"status"`
	Dates          []string   `json:"dates"`
	Note           string     `json:"note"`
	DecisionReason *string    `json:"decision_reason,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ReviewedAt     *time.Time `json:"reviewed_at,omitempty"`
	// IsSelf is true only when the CALLING guardian submitted this request. In a
	// multi-guardian family only the submitter may withdraw it (the backend
	// rejects a non-submitter's withdrawal), so the UI shows the withdraw action
	// only for own requests.
	IsSelf bool `json:"is_self"`
}

func toParentExcusedRequestResponse(req *activeModels.ExcusedAbsenceRequest, accountID int64) ParentExcusedRequestResponse {
	dates := make([]string, 0, len(req.Dates))
	for _, d := range req.Dates {
		dates = append(dates, d.String())
	}
	return ParentExcusedRequestResponse{
		ID:             strconv.FormatInt(req.ID, 10),
		StudentID:      strconv.FormatInt(req.StudentID, 10),
		Status:         req.Status,
		Dates:          dates,
		Note:           req.Note,
		DecisionReason: req.DecisionReason,
		CreatedAt:      req.CreatedAt,
		ReviewedAt:     req.ReviewedAt,
		IsSelf:         req.SubmittedBy == accountID,
	}
}

// submitSickNote reports the parent's child sick for one or more dates.
// Auth: parent-scope JWT (account from claims). Ownership of the child is
// verified inside the service against auth.account_tenants.
func (rs *Resource) submitSickNote(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}

	var req SubmitSickNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}

	dates, err := parseSickNoteDates(req.Dates)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Default an empty status to "sick" so older clients (which sent no status)
	// keep reporting Krankmeldungen unchanged. The service validates the value.
	status := req.Status
	if status == "" {
		status = activeModels.StudentStatusDaySick
	}

	result, err := rs.ParentService.SubmitSickNote(r.Context(), accountID, studentID, dates, req.Reason, status)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}

	statusDays := make([]StatusDayResponse, 0, len(result.StatusDays))
	for _, d := range result.StatusDays {
		statusDays = append(statusDays, toStatusDayResponse(d))
	}

	// Backward-compatibility (#1845 review): ALWAYS respond with the bare
	// status-day array — the shape every client (including tabs loaded before the
	// approval gate was enabled) already calls .map() on. When the school gates an
	// excused absence the request is created server-side but NOT returned here;
	// StatusDays is empty and the child stays "expected". New clients discover the
	// freshly-created pending request via GET .../excused-requests (they refetch
	// after a submit). Returning a {status_days, pending_request} object on this
	// path would crash an already-open #1735-era tab — which has the "excused"
	// option but expects an array — and make it report a false failure, so the
	// parent could resubmit and create a duplicate pending request.
	common.Respond(w, r, http.StatusCreated, statusDays, "Sick note submitted")
}

// listExcusedRequests returns the child's pending + recently-decided excused
// approval requests (#1845), so the parent UI can show their status.
func (rs *Resource) listExcusedRequests(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}

	rows, err := rs.ParentService.ListExcusedRequests(r.Context(), accountID, studentID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}

	out := make([]ParentExcusedRequestResponse, 0, len(rows))
	for _, req := range rows {
		out = append(out, toParentExcusedRequestResponse(req, accountID))
	}
	common.Respond(w, r, http.StatusOK, out, "Excused requests retrieved")
}

// withdrawExcusedRequest withdraws the caller's own pending excused approval
// request (#1845).
func (rs *Resource) withdrawExcusedRequest(w http.ResponseWriter, r *http.Request) {
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

	req, err := rs.ParentService.WithdrawExcusedRequest(r.Context(), accountID, studentID, requestID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toParentExcusedRequestResponse(req, accountID), "Excused request withdrawn")
}

// listSickDays returns the child's active sick days in the requested
// range (defaults to today .. +2 months).
func (rs *Resource) listSickDays(w http.ResponseWriter, r *http.Request) {
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

	rows, err := rs.ParentService.ListSickDays(r.Context(), accountID, studentID, from, to)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}

	out := make([]StatusDayResponse, 0, len(rows))
	for _, d := range rows {
		out = append(out, toStatusDayResponse(d))
	}
	common.Respond(w, r, http.StatusOK, out, "Sick days retrieved")
}

func parseSickDayRange(r *http.Request) (timezone.Date, timezone.Date, error) {
	today := timezone.TodayDate()

	fromRaw := r.URL.Query().Get("from")
	toRaw := r.URL.Query().Get("to")

	from := today
	// Two calendar months ahead, mirroring time.Time.AddDate(0, 2, 0).
	to := timezone.NewDate(today.Year, today.Month+2, today.Day)
	if fromRaw != "" {
		parsed, err := timezone.ParseDate(fromRaw)
		if err != nil {
			return timezone.Date{}, timezone.Date{}, errors.New("invalid from date, expected YYYY-MM-DD")
		}
		from = parsed
	}
	if toRaw != "" {
		parsed, err := timezone.ParseDate(toRaw)
		if err != nil {
			return timezone.Date{}, timezone.Date{}, errors.New("invalid to date, expected YYYY-MM-DD")
		}
		to = parsed
	}
	if to.Before(from) {
		return timezone.Date{}, timezone.Date{}, errors.New("to must be on or after from")
	}
	return from, to, nil
}

// ChildFeaturesResponse tells the parent UI which write actions the child's
// school currently allows, so it can avoid offering ones the backend rejects.
type ChildFeaturesResponse struct {
	SickNoteEnabled              bool `json:"sick_note_enabled"`
	ExcusedRequiresApproval      bool `json:"excused_requires_approval"`
	NotesEnabled                 bool `json:"notes_enabled"`
	RequestSubmitEnabled         bool `json:"request_submit_enabled"`
	PickupChangeEnabled          bool `json:"pickup_change_enabled"`
	PickupManageAllowed          bool `json:"pickup_manage_allowed"`
	GuardianContactManageAllowed bool `json:"guardian_contact_manage_allowed"`
	RelatedAccountsInviteEnabled bool `json:"related_accounts_invite_enabled"`
	RelatedAccountsRemoveEnabled bool `json:"related_accounts_remove_enabled"`
	MasterDataEditEnabled        bool `json:"master_data_edit_enabled"`
	MasterDataContactEditEnabled bool `json:"master_data_contact_edit_enabled"`
	MasterDataRequestEnabled     bool `json:"master_data_request_enabled"`
	MealPlanEnabled              bool `json:"meal_plan_enabled"`
	// HasOpenChangeRequest is STATE (not a capability): the child has a pending
	// change request awaiting an OGS decision, so the overview can badge the
	// Stammdaten entry.
	HasOpenChangeRequest bool `json:"has_open_change_request"`
	NewsEnabled          bool `json:"parent_news_enabled"`
}

// getChildFeatures returns the resolved parent-portal feature flags for the
// child's tenant.
func (rs *Resource) getChildFeatures(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}

	flags, err := rs.ParentService.ChildFeatures(r.Context(), accountID, studentID)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, ChildFeaturesResponse{
		SickNoteEnabled:              flags.SickNoteEnabled,
		ExcusedRequiresApproval:      flags.ExcusedRequiresApproval,
		NotesEnabled:                 flags.NotesEnabled,
		RequestSubmitEnabled:         flags.RequestSubmitEnabled,
		PickupChangeEnabled:          flags.PickupChangeEnabled,
		PickupManageAllowed:          flags.PickupManageAllowed,
		GuardianContactManageAllowed: flags.GuardianContactManageAllowed,
		RelatedAccountsInviteEnabled: flags.RelatedAccountsInviteEnabled,
		RelatedAccountsRemoveEnabled: flags.RelatedAccountsRemoveEnabled,
		MasterDataEditEnabled:        flags.MasterDataEditEnabled,
		MasterDataContactEditEnabled: flags.MasterDataContactEditEnabled,
		MasterDataRequestEnabled:     flags.MasterDataRequestEnabled,
		MealPlanEnabled:              flags.MealPlanEnabled,
		HasOpenChangeRequest:         flags.HasOpenChangeRequest,
		NewsEnabled:                  flags.NewsEnabled,
	}, "Child features retrieved")
}

// --- Meal plan (Essensplan) ---

// MealPlanEntryResponse is one dish of the read-only meal plan shown to parents.
type MealPlanEntryResponse struct {
	Date     string  `json:"date"` // YYYY-MM-DD
	Position int     `json:"position"`
	Dish     string  `json:"dish"`
	Note     *string `json:"note,omitempty"`
}

// getChildMealPlan returns the Monday-Friday meal plan for the child's school
// for the week containing week_start. Gated by operations.meal_plan_enabled for
// that tenant (404-like "disabled" is mapped to 403 meal_plan_disabled).
func (rs *Resource) getChildMealPlan(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}

	weekStart, err := timezone.ParseDate(r.URL.Query().Get("week_start"))
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("week_start must be in YYYY-MM-DD format")))
		return
	}

	rows, err := rs.ParentService.MealPlanWeek(r.Context(), accountID, studentID, weekStart)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}

	out := make([]MealPlanEntryResponse, 0, len(rows))
	for _, entry := range rows {
		out = append(out, MealPlanEntryResponse{
			Date:     entry.Date.String(),
			Position: entry.Position,
			Dish:     entry.Dish,
			Note:     entry.Note,
		})
	}
	common.Respond(w, r, http.StatusOK, out, "Meal plan retrieved")
}

// --- shared helpers ---

// parentAccountID reads the account id from the parent-scope JWT, or
// renders 401 and returns false.
func (rs *Resource) parentAccountID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(jwt.ErrTokenUnauthorized))
		return 0, false
	}
	return int64(claims.ID), true
}

func parsePathStudentID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return common.ParsePositiveInt64IDWithError(w, r, "studentId", "invalid student id")
}

func parseSickNoteDates(raw []string) ([]timezone.Date, error) {
	if len(raw) == 0 {
		return nil, errors.New("at least one date is required")
	}
	if len(raw) > maxSickNoteDays {
		return nil, errors.New("too many dates in a single submission")
	}
	seen := make(map[timezone.Date]struct{}, len(raw))
	dates := make([]timezone.Date, 0, len(raw))
	for _, s := range raw {
		d, err := timezone.ParseDate(s)
		if err != nil {
			return nil, errors.New("invalid date format, expected YYYY-MM-DD")
		}
		if _, dup := seen[d]; dup {
			continue
		}
		seen[d] = struct{}{}
		dates = append(dates, d)
	}
	return dates, nil
}

// renderParentWriteError maps the service sentinels to stable HTTP codes.
func renderParentWriteError(w http.ResponseWriter, r *http.Request, err error) {
	// Feedback-board errors (#1678) are classified in their own file so the
	// board's vocabulary stays next to its handlers.
	if renderFeedbackError(w, r, err) {
		return
	}
	switch {
	case errors.Is(err, parentService.ErrChildNotLinked):
		// Don't reveal whether the student exists elsewhere — treat an
		// unowned child as forbidden.
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "child_not_linked"))
	case errors.Is(err, parentService.ErrGuardianPermissionDenied):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "guardian_permission_denied"))
	case errors.Is(err, parentService.ErrSickNoteDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "sick_note_disabled"))
	case errors.Is(err, parentService.ErrNotesDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "notes_disabled"))
	case errors.Is(err, parentService.ErrMealPlanDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "meal_plan_disabled"))
	case errors.Is(err, parentService.ErrMealPlanWeekOutOfRange):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "meal_plan_week_out_of_range"))
	case errors.Is(err, parentService.ErrPickupChangeDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "pickup_change_disabled"))
	case errors.Is(err, parentService.ErrMasterDataEditDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "master_data_edit_disabled"))
	case errors.Is(err, parentService.ErrMasterDataRequestDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "master_data_request_disabled"))
	case errors.Is(err, parentService.ErrMasterDataFieldNotEditable):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "master_data_field_not_editable"))
	case errors.Is(err, parentService.ErrMasterDataInvalidValue):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "master_data_invalid_value"))
	case errors.Is(err, parentService.ErrMasterDataDuplicatePending):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "master_data_duplicate_pending"))
	case errors.Is(err, parentService.ErrMasterDataNoChanges):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "master_data_no_changes"))
	case errors.Is(err, parentService.ErrCareExceptionConflict):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "care_exception_conflict"))
	case errors.Is(err, parentService.ErrCareExceptionAlreadyLeft):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "care_exception_already_left"))
	case errors.Is(err, parentService.ErrCareExceptionRaced):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "care_exception_raced"))
	case errors.Is(err, parentService.ErrExcusedRequestNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, parentService.ErrExcusedRequestNotPending):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "excused_request_not_pending"))
	case errors.Is(err, parentService.ErrExcusedRequestOverlap):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "excused_request_overlap"))
	case errors.Is(err, parentService.ErrCareRequestNotPending):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "request_not_open"))
	case errors.Is(err, parentService.ErrCareRequestNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, parentService.ErrCareRequestAlreadyPending):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "care_request_already_pending"))
	case errors.Is(err, parentService.ErrInvalidCareRequestPayload):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "invalid_request_payload"))
	case errors.Is(err, parentService.ErrCareRequestFieldDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "care_request_field_disabled"))
	case errors.Is(err, parentService.ErrNoCareException):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "care_exception_no_time"))
	case errors.Is(err, parentService.ErrCareExceptionReasonRequired):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "care_exception_reason_required"))
	case errors.Is(err, parentService.ErrCareExceptionReasonTooLong):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "care_exception_reason_too_long"))
	case errors.Is(err, parentService.ErrPastCareDate):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "care_exception_past_date"))
	case errors.Is(err, parentService.ErrCareDateTooFar):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "care_exception_too_far"))
	case errors.Is(err, parentService.ErrInviteDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "invite_disabled"))
	case errors.Is(err, parentService.ErrRemoveDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "remove_disabled"))
	case errors.Is(err, authService.ErrCannotRemovePrimaryGuardian):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "primary_guardian_protected"))
	case errors.Is(err, authService.ErrCannotRemoveStaffManagedGuardian):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "staff_managed_guardian_protected"))
	case errors.Is(err, authService.ErrCannotRemoveOwnAccess):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "cannot_remove_own_access"))
	case errors.Is(err, authService.ErrInviteSocialWorkerManaged):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "guardian_social_worker_managed"))
	case errors.Is(err, parentService.ErrGuardianManagementDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "guardian_management_disabled"))
	case errors.Is(err, parentService.ErrGuardianNotLinked):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "guardian_not_linked"))
	case errors.Is(err, parentService.ErrGuardianHasOwnAccount):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "guardian_has_own_account"))
	case errors.Is(err, parentService.ErrGuardianSharedAcrossFamilies):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "guardian_shared_across_families"))
	case errors.Is(err, parentService.ErrGuardianSocialWorkerManaged):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "guardian_social_worker_managed"))
	case errors.Is(err, parentService.ErrGuardianRoleManaged):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "guardian_role_managed"))
	case errors.Is(err, parentService.ErrGuardianEmailConflict):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "guardian_email_conflict"))
	case errors.Is(err, parentService.ErrGuardianNoChange):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "guardian_no_change"))
	case errors.Is(err, parentService.ErrGuardianContactInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "guardian_contact_invalid"))
	case errors.Is(err, parentService.ErrGuardianRelationshipInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "guardian_relationship_invalid"))
	case errors.Is(err, enrollmentService.ErrOfferingChangeDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "offering_changes_disabled"))
	case errors.Is(err, enrollmentService.ErrCareOfferingsDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "care_offerings_disabled"))
	case errors.Is(err, enrollmentService.ErrOfferingChangeNoEnrollment):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "offering_changes_no_enrollment"))
	case errors.Is(err, enrollmentService.ErrOfferingChangeForbidden):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "offering_change_forbidden"))
	case errors.Is(err, enrollmentService.ErrOfferingChangeCapacityFull):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "offering_change_capacity_full"))
	case errors.Is(err, enrollmentService.ErrOfferingChangeInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "offering_change_invalid"))
	case errors.Is(err, enrollmentModels.ErrOfferingChangeAlreadyPending):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "offering_change_already_pending"))
	case errors.Is(err, enrollmentModels.ErrOfferingChangeNotPending):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "request_not_open"))
	case errors.Is(err, enrollmentModels.ErrOfferingChangeNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, parentService.ErrAnnouncementNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, parentService.ErrAnnouncementAckNotRequired):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "announcement_ack_not_required"))
	case errors.Is(err, parentService.ErrAnnouncementStale):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "announcement_stale"))
	case errors.Is(err, parentService.ErrAnnouncementNotAPoll):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "announcement_not_a_poll"))
	case errors.Is(err, parentService.ErrPollClosed):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "poll_closed"))
	case errors.Is(err, parentService.ErrInvalidPollResponse):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "invalid_poll_response"))
	case errors.Is(err, parentService.ErrChildNotAnswerable):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, parentService.ErrNoDates),
		errors.Is(err, parentService.ErrInvalidStatus),
		errors.Is(err, parentService.ErrEmptyNote),
		errors.Is(err, parentService.ErrNoteTooLong),
		errors.Is(err, parentService.ErrEmailRequired),
		errors.Is(err, parentService.ErrInvalidInviteInput):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
