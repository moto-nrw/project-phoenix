package account

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// Token Management Endpoints

// cleanupExpiredTokens handles cleanup of expired tokens
func (rs *Resource) cleanupExpiredTokens(w http.ResponseWriter, r *http.Request) {
	if rs.Sessions == nil {
		common.RenderError(w, r, common.ErrorServiceUnavailable(errors.New("token cleanup unavailable")))
		return
	}
	count, err := rs.Sessions.CleanupExpiredTokens(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	response := map[string]int{"cleaned_tokens": count}
	common.Respond(w, r, http.StatusOK, response, "Expired tokens cleaned up successfully")
}

// revokeAllTokens revokes every session of the account (administrative
// revoke) through Identity & Access.
func (rs *Resource) revokeAllTokens(ctx context.Context, accountID int) error {
	if rs.Sessions == nil {
		return errors.New("token revocation unavailable")
	}
	return rs.Sessions.RevokeAllTokensWithReason(ctx, int64(accountID), "administrative_revoke")
}

// getActiveTokens handles getting active tokens for an account
func (rs *Resource) getActiveTokens(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}
	if rs.Sessions == nil {
		common.RenderError(w, r, common.ErrorServiceUnavailable(errors.New("token listing unavailable")))
		return
	}

	tokens, err := rs.Sessions.ListActiveSessions(r.Context(), int64(accountID))
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	type TokenResponse struct {
		ID         int64  `json:"id"`
		Expiry     string `json:"expiry"`
		Mobile     bool   `json:"mobile"`
		Identifier string `json:"identifier,omitempty"`
		CreatedAt  string `json:"created_at"`
	}

	responses := make([]*TokenResponse, 0, len(tokens))
	for _, token := range tokens {
		resp := &TokenResponse{
			ID:        token.ID,
			Expiry:    token.Expiry.Format(time.RFC3339),
			Mobile:    token.Mobile,
			CreatedAt: token.CreatedAt.Format(time.RFC3339),
		}

		if token.Identifier != nil {
			resp.Identifier = *token.Identifier
		}

		responses = append(responses, resp)
	}

	common.Respond(w, r, http.StatusOK, responses, "Active tokens retrieved successfully")
}

// Parent Account Management Endpoints
