package notifications

import (
	"errors"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	notificationsService "github.com/moto-nrw/project-phoenix/services/notifications"
)

// notificationTypePattern bounds the path segment before it reaches the
// service. The catalogue validates the value itself; this only keeps obviously
// malformed input out of the logs and the query.
var notificationTypePattern = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)

// preferenceRequest is the body of a single toggle.
type preferenceRequest struct {
	Enabled *bool `json:"enabled"`
}

// Bind implements render.Binder. Enabled is a pointer so a missing field is
// rejected rather than silently read as false — switching something off by
// omission would be a bad surprise.
func (p *preferenceRequest) Bind(_ *http.Request) error {
	if p.Enabled == nil {
		return errors.New("enabled is required")
	}
	return nil
}

// listPreferences returns the staff catalogue with this account's decisions.
func (rs *Resource) listPreferences(w http.ResponseWriter, r *http.Request) {
	if rs.PreferenceService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("notification preferences are not configured")))
		return
	}

	accountID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	overview, err := rs.PreferenceService.GetForAccount(r.Context(), accountID, notificationsService.PortalStaff)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, overview, "Notification preferences retrieved successfully")
}

// setPreference records one decision for this account.
func (rs *Resource) setPreference(w http.ResponseWriter, r *http.Request) {
	if rs.PreferenceService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("notification preferences are not configured")))
		return
	}

	notificationType := chi.URLParam(r, "type")
	if !notificationTypePattern.MatchString(notificationType) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid notification type")))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)
	req := &preferenceRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	accountID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	err := rs.PreferenceService.SetForAccount(r.Context(), accountID, notificationType, *req.Enabled)
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

// deleteAllPreferences switches every decision of this account off. It backs
// the card's "Alle deaktivieren" action.
func (rs *Resource) deleteAllPreferences(w http.ResponseWriter, r *http.Request) {
	if rs.PreferenceService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("notification preferences are not configured")))
		return
	}

	accountID := int64(jwt.ClaimsFromCtx(r.Context()).ID)
	if err := rs.PreferenceService.DisableAllForAccount(r.Context(), accountID); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusNoContent, nil, "")
}
