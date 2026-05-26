package platform

import (
	"context"

	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// enrollmentOutboxAdapter bridges services/enrollment's OutboxEnqueuer
// to the platform OutboxService. Same shape as the auth adapter — each
// feature package declares its own OutboxEnqueuer interface so it
// doesn't import services/platform; this adapter does the type
// translation at factory wiring time.
type enrollmentOutboxAdapter struct {
	svc *OutboxService
}

// NewEnrollmentOutboxAdapter wraps an OutboxService to satisfy
// services/enrollment.OutboxEnqueuer.
func NewEnrollmentOutboxAdapter(svc *OutboxService) enrollmentService.OutboxEnqueuer {
	return &enrollmentOutboxAdapter{svc: svc}
}

func (a *enrollmentOutboxAdapter) Enqueue(ctx context.Context, req enrollmentService.OutboxEnqueueRequest) error {
	_, err := a.svc.Enqueue(ctx, EnqueueRequest{
		Kind:              req.Kind,
		Payload:           req.Payload,
		RelatedEntityType: req.RelatedEntityType,
		RelatedEntityID:   req.RelatedEntityID,
	})
	return err
}
