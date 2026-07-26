package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
	"golang.org/x/sync/semaphore"
)

// pushSender abstracts the actual Web Push HTTP request so tests can fake the
// push service. The production implementation wraps webpush-go.
type pushSender interface {
	Send(ctx context.Context, sub *webpush.Subscription, payload []byte, opts *webpush.Options) (*http.Response, error)
}

type webpushGoSender struct {
	client *http.Client
}

func (s webpushGoSender) Send(ctx context.Context, sub *webpush.Subscription, payload []byte, opts *webpush.Options) (*http.Response, error) {
	opts.HTTPClient = s.client
	return webpush.SendNotificationWithContext(ctx, payload, sub, opts)
}

// webPushPayload is the wire format the service worker receives. GDPR
// contract: ONLY display-safe fields — never Event.Data, never child names.
// Details are loaded authenticated after the user follows the deep link.
type webPushPayload struct {
	Title    string `json:"title"`
	Body     string `json:"body,omitempty"`
	DeepLink string `json:"deepLink,omitempty"`
	Type     string `json:"type,omitempty"`
	Priority string `json:"priority,omitempty"`
}

const (
	// maxPushPayloadBytes is a defensive bound well under the ~4KB Web Push
	// limit; our payloads are a few hundred bytes.
	maxPushPayloadBytes = 3800
	// Keep outbound work bounded without serializing a large tenant's devices.
	maxConcurrentPushSends = 8
	pushSendTimeout        = 10 * time.Second
)

// webPushChannel delivers notifications as Web Push messages (#2003) to the
// devices registered in iot.push_subscriptions. Delivery is fire-and-forget:
// per-subscription failures are logged, expired subscriptions (HTTP 404/410)
// are pruned, and Deliver only returns an error for whole-audience failures.
type webPushChannel struct {
	db     *bun.DB
	repo   iot.PushSubscriptionRepository
	vapid  VAPIDConfig
	sender pushSender
	logger *slog.Logger
	// Shared across deliveries so concurrent notification batches cannot each
	// consume maxConcurrentPushSends outbound connections.
	sendSem *semaphore.Weighted
}

// NewWebPushChannel returns the Web Push channel. With unset VAPID keys the
// channel stays inert (no-op Deliver) so environments without push keep
// today's behavior.
func NewWebPushChannel(db *bun.DB, repo iot.PushSubscriptionRepository, vapid VAPIDConfig, logger *slog.Logger) Channel {
	return &webPushChannel{
		db:      db,
		repo:    repo,
		vapid:   vapid,
		sender:  webpushGoSender{client: newWebPushHTTPClient()},
		logger:  logger,
		sendSem: semaphore.NewWeighted(maxConcurrentPushSends),
	}
}

func (c *webPushChannel) Name() string { return "web_push" }

