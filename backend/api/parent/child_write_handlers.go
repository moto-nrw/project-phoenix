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
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
)

const (
	dateFormatYYYYMMDD = "2006-01-02"
	// maxSickNoteDays caps a single submission so a parent can't report
	// hundreds of days in one request. A term-length range stays well
	// under this.
	maxSickNoteDays = 60
)

// --- Sick note ---

// SubmitSickNoteRequest is the wire shape for POST
// /parent/me/children/{studentId}/sick-note.
type SubmitSickNoteRequest struct {
	Dates  []string `json:"dates"`
	Reason string   `json:"reason"`
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
		Date:       d.Date.Format(dateFormatYYYYMMDD),
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

	rows, err := rs.ParentService.SubmitSickNote(r.Context(), accountID, studentID, dates, req.Reason)
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

func parseSickDayRange(r *http.Request) (time.Time, time.Time, error) {
	// Default to the current Berlin school day (as UTC midnight, matching the
	// DATE-column comparison). Raw time.Now().UTC() would roll to "yesterday"
	// in the first hours after Berlin midnight.
	today := timezone.TodayUTC()

	fromRaw := r.URL.Query().Get("from")
	toRaw := r.URL.Query().Get("to")

	from := today
	to := today.AddDate(0, 2, 0)
	if fromRaw != "" {
		parsed, err := time.Parse(dateFormatYYYYMMDD, fromRaw)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid from date, expected YYYY-MM-DD")
		}
		from = parsed
	}
	if toRaw != "" {
		parsed, err := time.Parse(dateFormatYYYYMMDD, toRaw)
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid to date, expected YYYY-MM-DD")
		}
		to = parsed
	}
	if to.Before(from) {
		return time.Time{}, time.Time{}, errors.New("to must be on or after from")
	}
	return from, to, nil
}

// ChildFeaturesResponse tells the parent UI which write actions the child's
// school currently allows, so it can avoid offering ones the backend rejects.
type ChildFeaturesResponse struct {
	SickNoteEnabled bool `json:"sick_note_enabled"`
	NotesEnabled    bool `json:"notes_enabled"`
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
		SickNoteEnabled: flags.SickNoteEnabled,
		NotesEnabled:    flags.NotesEnabled,
	}, "Child features retrieved")
}

// --- Parent notes ---

// AddNoteRequest is the wire shape for POST
// /parent/me/children/{studentId}/notes.
type AddNoteRequest struct {
	Body string `json:"body"`
}

// ParentNoteResponse is one note row, newest-first in lists.
type ParentNoteResponse struct {
	ID        string    `json:"id"`
	StudentID string    `json:"student_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}

func toParentNoteResponse(n *usersModels.StudentParentNote) ParentNoteResponse {
	return ParentNoteResponse{
		ID:        strconv.FormatInt(n.ID, 10),
		StudentID: strconv.FormatInt(n.StudentID, 10),
		Body:      n.Body,
		CreatedAt: n.CreatedAt,
	}
}

func toParentNoteResponses(notes []*usersModels.StudentParentNote) []ParentNoteResponse {
	out := make([]ParentNoteResponse, 0, len(notes))
	for _, n := range notes {
		out = append(out, toParentNoteResponse(n))
	}
	return out
}

// listNotes returns the newest notes for the child.
func (rs *Resource) listNotes(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}

	notes, err := rs.ParentService.ListParentNotes(r.Context(), accountID, studentID, parentService.ParentNoteDisplayLimit)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toParentNoteResponses(notes), "Notes retrieved")
}

// addNote appends a free-text note and returns the newest few.
func (rs *Resource) addNote(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.parentAccountID(w, r)
	if !ok {
		return
	}
	studentID, ok := parsePathStudentID(w, r)
	if !ok {
		return
	}

	var req AddNoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid request body")))
		return
	}

	notes, err := rs.ParentService.AddParentNote(r.Context(), accountID, studentID, req.Body)
	if err != nil {
		renderParentWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toParentNoteResponses(notes), "Note added")
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

func parseSickNoteDates(raw []string) ([]time.Time, error) {
	if len(raw) == 0 {
		return nil, errors.New("at least one date is required")
	}
	if len(raw) > maxSickNoteDays {
		return nil, errors.New("too many dates in a single submission")
	}
	seen := make(map[string]struct{}, len(raw))
	dates := make([]time.Time, 0, len(raw))
	for _, s := range raw {
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		d, err := time.Parse(dateFormatYYYYMMDD, s)
		if err != nil {
			return nil, errors.New("invalid date format, expected YYYY-MM-DD")
		}
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
	case errors.Is(err, parentService.ErrSickNoteDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "sick_note_disabled"))
	case errors.Is(err, parentService.ErrNotesDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "notes_disabled"))
	case errors.Is(err, parentService.ErrInviteDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "invite_disabled"))
	case errors.Is(err, parentService.ErrRemoveDisabled):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "remove_disabled"))
	case errors.Is(err, authService.ErrCannotRemovePrimaryGuardian):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, "primary_guardian_protected"))
	case errors.Is(err, parentService.ErrNoDates),
		errors.Is(err, parentService.ErrEmptyNote),
		errors.Is(err, parentService.ErrNoteTooLong),
		errors.Is(err, parentService.ErrEmailRequired):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
