package platform

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// DeliveryEvent is the provider-neutral event an adapter produces. Adapters
// translate a provider's wire format into this; nothing downstream knows which
// provider it came from beyond the Provider label.
type DeliveryEvent struct {
	Provider          string
	EventID           string
	Type              string
	OccurredAt        time.Time
	MessageID         string
	ProviderMessageID string
	FallbackMessageID string
	Recipient         string
	BounceClass       string
	ReasonCode        string
	Reason            string
	Raw               map[string]any
}

// IngestOutcome classifies what happened to one event. All four are normal:
// none of them is an error the provider should retry.
type IngestOutcome string

const (
	IngestApplied   IngestOutcome = "applied"   // status advanced
	IngestDuplicate IngestOutcome = "duplicate" // provider_event_id already stored
	IngestStale     IngestOutcome = "stale"     // lost the precedence comparison
	IngestUnmatched IngestOutcome = "unmatched" // no outbox row for this message
)

// BatchResult tallies an ingest batch for the webhook response body.
type BatchResult struct {
	Accepted  int `json:"accepted"`
	Applied   int `json:"applied"`
	Duplicate int `json:"duplicate"`
	Stale     int `json:"stale"`
	Unmatched int `json:"unmatched"`
}

// DeliveryMetrics is the narrow observability port. An interface rather than a
// direct dependency so tests can assert counters without touching the global
// Prometheus registry.
type DeliveryMetrics interface {
	RecordDeliveryEvent(provider, eventType, result string)
	RecordDeliveryStatus(status string)
}

// DeliverySettingsResolver is the slice of the settings service this needs.
// The *ForTenant variants are mandatory here: ingest runs outside tenant
// middleware, so there is no ambient tenant to resolve against.
type DeliverySettingsResolver interface {
	ResolveBoolForTenant(ctx context.Context, tenantID int64, key string) (bool, error)
	ResolveStringForTenant(ctx context.Context, tenantID int64, key string) (string, error)
	ResolveIntForTenant(ctx context.Context, tenantID int64, key string) (int, error)
}

// EmailDeliveryServiceConfig is the DI bundle.
type EmailDeliveryServiceConfig struct {
	OutboxRepo platformModels.EmailOutboxRepository
	EventRepo  platformModels.EmailDeliveryEventCleanupRepository
	Enqueuer   platformModels.OutboxEnqueuer
	Settings   DeliverySettingsResolver
	Metrics    DeliveryMetrics
	DB         *bun.DB
	Logger     *slog.Logger
}

// EmailDeliveryService turns provider delivery reports into outbox state.
//
// Everything runs under phoenix_admin: the webhook is unauthenticated, so the
// tenant is unknown until the outbox row is resolved, and the resolved row's
// tenant_id is the only trustworthy source for it.
type EmailDeliveryService struct {
	EmailDeliveryServiceConfig
}

// NewEmailDeliveryService builds the service.
func NewEmailDeliveryService(cfg EmailDeliveryServiceConfig) *EmailDeliveryService {
	return &EmailDeliveryService{EmailDeliveryServiceConfig: cfg}
}

func (s *EmailDeliveryService) getLogger() *slog.Logger {
	if s == nil || s.Logger == nil {
		return slog.Default()
	}
	return s.Logger
}

// IngestBatch processes every event, never aborting the batch because one
// event was bad. A single malformed or unmatched event must not cost the
// provider the rest of the delivery.
func (s *EmailDeliveryService) IngestBatch(ctx context.Context, events []DeliveryEvent) (BatchResult, error) {
	var result BatchResult
	for _, ev := range events {
		outcome, err := s.IngestEvent(ctx, ev)
		if err != nil {
			// A DB failure is the one case the provider SHOULD retry, so it
			// propagates and the handler answers 500.
			return result, err
		}
		result.Accepted++
		switch outcome {
		case IngestApplied:
			result.Applied++
		case IngestDuplicate:
			result.Duplicate++
		case IngestStale:
			result.Stale++
		case IngestUnmatched:
			result.Unmatched++
		}
	}
	return result, nil
}

