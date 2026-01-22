package internal

import (
	"encoding/json"
	"net/http"

	"github.com/moto-nrw/project-phoenix/logging"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// SyncUserRequest represents a request to sync a BetterAuth user to Go backend.
// This creates Person, Staff, and optionally Teacher records.
type SyncUserRequest struct {
	// BetterAuthUserID is the UUID from BetterAuth's user table
	BetterAuthUserID string `json:"betterauth_user_id"`
	// Email is the user's email address
	Email string `json:"email"`
	// Name is the user's display name (will be split into first/last)
	Name string `json:"name"`
	// OrganizationID is the BetterAuth organization UUID
	OrganizationID string `json:"organization_id"`
	// Role is the member's role in the organization (admin, member, owner)
	Role string `json:"role"`
}

// SyncUserResponse represents the response from syncing a user.
type SyncUserResponse struct {
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	PersonID  int64  `json:"person_id,omitempty"`
	StaffID   int64  `json:"staff_id,omitempty"`
	TeacherID int64  `json:"teacher_id,omitempty"`
}

// syncUser handles POST /api/internal/sync-user
// Creates Person, Staff, and Teacher records for a BetterAuth user.
// This is called by BetterAuth's afterAcceptInvitation hook.
func (rs *Resource) syncUser(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Check if service is available
	if rs.userSyncService == nil {
		if logging.Logger != nil {
			logging.Logger.Error("Internal sync-user API: user sync service unavailable")
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(SyncUserResponse{
			Status:  "error",
			Message: "user sync service unavailable",
		})
		return
	}

	// Parse request body
	var req SyncUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(SyncUserResponse{
			Status:  "error",
			Message: "invalid JSON body",
		})
		return
	}

	// Call the service to sync the user
	result, err := rs.userSyncService.SyncUser(r.Context(), authService.UserSyncParams{
		BetterAuthUserID: req.BetterAuthUserID,
		Email:            req.Email,
		Name:             req.Name,
		OrganizationID:   req.OrganizationID,
		Role:             req.Role,
	})

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(SyncUserResponse{
			Status:  "error",
			Message: err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(SyncUserResponse{
		Status:    "success",
		Message:   "user synced successfully",
		PersonID:  result.PersonID,
		StaffID:   result.StaffID,
		TeacherID: result.TeacherID,
	})
}
