package platform

import "context"

// OutboxEnqueueRequest is the transport-neutral enqueue input shared by the
// feature services (auth, enrollment) and the platform outbox service. It
// mirrors services/platform.EnqueueRequest field for field.
type OutboxEnqueueRequest struct {
	Kind              string
	Payload           map[string]any
	RelatedEntityType string
	RelatedEntityID   int64
	IdempotencyKey    string
}

// OutboxEnqueuer is the narrow contract feature services need from the
// platform email outbox. Declared here (models/platform is a leaf package)
// so services/auth and services/enrollment can depend on it without
// importing services/platform.
type OutboxEnqueuer interface {
	EnqueueOutbox(ctx context.Context, req OutboxEnqueueRequest) error
}

// OutboxRelatedReader is the read side of the same arrangement: it lets a
// feature service answer "what happened to the mails for this entity" without
// importing services/platform. Tenant-scoped.
type OutboxRelatedReader interface {
	FindByRelatedEntity(ctx context.Context, relatedType string, relatedID int64) ([]*EmailOutbox, error)
	FindByRelatedEntities(ctx context.Context, relatedType string, relatedIDs []int64) ([]*EmailOutbox, error)
}

// OutboxCanceller lets a feature service stop still-queued mails for an entity
// it just invalidated — used when a guardian's email address changes and the
// invitation to the old address must never leave the building.
type OutboxCanceller interface {
	CancelPendingByRelatedEntity(ctx context.Context, relatedType string, relatedID int64, reason string) (int64, error)
}
