package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/ports"
)

// Service is the public durable Delivery capability: it validates and
// normalizes the public intent values before it records or reads them.
type Service struct {
	store   ports.Store
	people  ports.GuardianDirectory
	observe ports.Observer
}

func NewService(store ports.Store, people ports.GuardianDirectory, observe ports.Observer) *Service {
	if store == nil || people == nil || observe == nil {
		panic("delivery application: store, guardian directory, and observer are required")
	}
	return &Service{store: store, people: people, observe: observe}
}

// enqueueInput is one validated intent with its encoded recipient and payload.
type enqueueInput struct {
	TenantID       int64
	OrganizationID int64
	Transport      domain.Transport
	Template       string
	IdempotencyKey string
	Related        deliveryContract.RelatedEntity
	Recipient      json.RawMessage
	Payload        json.RawMessage
	DeliverAfter   time.Time
}

func (s *Service) EnqueueEmail(ctx context.Context, intent deliveryContract.EmailIntent) (deliveryContract.Enqueued, error) {
	return s.enqueueEmail(ctx, intent, false)
}

// EnqueueSystemEmail persists a global identity mail in the caller's admin
// transaction. Global mail is not assigned to a placeholder school.
func (s *Service) EnqueueSystemEmail(ctx context.Context, intent deliveryContract.EmailIntent) (deliveryContract.Enqueued, error) {
	return s.enqueueEmail(ctx, intent, true)
}

func (s *Service) enqueueEmail(ctx context.Context, intent deliveryContract.EmailIntent, system bool) (deliveryContract.Enqueued, error) {
	intent.Template = strings.TrimSpace(intent.Template)
	intent.IdempotencyKey = strings.TrimSpace(intent.IdempotencyKey)
	intent.Related.Type = strings.TrimSpace(intent.Related.Type)
	intent.Recipient.Address = strings.TrimSpace(intent.Recipient.Address)
	intent.Recipient.Name = strings.TrimSpace(intent.Recipient.Name)
	if err := validateEmailScope(intent, system); err != nil {
		return deliveryContract.Enqueued{}, err
	}
	if intent.Template == "" {
		return deliveryContract.Enqueued{}, errors.New("delivery: email template is required")
	}
	parsedRecipient, err := mail.ParseAddress(intent.Recipient.Address)
	if err != nil {
		return deliveryContract.Enqueued{}, fmt.Errorf("delivery: invalid email recipient: %w", err)
	}
	intent.Recipient.Address = parsedRecipient.Address
	if intent.Recipient.Name == "" {
		intent.Recipient.Name = parsedRecipient.Name
	}
	if len(intent.Payload) == 0 || !json.Valid(intent.Payload) {
		return deliveryContract.Enqueued{}, errors.New("delivery: email payload must be valid JSON")
	}
	if err := validateRelated(intent.Related); err != nil {
		return deliveryContract.Enqueued{}, err
	}
	recipient, err := json.Marshal(intent.Recipient)
	if err != nil {
		return deliveryContract.Enqueued{}, fmt.Errorf("delivery: marshal email recipient: %w", err)
	}
	return s.enqueue(ctx, enqueueInput{
		TenantID: intent.TenantID, OrganizationID: intent.OrganizationID, Transport: domain.TransportEmail, Template: intent.Template,
		IdempotencyKey: intent.IdempotencyKey, Related: intent.Related,
		Recipient: recipient, Payload: intent.Payload, DeliverAfter: intent.DeliverAfter,
	}, deliveryContract.TransportEmail)
}

// validateEmailScope admits a school mail, a Träger mail (ADR 0106) or, on
// the system path, a global mail with neither.
func validateEmailScope(intent deliveryContract.EmailIntent, system bool) error {
	switch {
	case system && (intent.TenantID != 0 || intent.OrganizationID != 0):
		return errors.New("delivery: a system email has neither tenant nor Träger")
	case system:
		return nil
	case intent.TenantID > 0 && intent.OrganizationID == 0:
		return nil
	case intent.TenantID == 0 && intent.OrganizationID > 0:
		return nil
	default:
		return errors.New("delivery: tenant is required")
	}
}

