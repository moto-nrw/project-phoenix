package platform

import (
	"context"
	"net"

	"github.com/moto-nrw/project-phoenix/models/platform"
)

// OperatorInvitationService exposes the invitation-management operations
// needed by the operator invitations HTTP handlers. It mirrors the
// services/auth/invitation_interface.go pattern: a narrow, feature-scoped
// interface distinct from OperatorAuthService (which handles login,
// sessions, profile, password, and email-change).
//
// The concrete operatorAuthService struct satisfies both this interface
// and OperatorAuthService, so no runtime rewiring is needed — only the
// handler dependencies narrow.
//
// ListOperators is included because the list endpoint returns a combined
// view of pending invitations and existing operators for the operator
// management UI. Keeping it on this narrow interface lets the handler
// depend on a single service instead of two.
type OperatorInvitationService interface {
	InviteOperator(ctx context.Context, email string, displayName *string, createdByID int64, clientIP net.IP) error
	ValidateOperatorInvitation(ctx context.Context, token string) (*platform.OperatorInvitationToken, error)
	AcceptOperatorInvitation(ctx context.Context, token, displayName, password string, clientIP net.IP) (*platform.Operator, error)
	ListPendingOperatorInvitations(ctx context.Context) ([]*platform.OperatorInvitationToken, error)
	RevokeOperatorInvitation(ctx context.Context, invitationID int64, actorID int64, clientIP net.IP) error
	ResendOperatorInvitation(ctx context.Context, invitationID int64, actorID int64, clientIP net.IP) error
	CleanupExpiredOperatorInvitations(ctx context.Context) (int, error)
	ListOperators(ctx context.Context) ([]*platform.Operator, error)
}

// OperatorAuthAndInvitationService is a convenience interface for wiring
// sites (the factory, test helpers) that hold the concrete operator auth
// service and need to expose it through both narrow interfaces. Handler
// code should depend on the narrower OperatorAuthService or
// OperatorInvitationService, never on this combined interface.
type OperatorAuthAndInvitationService interface {
	OperatorAuthService
	OperatorInvitationService
}
