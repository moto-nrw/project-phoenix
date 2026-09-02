package staff

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/render"
)

// getPINStatus reports whether the calling account has a device PIN.
func (rs *Resource) getPINStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	accountID := rs.runtime.CurrentAccountID(ctx)
	if accountID == 0 {
		rs.failure(w, r, FailureUnauthorized, errors.New("invalid token"), "unauthorized")
		return
	}
	hasPIN, lastChanged, err := rs.runtime.PINStatus(ctx, accountID)
	if err != nil {
		rs.failure(w, r, FailureNotFound, errors.New("account not found"), "not_found")
		return
	}
	if !rs.isStaffAccount(ctx, accountID) {
		rs.failure(w, r, FailureForbidden, errors.New("only staff members can access PIN settings"), "forbidden")
		return
	}
	response := PINStatusResponse{HasPIN: hasPIN}
	if hasPIN {
		response.LastChanged = lastChanged
	}
	rs.respond(w, r, http.StatusOK, response, "PIN status retrieved successfully")
}

// updatePIN sets or replaces the calling account's device PIN.
func (rs *Resource) updatePIN(w http.ResponseWriter, r *http.Request) {
	req := &PINUpdateRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.failure(w, r, FailureInvalidRequest, err, "invalid_request")
		return
	}
	ctx := r.Context()
	accountID := rs.runtime.CurrentAccountID(ctx)
	if accountID == 0 {
		rs.failure(w, r, FailureUnauthorized, errors.New("invalid token"), "unauthorized")
		return
	}
	// The account-state checks run first, so a locked account keeps its own
	// message instead of the staff-only one.
	if err := rs.runtime.PINPreflight(ctx, accountID); err != nil {
		rs.runtime.PINFailure(w, r, err)
		return
	}
	if !rs.isStaffAccount(ctx, accountID) {
		rs.failure(w, r, FailureForbidden, errors.New("only staff members can manage PIN settings"), "forbidden")
		return
	}
	if err := rs.runtime.UpdatePIN(ctx, accountID, req.CurrentPIN, req.NewPIN); err != nil {
		rs.runtime.PINFailure(w, r, err)
		return
	}
	rs.respond(w, r, http.StatusOK, map[string]any{
		"success": true,
		"message": "PIN updated successfully",
	}, "PIN updated successfully")
}

// isStaffAccount reports whether the account may manage a device PIN. An
// account without a linked person is an administrator and passes; an account
// whose person carries no staff record does not.
func (rs *Resource) isStaffAccount(ctx context.Context, accountID int64) bool {
	personID, found, err := rs.runtime.PersonIDByAccount(ctx, accountID)
	if err != nil || !found {
		return true
	}
	if _, err := rs.membership.FindStaffByPerson(ctx, personID); err != nil {
		return false
	}
	return true
}