func (s *Service) EnqueueDeferredPush(ctx context.Context, intent deliveryContract.DeferredPushIntent) (deliveryContract.Enqueued, error) {
	if intent.TenantID <= 0 || strings.TrimSpace(intent.Template) == "" || strings.TrimSpace(intent.IdempotencyKey) == "" {
		return deliveryContract.Enqueued{}, errors.New("delivery: deferred push requires tenant, template, and idempotency key")
	}
	if len(intent.Payload) == 0 || !json.Valid(intent.Payload) {
		return deliveryContract.Enqueued{}, errors.New("delivery: invalid deferred push payload")
	}
	if err := validateRelated(intent.Related); err != nil {
		return deliveryContract.Enqueued{}, err
	}
	return s.enqueue(ctx, enqueueInput{
		TenantID: intent.TenantID, Transport: domain.TransportPush, Template: intent.Template,
		IdempotencyKey: intent.IdempotencyKey, Related: intent.Related,
		Recipient: json.RawMessage(`{"deferred":true}`), Payload: intent.Payload,
	}, deliveryContract.TransportPush)
}

func (s *Service) EnqueuePush(ctx context.Context, intent deliveryContract.PushIntent) (deliveryContract.Enqueued, error) {
	intent.Template = strings.TrimSpace(intent.Template)
	intent.IdempotencyKey = strings.TrimSpace(intent.IdempotencyKey)
	intent.Related.Type = strings.TrimSpace(intent.Related.Type)
	intent.Recipient.Endpoint = strings.TrimSpace(intent.Recipient.Endpoint)
	intent.Recipient.P256DH = strings.TrimSpace(intent.Recipient.P256DH)
	intent.Recipient.Auth = strings.TrimSpace(intent.Recipient.Auth)
	intent.Payload.Title = strings.TrimSpace(intent.Payload.Title)
	if intent.TenantID <= 0 {
		return deliveryContract.Enqueued{}, errors.New("delivery: tenant is required")
	}
	if intent.Template == "" || intent.Recipient.Endpoint == "" || intent.Recipient.P256DH == "" || intent.Recipient.Auth == "" {
		return deliveryContract.Enqueued{}, errors.New("delivery: push template and recipient keys are required")
	}
	if err := deliveryContract.ValidatePushEndpoint(intent.Recipient.Endpoint); err != nil {
		return deliveryContract.Enqueued{}, fmt.Errorf("delivery: invalid push endpoint: %w", err)
	}
	if intent.Payload.Title == "" {
		return deliveryContract.Enqueued{}, errors.New("delivery: push title is required")
	}
	if err := validateRelated(intent.Related); err != nil {
		return deliveryContract.Enqueued{}, err
	}
	recipient, err := json.Marshal(intent.Recipient)
	if err != nil {
		return deliveryContract.Enqueued{}, fmt.Errorf("delivery: marshal push recipient: %w", err)
	}
	payload, err := json.Marshal(intent.Payload)
	if err != nil {
		return deliveryContract.Enqueued{}, fmt.Errorf("delivery: marshal push payload: %w", err)
	}
	return s.enqueue(ctx, enqueueInput{
		TenantID: intent.TenantID, Transport: domain.TransportPush, Template: intent.Template,
		IdempotencyKey: intent.IdempotencyKey, Related: intent.Related,
		Recipient: recipient, Payload: payload,
	}, deliveryContract.TransportPush)
}

func (s *Service) enqueue(ctx context.Context, input enqueueInput, transport deliveryContract.Transport) (deliveryContract.Enqueued, error) {
	stored, err := s.record(ctx, input)
	if err != nil {
		if errors.Is(err, domain.ErrIdempotencyConflict) {
			return deliveryContract.Enqueued{}, deliveryContract.ErrIdempotencyConflict
		}
		return deliveryContract.Enqueued{}, err
	}
	return deliveryContract.Enqueued{ID: stored.ID, Transport: transport, Duplicate: stored.Duplicate}, nil
}

