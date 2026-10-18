package compose

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/email"
	deliveryModels "github.com/moto-nrw/project-phoenix/models/delivery"
	"github.com/moto-nrw/project-phoenix/modules/delivery"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailoutbox"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
)

// The transports behind the Delivery worker: the e-mail provider renders a
// claimed intent through the template registry and sends it, the push
// provider re-checks the subscription before it sends (#4013).

// MailPushProvider sends the claimed e-mail and push intents.
type MailPushProvider struct {
	Registry       *emailoutbox.TemplateRegistry
	Mailer         email.Mailer
	MailIdentity   email.ReplyToResolver
	Push           delivery.PushSender
	Logger         *slog.Logger
	Tenants        IntentTenantTransactions
	PushAuthorized func(context.Context, delivery.ClaimedIntent) (bool, error)
	DeferredPush   func(context.Context, delivery.ClaimedIntent) error
}

// IntentTenantTransactions runs a provider step in the tenant transaction
// of the claimed intent's school (TenantTransactions).
type IntentTenantTransactions interface {
	WithinTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error
}

type pushSubscriptionCleaner interface {
	DeleteExpiredIfUnchanged(context.Context, *deliveryModels.PushSubscription) (bool, error)
}

type pushSubscriptionAuthorizer interface {
	FindForTenantStaff(context.Context) ([]*deliveryModels.PushSubscription, error)
	FindForTenantAdmins(context.Context) ([]*deliveryModels.PushSubscription, error)
	FindForGuardians(context.Context, []int64, []int64) ([]*deliveryModels.PushSubscription, error)
	FindForStaffAccounts(context.Context, []int64) ([]*deliveryModels.PushSubscription, error)
	FindForSchoolAccounts(context.Context, []int64) ([]*deliveryModels.PushSubscription, error)
}

// NewPushAuthorizationChecker answers whether the claimed push still goes to
// a subscription its audience may receive.
func NewPushAuthorizationChecker(tenants IntentTenantTransactions, subscriptions pushSubscriptionAuthorizer) func(context.Context, delivery.ClaimedIntent) (bool, error) {
	return func(ctx context.Context, intent delivery.ClaimedIntent) (bool, error) {
		var eligible []*deliveryModels.PushSubscription
		err := tenants.WithinTenant(ctx, intent.TenantID, func(txCtx context.Context) error {
			var lookupErr error
			eligible, lookupErr = eligiblePushSubscriptions(txCtx, subscriptions, intent.PushRecipient)
			return lookupErr
		})
		if err != nil {
			return false, err
		}
		return containsPushSubscription(eligible, intent.PushRecipient), nil
	}
}

func eligiblePushSubscriptions(ctx context.Context, subscriptions pushSubscriptionAuthorizer, recipient delivery.PushRecipient) ([]*deliveryModels.PushSubscription, error) {
	switch recipient.AudienceScope {
	case string(notifications.ScopeTenant):
		return subscriptions.FindForTenantStaff(ctx)
	case string(notifications.ScopeAdmin):
		return subscriptions.FindForTenantAdmins(ctx)
	case string(notifications.ScopeGuardian):
		return subscriptions.FindForGuardians(ctx, []int64{recipient.AccountID}, recipient.StudentIDs)
	case string(notifications.ScopeStaff):
		if recipient.Portal == notifications.PortalSchool {
			return subscriptions.FindForSchoolAccounts(ctx, []int64{recipient.AccountID})
		}
		return subscriptions.FindForStaffAccounts(ctx, []int64{recipient.AccountID})
	default:
		return nil, nil
	}
}

