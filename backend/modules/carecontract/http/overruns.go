package carecontracthttp

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// OverrunResource serves /traeger/care-time-overruns (#4683, ADR 0156): the
// figures per school behind the right "Berichte lesen", the children by name
// only behind the right "Verträge verwalten". Both read each school in its
// own tenant transaction, so the routes carry no transaction.
type OverrunResource struct {
	evaluations   carecontract.OverrunEvaluations
	requireReport common.Middleware
	requireNames  common.Middleware
}

func NewOverrunResource(evaluations carecontract.OverrunEvaluations, requireReport, requireNames common.Middleware) *OverrunResource {
	if evaluations == nil || requireReport == nil || requireNames == nil {
		panic("care contract http: the evaluation and both right checks are required")
	}
	return &OverrunResource{evaluations: evaluations, requireReport: requireReport, requireNames: requireNames}
}

// Router returns the routes mounted at /traeger/care-time-overruns.
func (rs *OverrunResource) Router(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, _ common.Middleware) {
		r.With(rs.requireReport).Get("/", rs.summary)
		r.With(rs.requireNames).Get("/children", rs.children)
	})
	return r
}

func (rs *OverrunResource) summary(w http.ResponseWriter, r *http.Request) {
	rs.respond(w, r, rs.evaluations.OverrunSummary)
}

func (rs *OverrunResource) children(w http.ResponseWriter, r *http.Request) {
	rs.respond(w, r, rs.evaluations.OverrunChildren)
}

type evaluation func(context.Context, carecontract.Reviewer, carecontract.OverrunFilter) (carecontract.OverrunEvaluation, error)

func (rs *OverrunResource) respond(w http.ResponseWriter, r *http.Request, read evaluation) {
	reviewer, ok := reviewerOf(w, r)
	if !ok {
		return
	}
	filter, err := overrunFilter(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	result, err := read(r.Context(), reviewer, filter)
	if errors.Is(err, carecontract.ErrOverrunWindowInvalid) {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err != nil {
		renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, result, "Care time overruns retrieved successfully")
}

// overrunFilter reads from, to (YYYY-MM-DD) and the optional school_id.
func overrunFilter(r *http.Request) (carecontract.OverrunFilter, error) {
	query := r.URL.Query()
	from, err := calendar.ParseDate(query.Get("from"))
	if err != nil {
		return carecontract.OverrunFilter{}, errors.New("invalid from")
	}
	to, err := calendar.ParseDate(query.Get("to"))
	if err != nil {
		return carecontract.OverrunFilter{}, errors.New("invalid to")
	}
	schoolID, err := optionalID(r, "school_id")
	if err != nil {
		return carecontract.OverrunFilter{}, err
	}
	return carecontract.OverrunFilter{From: from, To: to, SchoolID: schoolID}, nil
}
