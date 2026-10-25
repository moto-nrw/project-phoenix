package settingshttp

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// TraegerOperations is the Träger level of the settings system (ADR 0086):
// the Träger values of the session's Träger, checked against its Träger
// rights. The Träger comes from the session, never from the request.
type TraegerOperations interface {
	Schema(ctx context.Context, organizationID int64, rights []string) (any, error)
	SetValue(ctx context.Context, organizationID, accountID int64, rights []string, key string, value any) error
	ResetValue(ctx context.Context, organizationID, accountID int64, rights []string, key string) error
	ClassifyError(error) string
}

// TraegerSettingsResource serves the Träger settings in the Träger-Portal.
// Reading needs the read right; writing one of the write rights, and the
// operations check the right the setting names.
type TraegerSettingsResource struct {
	operations  TraegerOperations
	readRight   string
	writeRights []string
}

// NewTraegerSettingsResource builds the resource. The rights are the names
// Identity & Access gives them; the root supplies them.
func NewTraegerSettingsResource(operations TraegerOperations, readRight string, writeRights ...string) *TraegerSettingsResource {
	if operations == nil || readRight == "" || len(writeRights) == 0 {
		panic("traeger settings http: operations and rights are required")
	}
	return &TraegerSettingsResource{operations: operations, readRight: readRight, writeRights: writeRights}
}

// TraegerRouter returns the routes mounted at /traeger/settings inside the
// Träger route group the portal supplies. The routes take no request
// transaction: the operations open the Träger transaction themselves, so a
// change has committed before the schools hear of it (ADR 0107).
func (rs *TraegerSettingsResource) TraegerRouter(protectedTraegerRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protectedTraegerRoutes(r, func(r chi.Router, _ common.Middleware) {
		r.With(common.RequiresPermission(rs.readRight)).Get("/schema", rs.schema)
		write := common.RequiresAnyPermission(rs.writeRights...)
		r.With(write).Put("/values/{key}", rs.setValue)
		r.With(write).Delete("/values/{key}", rs.resetValue)
	})
	return r
}

// traegerActor is the Träger, account and rights of the session.
func traegerActor(w http.ResponseWriter, r *http.Request) (organizationID, accountID int64, rights []string, ok bool) {
	principal, err := common.CurrentPrincipal(r.Context())
	if err != nil || principal.OrganizationID() <= 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(common.ErrUnauthorized))
		return 0, 0, nil, false
	}
	return principal.OrganizationID(), principal.AccountID(), principal.Permissions(), true
}

func (rs *TraegerSettingsResource) schema(w http.ResponseWriter, r *http.Request) {
	organizationID, _, rights, ok := traegerActor(w, r)
	if !ok {
		return
	}
	schema, err := rs.operations.Schema(r.Context(), organizationID, rights)
	if err != nil {
		rs.renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, schema, "Schema retrieved successfully")
}

func (rs *TraegerSettingsResource) setValue(w http.ResponseWriter, r *http.Request) {
	organizationID, accountID, rights, ok := traegerActor(w, r)
	if !ok {
		return
	}
	var req setValueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.operations.SetValue(r.Context(), organizationID, accountID, rights, chi.URLParam(r, "key"), req.Value); err != nil {
		rs.renderError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Value updated successfully")
}

func (rs *TraegerSettingsResource) resetValue(w http.ResponseWriter, r *http.Request) {
	organizationID, accountID, rights, ok := traegerActor(w, r)
	if !ok {
		return
	}
	if err := rs.operations.ResetValue(r.Context(), organizationID, accountID, rights, chi.URLParam(r, "key")); err != nil {
		rs.renderError(w, r, err)
		return
	}
	common.RespondNoContent(w, r)
}

func (rs *TraegerSettingsResource) renderError(w http.ResponseWriter, r *http.Request, err error) {
	switch rs.operations.ClassifyError(err) {
	case settingsErrorNotFound:
		common.RenderError(w, r, common.ErrorNotFound(err))
	case settingsErrorInvalid:
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case settingsErrorForbidden:
		common.RenderError(w, r, common.ErrorForbidden(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
