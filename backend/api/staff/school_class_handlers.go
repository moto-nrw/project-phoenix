package staff

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
)

// SchoolClassesResponse is the wire shape of a staff member's school class
// assignments (#1772).
type SchoolClassesResponse struct {
	StaffID       int64    `json:"staff_id"`
	SchoolClasses []string `json:"school_classes"`
}

// SchoolClassesRequest is the PUT body replacing a staff member's school
// class assignments.
type SchoolClassesRequest struct {
	SchoolClasses []string `json:"school_classes"`
}

// Bind implements render.Binder
func (req *SchoolClassesRequest) Bind(_ *http.Request) error {
	if req.SchoolClasses == nil {
		return errors.New("school_classes is required")
	}
	return nil
}

// schoolClassErrorRules classifies the class-teacher service sentinels:
// unknown staff → 404, empty class name → 400.
//
// Rendered through common.UnwrapRenderer so the user-facing branches carry
// just the sentinel message (ErrEmptySchoolClass is a deliberate German
// string) instead of EducationError's "education: {Op}: …" prefix; the 500
// fallback keeps the wrapped error for the logs. Same pattern as
// api/groups/errors.go.
var schoolClassErrorRules = []common.ErrorRule{
	{Target: educationSvc.ErrStaffNotFound, Render: func(error) render.Renderer {
		return common.ErrorNotFound(errors.New(common.MsgStaffNotFound))
	}},
	{Target: educationSvc.ErrEmptySchoolClass, Render: common.ErrorInvalidRequest},
}

var renderSchoolClass = common.UnwrapRenderer[*educationSvc.EducationError](schoolClassErrorRules, common.ErrorInternalServer)

func renderSchoolClassError(w http.ResponseWriter, r *http.Request, err error) {
	common.RenderError(w, r, renderSchoolClass(err))
}

// getStaffSchoolClasses returns the school classes assigned to a staff member.
func (rs *Resource) getStaffSchoolClasses(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidStaffID)))
		return
	}

	classes, err := rs.EducationService.GetStaffSchoolClasses(r.Context(), id)
	if err != nil {
		renderSchoolClassError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, SchoolClassesResponse{StaffID: id, SchoolClasses: classes}, "School classes retrieved successfully")
}

// updateStaffSchoolClasses replaces the school class assignments of a staff
// member with the submitted set.
func (rs *Resource) updateStaffSchoolClasses(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(common.MsgInvalidStaffID)))
		return
	}

	req := &SchoolClassesRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	if err := rs.EducationService.SetStaffSchoolClasses(r.Context(), id, req.SchoolClasses); err != nil {
		renderSchoolClassError(w, r, err)
		return
	}

	classes, err := rs.EducationService.GetStaffSchoolClasses(r.Context(), id)
	if err != nil {
		renderSchoolClassError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, SchoolClassesResponse{StaffID: id, SchoolClasses: classes}, "School classes updated successfully")
}
