package parent

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	notificationsService "github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
)

var parentNotificationTypePattern = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)

type parentPreferenceRequest struct {
	// Pointer so a missing field is rejected rather than read as false:
	// switching something off by omission would be a bad surprise.
	Enabled *bool `json:"enabled"`
}

// listNotificationPreferences returns the guardian catalogue with this
// account's decisions merged across every school the family belongs to.
func (rs *Resource) listNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	if rs.PreferenceService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("notification preferences are not configured")))
		return
	}

	accountID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	overview, err := rs.PreferenceService.GetForParent(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, overview, "Notification preferences retrieved successfully")
}

// setNotificationPreference applies one decision to every school of the family.
func (rs *Resource) setNotificationPreference(w http.ResponseWriter, r *http.Request) {
	if rs.PreferenceService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("notification preferences are not configured")))
		return
	}

	notificationType := chi.URLParam(r, "type")
	if !parentNotificationTypePattern.MatchString(notificationType) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid notification type")))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)
	req := parentPreferenceRequest{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if req.Enabled == nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("enabled is required")))
		return
	}

	accountID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	err := rs.PreferenceService.SetForParent(r.Context(), accountID, notificationType, *req.Enabled)
	switch {
	case errors.Is(err, notificationsService.ErrUnknownNotificationType):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	case err != nil:
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusNoContent, nil, "")
}

// deleteAllNotificationPreferences switches every decision off at every school.
func (rs *Resource) deleteAllNotificationPreferences(w http.ResponseWriter, r *http.Request) {
	if rs.PreferenceService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("notification preferences are not configured")))
		return
	}

	accountID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	if err := rs.PreferenceService.DisableAllForParent(r.Context(), accountID); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusNoContent, nil, "")
}
