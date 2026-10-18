// Package delivery is the public durable Delivery capability.
package delivery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
)

type engine interface {
	EnqueueDeferredPush(context.Context, deliveryContract.DeferredPushIntent) (deliveryContract.Enqueued, error)
	EnqueueEmail(context.Context, deliveryContract.EmailIntent) (deliveryContract.Enqueued, error)
	EnqueuePush(context.Context, deliveryContract.PushIntent) (deliveryContract.Enqueued, error)
	Cancel(context.Context, int64, deliveryContract.Transport, deliveryContract.RelatedEntity, string) (int64, error)
	Statuses(context.Context, int64, deliveryContract.Transport, deliveryContract.RelatedEntity) ([]deliveryContract.Status, error)
	ReplaceEmailDeliveries(context.Context, int64, deliveryContract.RelatedEntity, []deliveryContract.EmailDelivery) error
	DeleteEmailDeliveries(context.Context, int64, deliveryContract.RelatedEntity) (int64, error)
	EmailDeliveryStatuses(context.Context, int64, deliveryContract.RelatedEntity) ([]deliveryContract.EmailDeliveryStatus, error)
	AttachEmailOutbox(context.Context, int64, int64, int64) error
	ClaimFailedEmailDelivery(context.Context, int64, int64) (bool, error)
}

func (m *Module) ReplaceEmailDeliveries(ctx context.Context, tenantID int64, related deliveryContract.RelatedEntity, rows []deliveryContract.EmailDelivery) error {
	if tenantID <= 0 {
		return errors.New("delivery: tenant is required")
	}
	related.Type = strings.TrimSpace(related.Type)
	if err := validateRelated(related); err != nil {
		return err
	}
	for _, row := range rows {
		if err := validateReachability(row.Reachability); err != nil {
			return err
		}
	}
	return m.engine.ReplaceEmailDeliveries(ctx, tenantID, related, rows)
}

func (m *Module) DeleteEmailDeliveries(ctx context.Context, tenantID int64, related deliveryContract.RelatedEntity) (int64, error) {
	if tenantID <= 0 {
		return 0, errors.New("delivery: tenant is required")
	}
	related.Type = strings.TrimSpace(related.Type)
	if err := validateRelated(related); err != nil {
		return 0, err
	}
	return m.engine.DeleteEmailDeliveries(ctx, tenantID, related)
}

func (m *Module) EmailDeliveryStatuses(ctx context.Context, tenantID int64, related deliveryContract.RelatedEntity) ([]deliveryContract.EmailDeliveryStatus, error) {
	if tenantID <= 0 {
		return nil, errors.New("delivery: tenant is required")
	}
	related.Type = strings.TrimSpace(related.Type)
	if err := validateRelated(related); err != nil {
		return nil, err
	}
	return m.engine.EmailDeliveryStatuses(ctx, tenantID, related)
}

func (m *Module) AttachEmailOutbox(ctx context.Context, tenantID, deliveryID, outboxID int64) error {
	if tenantID <= 0 || deliveryID <= 0 || outboxID <= 0 {
		return errors.New("delivery: tenant, delivery, and outbox ids are required")
	}
	return m.engine.AttachEmailOutbox(ctx, tenantID, deliveryID, outboxID)
}

func (m *Module) ClaimFailedEmailDelivery(ctx context.Context, tenantID, deliveryID int64) (bool, error) {
	if tenantID <= 0 || deliveryID <= 0 {
		return false, errors.New("delivery: tenant and delivery ids are required")
	}
	return m.engine.ClaimFailedEmailDelivery(ctx, tenantID, deliveryID)
}

type Module struct{ engine engine }

func NewModule(engine engine) *Module {
	if engine == nil {
		panic("delivery: engine is required")
	}
	return &Module{engine: engine}
}

func (m *Module) EnqueueEmail(ctx context.Context, intent deliveryContract.EmailIntent) (deliveryContract.Enqueued, error) {
	return m.enqueueEmail(ctx, intent, false)
}

// EnqueueSystemEmail persists a global identity mail in the caller's admin
// transaction. Global mail is not assigned to a placeholder school.
func (m *Module) EnqueueSystemEmail(ctx context.Context, intent deliveryContract.EmailIntent) (deliveryContract.Enqueued, error) {
	return m.enqueueEmail(ctx, intent, true)
}

func (m *Module) enqueueEmail(ctx context.Context, intent deliveryContract.EmailIntent, system bool) (deliveryContract.Enqueued, error) {
	intent.Template = strings.TrimSpace(intent.Template)
	intent.IdempotencyKey = strings.TrimSpace(intent.IdempotencyKey)
	intent.Related.Type = strings.TrimSpace(intent.Related.Type)
	intent.Recipient.Address = strings.TrimSpace(intent.Recipient.Address)
	intent.Recipient.Name = strings.TrimSpace(intent.Recipient.Name)
	if (!system && intent.TenantID <= 0) || (system && intent.TenantID != 0) {
		return deliveryContract.Enqueued{}, errors.New("delivery: tenant is required")
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
	return m.engine.EnqueueEmail(ctx, intent)
}

func (m *Module) EnqueuePush(ctx context.Context, intent deliveryContract.PushIntent) (deliveryContract.Enqueued, error) {
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
	return m.engine.EnqueuePush(ctx, intent)
}

func (m *Module) Cancel(ctx context.Context, tenantID int64, transport deliveryContract.Transport, related deliveryContract.RelatedEntity, reason string) (int64, error) {
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
	return m.engine.Cancel(ctx, tenantID, transport, related, reason)
}

func (m *Module) Statuses(ctx context.Context, tenantID int64, transport deliveryContract.Transport, related deliveryContract.RelatedEntity) ([]deliveryContract.Status, error) {
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
	return m.engine.Statuses(ctx, tenantID, transport, related)
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
