package timetablehttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type planningTrackRequest struct {
	Name      string `json:"name"`
	Color     string `json:"color"`
	SortOrder int    `json:"sort_order"`
}

func (req *planningTrackRequest) Bind(_ *http.Request) error {
	invalid := common.CodeTimetablePlanningTrackInvalid
	if req.Name == "" {
		return invalidField(invalid, "name", "name is required")
	}
	if req.Color == "" {
		return invalidField(invalid, "color", "color is required")
	}
	if req.SortOrder < 0 {
		return invalidField(invalid, "sort_order", "sort_order cannot be negative")
	}
	return nil
}

type planningTrackOrderRequest struct {
	IDs []int64 `json:"ids"`
}

func (req *planningTrackOrderRequest) Bind(_ *http.Request) error {
	if len(req.IDs) == 0 {
		return invalidField(common.CodeTimetablePlanningTrackInvalid, "ids", "ids are required")
	}
	return nil
}

func (rs *Resource) listPlanningTracks(w http.ResponseWriter, r *http.Request) {
	service, ok := rs.planningTrackService(w, r)
	if !ok {
		return
	}
	tracks, err := service.ListAllPlanningTracks(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load planning tracks failed", err))
		return
	}
	common.Respond(w, r, http.StatusOK, tracks, "Planning tracks retrieved")
}

func (rs *Resource) createPlanningTrack(w http.ResponseWriter, r *http.Request) {
	service, ok := rs.planningTrackService(w, r)
	if !ok {
		return
	}
	req := new(planningTrackRequest)
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, bindErrorRenderer(err))
		return
	}
	track, err := service.AddPlanningTrack(r.Context(), timetable.PlanningTrackDraft{
		Name: req.Name, Color: req.Color, SortOrder: req.SortOrder,
	})
	if err != nil {
		renderPlanningTrackError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, track, "Planning track created")
}

func (rs *Resource) updatePlanningTrack(w http.ResponseWriter, r *http.Request) {
	service, ok := rs.planningTrackService(w, r)
	if !ok {
		return
	}
	id, ok := planningTrackID(w, r)
	if !ok {
		return
	}
	req := new(planningTrackRequest)
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, bindErrorRenderer(err))
		return
	}
	track, err := service.EditPlanningTrack(r.Context(), id, timetable.PlanningTrackDraft{
		Name: req.Name, Color: req.Color, SortOrder: req.SortOrder,
	})
	if err != nil {
		renderPlanningTrackError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, track, "Planning track updated")
}

func (rs *Resource) reorderPlanningTracks(w http.ResponseWriter, r *http.Request) {
	service, ok := rs.planningTrackService(w, r)
	if !ok {
		return
	}
	req := new(planningTrackOrderRequest)
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, bindErrorRenderer(err))
		return
	}
	if err := service.OrderPlanningTracks(r.Context(), req.IDs); err != nil {
		renderPlanningTrackError(w, r, err)
		return
	}
	tracks, err := service.ListAllPlanningTracks(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("reload planning tracks failed", err))
		return
	}
	common.Respond(w, r, http.StatusOK, tracks, "Planning tracks reordered")
}

func (rs *Resource) archivePlanningTrack(w http.ResponseWriter, r *http.Request) {
	service, ok := rs.planningTrackService(w, r)
	if !ok {
		return
	}
	id, ok := planningTrackID(w, r)
	if !ok {
		return
	}
	track, err := service.ArchivePlanningTrack(r.Context(), id)
	if err != nil {
		renderPlanningTrackError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, track, "Planning track archived")
}

func (rs *Resource) restorePlanningTrack(w http.ResponseWriter, r *http.Request) {
	service, ok := rs.planningTrackService(w, r)
	if !ok {
		return
	}
	id, ok := planningTrackID(w, r)
	if !ok {
		return
	}
	track, err := service.RestorePlanningTrack(r.Context(), id)
	if err != nil {
		renderPlanningTrackError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, track, "Planning track restored")
}

func (rs *Resource) planningTrackService(w http.ResponseWriter, r *http.Request) (timetable.PlanningTrackAdministration, bool) {
	if rs.PlanningTracks == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("planning track service is not configured")))
		return nil, false
	}
	return rs.PlanningTracks, true
}

func planningTrackID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil || rctx.URLParam("id") == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid planning track id")))
		return 0, false
	}
	return common.ParsePositiveInt64IDWithError(w, r, "id", "invalid planning track id")
}

func renderPlanningTrackError(w http.ResponseWriter, r *http.Request, err error) {
	common.RenderError(w, r, common.RenderWithRules(err, planningTrackErrorRules,
		common.ErrorInternalServerRenderer("planning track operation failed")))
}

// planningTrackErrorRules keep the editor's statuses and name each refusal
// with its own code (#2516); an invalid draft marks its field.
var planningTrackErrorRules = []common.ErrorRule{
	{Target: timetable.ErrPlanningTrackNotFound, Render: notFoundWithCode(common.CodeTimetablePlanningTrackNotFound)},
	{Match: isInvalidPlanningTrackField, Render: renderInvalidPlanningTrackField},
	{Target: timetable.ErrInvalidPlanningTrack, Render: invalidWithCode(common.CodeTimetablePlanningTrackInvalid)},
	{Target: timetable.ErrPlanningTrackArchived, Render: invalidWithCode(common.CodeTimetablePlanningTrackArchived)},
	{Target: timetable.ErrPlanningTrackNameTaken, Render: func(err error) render.Renderer {
		return common.ErrorConflictOnField(err, common.CodeTimetablePlanningTrackNameTaken, "name")
	}},
}

func isInvalidPlanningTrackField(err error) bool {
	var invalid *timetable.InvalidPlanningTrackError
	return errors.As(err, &invalid)
}

func renderInvalidPlanningTrackField(err error) render.Renderer {
	var invalid *timetable.InvalidPlanningTrackError
	errors.As(err, &invalid)
	return common.ErrorInvalidOnField(err, common.CodeTimetablePlanningTrackInvalid, invalid.Field)
}
