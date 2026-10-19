package compose

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/emailoutbox"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/notifications"
	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	delivery "github.com/moto-nrw/project-phoenix/modules/delivery/runtime"
	deliveryModels "github.com/moto-nrw/project-phoenix/modules/delivery/subscriptions"
)

// The transports behind the Delivery worker: the e-mail provider renders a
// claimed intent through the template registry and sends it, the push
// provider re-checks the subscription before it sends (#4013).

// MailPushProvider sends the claimed e-mail and push intents.
type MailPushProvider struct {
	Registry       *emailoutbox.TemplateRegistry
	Mailer         email.Mailer
	MailIdentity   email.ReplyToResolver
	Push           deliveryContract.PushSender
	Logger         *slog.Logger
	Tenants        IntentTenantTransactions
	PushAuthorized func(context.Context, deliveryContract.ClaimedIntent) (bool, error)
	DeferredPush   func(context.Context, deliveryContract.ClaimedIntent) error
}

// IntentTenantTransactions runs a provider step in the tenant transaction
// of the claimed intent's school (TenantTransactions).
type IntentTenantTransactions interface {
	WithinTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error
	WithinSystem(context.Context, func(context.Context) error) error
}

type pushSubscriptionCleaner interface {
	DeleteExpiredIfUnchanged(context.Context, *deliveryModels.PushSubscription) (bool, error)
}

// NewPushAuthorizationChecker answers whether the claimed push still goes to
// a subscription its audience may receive.
func NewPushAuthorizationChecker(tenants IntentTenantTransactions, subscriptions notifications.PushSubscriptionAuthorizer) func(context.Context, deliveryContract.ClaimedIntent) (bool, error) {
	return func(ctx context.Context, intent deliveryContract.ClaimedIntent) (bool, error) {
		var eligible []*deliveryModels.PushSubscription
		err := tenants.WithinTenant(ctx, intent.TenantID, func(txCtx context.Context) error {
			var lookupErr error
			eligible, lookupErr = notifications.EligiblePushSubscriptions(txCtx, subscriptions, intent.PushRecipient)
			return lookupErr
		})
		if err != nil {
			return false, err
		}
		return containsPushSubscription(eligible, intent.PushRecipient), nil
	}
}

func containsPushSubscription(subscriptions []*deliveryModels.PushSubscription, recipient deliveryContract.PushRecipient) bool {
	for _, subscription := range subscriptions {
		if subscription.ID == recipient.SubscriptionID && subscription.AccountID == recipient.AccountID &&
			subscription.Endpoint == recipient.Endpoint && subscription.P256dh == recipient.P256DH &&
			subscription.Auth == recipient.Auth && subscription.Portal == recipient.Portal &&
			subscription.UpdatedAt.Equal(recipient.UpdatedAt) {
			return true
		}
	}
	return false
}

// NewExpiredPushSubscriptionCleaner removes the subscription a push service
// reported gone, unless it changed since the intent was claimed.
func NewExpiredPushSubscriptionCleaner(tenants IntentTenantTransactions, subscriptions pushSubscriptionCleaner) func(context.Context, deliveryContract.ClaimedIntent) error {
	return func(ctx context.Context, intent deliveryContract.ClaimedIntent) error {
		return tenants.WithinTenant(ctx, intent.TenantID, func(txCtx context.Context) error {
			subscription := &deliveryModels.PushSubscription{
				AccountID: intent.PushRecipient.AccountID, Portal: intent.PushRecipient.Portal,
				Endpoint: intent.PushRecipient.Endpoint, P256dh: intent.PushRecipient.P256DH,
				Auth: intent.PushRecipient.Auth,
			}
			subscription.ID = intent.PushRecipient.SubscriptionID
			subscription.UpdatedAt = intent.PushRecipient.UpdatedAt
			_, err := subscriptions.DeleteExpiredIfUnchanged(txCtx, subscription)
			return err
		})
	}
}

// DurableEmails serves the e-mail outbox's durable store over the Delivery
// module.
type DurableEmails struct{ module *delivery.Module }

// NewDurableEmails binds the e-mail outbox's store to module.
func NewDurableEmails(module *delivery.Module) DurableEmails {
	return DurableEmails{module: module}
}

