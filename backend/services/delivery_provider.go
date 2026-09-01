package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/moto-nrw/project-phoenix/email"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/delivery"
	"github.com/moto-nrw/project-phoenix/services/announcement"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/moto-nrw/project-phoenix/services/platform"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type deliveryProvider struct {
	registry     *platform.TemplateRegistry
	mailer       email.Mailer
	mailIdentity email.ReplyToResolver
	vapid        notifications.VAPIDConfig
	client       *http.Client
	logger       *slog.Logger
	db           *bun.DB
}

type guardianDisplayQuery interface {
	GuardianDisplays(context.Context, []int64) ([]usersService.GuardianDisplay, error)
}

type guardianDisplayResolver struct{ query guardianDisplayQuery }

func (r guardianDisplayResolver) ResolveGuardianDisplays(ctx context.Context, ids []int64) ([]delivery.GuardianDisplay, error) {
	if len(ids) == 0 {
		return []delivery.GuardianDisplay{}, nil
	}
	profiles, err := r.query.GuardianDisplays(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("delivery: resolve guardian display data: %w", err)
	}
	result := make([]delivery.GuardianDisplay, 0, len(profiles))
	for _, profile := range profiles {
		result = append(result, delivery.GuardianDisplay{
			GuardianProfileID: profile.GuardianProfileID, FirstName: profile.FirstName, LastName: profile.LastName,
		})
	}
	return result, nil
}

type announcementDeliveryAdapter struct{ module *delivery.Module }

type durableEmailAdapter struct{ module *delivery.Module }

type durablePushAdapter struct{ module *delivery.Module }

func (a durablePushAdapter) EnqueuePush(ctx context.Context, input notifications.PushIntent) (bool, error) {
	stored, err := a.module.EnqueuePush(ctx, delivery.PushIntent{
		TenantID: input.TenantID, Template: input.Template,
		Recipient: delivery.PushRecipient{
			SubscriptionID: input.SubscriptionID, Endpoint: input.Endpoint, P256DH: input.P256DH,
			Auth: input.Auth, Portal: input.Portal,
		},
		Payload: delivery.PushPayload{
			Title: input.Title, Body: input.Body, DeepLink: input.DeepLink, Type: input.Type, Priority: input.Priority,
		},
		IdempotencyKey: input.IdempotencyKey,
		Related:        delivery.RelatedEntity{Type: input.RelatedType, ID: input.RelatedID},
	})
	return stored.Duplicate, err
}

type enrollmentDeliveryAdapter struct {
	module   *delivery.Module
	tenantID func(context.Context) (int64, error)
}

func (a enrollmentDeliveryAdapter) CountRelatedEmails(ctx context.Context, relatedType string, relatedID int64) (int, error) {
	tenantID, err := a.tenantID(ctx)
	if err != nil {
		return 0, err
	}
	rows, err := a.module.Statuses(ctx, tenantID, delivery.TransportEmail, delivery.RelatedEntity{Type: relatedType, ID: relatedID})
	return len(rows), err
}

func (a enrollmentDeliveryAdapter) CancelRelatedEmails(ctx context.Context, relatedType string, relatedID int64, reason string) (int64, error) {
	tenantID, err := a.tenantID(ctx)
	if err != nil {
		return 0, err
	}
	return a.module.Cancel(ctx, tenantID, delivery.TransportEmail, delivery.RelatedEntity{Type: relatedType, ID: relatedID}, reason)
}

func (a durableEmailAdapter) EnqueueEmail(ctx context.Context, input platform.DurableEmail) (platform.DurableEmailResult, error) {
	stored, err := a.module.EnqueueEmail(ctx, delivery.EmailIntent{
		TenantID: input.TenantID, Template: input.Template,
		Recipient: delivery.EmailRecipient{Address: input.Recipient}, Payload: input.Payload,
		IdempotencyKey: input.IdempotencyKey,
		Related:        delivery.RelatedEntity{Type: input.RelatedType, ID: input.RelatedID},
	})
	if err != nil {
		return platform.DurableEmailResult{}, err
	}
	return platform.DurableEmailResult{ID: stored.ID, Duplicate: stored.Duplicate}, nil
}

func (a durableEmailAdapter) CancelEmail(ctx context.Context, tenantID int64, relatedType string, relatedID int64, reason string) (int64, error) {
	return a.module.Cancel(ctx, tenantID, delivery.TransportEmail, delivery.RelatedEntity{Type: relatedType, ID: relatedID}, reason)
}

func (a announcementDeliveryAdapter) ReplaceForEntity(ctx context.Context, tenantID int64, relatedType string, relatedID int64, rows []announcement.EmailDelivery) error {
	values := make([]delivery.EmailDelivery, 0, len(rows))
	for _, row := range rows {
		values = append(values, delivery.EmailDelivery{
			OutboxID: row.OutboxID, GuardianProfileID: row.GuardianProfileID, AccountID: row.AccountID,
			RecipientEmail: row.RecipientEmail, Reachability: row.Reachability,
		})
	}
	return a.module.ReplaceEmailDeliveries(ctx, tenantID, delivery.RelatedEntity{Type: relatedType, ID: relatedID}, values)
}

func (a announcementDeliveryAdapter) DeleteForEntity(ctx context.Context, tenantID int64, relatedType string, relatedID int64) (int64, error) {
	return a.module.DeleteEmailDeliveries(ctx, tenantID, delivery.RelatedEntity{Type: relatedType, ID: relatedID})
}

