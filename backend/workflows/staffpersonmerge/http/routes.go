// Package http serves operator-only duplicate repair routes.
package http

import (
	"context"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/workflows/staffpersonmerge"
)

type Capability interface {
	Preview(rctx context.Context, schoolID int64) ([]staffpersonmerge.Candidate, error)
	Execute(rctx context.Context, schoolID int64, request staffpersonmerge.Request) error
}

type resource struct{ workflow Capability }

func Router(workflow Capability, bindScope func(http.Handler) http.Handler) http.Handler {
	if workflow == nil || bindScope == nil {
		panic("staff person merge HTTP: workflow and scope binding required")
	}
	rs := resource{workflow: workflow}
	r := chi.NewRouter()
	r.Use(bindScope, common.TenantTxMiddleware)
	r.Get("/{schoolId}", common.IDFetch("schoolId", "invalid school ID", workflow.Preview, errorRenderer, "staff person merge candidates"))
	r.Post("/{schoolId}", rs.execute)
	return r
}
func (rs resource) execute(w http.ResponseWriter, r *http.Request) {
	schoolID, err := strconv.ParseInt(chi.URLParam(r, "schoolId"), 10, 64)
	if err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}
	var input staffpersonmerge.Request
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := render.DecodeJSON(r.Body, &input); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}
	if err := rs.workflow.Execute(r.Context(), schoolID, input); err != nil {
		respondError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "staff persons merged")
}

var errorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: staffpersonmerge.ErrInvalid, Render: common.OperatorInvalidRequest},
	{Target: staffpersonmerge.ErrUnauthorized, Render: common.ErrorForbidden},
	{Target: staffpersonmerge.ErrConflict, Render: common.ErrorConflict},
}, common.ErrorInternalServer)

func respondError(w http.ResponseWriter, r *http.Request, err error) {
	common.RenderError(w, r, errorRenderer(err))
}
