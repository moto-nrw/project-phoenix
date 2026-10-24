package notifications

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

// emailSubscriptionResponse is the caller's own decision for one opt-in
// e-mail type (#3780).
type emailSubscriptionResponse struct {
	Enabled bool `json:"enabled"`
}

// getEmailSubscription returns the caller's decision for an e-mail type.
func (rs *Resource) getEmailSubscription(w http.ResponseWriter, r *http.Request) {
	if !rs.preferencesConfigured(w, r) {
		return
	}
	ctx := r.Context()
	enabled, err := rs.PreferenceService.EmailSubscribed(ctx, int64(rs.Runtime.Caller(ctx).ID),
		rs.Runtime.Permissions(ctx), chi.URLParam(r, "type"))
	if err != nil {
		rs.renderEmailSubscriptionError(w, r, err)
		return
	}
	rs.Runtime.Respond(w, r, http.StatusOK, emailSubscriptionResponse{Enabled: enabled}, "E-mail subscription retrieved successfully")
}

// setEmailSubscription records the caller's decision for an e-mail type.
func (rs *Resource) setEmailSubscription(w http.ResponseWriter, r *http.Request) {
	if !rs.preferencesConfigured(w, r) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)
	req := &preferenceRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.Runtime.Invalid(w, r, err)
		return
	}
	ctx := r.Context()
	if err := rs.PreferenceService.SetEmailSubscribed(ctx, int64(rs.Runtime.Caller(ctx).ID),
		rs.Runtime.Permissions(ctx), chi.URLParam(r, "type"), *req.Enabled); err != nil {
		rs.renderEmailSubscriptionError(w, r, err)
		return
	}
	rs.Runtime.Respond(w, r, http.StatusNoContent, nil, "")
}

func (rs *Resource) preferencesConfigured(w http.ResponseWriter, r *http.Request) bool {
	if rs.PreferenceService == nil {
		rs.Runtime.Internal(w, r, errors.New("notification preferences are not configured"))
		return false
	}
	return true
}

// renderEmailSubscriptionError maps the service's refusals: a push or unknown
// type is not found here, a missing permission is forbidden.
func (rs *Resource) renderEmailSubscriptionError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case rs.PreferenceService.IsUnknownType(err):
		rs.Runtime.NotFound(w, r, errors.New("unknown e-mail notification type"))
	case rs.PreferenceService.IsPermissionRequired(err):
		rs.Runtime.Forbidden(w, r, errors.New("insufficient permissions"))
	default:
		rs.Runtime.Internal(w, r, err)
	}
}
