// Package webhooks holds inbound provider webhooks.
//
// These routes are mounted at the ROOT, not under /api, and they never call
// common.ProtectedTenantGroup. That is deliberate and load-bearing: the HMAC
// signature is the credential, and the tenant is derived from the outbox row
// the event points at — never from the request. Moving these routes under an
// authenticated group would break every provider integration silently, which
// is why TestWebhookIsNotBehindTenantJWT exists.
package webhooks

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/webhooksig"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// DeliveryIngester is the slice of the delivery service the handler needs.
// An interface rather than the concrete service so the handler's status-code
// contract can be tested without a database.
type DeliveryIngester interface {
	IngestBatch(ctx context.Context, events []platformSvc.DeliveryEvent) (platformSvc.BatchResult, error)
}

// Resource is the webhook API resource.
type Resource struct {
	DeliveryService DeliveryIngester
	Verifier        *webhooksig.Verifier
	Logger          *slog.Logger
	rateLimiter     func(http.Handler) http.Handler
}

// ResourceConfig is the DI bundle for NewResource.
type ResourceConfig struct {
	DeliveryService DeliveryIngester
	Verifier        *webhooksig.Verifier
	Logger          *slog.Logger
	RateLimiter     func(http.Handler) http.Handler
}

// NewResource builds the resource.
func NewResource(cfg ResourceConfig) *Resource {
	return &Resource{
		DeliveryService: cfg.DeliveryService,
		Verifier:        cfg.Verifier,
		Logger:          cfg.Logger,
		rateLimiter:     cfg.RateLimiter,
	}
}

func (rs *Resource) getLogger() *slog.Logger {
	if rs == nil || rs.Logger == nil {
		return slog.Default()
	}
	return rs.Logger
}

// Router wires the webhook routes. Note the absence of any auth middleware —
// see the package doc.
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	if rs.rateLimiter != nil {
		r.Use(rs.rateLimiter)
	}
	r.Post("/email/delivery/{provider}", rs.ingestDeliveryEvents)
	return r
}