// EnqueueEmail stores an e-mail intent and reports whether it was a
// duplicate.
func (a DurableEmails) EnqueueEmail(ctx context.Context, input emailoutbox.DurableEmail) (emailoutbox.DurableEmailResult, error) {
	stored, err := a.module.EnqueueEmail(ctx, deliveryContract.EmailIntent{
		TenantID: input.TenantID, Template: input.Template,
		Recipient: deliveryContract.EmailRecipient{Address: input.Recipient}, Payload: input.Payload,
		IdempotencyKey: input.IdempotencyKey,
		Related:        deliveryContract.RelatedEntity{Type: input.RelatedType, ID: input.RelatedID},
		DeliverAfter:   input.DeliverAfter,
	})
	if err != nil {
		return emailoutbox.DurableEmailResult{}, err
	}
	return emailoutbox.DurableEmailResult{ID: stored.ID, Duplicate: stored.Duplicate}, nil
}

// CancelEmail cancels the pending e-mails of an entity.
func (a DurableEmails) CancelEmail(ctx context.Context, tenantID int64, relatedType string, relatedID int64, reason string) (int64, error) {
	return a.module.Cancel(ctx, tenantID, deliveryContract.TransportEmail, deliveryContract.RelatedEntity{Type: relatedType, ID: relatedID}, reason)
}

// SendEmail renders the claimed e-mail in its school's tenant transaction and
// hands it to the mailer.
func (p *MailPushProvider) SendEmail(ctx context.Context, intent deliveryContract.ClaimedIntent) (deliveryContract.ProviderResult, error) {
	var payload map[string]any
	if err := json.Unmarshal(intent.EmailPayload, &payload); err != nil {
		return deliveryContract.ProviderResult{}, fmt.Errorf("delivery provider: decode email payload: %w", err)
	}
	relatedType := ""
	if intent.RelatedEntityType != nil {
		relatedType = *intent.RelatedEntityType
	}
	relatedID := int64(0)
	if intent.RelatedEntityID != nil {
		relatedID = *intent.RelatedEntityID
	}
	row := &emailoutbox.Intent{
		ID: intent.ID, TenantID: intent.TenantID, Kind: intent.Template,
		RelatedEntityType: relatedType, RelatedEntityID: relatedID,
		Payload: payload, PayloadJSON: intent.EmailPayload, Attempts: intent.Attempts,
	}
	renderer, err := p.Registry.Lookup(intent.Template)
	if err != nil {
		return deliveryContract.ProviderResult{}, err
	}
	transport := emailoutbox.Transport{Mailer: p.Mailer, MailIdentity: p.MailIdentity, Tenants: p.Tenants, Logger: p.Logger}
	err = transport.Send(ctx, row, renderer, email.NewEmail(intent.EmailRecipient.Name, intent.EmailRecipient.Address))
	if errors.Is(err, emailoutbox.ErrRenderCancelled) {
		return deliveryContract.ProviderResult{}, fmt.Errorf("%w: %v", deliveryContract.ErrCancelled, err)
	}
	if errors.Is(err, emailoutbox.ErrRenderDeferred) {
		return deliveryContract.ProviderResult{}, fmt.Errorf("%w: %v", deliveryContract.ErrDeferred, err)
	}
	if err != nil {
		return deliveryContract.ProviderResult{}, err
	}
	return deliveryContract.ProviderResult{StatusCode: deliveryContract.ProviderAcceptedStatusCode}, nil
}

// SendPush sends the claimed push when its subscription is still authorized.
func (p *MailPushProvider) SendPush(ctx context.Context, intent deliveryContract.ClaimedIntent) (deliveryContract.ProviderResult, error) {
	if intent.PushRecipient.Deferred {
		if p.DeferredPush == nil {
			return deliveryContract.ProviderResult{}, errors.New("delivery: deferred push dispatcher is not configured")
		}
		err := p.DeferredPush(ctx, intent)
		return deliveryContract.ProviderResult{StatusCode: deliveryContract.ProviderAcceptedStatusCode}, err
	}
	if p.PushAuthorized == nil {
		return deliveryContract.ProviderResult{}, errors.New("delivery provider: push authorization is not configured")
	}
	allowed, err := p.PushAuthorized(ctx, intent)
	if err != nil {
		return deliveryContract.ProviderResult{}, fmt.Errorf("delivery provider: check push authorization: %w", err)
	}
	if !allowed {
		return deliveryContract.ProviderResult{}, deliveryContract.ErrCancelled
	}
	return p.Push.SendPush(ctx, intent)
}
