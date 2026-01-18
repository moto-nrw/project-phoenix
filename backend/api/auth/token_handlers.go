package auth

import (
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// cleanupExpiredTokens handles cleanup of expired tokens
func (rs *Resource) cleanupExpiredTokens(w http.ResponseWriter, r *http.Request) {
	count, err := rs.AuthService.CleanupExpiredTokens(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	response := map[string]int{"cleaned_tokens": count}
	common.Respond(w, r, http.StatusOK, response, "Expired tokens cleaned up successfully")
}

// revokeAllTokens handles revoking all tokens for an account
func (rs *Resource) revokeAllTokens(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	if err := rs.AuthService.RevokeAllTokens(r.Context(), accountID); err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	common.RespondNoContent(w, r)
}

// getActiveTokens handles getting active tokens for an account
func (rs *Resource) getActiveTokens(w http.ResponseWriter, r *http.Request) {
	accountID, ok := common.ParseIntIDWithError(w, r, "accountId", common.MsgInvalidAccountID)
	if !ok {
		return
	}

	tokens, err := rs.AuthService.GetActiveTokens(r.Context(), accountID)
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	type ActiveTokenResponse struct {
		ID         int64  `json:"id"`
		Token      string `json:"token"`
		Expiry     string `json:"expiry"`
		Mobile     bool   `json:"mobile"`
		Identifier string `json:"identifier,omitempty"`
		CreatedAt  string `json:"created_at"`
	}

	responses := make([]*ActiveTokenResponse, 0, len(tokens))
	for _, token := range tokens {
		resp := &ActiveTokenResponse{
			ID:        token.ID,
			Token:     token.Token,
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