// record persists one validated intent and observes the store's answer.
func (s *Service) record(ctx context.Context, input enqueueInput) (result domain.Enqueued, err error) {
	started := time.Now()
	defer func() {
		s.observe(domain.Observation{Operation: "enqueue", Transport: string(input.Transport), Template: input.Template, Duration: time.Since(started), Count: boolCount(err == nil), Err: err})
	}()
	nextRetryAt := time.Now()
	if input.DeliverAfter.After(nextRetryAt) {
		nextRetryAt = input.DeliverAfter
	}
	intent := domain.Intent{
		TenantID: input.TenantID, OrganizationID: input.OrganizationID, Transport: input.Transport, Template: input.Template,
		Recipient: input.Recipient, Payload: input.Payload, Status: string(domain.StatePending), NextRetryAt: nextRetryAt,
	}
	if input.IdempotencyKey != "" {
		intent.IdempotencyKey = &input.IdempotencyKey
	}
	if input.Related.Type != "" {
		intent.RelatedEntityType = &input.Related.Type
		intent.RelatedEntityID = &input.Related.ID
	}
	result, err = s.store.Enqueue(ctx, intent)
	if errors.Is(err, domain.ErrIdempotencyConflict) {
		s.observe(domain.Observation{Operation: "idempotency_conflict", Transport: string(input.Transport), Template: input.Template, Count: 1, Err: err})
	} else if result.Duplicate {
		s.observe(domain.Observation{Operation: "duplicate", Transport: string(input.Transport), Template: input.Template, Count: 1})
	}
	return result, err
}

func (s *Service) Cancel(ctx context.Context, tenantID int64, transport deliveryContract.Transport, related deliveryContract.RelatedEntity, reason string) (count int64, err error) {
	if tenantID <= 0 {
		return 0, errors.New("delivery: tenant is required")
	}
	if err := validateTransport(transport); err != nil {
		return 0, err
	}
	related.Type = strings.TrimSpace(related.Type)
	if err := validateRelated(related); err != nil {
		return 0, err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return 0, errors.New("delivery: cancellation reason is required")
	}
	started := time.Now()
	defer func() {
		s.observe(domain.Observation{Operation: "cancel", Transport: string(transport), Duration: time.Since(started), Count: int(count), Err: err})
	}()
	return s.store.Cancel(ctx, tenantID, domain.Transport(transport), related.Type, related.ID, reason, time.Now())
}

func (s *Service) Statuses(ctx context.Context, tenantID int64, transport deliveryContract.Transport, related deliveryContract.RelatedEntity) ([]deliveryContract.Status, error) {
	if tenantID <= 0 {
		return nil, errors.New("delivery: tenant is required")
	}
	if err := validateTransport(transport); err != nil {
		return nil, err
	}
	related.Type = strings.TrimSpace(related.Type)
	if err := validateRelated(related); err != nil {
		return nil, err
	}
	rows, err := s.intents(ctx, tenantID, domain.Transport(transport), related)
	if err != nil {
		return nil, err
	}
	statuses := make([]deliveryContract.Status, 0, len(rows))
	for _, row := range rows {
		var providerResult *deliveryContract.ProviderResult
		if len(row.ProviderResult) > 0 {
			providerResult = &deliveryContract.ProviderResult{}
			if err := json.Unmarshal(row.ProviderResult, providerResult); err != nil {
				return nil, fmt.Errorf("delivery: decode provider result for %s intent %d: %w", transport, row.ID, err)
			}
		}
		statuses = append(statuses, deliveryContract.Status{
			ID: row.ID, IdempotencyKey: row.IdempotencyKey, Transport: transport, State: deliveryContract.State(row.Status), Attempts: row.Attempts,
			LastError: row.LastError, ProviderResult: providerResult, NextAttemptAt: row.NextRetryAt,
			SentAt: row.SentAt, DeadLetterAt: row.DeadLetterAt, CancelledAt: row.CancelledAt,
		})
	}
	return statuses, nil
}

// intents reads the intents of one related entity and observes the store's answer.
func (s *Service) intents(ctx context.Context, tenantID int64, transport domain.Transport, related deliveryContract.RelatedEntity) (statuses []domain.Intent, err error) {
	started := time.Now()
	defer func() {
		s.observe(domain.Observation{Operation: "status_query", Transport: string(transport), Duration: time.Since(started), Count: len(statuses), Err: err})
	}()
	return s.store.Statuses(ctx, tenantID, transport, related.Type, related.ID)
}

func (s *Service) ReplaceEmailDeliveries(ctx context.Context, tenantID int64, related deliveryContract.RelatedEntity, rows []deliveryContract.EmailDelivery) error {
	if tenantID <= 0 {
		return errors.New("delivery: tenant is required")
	}
	related.Type = strings.TrimSpace(related.Type)
	if err := validateRelated(related); err != nil {
		return err
	}
	values := make([]domain.EmailDelivery, 0, len(rows))
	for _, row := range rows {
		if err := validateReachability(row.Reachability); err != nil {
			return err
		}
		values = append(values, domain.EmailDelivery{
			ID: row.ID, OutboxID: row.OutboxID, GuardianProfileID: row.GuardianProfileID,
			AccountID: row.AccountID, RecipientEmail: row.RecipientEmail, Reachability: row.Reachability,
		})
	}
	return s.store.ReplaceEmailDeliveries(ctx, tenantID, related.Type, related.ID, values)
}