// IngestEvent correlates one event to an outbox row, stores it, and advances
// the row's delivery status when the event outranks what is already there.
func (s *EmailDeliveryService) IngestEvent(ctx context.Context, ev DeliveryEvent) (IngestOutcome, error) {
	if s == nil || s.OutboxRepo == nil || s.EventRepo == nil || s.DB == nil {
		return "", errors.New("email delivery service not wired")
	}

	status := statusForEvent(ev.Type, optionalString(ev.BounceClass))
	if status == "" {
		return IngestUnmatched, nil
	}

	var (
		row      *platformModels.EmailOutbox
		outcome  IngestOutcome
		applied  bool
		newState string
	)

	if err := tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
		resolved, err := s.resolveOutbox(adminCtx, ev)
		if err != nil {
			return err
		}
		if resolved == nil {
			outcome = IngestUnmatched
			return nil
		}
		row = resolved.Outbox

		event := buildEvent(row, resolved.Attempt, ev, status)
		inserted, err := s.EventRepo.CreateIfNew(adminCtx, event)
		if err != nil {
			return err
		}
		if !inserted {
			// Same provider_event_id already stored: a replay. Returning here
			// keeps the whole operation idempotent — the status was already
			// applied (or rejected) when the event first arrived.
			outcome = IngestDuplicate
			return nil
		}
		if !resolved.Current {
			outcome = IngestStale
			return nil
		}

		advanced, err := s.OutboxRepo.ApplyDeliveryStatus(adminCtx, row.ID, platformModels.DeliveryTransition{
			Status:     status,
			Rank:       DeliveryRank(status),
			OccurredAt: ev.OccurredAt,
			Detail:     deliveryDetail(ev),
		})
		if err != nil {
			return err
		}
		applied = advanced
		newState = status
		if advanced {
			outcome = IngestApplied
		} else {
			outcome = IngestStale
		}
		return nil
	}); err != nil {
		return "", fmt.Errorf("ingest delivery event: %w", err)
	}

	s.recordMetrics(ev, outcome, applied, newState)

	if applied && IsTerminalFailure(newState) {
		// Best effort by design: a failed notification must never turn into a
		// 500 that makes the provider retry the whole batch.
		s.notifyDeliveryFailure(ctx, row, ev, newState)
	}

	return outcome, nil
}

type resolvedDispatch struct {
	Outbox  *platformModels.EmailOutbox
	Attempt *platformModels.EmailDispatchAttempt
	Current bool
}

// resolveOutbox finds the outbox row an event refers to, in decreasing order
// of trustworthiness.
func (s *EmailDeliveryService) resolveOutbox(ctx context.Context, ev DeliveryEvent) (*resolvedDispatch, error) {
	if id := normalizeMessageID(ev.MessageID); id != "" {
		attempt, err := s.OutboxRepo.FindDispatchAttemptByMessageID(ctx, id)
		if err != nil {
			return nil, err
		}
		if attempt != nil {
			return s.loadResolvedDispatch(ctx, attempt)
		}
	}
	if ev.ProviderMessageID != "" {
		attempt, err := s.OutboxRepo.FindDispatchAttemptByProviderMessageID(ctx, ev.ProviderMessageID)
		if err != nil {
			return nil, err
		}
		if attempt != nil {
			return s.loadResolvedDispatch(ctx, attempt)
		}
	}
	if resolved, err := s.resolveByLocalPart(ctx, ev.MessageID); err != nil || resolved != nil {
		return resolved, err
	}
	return s.resolveByLocalPart(ctx, ev.FallbackMessageID)
}

func (s *EmailDeliveryService) loadResolvedDispatch(ctx context.Context, attempt *platformModels.EmailDispatchAttempt) (*resolvedDispatch, error) {
	row, err := s.OutboxRepo.FindByID(ctx, attempt.OutboxID)
	if err != nil {
		return nil, err
	}
	current := row.MessageID != nil && normalizeMessageID(*row.MessageID) == attempt.MessageID
	return &resolvedDispatch{Outbox: row, Attempt: attempt, Current: current}, nil
}

