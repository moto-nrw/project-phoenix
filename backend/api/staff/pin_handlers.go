package staff

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authmodel "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// getPINStatus handles getting the current user's PIN status
func (rs *Resource) getPINStatus(w http.ResponseWriter, r *http.Request) {
	// Get user from JWT context
	userClaims := jwt.ClaimsFromCtx(r.Context())
	if userClaims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("invalid token")))
		return
	}

	// Get account directly
	account, err := rs.AuthService.GetAccountByID(r.Context(), userClaims.ID)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("account not found")))
		return
	}

	// Ensure the account belongs to a staff member (admins without person records are allowed)
	person, err := rs.PersonService.FindByAccountID(r.Context(), int64(account.ID))
	if err == nil && person != nil {
		if _, err := rs.PersonService.GetStaffByPersonID(r.Context(), person.ID); err != nil {
			common.RenderError(w, r, common.ErrorForbidden(errors.New("only staff members can access PIN settings")))
			return
		}
	}

	// Build response using account PIN data
	response := PINStatusResponse{
		HasPIN: account.HasPIN(),
	}

	// Include last changed timestamp if available (use UpdatedAt as proxy)
	if account.HasPIN() {
		response.LastChanged = &account.UpdatedAt
	}

	common.Respond(w, r, http.StatusOK, response, "PIN status retrieved successfully")
}

// updatePIN handles updating the current user's PIN
func (rs *Resource) updatePIN(w http.ResponseWriter, r *http.Request) {
	req := &PINUpdateRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	userClaims := jwt.ClaimsFromCtx(r.Context())
	if userClaims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("invalid token")))
		return
	}

	account, err := rs.AuthService.GetAccountByID(r.Context(), userClaims.ID)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(errors.New("account not found")))
		return
	}

	// Validate access
	if renderErr := rs.checkAccountLocked(r.Context(), account); renderErr != nil {
		common.RenderError(w, r, renderErr)
		return
	}
	if renderErr := rs.checkStaffPINAccess(r.Context(), int64(account.ID)); renderErr != nil {
		common.RenderError(w, r, renderErr)
		return
	}

	// Verify current PIN if exists
	result, renderErr := verifyCurrentPIN(account, req.CurrentPIN)
	if renderErr != nil {
		// Only increment attempts for actual verification failures, not missing input.
		// Routed through the service so the atomic increment closes the
		// read-modify-write lockout race (issue #586).
		if result == pinVerificationFailed {
			if updateErr := rs.AuthService.RecordFailedPINAttempt(r.Context(), int64(account.ID)); updateErr != nil {
				rs.getLogger().Error("failed to update account PIN attempts",
					slog.String("error", updateErr.Error()))
			}
		}
		common.RenderError(w, r, renderErr)
		return
	}

	// Set new PIN
	if account.HashPIN(req.NewPIN) != nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("failed to hash PIN")))
		return
	}

	tenantID := tenant.FromContext(r.Context())
	if err := tenant.WithTenantTx(r.Context(), rs.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if updateErr := rs.AuthService.UpdateAccount(ctx, account); updateErr != nil {
			return updateErr
		}
		// Clear any prior lockout via the atomic reset rather than mutating
		// the in-memory account and persisting it again.
		return rs.AuthService.ResetPINLockout(ctx, int64(account.ID))
	}); err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "PIN updated successfully",
	}, "PIN updated successfully")
}

// checkAccountLocked checks if account is PIN locked. The lockout decision is
// resolved by the auth service (clock injected) rather than on the model
// (issue #586, Rule 12).
func (rs *Resource) checkAccountLocked(_ context.Context, account *authmodel.Account) render.Renderer {
	if rs.AuthService.IsPINLocked(account, time.Now()) {
		return common.ErrorForbidden(errors.New("account is temporarily locked due to failed PIN attempts"))
	}
	return nil
}

// checkStaffPINAccess verifies the account belongs to a staff member
func (rs *Resource) checkStaffPINAccess(ctx context.Context, accountID int64) render.Renderer {
	person, err := rs.PersonService.FindByAccountID(ctx, accountID)
	if err != nil || person == nil {
		return nil // No person = likely admin, allow
	}

	if _, err := rs.PersonService.GetStaffByPersonID(ctx, person.ID); err != nil {
		return common.ErrorForbidden(errors.New("only staff members can manage PIN settings"))
	}
	return nil
}

// verifyCurrentPIN validates current PIN when updating existing PIN
// pinVerificationResult indicates the outcome of PIN verification
type pinVerificationResult int

const (
	pinVerificationNotRequired  pinVerificationResult = iota // No PIN exists, verification skipped
	pinVerificationMissingInput                              // PIN required but input was missing (validation error)
	pinVerificationFailed                                    // PIN provided but incorrect (auth failure)
	pinVerificationPassed                                    // PIN verified successfully
)

// verifyCurrentPIN checks the current PIN and returns both the result type and any error
func verifyCurrentPIN(account interface {
	HasPIN() bool
	VerifyPIN(string) bool
}, currentPIN *string) (pinVerificationResult, render.Renderer) {
	if !account.HasPIN() {
		return pinVerificationNotRequired, nil
	}

	if currentPIN == nil || *currentPIN == "" {
		return pinVerificationMissingInput, common.ErrorInvalidRequest(errors.New("current PIN is required when updating existing PIN"))
	}

	if !account.VerifyPIN(*currentPIN) {
		return pinVerificationFailed, common.ErrorUnauthorized(errors.New("current PIN is incorrect"))
	}
	return pinVerificationPassed, nil
}
