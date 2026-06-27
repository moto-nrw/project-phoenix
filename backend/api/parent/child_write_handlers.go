package parent

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
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
// empty status defaults to "sick" so older clients keep working.
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

	rows, err := rs.ParentService.SubmitSickNote(r.Context(), accountID, studentID, dates, req.Reason, status)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}

	out := make([]StatusDayResponse, 0, len(rows))
	for _, d := range rows {
		out = append(out, toStatusDayResponse(d))
	}
	common.Respond(w, r, http.StatusCreated, out, "Sick note submitted")
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
	NotesEnabled                 bool `json:"notes_enabled"`
	PickupChangeEnabled          bool `json:"pickup_change_enabled"`
	RelatedAccountsInviteEnabled bool `json:"related_accounts_invite_enabled"`
	RelatedAccountsRemoveEnabled bool `json:"related_accounts_remove_enabled"`
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
		NotesEnabled:                 flags.NotesEnabled,
		PickupChangeEnabled:          flags.PickupChangeEnabled,
		RelatedAccountsInviteEnabled: flags.RelatedAccountsInviteEnabled,
		RelatedAccountsRemoveEnabled: flags.RelatedAccountsRemoveEnabled,
	}, "Child features retrieved")
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
	studentID, err := strconv.ParseInt(chi.URLParam(r, "studentId"), 10, 64)
	if err != nil || studentID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid student id")))
		return 0, false
	}
	return studentID, true
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
	case errors.Is(err, parentService.ErrPickupChangeDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "pickup_change_disabled"))
	case errors.Is(err, parentService.ErrCareExceptionConflict):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "care_exception_conflict"))
	case errors.Is(err, parentService.ErrCareExceptionRaced):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "care_exception_raced"))
	case errors.Is(err, parentService.ErrNoCareException):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "care_exception_no_time"))
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
