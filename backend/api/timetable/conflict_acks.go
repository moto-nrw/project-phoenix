// Package timetable — per-user conflict acknowledgements (#2139).
//
//	GET    /api/timetable/conflict-acks
//	PUT    /api/timetable/conflict-acks/{fingerprint}
//	DELETE /api/timetable/conflict-acks/{fingerprint}
//
// A user can hide ONE concrete, reviewed conflict from the Betreuungsplan
// banner. The acknowledgement is keyed by the conflict's fingerprint (stable
// identity: kind, person, involved instances, date, overlap window, relevant
// rooms), stored per tenant AND per account — it is user data state, not a
// tenant-wide setting, and never disables a whole conflict category. When the
// conflict changes, its fingerprint changes and the stored ack stops
// matching, so the warning resurfaces on its own.
//
// Permission: SchedulesRead — anyone who can see the plan (and thus the
// banner) may manage their own view state. Writes only ever touch rows of the
// calling account in the calling tenant.
package timetable

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// conflictAcksResponse is the 200 body for GET /conflict-acks.
type conflictAcksResponse struct {
	Fingerprints []string `json:"fingerprints"`
}

// listConflictAcks handles GET /api/timetable/conflict-acks.
func (rs *Resource) listConflictAcks(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.conflictAckAccount(w, r)
	if !ok {
		return
	}
	fingerprints, err := rs.TimetableData.ListConflictAcks(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("list conflict acks failed", err))
		return
	}
	common.Respond(w, r, http.StatusOK, conflictAcksResponse{Fingerprints: fingerprints},
		"Conflict acknowledgements retrieved")
}

// acknowledgeConflict handles PUT /api/timetable/conflict-acks/{fingerprint}.
// Idempotent: acknowledging an already-acknowledged fingerprint succeeds.
func (rs *Resource) acknowledgeConflict(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.conflictAckAccount(w, r)
	if !ok {
		return
	}
	fingerprint, ok := conflictAckFingerprintParam(w, r)
	if !ok {
		return
	}
	if err := rs.TimetableData.AcknowledgeConflict(r.Context(), accountID, fingerprint); err != nil {
		renderConflictAckError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{"fingerprint": fingerprint},
		"Conflict acknowledged")
}

// unacknowledgeConflict handles DELETE /api/timetable/conflict-acks/{fingerprint}.
// Idempotent: removing an unknown fingerprint succeeds — the goal state
// ("not acknowledged") already holds.
func (rs *Resource) unacknowledgeConflict(w http.ResponseWriter, r *http.Request) {
	accountID, ok := rs.conflictAckAccount(w, r)
	if !ok {
		return
	}
	fingerprint, ok := conflictAckFingerprintParam(w, r)
	if !ok {
		return
	}
	if err := rs.TimetableData.UnacknowledgeConflict(r.Context(), accountID, fingerprint); err != nil {
		renderConflictAckError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{"fingerprint": fingerprint},
		"Conflict acknowledgement removed")
}

// conflictAckAccount resolves the calling account from the JWT. A zero ID
// means the token carries no tenant account (should be unreachable behind the
// tenant middleware) — reject rather than storing rows nobody owns.
func (rs *Resource) conflictAckAccount(w http.ResponseWriter, r *http.Request) (int64, bool) {
	if rs.TimetableData == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable resource not fully wired")))
		return 0, false
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("no account in token")))
		return 0, false
	}
	return int64(claims.ID), true
}

// conflictAckFingerprintParam validates the {fingerprint} path segment before
// it reaches the service: lowercase hex, 16-64 chars (the same shape the
// detection emits and the DB CHECK enforces).
func conflictAckFingerprintParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	fingerprint := chi.URLParam(r, "fingerprint")
	if !scheduleModel.ValidConflictAckFingerprint(fingerprint) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid fingerprint")))
		return "", false
	}
	return fingerprint, true
}

// renderConflictAckError maps service errors: validation → 400, rest → 500.
func renderConflictAckError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, scheduleSvc.ErrInvalidConflictFingerprint) {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	common.RenderError(w, r, common.ErrorInternalServerWrap("conflict ack failed", err))
}