func newWebPushHTTPClient() *http.Client {
	return &http.Client{
		Timeout: pushSendTimeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func (c *webPushChannel) getLogger() *slog.Logger {
	if c.logger == nil {
		return slog.Default()
	}
	return c.logger
}

func (c *webPushChannel) Deliver(ctx context.Context, event Event) error {
	if !c.vapid.Configured() {
		c.getLogger().Debug("web push channel has no VAPID keys configured, skipping delivery",
			"notification_type", event.Type,
			"tenant_id", event.Audience.TenantID,
		)
		return nil
	}

	payload, err := json.Marshal(webPushPayload{
		Title:    event.Title,
		Body:     event.Body,
		DeepLink: event.DeepLink,
		Type:     event.Type,
		Priority: event.Priority,
	})
	if err != nil {
		return fmt.Errorf("marshaling web push payload: %w", err)
	}
	if len(payload) > maxPushPayloadBytes {
		return fmt.Errorf("web push payload exceeds %d bytes (%d)", maxPushPayloadBytes, len(payload))
	}

	// Read subscriptions under RLS, then commit before any network request.
	// The bounded send batch runs asynchronously so push-service latency never
	// delays the request that produced the notification.
	var subs []*iot.PushSubscription
	err = tenant.WithTenantTx(ctx, c.db, event.Audience.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		resolved, err := c.resolveSubscriptions(txCtx, event.Audience)
		if err != nil {
			return err
		}
		subs = resolved
		return nil
	})
	if err != nil || len(subs) == 0 {
		return err
	}

	dispatchCtx := context.WithoutCancel(ctx)
	go func() {
		c.sendAll(dispatchCtx, event, payload, subs)
	}()
	return nil
}

// resolveSubscriptions maps the audience scope to registered devices.
// ScopeGroup is deliberately unsupported: unlike SSE there is no persisted
// device-to-group membership, and no producer targets groups with
// push-worthy events yet. Documented follow-up in docs/notifications.md.
func (c *webPushChannel) resolveSubscriptions(ctx context.Context, audience Audience) ([]*iot.PushSubscription, error) {
	switch audience.Scope {
	case ScopeTenant:
		return c.repo.FindForTenantStaff(ctx)
	case ScopeAdmin:
		return c.repo.FindForTenantAdmins(ctx)
	case ScopeGuardian:
		return c.repo.FindForGuardians(ctx, guardianAccountIDs(audience))
	case ScopeGroup:
		c.getLogger().Debug("web push does not support group scope, skipping",
			"tenant_id", audience.TenantID,
			"active_group_id", audience.ActiveGroupID,
		)
		return nil, nil
	default:
		return nil, fmt.Errorf("unknown audience scope %q", audience.Scope)
	}
}

// sendAll pushes the payload to every subscription. Per-subscription errors
// never abort the loop; 404/410 responses prune the dead subscription.
func (c *webPushChannel) sendAll(ctx context.Context, event Event, payload []byte, subs []*iot.PushSubscription) {
	ttl, urgency := pushOptionsForPriority(event.Priority)
	var wg sync.WaitGroup

	for _, sub := range subs {
		if err := c.sendSem.Acquire(ctx, 1); err != nil {
			break
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer c.sendSem.Release(1)
			c.sendOne(ctx, event, payload, sub, ttl, urgency)
		}()
	}
	wg.Wait()
}

func (c *webPushChannel) sendOne(
	ctx context.Context,
	event Event,
	payload []byte,
	sub *iot.PushSubscription,
	ttl int,
	urgency webpush.Urgency,
) {
	if err := iot.ValidatePushEndpoint(sub.Endpoint); err != nil {
		c.getLogger().Warn("refusing untrusted web push endpoint",
			"tenant_id", sub.TenantID,
			"subscription_id", sub.ID,
			"error", err.Error(),
		)
		return
	}

	sendCtx, cancel := context.WithTimeout(ctx, pushSendTimeout)
	defer cancel()

	resp, err := c.sender.Send(sendCtx, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys:     webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth},
	}, payload, &webpush.Options{
		Subscriber:      c.vapid.webPushSubscriber(),
		VAPIDPublicKey:  c.vapid.PublicKey,
		VAPIDPrivateKey: c.vapid.PrivateKey,
		TTL:             ttl,
		Urgency:         urgency,
	})
	if err != nil {
		c.getLogger().Warn("web push send failed",
			"notification_type", event.Type,
			"tenant_id", sub.TenantID,
			"subscription_id", sub.ID,
			"error", err.Error(),
		)
		return
	}
	c.handleResponse(ctx, event, sub, resp)
}

func (c *webPushChannel) handleResponse(ctx context.Context, event Event, sub *iot.PushSubscription, resp *http.Response) {
	if resp == nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		// The push service says this subscription no longer exists — prune it.
		deleted, err := c.deleteExpiredSubscription(ctx, sub)
		if err != nil {
			c.getLogger().Warn("failed to prune expired push subscription",
				"subscription_id", sub.ID,
				"tenant_id", sub.TenantID,
				"error", err.Error(),
			)
			return
		}
		if !deleted {
			c.getLogger().Debug("kept refreshed push subscription after stale expiry response",
				"subscription_id", sub.ID,
				"tenant_id", sub.TenantID,
				"status", resp.StatusCode,
			)
			return
		}
		c.getLogger().Info("pruned expired push subscription",
			"subscription_id", sub.ID,
			"tenant_id", sub.TenantID,
			"status", resp.StatusCode,
		)
	default:
		c.getLogger().Warn("web push service rejected notification",
			"notification_type", event.Type,
			"subscription_id", sub.ID,
			"tenant_id", sub.TenantID,
			"status", resp.StatusCode,
		)
	}
}

func (c *webPushChannel) deleteExpiredSubscription(ctx context.Context, sub *iot.PushSubscription) (bool, error) {
	// Unit tests use repository fakes without a database. Production always
	// opens a short tenant transaction so RLS applies to the cleanup.
	if c.db == nil {
		return c.repo.DeleteExpiredIfUnchanged(ctx, sub)
	}
	var deleted bool
	err := tenant.WithTenantTx(ctx, c.db, sub.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		deleted, err = c.repo.DeleteExpiredIfUnchanged(txCtx, sub)
		return err
	})
	return deleted, err
}

// pushOptionsForPriority maps the abstraction's priority to Web Push TTL and
// urgency. High-priority events (something is overdue NOW) expire after an
// hour — delivering them a day later would mislead; normal/low keep a day.
func pushOptionsForPriority(priority string) (ttl int, urgency webpush.Urgency) {
	switch priority {
	case PriorityHigh:
		return 3600, webpush.UrgencyHigh
	case PriorityLow:
		return 86400, webpush.UrgencyLow
	default:
		return 86400, webpush.UrgencyNormal
	}
}
