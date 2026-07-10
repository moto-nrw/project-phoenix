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
