// Package webhooks takes in what mail providers report about their mails.
//
// These routes hang at the root, outside every session and tenant group, and
// that is load-bearing: the signature over the raw body is the credential, and
// the school comes from the mail an event matches, never from the request.
package webhooks

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// maxWebhookBody bounds what is read before the signature is known.
const maxWebhookBody = 1 << 20

// ErrBodyTooLarge reports a delivery above maxWebhookBody.
var ErrBodyTooLarge = errors.New("the webhook body is too large")

// Ingest is Delivery's narrow capability for one webhook delivery.
type Ingest interface {
	IngestEmailWebhook(ctx context.Context, provider string, header func(string) string, body []byte) error
}

// Resource serves the provider webhooks. Failed renders a failure; the root
// owns the error envelope and the status of each failure.
type Resource struct {
	ingest    Ingest
	failed    func(http.ResponseWriter, *http.Request, error)
	rateLimit func(http.Handler) http.Handler
}

// NewResource builds the resource. rateLimit may be nil.
func NewResource(ingest Ingest, failed func(http.ResponseWriter, *http.Request, error), rateLimit func(http.Handler) http.Handler) *Resource {
	return &Resource{ingest: ingest, failed: failed, rateLimit: rateLimit}
}

// Router has no session middleware on purpose: see the package comment.
func (rs *Resource) Router() chi.Router {
	router := chi.NewRouter()
	if rs.rateLimit != nil {
		router.Use(rs.rateLimit)
	}
	router.Post("/email/delivery/{provider}", rs.deliver)
	return router
}

func (rs *Resource) deliver(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBody))
	if err != nil {
		rs.failed(w, r, ErrBodyTooLarge)
		return
	}
	if err := rs.ingest.IngestEmailWebhook(r.Context(), chi.URLParam(r, "provider"), r.Header.Get, body); err != nil {
		rs.failed(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
