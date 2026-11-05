package operator

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
)

// TraegerOperations is the operator's side of the Träger level (ADR 0086,
// #5748): the settings of the Träger the route names, with the registry's
// Träger-level keys, validation and write rights of the Träger-Portal. The
// operations open the Träger transaction of that Träger themselves and run
// beforeChange after their checks, before it.
type TraegerOperations interface {
	OperatorSchema(ctx context.Context, organizationID int64) (any, error)
	OperatorSetValue(ctx context.Context, organizationID, operatorID int64, key string, value any, beforeChange func(context.Context) error) error
	OperatorResetValue(ctx context.Context, organizationID, operatorID int64, key string, beforeChange func(context.Context) error) error
}

// TraegerChangeAudit records the operator audit entry of a change of key of
// the Träger the request names. It answers false after writing the error
// response itself, an unknown Träger included; the root binds it to the
// operator audit of Organisation & Tenancy.
type TraegerChangeAudit func(w http.ResponseWriter, r *http.Request, key string, reset bool) bool

// errChangeNotAudited stops a change whose operator audit failed; the audit
// has written the response.
var errChangeNotAudited = errors.New("operator audit refused the change")

// TraegerSettingsResource serves the Träger settings in the operator
// dashboard at /operator/organizations/{id}/traeger-settings. The operator
// router mounts the handlers behind its session chain.
type TraegerSettingsResource struct {
	operations TraegerOperations
	operatorID OperatorIdentity
	audit      TraegerChangeAudit
}

// NewTraegerSettingsResource builds the resource; every collaborator is
// required.
func NewTraegerSettingsResource(operations TraegerOperations, operatorID OperatorIdentity, audit TraegerChangeAudit) *TraegerSettingsResource {
	if operations == nil || operatorID == nil || audit == nil {
		panic("operator traeger settings: operations, the operator identity and the audit are required")
	}
	return &TraegerSettingsResource{operations: operations, operatorID: operatorID, audit: audit}
}

// beforeChange binds the operator audit of one request to the change.
func (rs *TraegerSettingsResource) beforeChange(w http.ResponseWriter, r *http.Request, key string, reset bool) func(context.Context) error {
	return func(context.Context) error {
		if !rs.audit(w, r, key, reset) {
			return errChangeNotAudited
		}
		return nil
	}
}

// renderChangeError renders a failed change unless the audit already
// answered.
func renderChangeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errChangeNotAudited) {
		return
	}
	renderOperatorSettingsError(w, r, err)
}

// GetTraegerSettingsSchema answers GET .../traeger-settings/schema.
func (rs *TraegerSettingsResource) GetTraegerSettingsSchema(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid organization ID")
	if !ok {
		return
	}
	schema, err := rs.operations.OperatorSchema(r.Context(), organizationID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("Failed to retrieve Träger settings schema", err))
		return
	}
	common.Respond(w, r, http.StatusOK, schema, "Schema retrieved successfully")
}

// SetTraegerSettingValue answers PUT .../traeger-settings/values/{key}.
func (rs *TraegerSettingsResource) SetTraegerSettingValue(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid organization ID")
	if !ok {
		return
	}
	var req setSchoolSettingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.OperatorInvalidRequest(err))
		return
	}
	key := chi.URLParam(r, "key")
	err := rs.operations.OperatorSetValue(r.Context(), organizationID, rs.operatorID(r.Context()), key, req.Value, rs.beforeChange(w, r, key, false))
	if err != nil {
		renderChangeError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Value updated successfully")
}

// ResetTraegerSettingValue answers DELETE .../traeger-settings/values/{key}.
func (rs *TraegerSettingsResource) ResetTraegerSettingValue(w http.ResponseWriter, r *http.Request) {
	organizationID, ok := common.ParseInt64IDWithError(w, r, "id", "invalid organization ID")
	if !ok {
		return
	}
	key := chi.URLParam(r, "key")
	if err := rs.operations.OperatorResetValue(r.Context(), organizationID, rs.operatorID(r.Context()), key, rs.beforeChange(w, r, key, true)); err != nil {
		renderChangeError(w, r, err)
		return
	}
	common.RespondNoContent(w, r)
}
