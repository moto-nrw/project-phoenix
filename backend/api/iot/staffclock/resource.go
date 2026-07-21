package staffclock

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	staffclockSvc "github.com/moto-nrw/project-phoenix/services/iot/staffclock"
)

type Resource struct {
	service *staffclockSvc.Service
}

func NewResource(service *staffclockSvc.Service) *Resource {
	return &Resource{service: service}
}

func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Post("/staff-clock", rs.execute)
	r.Post("/staff-clock/state", rs.getState)
	return r
}

func (rs *Resource) getState(w http.ResponseWriter, r *http.Request) {
	req := new(StateRequest)
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "invalid_staff_clock_request"))
		return
	}
	state, err := rs.service.GetState(r.Context(), req.RFIDTag)
	if err != nil {
		common.RenderError(w, r, classifyError(err))
		return
	}
	common.Respond(w, r, http.StatusOK, state, "Staff clock state retrieved")
}

func (rs *Resource) execute(w http.ResponseWriter, r *http.Request) {
	req := new(CommandRequest)
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "invalid_staff_clock_request"))
		return
	}
	state, err := rs.service.Execute(r.Context(), staffclockSvc.Command{
		RFIDTag:                req.RFIDTag,
		Action:                 req.Action,
		Status:                 req.Status,
		Reason:                 req.Reason,
		PlannedDurationMinutes: req.PlannedDurationMinutes,
	})
	if err != nil {
		common.RenderError(w, r, classifyError(err))
		return
	}
	common.Respond(w, r, http.StatusOK, state, "Staff clock action completed")
}
