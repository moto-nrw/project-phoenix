package students

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
)

// CompanionResponse is one child this child walks home with, plus the weekdays
// they walk together.
type CompanionResponse struct {
	CompanionStudentID int64    `json:"companion_student_id"`
	FirstName          string   `json:"first_name,omitempty"`
	LastName           string   `json:"last_name,omitempty"`
	Weekdays           []string `json:"weekdays"`
}

// UpdateCompanionsRequest replaces the child's complete "läuft mit" list. An
// empty list clears every link — that is how a Laufgemeinschaft is dissolved
// from this child's side.
type UpdateCompanionsRequest struct {
	Companions []CompanionEntry `json:"companions"`
}

// CompanionEntry is one submitted link.
type CompanionEntry struct {
	CompanionStudentID int64    `json:"companion_student_id"`
	Weekdays           []string `json:"weekdays"`
}

// Bind satisfies render.Binder.
func (req *UpdateCompanionsRequest) Bind(_ *http.Request) error {
	if req.Companions == nil {
		req.Companions = []CompanionEntry{}
	}
	return nil
}

// getStudentCompanions returns the children this child walks home with.
//
// Read access follows the same scope rules as the rest of the child's data —
// a companion's NAME is another child's personal data, so this must never be
// looser than reading the child's own record.
func (rs *Resource) getStudentCompanions(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}
	if !rs.checkStudentReadAccess(r, student) {
		renderError(w, r, common.ErrorForbidden(errors.New("read access required to view departure companions")))
		return
	}

	links, err := rs.StudentService.ListCompanions(r.Context(), student.ID)
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to load departure companions", err))
		return
	}

	common.Respond(w, r, http.StatusOK, toCompanionResponses(links), "Departure companions retrieved")
}

// updateStudentCompanions replaces the child's complete companion list.
//
// Write access is the strict check (admin or the child's group supervisor):
// the write also changes what shows up on the OTHER child's card, so a
// read-scope-widened staff member must not be able to trigger it.
func (rs *Resource) updateStudentCompanions(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}
	if !rs.checkStudentFullAccess(r, student) {
		renderError(w, r, common.ErrorForbidden(errors.New("full access required to edit departure companions")))
		return
	}

	req := &UpdateCompanionsRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	if err := rs.StudentService.ReplaceCompanions(r.Context(), student.ID, toCompanionLinks(req.Companions)); err != nil {
		renderError(w, r, companionErrorResponse(err))
		return
	}

	updated, err := rs.StudentService.ListCompanions(r.Context(), student.ID)
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to reload departure companions", err))
		return
	}

	common.Respond(w, r, http.StatusOK, toCompanionResponses(updated), "Departure companions updated")
}

// enrichWithCompanions fills CompanionStudentIDs for the given day.
//
// Only the ids travel, not the names: the Kindersuche already knows every child
// on the page, so it can resolve a companion that is visible and quietly ignore
// one that is filtered out. Shipping names here would leak children the caller
// filtered away.
//
// A failure is logged and swallowed — losing the grouping must never fail the
// whole student list.
func (rs *Resource) enrichWithCompanions(ctx context.Context, responses []StudentResponse, params *studentListParams, day time.Time) {
	if !params.includeCompanions {
		return
	}

	studentIDs := collectFullAccessStudentIDs(responses)
	if len(studentIDs) == 0 {
		return
	}

	weekday := companionWeekdayForDate(day)
	byStudent, err := rs.StudentService.CompanionIDsForWeekday(ctx, studentIDs, weekday)
	if err != nil {
		rs.Logger.Error("failed to load departure companions for list",
			slog.String("error", err.Error()),
			slog.Int("weekday", weekday),
		)
		return
	}

	for i := range responses {
		if companions, ok := byStudent[responses[i].ID]; ok && len(companions) > 0 {
			responses[i].CompanionStudentIDs = companions
		}
	}
}

// companionWeekdayForDate maps a date onto the stored 1..5 weekday.
//
// Saturday and Sunday fall back to Monday rather than resolving to "no
// weekday": OGS care does not run on the weekend, so a staff member opening
// the Kindersuche on a Sunday is looking ahead at the coming week. Returning
// nothing there would make the Laufgemeinschaft grouping look broken (every
// child in "Ohne Laufgemeinschaft") for a reason no one can see.
func companionWeekdayForDate(day time.Time) int {
	switch day.Weekday() {
	case time.Tuesday:
		return 2
	case time.Wednesday:
		return 3
	case time.Thursday:
		return 4
	case time.Friday:
		return 5
	default:
		return 1
	}
}

// toCompanionLinks converts the wire entries into the service's link shape.
func toCompanionLinks(entries []CompanionEntry) []userModels.CompanionLink {
	links := make([]userModels.CompanionLink, 0, len(entries))
	for _, entry := range entries {
		links = append(links, userModels.CompanionLink{
			CompanionStudentID: entry.CompanionStudentID,
			Weekdays:           entry.Weekdays,
		})
	}
	return links
}

func toCompanionResponses(links []userModels.CompanionLink) []CompanionResponse {
	out := make([]CompanionResponse, 0, len(links))
	for _, link := range links {
		weekdays := link.Weekdays
		if weekdays == nil {
			weekdays = []string{}
		}
		out = append(out, CompanionResponse{
			CompanionStudentID: link.CompanionStudentID,
			FirstName:          link.FirstName,
			LastName:           link.LastName,
			Weekdays:           weekdays,
		})
	}
	return out
}

// companionErrorResponse maps the validation sentinels to 400/404 and anything
// else to 500. The German sentinel texts reach the UI verbatim.
func companionErrorResponse(err error) render.Renderer {
	switch {
	case errors.Is(err, usersService.ErrCompanionNotFound):
		return common.ErrorNotFound(err)
	case errors.Is(err, usersService.ErrStudentNotFound):
		return common.ErrorNotFound(err)
	case errors.Is(err, usersService.ErrDuplicateCompanion),
		errors.Is(err, usersService.ErrCompanionWeekdayRequired),
		errors.Is(err, usersService.ErrTooManyCompanions),
		errors.Is(err, userModels.ErrCompanionSelfLink),
		errors.Is(err, userModels.ErrCompanionInvalidWeekday):
		return common.ErrorInvalidRequest(err)
	default:
		return common.ErrorInternalServerWrap("failed to update departure companions", err)
	}
}
