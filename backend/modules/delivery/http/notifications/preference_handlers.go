package notifications

import (
	"errors"
	"net/http"
	"regexp"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
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
		rs.Runtime.Internal(w, r, errors.New("notification preferences are not configured"))
		return
	}

	accountID := int64(rs.Runtime.Caller(r.Context()).ID)
	overview, err := rs.PreferenceService.GetForAccount(r.Context(), accountID, requestPortal(r))
	if err != nil {
		rs.Runtime.Internal(w, r, err)
		return
	}

	rs.Runtime.Respond(w, r, http.StatusOK, overview, "Notification preferences retrieved successfully")
}

// setPreference records one decision for this account.
func (rs *Resource) setPreference(w http.ResponseWriter, r *http.Request) {
	if rs.PreferenceService == nil {
		rs.Runtime.Internal(w, r, errors.New("notification preferences are not configured"))
		return
	}

	notificationType := chi.URLParam(r, "type")
	if !notificationTypePattern.MatchString(notificationType) {
		rs.Runtime.Invalid(w, r, errors.New("invalid notification type"))
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)
	req := &preferenceRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.Runtime.Invalid(w, r, err)
		return
	}

	accountID := int64(rs.Runtime.Caller(r.Context()).ID)
	err := rs.PreferenceService.SetForPortalAccount(r.Context(), accountID, requestPortal(r), notificationType, *req.Enabled)
	switch {
	case rs.PreferenceService.IsUnknownType(err):
		rs.Runtime.Invalid(w, r, err)
		return
	case err != nil:
		rs.Runtime.Internal(w, r, err)
		return
	}

	rs.Runtime.Respond(w, r, http.StatusNoContent, nil, "")
}

// deleteAllPreferences switches every decision of this account off. It backs
// the card's "Alle deaktivieren" action.
func (rs *Resource) deleteAllPreferences(w http.ResponseWriter, r *http.Request) {
	if rs.PreferenceService == nil {
		rs.Runtime.Internal(w, r, errors.New("notification preferences are not configured"))
		return
	}

	accountID := int64(rs.Runtime.Caller(r.Context()).ID)
	if err := rs.PreferenceService.DisableAllForPortalAccount(r.Context(), accountID, requestPortal(r)); err != nil {
		rs.Runtime.Internal(w, r, err)
		return
	}

	rs.Runtime.Respond(w, r, http.StatusNoContent, nil, "")
}
