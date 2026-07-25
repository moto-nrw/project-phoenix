package webhooks

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/auth/webhooksig"
	"github.com/moto-nrw/project-phoenix/observability"
)

// maxWebhookBody caps the request body. Signature verification reads the whole
// body into memory, so this is also the memory bound per request.
const maxWebhookBody = 1 << 20 // 1 MiB

// ingestDeliveryEvents accepts provider delivery reports.
//
// Status codes are chosen around what makes a provider RETRY:
//   - 202 for every ingest outcome, including unmatched and stale. Events
//     about messages we never sent are normal on a shared provider account;
//     answering 4xx would make the provider hammer us forever.
//   - 401 for any signature problem, with no detail — distinguishing "wrong
//     secret" from "tampered body" would build a signing oracle.
//   - 503 when no secret is configured, so an operator can tell a
//     misconfiguration from an attack.
//   - 500 only on DB failure. That is the one case a retry can fix.
func (rs *Resource) ingestDeliveryEvents(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	adapter, ok := adapterFor(provider)
	if !ok {
		observability.RecordEmailWebhook("unknown", "malformed")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown provider"})
		return
	}
	// Only registered adapter names may become metric labels. The raw path is
	// unauthenticated input and would otherwise create unbounded cardinality.
	provider = adapter.Name()

	if !rs.Verifier.Configured() {
		observability.RecordEmailWebhook(provider, "not_configured")
		rs.getLogger().Error("email delivery webhook called but EMAIL_WEBHOOK_SIGNING_SECRET is unset",
			slog.String("provider", provider))
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "webhook not configured"})
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookBody))
	if err != nil {
		observability.RecordEmailWebhook(provider, "malformed")
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
		return
	}

	// Verify the RAW body before decoding anything — a decoder run on
	// unauthenticated input is attack surface we do not need.
	if err := rs.Verifier.Verify(
		r.Header.Get(webhooksig.HeaderTimestamp),
		r.Header.Get(webhooksig.HeaderSignature),
		body,
		time.Now(),
	); err != nil {
		result := "invalid_signature"
		if err == webhooksig.ErrTimestampSkew {
			result = "stale_timestamp"
		}
		observability.RecordEmailWebhook(provider, result)
		rs.getLogger().Warn("email delivery webhook signature rejected",
			slog.String("provider", provider),
			slog.String("reason", result))
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid signature"})
		return
	}

	events, err := adapter.Parse(body)
	if err != nil {
		observability.RecordEmailWebhook(provider, "malformed")
		rs.getLogger().Warn("email delivery webhook payload rejected",
			slog.String("provider", provider),
			slog.String("error", err.Error()))
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}

	result, err := rs.DeliveryService.IngestBatch(r.Context(), events)
	if err != nil {
		rs.getLogger().Error("email delivery ingest failed",
			slog.String("provider", provider),
			slog.String("error", err.Error()))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "ingest failed"})
		return
	}

	observability.RecordEmailWebhook(provider, "accepted")
	writeJSON(w, http.StatusAccepted, result)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
