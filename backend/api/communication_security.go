package api

import (
	"context"
)

// CommunicationSession binds Communication's request session port to
// Identity's session token adapter: the authenticated account and its
// permissions.
type CommunicationSession struct{}

// AccountID is the authenticated account of the request, 0 without one.
func (CommunicationSession) AccountID(ctx context.Context) int64 {
	claims, _ := requestTokenReads.Claims(ctx)
	return claims.AccountID
}

// Permissions are the permissions of the authenticated request.
func (CommunicationSession) Permissions(ctx context.Context) []string {
	return requestTokenReads.Permissions(ctx)
}
