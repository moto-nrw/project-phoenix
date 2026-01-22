package internal

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/moto-nrw/project-phoenix/logging"
)

// ValidateEmailsRequest represents a request to validate multiple email addresses.
type ValidateEmailsRequest struct {
	Emails []string `json:"emails"`
}

// ValidateEmailsResponse contains the validation results.
type ValidateEmailsResponse struct {
	Available   []string `json:"available"`
	Unavailable []string `json:"unavailable"`
}

// validateEmails handles POST /api/internal/validate-emails
// This endpoint checks which emails are already registered in the Go backend.
// Used by BetterAuth to validate emails before creating invitations.
func (rs *Resource) validateEmails(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check if account repo is available
	if rs.accountRepo == nil {
		if logging.Logger != nil {
			logging.Logger.Error("Internal validate-emails API: account repository unavailable")
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "account repository unavailable",
		})
		return
	}

	// Parse request body
	var req ValidateEmailsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "invalid JSON body",
		})
		return
	}

	// Validate request
	if len(req.Emails) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "emails array is required and must not be empty",
		})
		return
	}

	// Limit the number of emails to check (prevent abuse)
	const maxEmails = 50
	if len(req.Emails) > maxEmails {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "too many emails, maximum is 50",
		})
		return
	}

	// Check each email
	available := make([]string, 0, len(req.Emails))
	unavailable := make([]string, 0)

	for _, email := range req.Emails {
		// Normalize email
		normalizedEmail := strings.ToLower(strings.TrimSpace(email))
		if normalizedEmail == "" {
			continue
		}

		// Check if email exists in auth.accounts
		_, err := rs.accountRepo.FindByEmail(r.Context(), normalizedEmail)
		if err != nil {
			// Email not found = available
			available = append(available, normalizedEmail)
		} else {
			// Email found = unavailable
			unavailable = append(unavailable, normalizedEmail)
		}
	}

	if logging.Logger != nil {
		logging.Logger.WithFields(map[string]interface{}{
			"total_checked": len(req.Emails),
			"available":     len(available),
			"unavailable":   len(unavailable),
		}).Info("Internal validate-emails API: email validation completed")
	}

	// Return response
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ValidateEmailsResponse{
		Available:   available,
		Unavailable: unavailable,
	})
}
