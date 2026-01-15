package staff

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/jwt"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
)

// PINStatusResponse represents the PIN status response
type PINStatusResponse struct {
	HasPIN      bool       `json:"has_pin"`
	LastChanged *time.Time `json:"last_changed,omitempty"`
}

// PINUpdateRequest represents a PIN update request
type PINUpdateRequest struct {
	CurrentPIN *string `json:"current_pin,omitempty"` // null for first-time setup
	NewPIN     string  `json:"new_pin"`
}

// Bind validates the PIN update request
func (req *PINUpdateRequest) Bind(_ *http.Request) error {
	if req.NewPIN == "" {
		return errors.New("new PIN is required")
	}

	// Validate PIN format (4 digits)
	if len(req.NewPIN) != 4 {
		return errors.New("PIN must be exactly 4 digits")
	}

	// Check if PIN contains only digits
	for _, char := range req.NewPIN {
		if char < '0' || char > '9' {
			return errors.New("PIN must contain only digits")
		}
	}

	return nil
}

// getPINStatus handles getting the current user's PIN status
func (rs *Resource) getPINStatus(w http.ResponseWriter, r *http.Request) {
	// Get user from JWT context
	userClaims := jwt.ClaimsFromCtx(r.Context())
	if userClaims.ID == 0 {
		common.RenderError(w, r, ErrorUnauthorized(errors.New("invalid token")))
		return
	}

	// Get account directly
	account, err := rs.AuthService.GetAccountByID(r.Context(), userClaims.ID)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New("account not found")))
		return
	}

	// Ensure the account belongs to a staff member (admins without person records are allowed)
	person, err := rs.PersonService.FindByAccountID(r.Context(), int64(account.ID))
	if err == nil && person != nil {
		if _, err := rs.PersonService.GetStaffByPersonID(r.Context(), person.ID); err != nil {
			common.RenderError(w, r, ErrorForbidden(errors.New("only staff members can access PIN settings")))
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
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	userClaims := jwt.ClaimsFromCtx(r.Context())
	if userClaims.ID == 0 {
		common.RenderError(w, r, ErrorUnauthorized(errors.New("invalid token")))
		return
	}

	account, err := rs.AuthService.GetAccountByID(r.Context(), userClaims.ID)
	if err != nil {
		common.RenderError(w, r, ErrorNotFound(errors.New("account not found")))
		return
	}

	// Validate access
	if renderErr := rs.checkAccountLocked(account); renderErr != nil {
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
		// Only increment attempts for actual verification failures, not missing input
		if result == pinVerificationFailed {
			account.IncrementPINAttempts()
			if updateErr := rs.AuthService.UpdateAccount(r.Context(), account); updateErr != nil {
				if logger.Logger != nil {
					logger.Logger.WithError(updateErr).Warn("failed to update account PIN attempts")
				}
			}
		}
		common.RenderError(w, r, renderErr)
		return
	}

	// Set new PIN
	if account.HashPIN(req.NewPIN) != nil {
		common.RenderError(w, r, ErrorInternalServer(errors.New("failed to hash PIN")))
		return
	}
	account.ResetPINAttempts()

	if err := rs.AuthService.UpdateAccount(r.Context(), account); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "PIN updated successfully",
	}, "PIN updated successfully")
}

// checkAccountLocked checks if account is PIN locked
func (rs *Resource) checkAccountLocked(account interface{ IsPINLocked() bool }) render.Renderer {
	if account.IsPINLocked() {
		return ErrorForbidden(errors.New("account is temporarily locked due to failed PIN attempts"))
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
		return ErrorForbidden(errors.New("only staff members can manage PIN settings"))
	}
	return nil
}

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
		return pinVerificationMissingInput, ErrorInvalidRequest(errors.New("current PIN is required when updating existing PIN"))
	}

	if !account.VerifyPIN(*currentPIN) {
		return pinVerificationFailed, ErrorUnauthorized(errors.New("current PIN is incorrect"))
	}
	return pinVerificationPassed, nil
}
