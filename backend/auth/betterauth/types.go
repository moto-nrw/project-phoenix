package betterauth

import (
	"errors"
	"time"
)

// Errors returned by the BetterAuth client.
var (
	// ErrNoSession indicates no valid session was found (user not authenticated).
	ErrNoSession = errors.New("no valid session")

	// ErrNoActiveOrg indicates the session exists but has no active organization selected.
	ErrNoActiveOrg = errors.New("no active organization in session")

	// ErrMemberNotFound indicates the user is not a member of the active organization.
	ErrMemberNotFound = errors.New("member not found in organization")
)

// SessionResponse represents BetterAuth's /api/auth/get-session response.
// This is the primary response used to validate authentication and get user context.
type SessionResponse struct {
	User    UserInfo    `json:"user"`
	Session SessionInfo `json:"session"`
}

// UserInfo contains user identity information from BetterAuth.
type UserInfo struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// SessionInfo contains session metadata from BetterAuth.
// The ActiveOrganizationID is critical for multi-tenancy - it identifies which
// OGS (after-school center) the user is currently working in.
type SessionInfo struct {
	ID                   string    `json:"id"`
	ActiveOrganizationID string    `json:"activeOrganizationId"`
	ExpiresAt            time.Time `json:"expiresAt"`
}

// MemberInfo represents a user's membership in an organization.
// Retrieved from GET /api/auth/organization/get-active-member.
//
// CRITICAL: BetterAuth returns the role NAME (e.g., "supervisor"), NOT the
// permissions array. Permissions must be resolved using the RolePermissions
// map in auth/tenant/roles.go.
type MemberInfo struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organizationId"`
	UserID         string `json:"userId"`
	Role           string `json:"role"`
	CreatedAt      string `json:"createdAt"`
}
