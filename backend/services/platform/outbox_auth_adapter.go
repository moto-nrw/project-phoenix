package platform

import (
	"context"

	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// authOutboxAdapter bridges services/auth's OutboxEnqueuer interface to
// the platform OutboxService. The auth package can't import services/
// platform without a dep cycle (platform imports auth indirectly via
// other shared types), so each package declares its own interface and
// this adapter maps between them.
type authOutboxAdapter struct {
	svc *OutboxService
}

// NewAuthOutboxAdapter wraps an OutboxService so it satisfies
// services/auth.OutboxEnqueuer. Pass the result to
// auth.GuardianInvitationServiceConfig.OutboxEnqueuer at factory wiring.
func NewAuthOutboxAdapter(svc *OutboxService) authService.OutboxEnqueuer {
	return &authOutboxAdapter{svc: svc}
}

// Enqueue translates the auth.OutboxEnqueueRequest to the platform-
// native EnqueueRequest and forwards. Returns the underlying error
// (caller decides how to surface).
func (a *authOutboxAdapter) Enqueue(ctx context.Context, req authService.OutboxEnqueueRequest) error {
	_, err := a.svc.Enqueue(ctx, EnqueueRequest{
		Kind:              req.Kind,
		Payload:           req.Payload,
		RelatedEntityType: req.RelatedEntityType,
		RelatedEntityID:   req.RelatedEntityID,
	})
	return err
}