// resolveByLocalPart is the last-resort fallback for relays that rewrite the
// Message-ID: our minted local part is "ob.{tenantID}.{outboxID}.{uuid}".
//
// The parsed values are attacker-controlled, so they are treated purely as a
// lookup hint: the row is fetched by id and then re-checked against the parsed
// tenant AND against having actually been dispatched. Without those checks,
// anyone who can reach the webhook could aim an event at an arbitrary row.
func (s *EmailDeliveryService) resolveByLocalPart(ctx context.Context, messageID string) (*resolvedDispatch, error) {
	tenantID, outboxID, correlationToken, ok := parseOutboxRef(normalizeMessageID(messageID))
	if !ok {
		return nil, nil
	}
	attempt, err := s.OutboxRepo.FindDispatchAttemptByCorrelationToken(ctx, correlationToken)
	if err != nil {
		return nil, err
	}
	if attempt == nil || attempt.OutboxID != outboxID || attempt.GetTenantID() != tenantID {
		return nil, nil
	}
	return s.loadResolvedDispatch(ctx, attempt)
}

func (s *EmailDeliveryService) recordMetrics(ev DeliveryEvent, outcome IngestOutcome, applied bool, status string) {
	if s.Metrics == nil {
		return
	}
	s.Metrics.RecordDeliveryEvent(ev.Provider, ev.Type, string(outcome))
	if applied {
		s.Metrics.RecordDeliveryStatus(status)
	}
}

