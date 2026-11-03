package parentbillinghttp

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/api/common"
	capability "github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// CostBearerRouter returns the routes of the cost bearers of a Träger
// mounted at /traeger/cost-bearers (#5627). Like the contribution rules,
// every route sits behind the Träger session, needs the Träger right
// "Abrechnung" and runs in the Träger transaction of the session's Träger
// (ADR 0105).
func (rs *RuleResource) CostBearerRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, withTx common.Middleware) {
		billing := r.With(rs.requireBilling, withTx)
		billing.Get("/", rs.listCostBearers)
		billing.Post("/", rs.saveCostBearer)
		billing.Put("/{id}", rs.saveCostBearer)
		billing.Delete("/{id}", rs.deleteCostBearer)
	})
	return r
}

// CostBearerRequest is the body of POST /traeger/cost-bearers and PUT
// /traeger/cost-bearers/{id}. A missing active is false, so a form always
// sends it.
type CostBearerRequest struct {
	Name          string `json:"name"`
	Address       string `json:"address"`
	FileReference string `json:"file_reference"`
	Active        bool   `json:"active"`
}

// Bind accepts the body; the Owner validates the cost bearer.
func (req *CostBearerRequest) Bind(*http.Request) error { return nil }

func (req *CostBearerRequest) input() capability.CostBearerInput {
	return capability.CostBearerInput(*req)
}

func (rs *RuleResource) listCostBearers(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	bearers, err := rs.rules.ListCostBearers(r.Context(), organizationID)
	if err != nil {
		renderCostBearerError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, bearers, "Cost bearers retrieved successfully")
}

// saveCostBearer creates a cost bearer on POST and changes the one {id}
// names on PUT.
func (rs *RuleResource) saveCostBearer(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	bearerID, status, ok := costBearerTarget(w, r)
	if !ok {
		return
	}
	req := &CostBearerRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	bearer, err := rs.rules.SaveCostBearer(r.Context(), organizationID, bearerID, req.input())
	if err != nil {
		renderCostBearerError(w, r, err)
		return
	}
	common.Respond(w, r, status, bearer, "Cost bearer saved successfully")
}

// costBearerTarget names what a save writes: a new cost bearer on POST
// (zero, 201), the existing one {id} names on PUT (200). A PUT never
// creates: an ID that is not positive names no cost bearer and answers 404,
// so a repeated PUT cannot add duplicates.
func costBearerTarget(w http.ResponseWriter, r *http.Request) (int64, int, bool) {
	if r.Method != http.MethodPut {
		return 0, http.StatusCreated, true
	}
	id, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return 0, 0, false
	}
	if id <= 0 {
		renderCostBearerError(w, r, capability.ErrCostBearerNotFound)
		return 0, 0, false
	}
	return id, http.StatusOK, true
}

func (rs *RuleResource) deleteCostBearer(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := traegerOf(w, r)
	if !ok {
		return
	}
	bearerID, err := common.ParseIDParam(r, "id")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.rules.DeleteCostBearer(r.Context(), organizationID, bearerID); err != nil {
		renderCostBearerError(w, r, err)
		return
	}
	common.RespondNoContent(w, r)
}

// renderCostBearerError answers a refused cost bearer.
func renderCostBearerError(w http.ResponseWriter, r *http.Request, err error) {
	var fields validation.Errors
	switch {
	case errors.As(err, &fields):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, common.CodeParentBillingCostBearerInvalid))
	case errors.Is(err, capability.ErrCostBearerNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, common.CodeParentBillingCostBearerNotFound))
	case errors.Is(err, capability.ErrCostBearerInUse):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, common.CodeParentBillingCostBearerInUse))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