func (s *Service) DeleteEmailDeliveries(ctx context.Context, tenantID int64, related deliveryContract.RelatedEntity) (int64, error) {
	if tenantID <= 0 {
		return 0, errors.New("delivery: tenant is required")
	}
	related.Type = strings.TrimSpace(related.Type)
	if err := validateRelated(related); err != nil {
		return 0, err
	}
	return s.store.DeleteEmailDeliveries(ctx, tenantID, related.Type, related.ID)
}

func (s *Service) EmailDeliveryStatuses(ctx context.Context, tenantID int64, related deliveryContract.RelatedEntity) (statuses []deliveryContract.EmailDeliveryStatus, err error) {
	if tenantID <= 0 {
		return nil, errors.New("delivery: tenant is required")
	}
	related.Type = strings.TrimSpace(related.Type)
	if err := validateRelated(related); err != nil {
		return nil, err
	}
	started := time.Now()
	defer func() {
		s.observe(domain.Observation{Operation: "delivery_status_query", Transport: string(domain.TransportEmail), Duration: time.Since(started), Count: len(statuses), Err: err})
	}()
	statuses, err = s.store.EmailDeliveryStatuses(ctx, tenantID, related.Type, related.ID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(statuses))
	for _, status := range statuses {
		if status.GuardianProfileID != nil {
			ids = append(ids, *status.GuardianProfileID)
		}
	}
	rows, err := s.people.ResolveGuardianDisplays(ctx, ids)
	if err != nil {
		return nil, err
	}
	displays := make(map[int64]deliveryContract.GuardianDisplay, len(rows))
	for _, row := range rows {
		displays[row.GuardianProfileID] = row
	}
	for index := range statuses {
		if statuses[index].GuardianProfileID == nil {
			continue
		}
		display := displays[*statuses[index].GuardianProfileID]
		statuses[index].FirstName = display.FirstName
		statuses[index].LastName = display.LastName
	}
	sort.SliceStable(statuses, func(i, j int) bool {
		left := strings.ToLower(statuses[i].LastName + "\x00" + statuses[i].FirstName)
		right := strings.ToLower(statuses[j].LastName + "\x00" + statuses[j].FirstName)
		if left == right {
			return statuses[i].DeliveryID < statuses[j].DeliveryID
		}
		return left < right
	})
	return statuses, nil
}

func (s *Service) AttachEmailOutbox(ctx context.Context, tenantID, deliveryID, outboxID int64) error {
	if tenantID <= 0 || deliveryID <= 0 || outboxID <= 0 {
		return errors.New("delivery: tenant, delivery, and outbox ids are required")
	}
	return s.store.AttachEmailOutbox(ctx, tenantID, deliveryID, outboxID)
}

func (s *Service) ClaimFailedEmailDelivery(ctx context.Context, tenantID, deliveryID int64) (bool, error) {
	if tenantID <= 0 || deliveryID <= 0 {
		return false, errors.New("delivery: tenant and delivery ids are required")
	}
	return s.store.ClaimFailedEmailDelivery(ctx, tenantID, deliveryID)
}

func validateTransport(transport deliveryContract.Transport) error {
	if transport != deliveryContract.TransportEmail && transport != deliveryContract.TransportPush {
		return fmt.Errorf("delivery: unknown transport %q", transport)
	}
	return nil
}

func validateRelated(related deliveryContract.RelatedEntity) error {
	if (related.Type == "") != (related.ID == 0) || related.ID < 0 {
		return errors.New("delivery: related entity type and positive id must be supplied together")
	}
	return nil
}

func validateReachability(value string) error {
	switch value {
	case deliveryContract.ReachabilityOK, deliveryContract.ReachabilityNoEmail, deliveryContract.ReachabilityNoPortal, deliveryContract.ReachabilityExcluded:
		return nil
	default:
		return fmt.Errorf("delivery: unknown reachability %q", value)
	}
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}