func (a announcementDeliveryAdapter) ListForEntity(ctx context.Context, tenantID int64, relatedType string, relatedID int64) ([]announcement.EmailDeliveryStatus, error) {
	rows, err := a.module.EmailDeliveryStatuses(ctx, tenantID, delivery.RelatedEntity{Type: relatedType, ID: relatedID})
	if err != nil {
		return nil, err
	}
	result := make([]announcement.EmailDeliveryStatus, 0, len(rows))
	for _, row := range rows {
		result = append(result, announcement.EmailDeliveryStatus{
			DeliveryID: row.DeliveryID, GuardianProfileID: row.GuardianProfileID, AccountID: row.AccountID,
			FirstName: row.FirstName, LastName: row.LastName, RecipientEmail: row.RecipientEmail,
			Reachability: row.Reachability, EmailStatus: row.EmailStatus, LastError: row.LastError,
			SentAt: row.SentAt, Attempts: row.Attempts,
		})
	}
	return result, nil
}

func (a announcementDeliveryAdapter) AttachOutbox(ctx context.Context, tenantID, deliveryID, outboxID int64) error {
	return a.module.AttachEmailOutbox(ctx, tenantID, deliveryID, outboxID)
}

func (a announcementDeliveryAdapter) ClaimFailedDelivery(ctx context.Context, tenantID, deliveryID int64) (bool, error) {
	return a.module.ClaimFailedEmailDelivery(ctx, tenantID, deliveryID)
}

func (p *deliveryProvider) SendEmail(ctx context.Context, intent delivery.ClaimedIntent) (delivery.ProviderResult, error) {
	var payload map[string]any
	if err := json.Unmarshal(intent.EmailPayload, &payload); err != nil {
		return delivery.ProviderResult{}, fmt.Errorf("delivery provider: decode email payload: %w", err)
	}
	row := &platformModels.EmailOutbox{Kind: intent.Template, Payload: payload, Attempts: intent.Attempts}
	row.ID = intent.ID
	row.SetTenantID(intent.TenantID)
	renderer, err := p.registry.Lookup(intent.Template)
	if err != nil {
		return delivery.ProviderResult{}, err
	}
	var message *email.Message
	err = tenant.WithTenantTx(ctx, p.db, intent.TenantID, func(tenantCtx context.Context, _ bun.Tx) error {
		var renderErr error
		message, renderErr = renderer.Render(tenantCtx, row)
		if renderErr == nil {
			p.applyReplyTo(tenantCtx, intent, message)
		}
		return renderErr
	})
	if err != nil {
		if errors.Is(err, platform.ErrRenderCancelled) {
			return delivery.ProviderResult{}, fmt.Errorf("%w: %v", delivery.ErrCancelled, err)
		}
		return delivery.ProviderResult{}, err
	}
	if err := sendRenderedEmail(ctx, p.mailer, intent.EmailRecipient, message); err != nil {
		return delivery.ProviderResult{}, fmt.Errorf("delivery provider: send email: %w", err)
	}
	return delivery.ProviderResult{StatusCode: http.StatusAccepted}, nil
}

func sendRenderedEmail(ctx context.Context, mailer email.Mailer, recipient delivery.EmailRecipient, message *email.Message) error {
	if message == nil {
		return errors.New("renderer returned no email message")
	}
	contextMailer, ok := mailer.(email.ContextMailer)
	if !ok {
		return errors.New("mailer does not support cancellation")
	}
	message.To = email.NewEmail(recipient.Name, recipient.Address)
	return contextMailer.SendContext(ctx, *message)
}

func (p *deliveryProvider) applyReplyTo(ctx context.Context, intent delivery.ClaimedIntent, message *email.Message) {
	if p.mailIdentity == nil || message == nil || message.ReplyTo.Address != "" {
		return
	}
	identity, err := p.mailIdentity.ResolveReplyTo(ctx, intent.TenantID)
	if err != nil {
		p.logger.Warn("delivery: reply-to lookup failed", slog.Int64("intent_id", intent.ID), slog.String("error", err.Error()))
		return
	}
	if !identity.IsZero() {
		message.ReplyTo = email.NewEmail(identity.Name, identity.Address)
	}
}

func (p *deliveryProvider) SendPush(ctx context.Context, intent delivery.ClaimedIntent) (delivery.ProviderResult, error) {
	if !p.vapid.Configured() {
		return delivery.ProviderResult{}, errors.New("delivery provider: web push is not configured")
	}
	payload, err := json.Marshal(intent.PushPayload)
	if err != nil {
		return delivery.ProviderResult{}, fmt.Errorf("delivery provider: encode push payload: %w", err)
	}
	response, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{
		Endpoint: intent.PushRecipient.Endpoint,
		Keys:     webpush.Keys{P256dh: intent.PushRecipient.P256DH, Auth: intent.PushRecipient.Auth},
	}, &webpush.Options{
		Subscriber: p.vapid.Subscriber, VAPIDPublicKey: p.vapid.PublicKey,
		VAPIDPrivateKey: p.vapid.PrivateKey, TTL: 300, HTTPClient: p.client,
	})
	if err != nil {
		return delivery.ProviderResult{}, fmt.Errorf("delivery provider: send push: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return delivery.ProviderResult{StatusCode: response.StatusCode}, fmt.Errorf("delivery provider: push service returned %s", response.Status)
	}
	return delivery.ProviderResult{StatusCode: response.StatusCode}, nil
}

func newDeliveryHTTPClient() *http.Client {
	return &http.Client{
		Timeout:       10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}
