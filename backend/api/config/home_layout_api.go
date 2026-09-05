package config

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Start page composition (#2875).
//
// Two audiences share one surface. Everybody who can open the app reads
// GET /api/settings/home-layout, because the start page cannot render without
// knowing what this person chose and what the school prescribes; only
// config:update may write the school's prescription. That is why the read
// carries no permission requirement and the policy write does.
//
// The block keys in these payloads are opaque here. The catalogue of start page
// blocks lives in the frontend; this adapter passes keys through and the
// application layer checks their shape.

type homeLayoutOverridesRequest struct {
	// Overrides carries only the deviations from the recommended start page.
	// An absent block means "undecided", not "hidden" — that is what lets a
	// block added later reach existing accounts in its intended state.
	Overrides map[string]bool `json:"overrides"`
}

type homeBlockPoliciesRequest struct {
	// Policies maps a block key to "optional", "required" or "disabled".
	// Optional entries are dropped before storing: no opinion is the default.
	Policies map[string]string `json:"policies"`
}

// getHomeLayout returns the person's own composition together with the
// school's prescription, in one call — the start page needs both to decide
// what to render and must not paint twice.
func (rs *SettingsResource) getHomeLayout(w http.ResponseWriter, r *http.Request) {
	actor := rs.runtime.Actor(r.Context())
	if actor.AccountID <= 0 {
		rs.runtime.RenderError(w, r, http.StatusUnauthorized, errors.New("no account context"))
		return
	}

	view, err := rs.operations.HomeLayout(r.Context(), actor.TenantID, actor.AccountID, actor.Permissions)
	if err != nil {
		rs.renderSettingsError(w, r, err)
		return
	}

	rs.runtime.Respond(w, r, http.StatusOK, view, "")
}

// setHomeLayout replaces the person's own composition.
func (rs *SettingsResource) setHomeLayout(w http.ResponseWriter, r *http.Request) {
	actor := rs.runtime.Actor(r.Context())
	if actor.AccountID <= 0 {
		rs.runtime.RenderError(w, r, http.StatusUnauthorized, errors.New("no account context"))
		return
	}

	var req homeLayoutOverridesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.runtime.RenderError(w, r, http.StatusBadRequest, err)
		return
	}

	if err := rs.operations.SetHomeLayout(r.Context(), actor.TenantID, actor.AccountID, req.Overrides); err != nil {
		rs.renderSettingsError(w, r, err)
		return
	}

	rs.runtime.Respond(w, r, http.StatusOK, nil, "Startseite gespeichert")
}

// resetHomeLayout restores the start page recommended for this person's role.
func (rs *SettingsResource) resetHomeLayout(w http.ResponseWriter, r *http.Request) {
	actor := rs.runtime.Actor(r.Context())
	if actor.AccountID <= 0 {
		rs.runtime.RenderError(w, r, http.StatusUnauthorized, errors.New("no account context"))
		return
	}

	if err := rs.operations.ResetHomeLayout(r.Context(), actor.TenantID, actor.AccountID); err != nil {
		rs.renderSettingsError(w, r, err)
		return
	}

	rs.runtime.RespondNoContent(w, r)
}

// setHomeBlockPolicies replaces what the school prescribes for everybody.
func (rs *SettingsResource) setHomeBlockPolicies(w http.ResponseWriter, r *http.Request) {
	actor := rs.runtime.Actor(r.Context())
	if actor.AccountID <= 0 {
		rs.runtime.RenderError(w, r, http.StatusUnauthorized, errors.New("no account context"))
		return
	}

	var req homeBlockPoliciesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		rs.runtime.RenderError(w, r, http.StatusBadRequest, err)
		return
	}

	err := rs.operations.SetHomeBlockPolicies(r.Context(), actor.TenantID, actor.AccountID, actor.Permissions, req.Policies)
	if err != nil {
		rs.renderSettingsError(w, r, err)
		return
	}

	rs.runtime.Respond(w, r, http.StatusOK, nil, "Vorgabe gespeichert")
}
