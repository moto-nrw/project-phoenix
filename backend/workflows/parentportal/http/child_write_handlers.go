package parent

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
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
	// RecipientGuardianProfileIDs are the co-guardians the family picked for
	// this request. They travel WITH the creation so the share is written in
	// the same transaction (#2267); empty shares with nobody.
	RecipientGuardianProfileIDs []string `json:"recipient_guardian_profile_ids"`
}

// SickNoteEnvelopeResponse is the ?envelope=1 shape of the sick-note create.
// The bare status-day array stays the default so a tab loaded before this
// change keeps working; a client that wants to know whether the school gated
// the absence into a pending request asks for the envelope (#2267).
type SickNoteEnvelopeResponse struct {
	StatusDays     []StatusDayResponse           `json:"status_days"`
	PendingRequest *ParentExcusedRequestResponse `json:"pending_request"`
}

// pendingSickNoteRequest projects the created request, or nil when the school
// applied the absence directly.
func pendingSickNoteRequest(result *parentService.SickNoteResult, accountID int64) *ParentExcusedRequestResponse {
	if result == nil || result.PendingRequest == nil {
		return nil
	}
	response := toParentExcusedRequestResponse(result.PendingRequest, accountID)
	return &response
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

func toStatusDayResponse(d *parentService.StudentStatusDaySnapshot) StatusDayResponse {
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

// ParentExcusedRequestResponse is the legacy-named parent projection of a
// pending or recently decided absence approval request.
type ParentExcusedRequestResponse struct {
	ID             string     `json:"id"`
	StudentID      string     `json:"student_id"`
	AbsenceStatus  string     `json:"absence_status"`
	Status         string     `json:"status"`
	Dates          []string   `json:"dates"`
	Note           string     `json:"note"`
	DecisionReason *string    `json:"decision_reason,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ReviewedAt     *time.Time `json:"reviewed_at,omitempty"`
	// IsSelf is true only when the CALLING guardian submitted this request. In a
	// multi-guardian family only the submitter may edit it (the backend rejects
	// a non-submitter as not-found), so the UI shows the edit action only for
	// own requests.
	IsSelf bool `json:"is_self"`
}

func toParentExcusedRequestResponse(req *parentService.ExcusedAbsenceRequestSnapshot, accountID int64) ParentExcusedRequestResponse {
	dates := make([]string, 0, len(req.Dates))
	for _, d := range req.Dates {
		dates = append(dates, d.String())
	}
	return ParentExcusedRequestResponse{
		ID:             strconv.FormatInt(req.ID, 10),
		StudentID:      strconv.FormatInt(req.StudentID, 10),
		AbsenceStatus:  req.AbsenceStatus,
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
		status = absencerecords.StudentStatusDaySick
	}

	recipients, ok := parseCreateRecipients(w, r, req.RecipientGuardianProfileIDs)
	if !ok {
		return
	}

	result, err := rs.ParentService.SubmitSickNote(r.Context(), accountID, studentID, dates, req.Reason, status, recipients)
	if err != nil {
		renderParentRequestError(w, r, err)
		return
	}

	respondSickNote(w, r, result, accountID)
}

// listExcusedRequests returns the child's pending + recently decided absence
// approval requests, so the parent UI can show their status.
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
	to := timezone.NewDate(today.Year(), today.Month()+2, today.Day())
	if fromRaw != "" {
		parsed, err := timezone.ParseDate(fromRaw)
		if err != nil {
			return timezone.Date(""), timezone.Date(""), errors.New("invalid from date, expected YYYY-MM-DD")
		}
		from = parsed
	}
	if toRaw != "" {
		parsed, err := timezone.ParseDate(toRaw)
		if err != nil {
			return timezone.Date(""), timezone.Date(""), errors.New("invalid to date, expected YYYY-MM-DD")
		}
		to = parsed
	}
	if to.Before(from) {
		return timezone.Date(""), timezone.Date(""), errors.New("to must be on or after from")
	}
	return from, to, nil
}

// ChildFeaturesResponse tells the parent UI which write actions the child's
// school currently allows, so it can avoid offering ones the backend rejects.
type ChildFeaturesResponse struct {
	// CareEnded is state, not a capability: the child has left the OGS, so
	// every write flag below is false (#2487).
	CareEnded               bool `json:"care_ended"`
	SickNoteEnabled         bool `json:"sick_note_enabled"`
	ExcusedNoteEnabled      bool `json:"excused_note_enabled"`
	SickRequiresApproval    bool `json:"sick_requires_approval"`
	ExcusedRequiresApproval bool `json:"excused_requires_approval"`
	NotesEnabled            bool `json:"notes_enabled"`
	RequestSubmitEnabled    bool `json:"request_submit_enabled"`
	PickupChangeEnabled     bool `json:"pickup_change_enabled"`
	// PickupChangeModeEnabled says the one-day change may also ask for a
	// different Abholart (#3945).
	PickupChangeModeEnabled bool `json:"pickup_change_mode_enabled"`
	// PickupChangeCutoffTime is the same-day cutoff for the one-day pickup
	// change as HH:MM, "" without one (#3163). PickupChangeTodayClosed is true
	// once it has passed: today is closed for guardians, later days are not.
	PickupChangeCutoffTime       string `json:"pickup_change_cutoff_time"`
	PickupChangeTodayClosed      bool   `json:"pickup_change_today_closed"`
	PickupManageAllowed          bool   `json:"pickup_manage_allowed"`
	GuardianContactManageAllowed bool   `json:"guardian_contact_manage_allowed"`
	AbsenceRequestEditEnabled    bool   `json:"absence_request_edit_enabled"`
	RelatedAccountsInviteEnabled bool   `json:"related_accounts_invite_enabled"`
	RelatedAccountsRemoveEnabled bool   `json:"related_accounts_remove_enabled"`
	MasterDataEditEnabled        bool   `json:"master_data_edit_enabled"`
	MasterDataContactEditEnabled bool   `json:"master_data_contact_edit_enabled"`
	MasterDataRequestEnabled     bool   `json:"master_data_request_enabled"`
	MealPlanEnabled              bool   `json:"meal_plan_enabled"`
	MealRegistrationEnabled      bool   `json:"meal_registration_enabled"`
	// HasOpenChangeRequest is STATE (not a capability): the child has a pending
	// change request awaiting an OGS decision, so the overview can badge the
	// Stammdaten entry.
	HasOpenChangeRequest bool `json:"has_open_change_request"`
	NewsEnabled          bool `json:"parent_news_enabled"`
	// ReasonRequired says this school makes the family state a reason for a
	// request (operations.parent_request_reason_policy, #2267), so the portal
	// marks the note field as required up front.
	ReasonRequired bool `json:"reason_required"`
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
		CareEnded:                    flags.CareEnded,
		SickNoteEnabled:              flags.SickNoteEnabled,
		ExcusedNoteEnabled:           flags.ExcusedNoteEnabled,
		SickRequiresApproval:         flags.SickRequiresApproval,
		ExcusedRequiresApproval:      flags.ExcusedRequiresApproval,
		NotesEnabled:                 flags.NotesEnabled,
		RequestSubmitEnabled:         flags.RequestSubmitEnabled,
		PickupChangeEnabled:          flags.PickupChangeEnabled,
		PickupChangeModeEnabled:      flags.PickupChangeModeEnabled,
		PickupChangeCutoffTime:       flags.PickupChangeCutoffTime,
		PickupChangeTodayClosed:      flags.PickupChangeTodayClosed,
		PickupManageAllowed:          flags.PickupManageAllowed,
		GuardianContactManageAllowed: flags.GuardianContactManageAllowed,
		AbsenceRequestEditEnabled:    flags.AbsenceRequestEditEnabled,
		RelatedAccountsInviteEnabled: flags.RelatedAccountsInviteEnabled,
		RelatedAccountsRemoveEnabled: flags.RelatedAccountsRemoveEnabled,
		MasterDataEditEnabled:        flags.MasterDataEditEnabled,
		MasterDataContactEditEnabled: flags.MasterDataContactEditEnabled,
		MasterDataRequestEnabled:     flags.MasterDataRequestEnabled,
		MealPlanEnabled:              flags.MealPlanEnabled,
		MealRegistrationEnabled:      flags.MealRegistrationEnabled,
		HasOpenChangeRequest:         flags.HasOpenChangeRequest,
		NewsEnabled:                  flags.NewsEnabled,
		ReasonRequired:               flags.ReasonRequired,
	}, "Child features retrieved")
}

// --- Meal plan (Essensplan) ---

// MealPlanEntryResponse is one dish of the read-only meal plan shown to parents.
type MealPlanEntryResponse struct {
	Date     string  `json:"date"` // YYYY-MM-DD
	Position int     `json:"position"`
	Dish     string  `json:"dish"`
	Note     *string `json:"note,omitempty"`
	// Allergens are the codes of the main allergens; empty when unmarked.
	Allergens []string `json:"allergens"`
}

type MealParticipationDayResponse struct {
	Date          string    `json:"date"`
	Participating bool      `json:"participating"`
	Regular       bool      `json:"regular_participating"`
	Source        string    `json:"source"`
	Changeable    bool      `json:"changeable"`
	CanRegister   bool      `json:"can_register"`
	CanCancel     bool      `json:"can_cancel"`
	RegisterUntil time.Time `json:"register_until"`
	CancelUntil   time.Time `json:"cancel_until"`
}

type MealParticipationResponse struct {
	Weekdays           []parentService.MealWeekday    `json:"weekdays"`
	EffectiveFrom      string                         `json:"effective_from,omitempty"`
	CutoffTime         string                         `json:"cutoff_time"`
	Model              string                         `json:"model"`
	ScheduleChangeable bool                           `json:"schedule_changeable"`
	SickCancels        bool                           `json:"sick_cancels_after_deadline"`
	Days               []MealParticipationDayResponse `json:"days"`
}

type ReplaceMealParticipationRequest struct {
	Weekdays []parentService.MealWeekday `json:"weekdays"`
}

type SetMealParticipationDayRequest struct {
	Participating *bool `json:"participating"`
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
			Date:     string(entry.Date),
			Position: entry.Position,
			Dish:     entry.Dish,
			Note:     entry.Note,
			// Never null on the wire: an unmarked dish has an empty list.
			Allergens: append([]string{}, entry.Allergens...),
		})
	}
	common.Respond(w, r, http.StatusOK, out, "Meal plan retrieved")
}

func (rs *Resource) getMealParticipation(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	from, fromErr := timezone.ParseDate(r.URL.Query().Get("from"))
	to, toErr := timezone.ParseDate(r.URL.Query().Get("to"))
	if fromErr != nil || toErr != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("from and to must be in YYYY-MM-DD format")))
		return
	}
	plan, err := rs.ParentService.MealParticipation(r.Context(), accountID, studentID, from, to)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, mealParticipationResponse(plan), "Meal participation retrieved")
}

func (rs *Resource) replaceMealParticipation(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	var request ReplaceMealParticipationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid JSON body")))
		return
	}
	effectiveFrom, err := rs.ParentService.ReplaceMealParticipationSchedule(r.Context(), accountID, studentID, request.Weekdays)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{"effective_from": string(effectiveFrom)}, "Regular meal participation saved")
}

func (rs *Resource) setMealParticipationDay(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	date, err := timezone.ParseDate(chi.URLParam(r, "date"))
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("date must be in YYYY-MM-DD format")))
		return
	}
	var request SetMealParticipationDayRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Participating == nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("participating is required")))
		return
	}
	if err := rs.ParentService.SetMealParticipationDay(r.Context(), accountID, studentID, date, *request.Participating); err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Meal participation day saved")
}

// MealWeekChangeResponse lists the days a week change changed and the days
// it left because their deadline had passed.
type MealWeekChangeResponse struct {
	Changed []string `json:"changed"`
	Locked  []string `json:"locked"`
}

func (rs *Resource) setMealParticipationWeek(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	date, err := timezone.ParseDate(chi.URLParam(r, "date"))
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("date must be in YYYY-MM-DD format")))
		return
	}
	var request SetMealParticipationDayRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Participating == nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("participating is required")))
		return
	}
	change, err := rs.ParentService.SetMealParticipationWeek(r.Context(), accountID, studentID, date, *request.Participating)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, MealWeekChangeResponse{Changed: change.Changed, Locked: change.Locked}, "Meal participation week saved")
}

func (rs *Resource) clearMealParticipationDay(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}
	date, err := timezone.ParseDate(chi.URLParam(r, "date"))
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("date must be in YYYY-MM-DD format")))
		return
	}
	if err := rs.ParentService.ClearMealParticipationDay(r.Context(), accountID, studentID, date); err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Meal participation day reset")
}

func mealParticipationResponse(plan parentService.MealParticipationPlan) MealParticipationResponse {
	days := make([]MealParticipationDayResponse, 0, len(plan.Days))
	for _, day := range plan.Days {
		days = append(days, MealParticipationDayResponse{
			Date: string(day.Date), Participating: day.Participating, Regular: day.Regular, Source: string(day.Source), Changeable: day.Changeable,
			CanRegister: day.CanRegister, CanCancel: day.CanCancel, RegisterUntil: day.RegisterUntil, CancelUntil: day.CancelUntil,
		})
	}
	return MealParticipationResponse{Weekdays: plan.Weekdays, EffectiveFrom: string(plan.EffectiveFrom), CutoffTime: plan.CutoffTime, Model: plan.Model, ScheduleChangeable: plan.ScheduleChangeable, SickCancels: plan.SickCancels, Days: days}
}

// --- shared helpers ---

// parentAccountID reads the account id from the parent-scope JWT, or
// renders 401 and returns false.
func (rs *Resource) parentAccountID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(sessiontoken.ErrTokenUnauthorized))
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
	if renderParentPickupPermissionError(w, r, err) {
		return
	}
	if renderParentCareWriteError(w, r, err) {
		return
	}
	if renderParentMasterDataWriteError(w, r, err) {
		return
	}
	if renderParentGuardianWriteError(w, r, err) {
		return
	}
	if renderParentOfferingWriteError(w, r, err) {
		return
	}
	if renderParentAnnouncementWriteError(w, r, err) {
		return
	}
	common.RenderError(w, r, common.ErrorInternalServer(err))
}

// renderParentCareExceptionDateError keeps the date-specific care-exception
// responses together because every one is a rejected parent input.
func renderParentCareExceptionDateError(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, parentService.ErrPastCareDate):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCareCareExceptionPastDate))
	case errors.Is(err, parentService.ErrCareDateTooFar):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeCareCareExceptionTooFar))
	case errors.Is(err, parentService.ErrCareExceptionNotCareDay):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		return false
	}
	return true
}

func respondSickNote(w http.ResponseWriter, r *http.Request, result *parentService.SickNoteResult, accountID int64) {
	statusDays := make([]StatusDayResponse, 0, len(result.StatusDays))
	for _, d := range result.StatusDays {
		statusDays = append(statusDays, toStatusDayResponse(d))
	}

	// ?envelope=1 is the new client's opt-in to the full result. Old tabs keep
	// the bare array below, which is what they call .map() on (#2267).
	if r.URL.Query().Get("envelope") == "1" {
		common.Respond(w, r, http.StatusCreated, SickNoteEnvelopeResponse{
			StatusDays:     statusDays,
			PendingRequest: pendingSickNoteRequest(result, accountID),
		}, "Absence submitted")
		return
	}

	// Backward-compatibility (#1845 review): ALWAYS respond with the bare
	// status-day array — the shape every client (including tabs loaded before the
	// approval gate was enabled) already calls .map() on. When the school gates an
	// absence the request is created server-side but NOT returned here;
	// StatusDays is empty and the child stays "expected". New clients discover the
	// freshly-created pending request via GET .../excused-requests (they refetch
	// after a submit). Returning a {status_days, pending_request} object on this
	// path would crash an already-open #1735-era tab — which has the "excused"
	// option but expects an array — and make it report a false failure, so the
	// parent could resubmit and create a duplicate pending request.
	common.Respond(w, r, http.StatusCreated, statusDays, "Absence submitted")
}
