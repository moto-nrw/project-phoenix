// Package timetable — POST /api/timetable/templates/{id}/end handler.
//
// "Dieser und alle folgenden löschen": caps the template's schedules and
// rosters at the effective date, then deletes planned future materialized
// instances. Historical rows remain untouched.
package timetable

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

type endTemplateRequest struct {
	EffectiveDate string `json:"effective_date"`
}

func (req *endTemplateRequest) Bind(_ *http.Request) error {
	if req.EffectiveDate == "" {
		return errors.New("effective_date is required (YYYY-MM-DD)")
	}
	return nil
}

type endTemplateResponse struct {
	TemplateID       int64  `json:"template_id"`
	EffectiveDate    string `json:"effective_date"`
	DeletedInstances int    `json:"deleted_instances"`
}

func (rs *Resource) endTemplate(w http.ResponseWriter, r *http.Request) {
	id, ok := templateIDFromRequest(w, r)
	if !ok {
		return
	}
	if rs.TemplateSplitService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("template split service not wired")))
		return
	}
	req := &endTemplateRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	effectiveDate, err := timezone.ParseDate(req.EffectiveDate)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid effective_date format, expected YYYY-MM-DD")))
		return
	}

	result, err := rs.TemplateSplitService.EndFromDate(r.Context(), scheduleSvc.TemplateEndInput{
		TemplateID:    id,
		EffectiveDate: effectiveDate,
	})
	if err != nil {
		renderTemplateEndError(w, r, err)
		return
	}

	common.Respond(w, r, http.StatusOK, endTemplateResponse{
		TemplateID:       result.TemplateID,
		EffectiveDate:    result.EffectiveDate.String(),
		DeletedInstances: result.DeletedInstances,
	}, "Template ended")
}

func renderTemplateEndError(w http.ResponseWriter, r *http.Request, err error) {
	if renderTemplateCareOfferingConflict(w, r, err) {
		return
	}
	switch {
	case errors.Is(err, scheduleSvc.ErrSplitTemplateNotFound):
		common.RenderError(w, r, common.ErrorNotFound(errors.New("template not found")))
	case errors.Is(err, scheduleSvc.ErrSplitInvalidInput):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServerWrap("end template failed", err))
	}
}
