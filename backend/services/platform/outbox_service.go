// Package platform contains the platform-level (cross-tenant or
// operator-only) services. The outbox service in this file is shared
// across features: enrollment, guardian invitations, future audit
// notifications, etc. Each feature registers a Renderer for its own
// kinds at startup; the worker doesn't know what's being sent.
package platform

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/moto-nrw/project-phoenix/email"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
)

// Renderer turns an outbox row into a concrete email message ready for
// dispatch. The worker calls Render with a tenant-scoped context, so
// renderers can read tenant-scoped settings or repos as needed.
//
// Returning an error from Render counts as a delivery failure — the
// worker treats it as a retryable error subject to the same backoff
// schedule as a transient SMTP failure. If a renderer is permanently
// broken (e.g., template missing), it should still return an error;
// the worker's MaxAttempts limit will eventually move the row to
// 'failed'.
type Renderer interface {
	Render(ctx context.Context, row *platformModels.EmailOutbox) (*email.Message, error)
}

// ErrRenderCancelled is the one render outcome that is neither a success nor a
// failure: the row must not be sent, and retrying cannot change that. A renderer
// returns it (wrapped, with a reason) when the fact that authorized the mail no
// longer holds at the moment of sending — a guardian whose access to the child
// an appointment concerns was revoked while the mail waited in the outbox. The
// worker retires such a row immediately instead of burning the retry budget on
// something it must never deliver.
var ErrRenderCancelled = errors.New("render cancelled")

// RendererFunc adapts a plain function to the Renderer interface for
// brevity at registration sites.
type RendererFunc func(ctx context.Context, row *platformModels.EmailOutbox) (*email.Message, error)

// Render satisfies Renderer.
func (f RendererFunc) Render(ctx context.Context, row *platformModels.EmailOutbox) (*email.Message, error) {
	return f(ctx, row)
}

// TemplateRegistry maps email kind → Renderer. Populated at startup by
// each feature's init() or factory wiring. Lookups happen on every
// worker tick, so the map is read-mostly; we use a RWMutex.
//
// The registry is intentionally process-global: there is exactly one
// outbox worker per process, and renderers are statically known at
// build time. A struct-scoped registry would just push the wiring into
// the factory without buying anything.
type TemplateRegistry struct {
	mu        sync.RWMutex
	renderers map[string]Renderer
}

// NewTemplateRegistry constructs an empty registry.
func NewTemplateRegistry() *TemplateRegistry {
	return &TemplateRegistry{renderers: make(map[string]Renderer)}
}

// Register associates a kind string with a Renderer. Calling Register
// twice for the same kind is allowed and overwrites — useful in tests.
func (r *TemplateRegistry) Register(kind string, renderer Renderer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.renderers[kind] = renderer
}

// Lookup returns the Renderer for `kind` or an error if none is
// registered. Worker treats "no renderer" as a permanent failure —
// the row goes straight to 'failed' instead of retrying.
func (r *TemplateRegistry) Lookup(kind string) (Renderer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rdr, ok := r.renderers[kind]
	if !ok {
		return nil, fmt.Errorf("no renderer registered for kind %q", kind)
	}
	return rdr, nil
}

// Kinds returns the set of registered kinds. Useful for diagnostic
// endpoints + tests.
func (r *TemplateRegistry) Kinds() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.renderers))
	for k := range r.renderers {
		out = append(out, k)
	}
	return out
}

// OutboxService wraps the EmailOutboxRepository with a tenant-aware
// Enqueue helper. The worker pickup path doesn't need this service —
// it talks to the repository directly through phoenix_admin to bypass
// RLS. The service exists so feature code (e.g., enrollment submit
// handler, decision service, guardian invitation service) has a single
// call site for "enqueue this email".
type OutboxService struct {
	repo platformModels.EmailOutboxRepository
}

// NewOutboxService builds the service.
func NewOutboxService(repo platformModels.EmailOutboxRepository) *OutboxService {
	return &OutboxService{repo: repo}
}

// EnqueueRequest is the payload Enqueue accepts. Builder fields stay
// flat — Enqueue is called inline from feature code, no need for a
// fluent builder.
type EnqueueRequest struct {
	Kind              string
	Payload           map[string]any
	RelatedEntityType string // optional, e.g., "enrollment_request"
	RelatedEntityID   int64  // optional, paired with RelatedEntityType
	IdempotencyKey    string // optional; duplicate tenant/key enqueues are ignored
}

// Enqueue creates a pending outbox row in the current tenant
// transaction. Caller is responsible for being inside a tenant tx
// (otherwise the RLS policy will reject the INSERT).
// EnqueueOutbox implements platformModels.OutboxEnqueuer for the feature
// services (guardian invitations, enrollment emails), discarding the
// created row.
func (s *OutboxService) EnqueueOutbox(ctx context.Context, req platformModels.OutboxEnqueueRequest) error {
	_, err := s.Enqueue(ctx, EnqueueRequest{
		Kind:              req.Kind,
		Payload:           req.Payload,
		RelatedEntityType: req.RelatedEntityType,
		RelatedEntityID:   req.RelatedEntityID,
		IdempotencyKey:    req.IdempotencyKey,
	})
	return err
}

func (s *OutboxService) Enqueue(ctx context.Context, req EnqueueRequest) (*platformModels.EmailOutbox, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("outbox service not wired")
	}
	if req.Kind == "" {
		return nil, errors.New("outbox kind is required")
	}
	if req.Payload == nil {
		req.Payload = map[string]any{}
	}

	row := &platformModels.EmailOutbox{
		Kind:    req.Kind,
		Payload: req.Payload,
		Status:  platformModels.EmailOutboxStatusPending,
	}
	if req.IdempotencyKey != "" {
		key := req.IdempotencyKey
		row.IdempotencyKey = &key
	}
	if req.RelatedEntityType != "" {
		t := req.RelatedEntityType
		row.RelatedEntityType = &t
	}
	if req.RelatedEntityID > 0 {
		id := req.RelatedEntityID
		row.RelatedEntityID = &id
	}
	if err := s.repo.Create(ctx, row); err != nil {
		return nil, fmt.Errorf("enqueue email outbox row: %w", err)
	}
	return row, nil
}

// CancelPendingByRelatedEntity marks every still-pending outbox row for a
// related entity as failed so the worker never sends it — used when the
// triggering entity is retracted before the async send. Tenant-scoped; must run
// inside the caller's tenant tx. Returns the number of rows cancelled.
func (s *OutboxService) CancelPendingByRelatedEntity(ctx context.Context, relatedType string, relatedID int64, reason string) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, errors.New("outbox service not wired")
	}
	return s.repo.CancelPendingByRelatedEntity(ctx, relatedType, relatedID, reason)
}

// FindByRelatedEntity surfaces the per-feature lookup so admin UIs can
// show "all email rows for this enrollment request". Tenant-scoped.
func (s *OutboxService) FindByRelatedEntity(ctx context.Context, relatedType string, relatedID int64) ([]*platformModels.EmailOutbox, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("outbox service not wired")
	}
	return s.repo.FindByRelatedEntity(ctx, relatedType, relatedID)
}
