// Package emailoutbox is the Delivery-owned e-mail outbox application: the
// tenant-aware enqueue facade feature producers use, the registry of
// renderers that turn a claimed e-mail intent into a message, and the reply
// address of the sending school. The root supplies every feature's renderers
// at startup; the worker doesn't know what's being sent.
package emailoutbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Intent is the claimed Delivery e-mail intent a renderer turns into a
// message: the intent's identity and school, its template kind, the decoded
// payload the producer queued and the delivery attempts made so far.
type Intent struct {
	ID       int64
	TenantID int64
	Kind     string
	Payload  map[string]any
	Attempts int
}

// GetTenantID reports the school the intent was queued for.
func (i *Intent) GetTenantID() int64 { return i.TenantID }

// Renderer turns a claimed intent into a concrete email message ready for
// dispatch. The worker calls Render with a tenant-scoped context, so
// renderers can read tenant-scoped settings or repos as needed.
//
// Returning an error from Render counts as a delivery failure. Delivery
// retries it with backoff and eventually moves it to dead_letter.
type Renderer interface {
	Render(ctx context.Context, intent *Intent) (*email.Message, error)
}

// ErrRenderCancelled is the one render outcome that is neither a success nor a
// failure: the intent must not be sent, and retrying cannot change that. A
// renderer returns it (wrapped, with a reason) when the fact that authorized
// the mail no longer holds at the moment of sending — a guardian whose access
// to the child an appointment concerns was revoked while the mail waited in
// the outbox. Delivery maps it to a token-fenced cancelled finalization
// instead of retrying something it must never send.
var ErrRenderCancelled = errors.New("render cancelled")

// RendererFunc adapts a plain function to the Renderer interface for
// brevity at registration sites.
type RendererFunc func(ctx context.Context, intent *Intent) (*email.Message, error)

// Render satisfies Renderer.
func (f RendererFunc) Render(ctx context.Context, intent *Intent) (*email.Message, error) {
	return f(ctx, intent)
}

// TemplateRegistry maps email kind → Renderer. The root composition supplies
// every renderer at construction; the registry never changes afterwards, so
// the worker reads it without locking.
type TemplateRegistry struct {
	renderers map[string]Renderer
}

// NewTemplateRegistry constructs a registry over the given kind → Renderer
// map. The map is copied, so later changes to it do not reach the registry.
func NewTemplateRegistry(renderers map[string]Renderer) *TemplateRegistry {
	copied := make(map[string]Renderer, len(renderers))
	for kind, renderer := range renderers {
		copied[kind] = renderer
	}
	return &TemplateRegistry{renderers: copied}
}

// Lookup returns the Renderer for `kind` or an error if none is
// registered. Worker treats "no renderer" as a permanent failure —
// the row goes straight to 'failed' instead of retrying.
func (r *TemplateRegistry) Lookup(kind string) (Renderer, error) {
	rdr, ok := r.renderers[kind]
	if !ok {
		return nil, fmt.Errorf("no renderer registered for kind %q", kind)
	}
	return rdr, nil
}

// Kinds returns the set of registered kinds. Useful for diagnostic
// endpoints + tests.
func (r *TemplateRegistry) Kinds() []string {
	out := make([]string, 0, len(r.renderers))
	for k := range r.renderers {
		out = append(out, k)
	}
	return out
}

// Service is the tenant-aware enqueue facade over Delivery. It keeps the
// feature producers on one enqueue contract while Delivery owns persistence
// and worker state.
type Service struct {
	delivery DurableEmailPort
}

// DurableEmail is one e-mail intent the facade hands to Delivery.
type DurableEmail struct {
	TenantID       int64
	Template       string
	Recipient      string
	Payload        json.RawMessage
	RelatedType    string
	RelatedID      int64
	IdempotencyKey string
}

// DurableEmailResult reports the stored intent.
type DurableEmailResult struct {
	ID        int64
	Duplicate bool
}

// DurableEmailPort is the Delivery capability the facade enqueues and
// cancels through; the root binds it to the Delivery module.
type DurableEmailPort interface {
	EnqueueEmail(context.Context, DurableEmail) (DurableEmailResult, error)
	CancelEmail(context.Context, int64, string, int64, string) (int64, error)
}

// NewService builds the enqueue facade used by feature producers.
func NewService(delivery DurableEmailPort) *Service {
	return &Service{delivery: delivery}
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

// Enqueued reports the intent an Enqueue call stored. ID is the stored
// intent's identity as Delivery reports it.
type Enqueued struct {
	ID int64
}

// Enqueue queues an e-mail intent in the current tenant transaction. Caller
// is responsible for being inside a tenant tx (otherwise the RLS policy will
// reject the INSERT).
func (s *Service) Enqueue(ctx context.Context, req EnqueueRequest) (*Enqueued, error) {
	if s == nil || s.delivery == nil {
		return nil, errors.New("outbox service not wired")
	}
	if req.Kind == "" {
		return nil, errors.New("outbox kind is required")
	}
	if req.Payload == nil {
		return nil, errors.New("outbox payload is required")
	}
	recipient, ok := req.Payload["recipient_email"].(string)
	if !ok || recipient == "" {
		return nil, errors.New("outbox recipient_email is required")
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("enqueue email outbox row: tenant is required: %w", err)
	}
	payload, err := json.Marshal(req.Payload)
	if err != nil {
		return nil, fmt.Errorf("enqueue email outbox row: encode payload: %w", err)
	}
	stored, err := s.delivery.EnqueueEmail(ctx, DurableEmail{
		TenantID: tenantID.Int64(), Template: req.Kind, Recipient: recipient, Payload: payload,
		RelatedType: req.RelatedEntityType, RelatedID: req.RelatedEntityID, IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		return nil, fmt.Errorf("enqueue email outbox row: %w", err)
	}
	return &Enqueued{ID: stored.ID}, nil
}

// CancelPendingByRelatedEntity cancels unsent outbox rows for a related entity.
// It runs in the caller's tenant transaction and retains rows for audit and
// status reads.
func (s *Service) CancelPendingByRelatedEntity(ctx context.Context, relatedType string, relatedID int64, reason string) (int64, error) {
	if s == nil || s.delivery == nil {
		return 0, errors.New("outbox service not wired")
	}
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, fmt.Errorf("cancel email outbox rows: tenant is required: %w", err)
	}
	return s.delivery.CancelEmail(ctx, tenantID.Int64(), relatedType, relatedID, reason)
}
