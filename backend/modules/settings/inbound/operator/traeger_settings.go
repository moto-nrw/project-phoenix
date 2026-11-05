package operator

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
)

// TraegerOperations is the operator's side of the Träger level (ADR 0086,
// #5748): the settings of the Träger the route names, with the registry's
// Träger-level keys, validation and write rights of the Träger-Portal. The
// operations open the Träger transaction of that Träger themselves.
type TraegerOperations interface {
	OperatorSchema(ctx context.Context, organizationID int64) (any, error)
	OperatorSetValue(ctx context.Context, organizationID, operatorID int64, key string, value any) error
	OperatorResetValue(ctx context.Context, organizationID, operatorID int64, key string) error
}

// TraegerSettingsResource serves the Träger settings in the operator
// dashboard at /operator/organizations/{id}/traeger-settings. The operator
// router mounts the handlers behind its session chain and the operator
// audit of Organisation & Tenancy.
type TraegerSettingsResource struct {
	operations TraegerOperations
	operatorID OperatorIdentity
}

// NewTraegerSettingsResource builds the resource; both collaborators are
// required.
func NewTraegerSettingsResource(operations TraegerOperations, operatorID OperatorIdentity) *TraegerSettingsResource {
	if operations == nil || operatorID == nil {
		panic("operator traeger settings: operations and the operator identity are required")
	}
	return &TraegerSettingsResource{operations: operations, operatorID: operatorID}
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
	err := rs.operations.OperatorSetValue(r.Context(), organizationID, rs.operatorID(r.Context()), chi.URLParam(r, "key"), req.Value)
	if err != nil {
		renderOperatorSettingsError(w, r, err)
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
	if err := rs.operations.OperatorResetValue(r.Context(), organizationID, rs.operatorID(r.Context()), chi.URLParam(r, "key")); err != nil {
		renderOperatorSettingsError(w, r, err)
		return
	}
	common.RespondNoContent(w, r)
}