// notifyDeliveryFailure enqueues an ops notice for a hard bounce or complaint.
func (s *EmailDeliveryService) notifyDeliveryFailure(ctx context.Context, row *platformModels.EmailOutbox, ev DeliveryEvent, status string) {
	if s.Enqueuer == nil || s.Settings == nil || row == nil {
		return
	}
	// Loop guard: a bouncing admin address must not produce a bounce notice
	// that bounces and produces another one.
	if row.Kind == platformModels.EmailKindDeliveryFailureNotice {
		return
	}

	tenantID := row.GetTenantID()
	enabled, err := s.Settings.ResolveBoolForTenant(ctx, tenantID, configModels.KeyEmailDeliveryAlertsEnabled)
	if err != nil || !enabled {
		return
	}
	raw, err := s.Settings.ResolveStringForTenant(ctx, tenantID, configModels.KeyEmailDeliveryAlertEmails)
	if err != nil {
		return
	}
	recipients := splitEmails(raw)
	if len(recipients) == 0 {
		return
	}

	// Enqueue writes through RLS and therefore needs a TENANT transaction, not
	// the admin one the rest of ingest runs in.
	if err := tenant.WithTenantTx(ctx, s.DB, tenantID, func(tenantCtx context.Context, _ bun.Tx) error {
		for _, recipient := range recipients {
			payload := map[string]any{
				DeliveryNoticePayloadRecipient:      recipient,
				DeliveryNoticePayloadFailedTo:       ev.Recipient,
				DeliveryNoticePayloadFailedStatus:   status,
				DeliveryNoticePayloadFailedKind:     row.Kind,
				DeliveryNoticePayloadReason:         ev.Reason,
				DeliveryNoticePayloadReasonCode:     ev.ReasonCode,
				DeliveryNoticePayloadOccurredAtText: ev.OccurredAt.Format("02.01.2006 15:04"),
			}
			if err := s.Enqueuer.EnqueueOutbox(tenantCtx, platformModels.OutboxEnqueueRequest{
				Kind:              platformModels.EmailKindDeliveryFailureNotice,
				Payload:           payload,
				RelatedEntityType: derefString(row.RelatedEntityType),
				RelatedEntityID:   derefInt64(row.RelatedEntityID),
				// Repeated bounce events for the same message must not flood
				// the office: the unique (tenant_id, idempotency_key) index
				// turns any further enqueue into a no-op.
				IdempotencyKey: fmt.Sprintf("delivery-failure-%d-%s-%s", row.ID, status, recipient),
			}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		s.getLogger().Warn("delivery failure notice could not be queued",
			slog.Int64("outbox_id", row.ID),
			slog.Int64("tenant_id", tenantID),
			slog.String("delivery_status", status),
			slog.String("error", err.Error()))
	}
}

// CleanupExpiredDeliveryEvents deletes delivery events past the tenant's
// retention window. Returns the number of rows removed.
func (s *EmailDeliveryService) CleanupExpiredDeliveryEvents(ctx context.Context, tenantID int64) (int64, error) {
	if s == nil || s.EventRepo == nil || s.DB == nil {
		return 0, errors.New("email delivery service not wired")
	}
	days := defaultDeliveryRetentionDays
	if s.Settings != nil {
		resolved, err := s.Settings.ResolveIntForTenant(ctx, tenantID, configModels.KeyEmailDeliveryRetentionDays)
		if err != nil {
			return 0, fmt.Errorf("resolve delivery event retention: %w", err)
		}
		if resolved <= 0 {
			return 0, fmt.Errorf("resolve delivery event retention: invalid value %d", resolved)
		}
		days = resolved
	}
	cutoff := time.Now().AddDate(0, 0, -days)

	var deleted int64
	if err := tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
		var err error
		deleted, err = s.EventRepo.DeleteOlderThan(adminCtx, tenantID, cutoff)
		return err
	}); err != nil {
		return 0, fmt.Errorf("cleanup delivery events: %w", err)
	}
	return deleted, nil
}

// defaultDeliveryRetentionDays mirrors the registry default for deployments
// where the settings service is not wired.
const defaultDeliveryRetentionDays = 90

func buildEvent(row *platformModels.EmailOutbox, attempt *platformModels.EmailDispatchAttempt, ev DeliveryEvent, _ string) *platformModels.EmailDeliveryEvent {
	event := &platformModels.EmailDeliveryEvent{
		OutboxID:          row.ID,
		DispatchAttemptID: attempt.ID,
		Provider:          ev.Provider,
		ProviderEventID:   ev.EventID,
		EventType:         ev.Type,
		OccurredAt:        ev.OccurredAt,
		ReceivedAt:        time.Now(),
		Raw:               ev.Raw,
	}
	event.SetTenantID(row.GetTenantID())
	event.ProviderMessageID = optionalString(ev.ProviderMessageID)
	event.BounceClass = optionalString(ev.BounceClass)
	event.ReasonCode = optionalString(ev.ReasonCode)
	event.Reason = optionalString(ev.Reason)
	event.Recipient = optionalString(ev.Recipient)
	return event
}

// deliveryDetail is the short provider explanation stored on the outbox row.
// The frontend renders its own German text and uses this only as a
// supplementary technical hint, so it stays raw but bounded.
func deliveryDetail(ev DeliveryEvent) string {
	parts := make([]string, 0, 2)
	if ev.ReasonCode != "" {
		parts = append(parts, ev.ReasonCode)
	}
	if ev.Reason != "" {
		parts = append(parts, ev.Reason)
	}
	detail := strings.Join(parts, " ")
	const maxDetail = 512
	if len(detail) > maxDetail {
		detail = detail[:maxDetail]
	}
	return detail
}

// normalizeMessageID strips the angle brackets RFC 5322 puts around a
// Message-ID; providers are inconsistent about including them.
func normalizeMessageID(id string) string {
	return strings.Trim(strings.TrimSpace(id), "<>")
}

// parseOutboxRef extracts tenant and outbox id from a minted Message-ID local
// part of the form "ob.{tenantID}.{outboxID}.{uuid}@{domain}".
func parseOutboxRef(messageID string) (tenantID, outboxID int64, correlationToken string, ok bool) {
	local, _, found := strings.Cut(messageID, "@")
	if !found {
		local = messageID
	}
	parts := strings.SplitN(local, ".", 4)
	if len(parts) < 4 || parts[0] != "ob" {
		return 0, 0, "", false
	}
	tenantID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || tenantID <= 0 {
		return 0, 0, "", false
	}
	outboxID, err = strconv.ParseInt(parts[2], 10, 64)
	if err != nil || outboxID <= 0 {
		return 0, 0, "", false
	}
	correlationToken = parts[3]
	if _, err := uuid.FromString(correlationToken); err != nil {
		return 0, 0, "", false
	}
	return tenantID, outboxID, correlationToken, true
}

func splitEmails(raw string) []string {
	var out []string
	for part := range strings.SplitSeq(raw, ",") {
		if s := strings.TrimSpace(part); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func optionalString(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func derefInt64(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}