func containsPushSubscription(subscriptions []*deliveryModels.PushSubscription, recipient delivery.PushRecipient) bool {
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
func NewExpiredPushSubscriptionCleaner(tenants IntentTenantTransactions, subscriptions pushSubscriptionCleaner) func(context.Context, delivery.ClaimedIntent) error {
	return func(ctx context.Context, intent delivery.ClaimedIntent) error {
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
	stored, err := a.module.EnqueueEmail(ctx, delivery.EmailIntent{
		TenantID: input.TenantID, Template: input.Template,
		Recipient: delivery.EmailRecipient{Address: input.Recipient}, Payload: input.Payload,
		IdempotencyKey: input.IdempotencyKey,
		Related:        delivery.RelatedEntity{Type: input.RelatedType, ID: input.RelatedID},
		DeliverAfter:   input.DeliverAfter,
	})
	if err != nil {
		return emailoutbox.DurableEmailResult{}, err
	}
	return emailoutbox.DurableEmailResult{ID: stored.ID, Duplicate: stored.Duplicate}, nil
}

// CancelEmail cancels the pending e-mails of an entity.
func (a DurableEmails) CancelEmail(ctx context.Context, tenantID int64, relatedType string, relatedID int64, reason string) (int64, error) {
	return a.module.Cancel(ctx, tenantID, delivery.TransportEmail, delivery.RelatedEntity{Type: relatedType, ID: relatedID}, reason)
}

// SendEmail renders the claimed e-mail in its school's tenant transaction and
// hands it to the mailer.
func (p *MailPushProvider) SendEmail(ctx context.Context, intent delivery.ClaimedIntent) (delivery.ProviderResult, error) {
	var payload map[string]any
	if err := json.Unmarshal(intent.EmailPayload, &payload); err != nil {
		return delivery.ProviderResult{}, fmt.Errorf("delivery provider: decode email payload: %w", err)
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
		return delivery.ProviderResult{}, err
	}
	var message *email.Message
	err = p.Tenants.WithinTenant(ctx, intent.TenantID, func(tenantCtx context.Context) error {
		var renderErr error
		message, renderErr = renderer.Render(tenantCtx, row)
		if renderErr == nil {
			p.applyReplyTo(tenantCtx, intent, message)
		}
		return renderErr
	})
	if err != nil {
		if errors.Is(err, emailoutbox.ErrRenderCancelled) {
			return delivery.ProviderResult{}, fmt.Errorf("%w: %v", delivery.ErrCancelled, err)
		}
		if errors.Is(err, emailoutbox.ErrRenderDeferred) {
			return delivery.ProviderResult{}, fmt.Errorf("%w: %v", delivery.ErrDeferred, err)
		}
		return delivery.ProviderResult{}, err
	}
	sendErr := sendRenderedEmail(ctx, p.Mailer, intent.EmailRecipient, message)
	if reporter, ok := renderer.(interface {
		RecordDelivery(context.Context, *emailoutbox.Intent, error) error
	}); ok {
		if err := p.Tenants.WithinTenant(ctx, intent.TenantID, func(recordCtx context.Context) error {
			return reporter.RecordDelivery(recordCtx, row, sendErr)
		}); err != nil {
			p.Logger.Error("record email delivery failed",
				slog.Int64("intent_id", intent.ID),
				slog.String("error", err.Error()))
		}
	}
	if sendErr != nil {
		return delivery.ProviderResult{}, fmt.Errorf("delivery provider: send email: %w", sendErr)
	}
	return delivery.ProviderResult{StatusCode: delivery.ProviderAcceptedStatusCode}, nil
}

func sendRenderedEmail(ctx context.Context, mailer email.Mailer, recipient delivery.EmailRecipient, message *email.Message) error {
	if message == nil {
		return errors.New("renderer returned no email message")
	}
	message.To = email.NewEmail(recipient.Name, recipient.Address)
	contextMailer, ok := mailer.(email.ContextMailer)
	if ok {
		return contextMailer.SendContext(ctx, *message)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return mailer.Send(*message)
}

func (p *MailPushProvider) applyReplyTo(ctx context.Context, intent delivery.ClaimedIntent, message *email.Message) {
	if intent.TenantID == 0 || p.MailIdentity == nil || message == nil || message.ReplyTo.Address != "" {
		return
	}
	identity, err := p.MailIdentity.ResolveReplyTo(ctx, intent.TenantID)
	if err != nil {
		p.Logger.Warn("delivery: reply-to lookup failed",
			slog.Int64("intent_id", intent.ID),
			slog.String("error", err.Error()),
		)
		return
	}
	if !identity.IsZero() {
		message.ReplyTo = email.NewEmail(identity.Name, identity.Address)
	}
}

// SendPush sends the claimed push when its subscription is still authorized.
func (p *MailPushProvider) SendPush(ctx context.Context, intent delivery.ClaimedIntent) (delivery.ProviderResult, error) {
	if intent.PushRecipient.Deferred {
		if p.DeferredPush == nil {
			return delivery.ProviderResult{}, errors.New("delivery: deferred push dispatcher is not configured")
		}
		err := p.DeferredPush(ctx, intent)
		return delivery.ProviderResult{StatusCode: delivery.ProviderAcceptedStatusCode}, err
	}
	if p.PushAuthorized == nil {
		return delivery.ProviderResult{}, errors.New("delivery provider: push authorization is not configured")
	}
	allowed, err := p.PushAuthorized(ctx, intent)
	if err != nil {
		return delivery.ProviderResult{}, fmt.Errorf("delivery provider: check push authorization: %w", err)
	}
	if !allowed {
		return delivery.ProviderResult{}, delivery.ErrCancelled
	}
	return p.Push.SendPush(ctx, intent)
}
