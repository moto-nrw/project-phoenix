package internal

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/moto-nrw/project-phoenix/logging"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// CreateInvitationRequest represents a request to create an invitation via internal API.
// This is used by the SaaS admin console to create invitations without requiring
// a Go backend authenticated user.
type CreateInvitationRequest struct {
	Email     string  `json:"email"`
	RoleID    int64   `json:"role_id"`
	FirstName *string `json:"first_name,omitempty"`
	LastName  *string `json:"last_name,omitempty"`
	Position  *string `json:"position,omitempty"`
}

// CreateInvitationResponse represents the response from creating an invitation.
type CreateInvitationResponse struct {
	Status string      `json:"status"`
	Data   interface{} `json:"data,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// InvitationData contains the invitation details returned in the response.
type InvitationData struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	RoleID    int64     `json:"role_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	FirstName *string   `json:"first_name,omitempty"`
	LastName  *string   `json:"last_name,omitempty"`
	Position  *string   `json:"position,omitempty"`
}

// createInvitation handles POST /api/internal/invitations
// This endpoint creates invitations for the SaaS admin console.
// It uses the system admin account as the "created_by" value.
func (rs *Resource) createInvitation(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check if invitation service is available
	if rs.invitationService == nil {
		if logging.Logger != nil {
			logging.Logger.Error("Internal invitation API: invitation service unavailable")
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(CreateInvitationResponse{
			Status: "error",
			Error:  "invitation service unavailable",
		})
		return
	}

	// Check if account repo is available
	if rs.accountRepo == nil {
		if logging.Logger != nil {
			logging.Logger.Error("Internal invitation API: account repository unavailable")
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(CreateInvitationResponse{
			Status: "error",
			Error:  "account repository unavailable",
		})
		return
	}

	// Parse request body
	var req CreateInvitationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(CreateInvitationResponse{
			Status: "error",
			Error:  "invalid JSON body",
		})
		return
	}

	// Validate required fields
	if req.Email == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(CreateInvitationResponse{
			Status: "error",
			Error:  "email is required",
		})
		return
	}
	if req.RoleID <= 0 {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(CreateInvitationResponse{
			Status: "error",
			Error:  "role_id is required and must be positive",
		})
		return
	}

	// Find the system admin account to use as "created_by"
	// The admin account is created during migration 1.6.2
	adminAccount, err := rs.accountRepo.FindByUsername(r.Context(), "admin")
	if err != nil {
		if logging.Logger != nil {
			logging.Logger.WithError(err).Error("Internal invitation API: failed to find admin account")
		}
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(CreateInvitationResponse{
			Status: "error",
			Error:  "system admin account not found - please ensure migrations have run",
		})
		return
	}

	// Build the invitation request
	invitationReq := authService.InvitationRequest{
		Email:     req.Email,
		RoleID:    req.RoleID,
		CreatedBy: adminAccount.ID,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Position:  req.Position,
	}

	// Create the invitation
	invitation, err := rs.invitationService.CreateInvitation(r.Context(), invitationReq)
	if err != nil {
		if logging.Logger != nil {
			logging.Logger.WithFields(map[string]interface{}{
				"email":   req.Email,
				"role_id": req.RoleID,
			}).WithError(err).Error("Internal invitation API: failed to create invitation")
		}

		// Check for specific errors
		errMsg := "failed to create invitation"
		statusCode := http.StatusInternalServerError

		// Check if it's a known error (unwrap AuthError if present)
		var authErr *authService.AuthError
		if errors.As(err, &authErr) {
			if errors.Is(authErr.Err, authService.ErrEmailAlreadyExists) {
				errMsg = "email address is already registered"
				statusCode = http.StatusConflict
			}
		} else if errors.Is(err, authService.ErrEmailAlreadyExists) {
			errMsg = "email address is already registered"
			statusCode = http.StatusConflict
		}

		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(CreateInvitationResponse{
			Status: "error",
			Error:  errMsg,
		})
		return
	}

	// Log success
	if logging.Logger != nil {
		logging.Logger.WithFields(map[string]interface{}{
			"invitation_id": invitation.ID,
			"email":         invitation.Email,
			"role_id":       invitation.RoleID,
			"created_by":    adminAccount.ID,
		}).Info("Internal invitation API: invitation created successfully")
	}

	// Return success response
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(CreateInvitationResponse{
		Status: "success",
		Data: InvitationData{
			ID:        invitation.ID,
			Email:     invitation.Email,
			RoleID:    invitation.RoleID,
			Token:     invitation.Token,
			ExpiresAt: invitation.ExpiresAt,
			FirstName: invitation.FirstName,
			LastName:  invitation.LastName,
			Position:  invitation.Position,
		},
	})
}
