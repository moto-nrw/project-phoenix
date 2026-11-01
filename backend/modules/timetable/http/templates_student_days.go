// Package timetablehttp — POST /api/timetable/templates/{id}/students/{studentId}/remove-days.
//
// "Dort auch entfernen" (#5102): a booking change took weekdays away from a
// child, but a hand-maintained Regeltermin still plans the child there. The
// office confirms, and the child leaves that Regeltermin on exactly those
// weekdays from the effective date on. Past days and attendance stay.
package timetablehttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	timetableModule "github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type removeStudentTemplateDaysRequest struct {
	Weekdays      []int  `json:"weekdays"`
	EffectiveFrom string `json:"effective_from"`
	// EffectiveUntil is optional (#5497): the day a later booking carries
	// the Regeltermin again (exclusive). The child stays planned from then.
	EffectiveUntil string `json:"effective_until,omitempty"`
}

func (req *removeStudentTemplateDaysRequest) Bind(_ *http.Request) error {
	if len(req.Weekdays) == 0 {
		return errors.New("weekdays are required")
	}
	if req.EffectiveFrom == "" {
		return errors.New("effective_from is required (YYYY-MM-DD)")
	}
	return nil
}

type removeStudentTemplateDaysResponse struct {
	RosterRows  int `json:"roster_rows"`
	Occurrences int `json:"occurrences"`
}

func (rs *Resource) removeStudentTemplateDays(w http.ResponseWriter, r *http.Request) {
	templateID, ok := templateIDFromRequest(w, r)
	if !ok {
		return
	}
	studentID, ok := common.ParsePositiveInt64IDWithError(w, r, "studentId", "invalid student id")
	if !ok {
		return
	}
	if rs.StudentTemplateDays == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("student template days not wired")))
		return
	}
	req := &removeStudentTemplateDaysRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	from, err := calendar.ParseDate(req.EffectiveFrom)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid effective_from format, expected YYYY-MM-DD")))
		return
	}
	until, ok := optionalRequestDate(w, r, req.EffectiveUntil, "effective_until")
	if !ok {
		return
	}
	result, err := rs.StudentTemplateDays.RemoveStudentTemplateDays(r.Context(), timetableModule.StudentTemplateDaysRemoval{
		TemplateID: templateID, StudentID: studentID, Weekdays: req.Weekdays, From: from, Until: until,
	})
	switch {
	case errors.Is(err, timetableModule.ErrGroupNotFound):
		renderTemplateNotFound(w, r)
	case errors.Is(err, timetableModule.ErrStudentTemplateDaysInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case err != nil:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	default:
		// The child left the template's planned occurrences (#5017).
		rs.broadcastStaffingDeviationChanged(r.Context(), "template_student_days")
		common.Respond(w, r, http.StatusOK, removeStudentTemplateDaysResponse{
			RosterRows: result.RosterRows, Occurrences: result.Occurrences,
		}, "Student removed from template days")
	}
}

// optionalRequestDate parses an optional YYYY-MM-DD body field; empty is nil.
func optionalRequestDate(w http.ResponseWriter, r *http.Request, value, field string) (*calendar.Date, bool) {
	if value == "" {
		return nil, true
	}
	date, err := calendar.ParseDate(value)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid "+field+" format, expected YYYY-MM-DD")))
		return nil, false
	}
	return &date, true
}
