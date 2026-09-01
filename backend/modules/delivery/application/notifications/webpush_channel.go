package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	deliveryModels "github.com/moto-nrw/project-phoenix/models/delivery"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
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

// ErrNoWebPushSubscribers reports that no push service accepted a durable
// notification because VAPID is unavailable or the audience has no devices.
var ErrNoWebPushSubscribers = errors.New("no web push subscribers are available")

// webPushChannel snapshots notifications into the durable Delivery outbox.
// DeliverSynchronously is the explicit fail-closed exception used by producers
// that must know whether a push service accepted the message before returning.
type webPushChannel struct {
	db            *bun.DB
	repo          deliveryModels.PushSubscriptionRepository
	vapid         VAPIDConfig
	sender        pushSender
	logger        *slog.Logger
	tenantRuntime *tenant.UnitOfWork
	// Shared across deliveries so concurrent notification batches cannot each
	// consume maxConcurrentPushSends outbound connections.
	sendSlots chan struct{}
	outbox    PushOutbox
}

type PushIntent struct {
	TenantID       int64
	Template       string
	IdempotencyKey string
	RelatedType    string
	RelatedID      int64
	SubscriptionID int64
	AccountID      int64
	Endpoint       string
	P256DH         string
	Auth           string
	Portal         string
	UpdatedAt      time.Time
	AudienceScope  AudienceScope
	StudentIDs     []int64
	Title          string
	Body           string
	DeepLink       string
	Type           string
	Priority       string
}

type PushOutbox interface {
	EnqueuePush(context.Context, PushIntent) (bool, error)
}

func (c *webPushChannel) SetTenantRuntime(runtime tenant.UnitOfWork) {
	c.tenantRuntime = &runtime
}

func (c *webPushChannel) withTenantRuntime(ctx context.Context) context.Context {
	if c.tenantRuntime == nil {
		return ctx
	}
	return tenant.WithUnitOfWork(ctx, *c.tenantRuntime)
}

// NewWebPushChannel returns the Web Push channel. With unset VAPID keys the
// channel stays inert (no-op Deliver) so environments without push keep
// today's behavior.
func NewWebPushChannel(db *bun.DB, repo deliveryModels.PushSubscriptionRepository, vapid VAPIDConfig, logger *slog.Logger) Channel {
	return &webPushChannel{
		db:        db,
		repo:      repo,
		vapid:     vapid,
		sender:    webpushGoSender{client: newWebPushHTTPClient()},
		logger:    logger,
		sendSlots: make(chan struct{}, maxConcurrentPushSends),
	}
}

func NewDurableWebPushChannel(db *bun.DB, repo deliveryModels.PushSubscriptionRepository, vapid VAPIDConfig, outbox PushOutbox, logger *slog.Logger) Channel {
	channel := NewWebPushChannel(db, repo, vapid, logger).(*webPushChannel)
	channel.outbox = outbox
	return channel
}

func (c *webPushChannel) durableChannel() {}

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
	ctx = c.withTenantRuntime(ctx)
	if !c.vapid.Configured() {
		c.getLogger().Debug("web push channel has no VAPID keys configured, skipping delivery",
			"notification_type", event.Type,
			"tenant_id", event.Audience.TenantID,
		)
		return nil
	}
	if _, err := marshalPushPayload(event); err != nil {
		return err
	}

	if c.outbox == nil {
		return errors.New("web push durable outbox is not configured")
	}
	if event.IdempotencyKey == "" {
		return errors.New("web push event requires an idempotency key")
	}
	enqueue := func(txCtx context.Context) error {
		subs, err := c.resolveEventSubscriptions(txCtx, event)
		if err != nil {
			return err
		}
		for _, sub := range subs {
			deepLink := event.DeepLink
			if sub.Portal == deliveryModels.PushPortalSchool {
				if _, err := schoolPushPayload(event); err != nil {
					return err
				}
				deepLink = event.SchoolDeepLink
				if deepLink == "" {
					deepLink = "/school"
				}
			}
			_, err := c.outbox.EnqueuePush(txCtx, PushIntent{
				TenantID: event.Audience.TenantID, Template: event.Type,
				IdempotencyKey: fmt.Sprintf("%s:subscription:%d", event.IdempotencyKey, sub.ID),
				RelatedType:    event.RelatedType, RelatedID: event.RelatedID,
				SubscriptionID: sub.ID, AccountID: sub.AccountID, Endpoint: sub.Endpoint, P256DH: sub.P256dh, Auth: sub.Auth, Portal: sub.Portal, UpdatedAt: sub.UpdatedAt,
				AudienceScope: event.Audience.Scope, StudentIDs: append([]int64(nil), event.Audience.StudentIDs...),
				Title: event.Title, Body: event.Body, DeepLink: deepLink, Type: event.Type, Priority: event.Priority,
			})
			if err != nil {
				return err
			}
		}
		return nil
	}
	if _, active := tenant.TransactionFromContext(ctx); active {
		return enqueue(ctx)
	}
	return tenant.WithTenantTx(ctx, c.db, event.Audience.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		return enqueue(txCtx)
	})
}

// DeliverSynchronously waits for the push service to accept every current
// subscription. It is reserved for durable producers that can retry a failed
// attempt; normal notifications continue to use Deliver's async path.
func (c *webPushChannel) DeliverSynchronously(ctx context.Context, event Event) error {
	ctx = c.withTenantRuntime(ctx)
	if !c.vapid.Configured() {
		return ErrNoWebPushSubscribers
	}
	payload, err := marshalPushPayload(event)
	if err != nil {
		return err
	}
	var subs []*deliveryModels.PushSubscription
	err = tenant.WithTenantTx(ctx, c.db, event.Audience.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		var resolveErr error
		subs, resolveErr = c.resolveEventSubscriptions(txCtx, event)
		return resolveErr
	})
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		return ErrNoWebPushSubscribers
	}
	return c.sendAllSynchronously(ctx, event, payload, subs)
}

// marshalPushPayload renders one event into the wire payload and enforces the
// size bound. Shared by the single and batched paths so the GDPR-safe field
// set and the cap cannot drift apart.
func marshalPushPayload(event Event) ([]byte, error) {
	payload, err := json.Marshal(webPushPayload{
		Title:    event.Title,
		Body:     event.Body,
		DeepLink: event.DeepLink,
		Type:     event.Type,
		Priority: event.Priority,
	})
	if err != nil {
		return nil, fmt.Errorf("marshaling web push payload: %w", err)
	}
	if len(payload) > maxPushPayloadBytes {
		return nil, fmt.Errorf("web push payload exceeds %d bytes (%d)", maxPushPayloadBytes, len(payload))
	}
	return payload, nil
}

// schoolPushPayload renders the school-portal variant of the payload: the
// same display-safe fields with the school host's deep link. A notification
// without a place in moto schule opens the portal root.
func schoolPushPayload(event Event) ([]byte, error) {
	school := event
	school.DeepLink = event.SchoolDeepLink
	if school.DeepLink == "" {
		school.DeepLink = "/school"
	}
	return marshalPushPayload(school)
}

// payloadForSubscription picks the portal-specific wire payload. Staff and
// parent devices get the payload as marshalled; school devices (#2208) get
// the school variant, rendered once per send batch.
type portalPayloads struct {
	event  Event
	base   []byte
	school []byte
	err    error
	once   sync.Once
}

func (p *portalPayloads) forSubscription(sub *deliveryModels.PushSubscription) ([]byte, error) {
	if sub.Portal != deliveryModels.PushPortalSchool {
		return p.base, nil
	}
	p.once.Do(func() {
		p.school, p.err = schoolPushPayload(p.event)
	})
	return p.school, p.err
}

// resolveSubscriptions maps the audience scope to registered devices.
// ScopeGroup is deliberately unsupported: unlike SSE there is no persisted
// device-to-group membership, and no producer targets groups with
// push-worthy events yet. Documented follow-up in docs/notifications.md.
func (c *webPushChannel) resolveEventSubscriptions(ctx context.Context, event Event) ([]*deliveryModels.PushSubscription, error) {
	audience := event.Audience
	switch audience.Scope {
	case ScopeTenant:
		return c.repo.FindForTenantStaff(ctx)
	case ScopeAdmin:
		return c.repo.FindForTenantAdmins(ctx)
	case ScopeGuardian:
		// Audience.StudentIDs (when set) is re-checked here for the same reason
		// ScopeStaff re-checks eligibility below: the recipient list was decided
		// in an earlier transaction, and a guardian's access to those children can
		// be revoked in between.
		return c.repo.FindForGuardians(ctx, guardianAccountIDs(audience), audience.StudentIDs)
	case ScopeStaff:
		// Eligibility is re-checked here rather than trusted from the recipient
		// list: that list was assembled in an earlier transaction, and an
		// account can be deactivated or unmapped from the school in between.
		var subs []*deliveryModels.PushSubscription
		if deliversToStaffPortal(event) {
			staffSubs, err := c.repo.FindForStaffAccounts(ctx, staffAccountIDs(audience))
			if err != nil {
				return nil, err
			}
			subs = staffSubs
		}
		if !deliversToSchoolPortal(event) {
			return subs, nil
		}
		schoolSubs, err := c.repo.FindForSchoolAccounts(ctx, staffAccountIDs(audience))
		if err != nil {
			return nil, err
		}
		return dedupeSubscriptionsByEndpoint(append(subs, schoolSubs...)), nil
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

// dedupeSubscriptionsByEndpoint keeps one subscription per push endpoint.
// The same browser can be registered in the OGS portal and in moto schule
// (#2208): the rows differ by portal, but the endpoint is the device the push
// service delivers to, so sending both would show one person the same message
// twice on one device.
//
// A push endpoint belongs to exactly one service worker on one origin, so at
// most one of the two rows is the device's current registration and the other
// one is a leftover from the portal it was last registered in. The most
// recently written row wins (Upsert stamps updated_at on every re-subscribe):
// that is the portal the device actually opens, and therefore the row whose
// payload carries a deep link that resolves there. Preferring the OGS row
// unconditionally would hand a school device a /team-chat/... link, which is
// not a route in moto schule.
//
// Deduplication stops at the endpoint on purpose. A person registered in both
// staff portals on one browser holds two service-worker registrations, one per
// origin, and nothing the two share identifies the machine they run on: a push
// endpoint is per origin, and so is any identifier the client could generate
// and store. The User-Agent is not a substitute — two phones of the same model
// on the same OS send the same string, so collapsing by it would silently drop
// one of two real devices. A missing notification is the worse failure, so a
// dual-role account that keeps both portals subscribed is notified in both and
// can switch the notification off in the portal it does not use.
func dedupeSubscriptionsByEndpoint(subs []*deliveryModels.PushSubscription) []*deliveryModels.PushSubscription {
	position := make(map[string]int, len(subs))
	deduped := make([]*deliveryModels.PushSubscription, 0, len(subs))
	for _, sub := range subs {
		if sub == nil {
			continue
		}
		index, duplicate := position[sub.Endpoint]
		if !duplicate {
			position[sub.Endpoint] = len(deduped)
			deduped = append(deduped, sub)
			continue
		}
		if newerRegistration(sub, deduped[index]) {
			deduped[index] = sub
		}
	}
	return deduped
}

// newerRegistration orders two rows of one endpoint by when they were last
// written. Rows stamped in the same transaction fall back to the row id, so
// the surviving portal is stable across deliveries instead of following map
// iteration order.
func newerRegistration(candidate, incumbent *deliveryModels.PushSubscription) bool {
	if candidate.UpdatedAt.After(incumbent.UpdatedAt) {
		return true
	}
	if incumbent.UpdatedAt.After(candidate.UpdatedAt) {
		return false
	}
	return candidate.ID > incumbent.ID
}

// deliversToStaffPortal and deliversToSchoolPortal decide which of the two
// staff-side portals an event reaches. For a catalogue type the catalogue
// decides: every staff type reaches the OGS portal, and the school portal is
// added for the types it offers.
//
// TypeTest is the exception. It carries no catalogue entry and exists to prove
// one thing: that the portal the person is looking at receives notifications.
// Fanning it out to both portals would answer a question nobody asked — a
// Lehrkraft pressing "Testbenachrichtigung" in moto schule would also light up
// her OGS devices, and an OGS admin would push a test into a portal she may
// not even use. So the test stays in the portal that requested it
// (Event.Portal, empty meaning the OGS portal).
func deliversToStaffPortal(event Event) bool {
	if event.Type == TypeTest {
		return event.Portal != PortalSchool
	}
	return true
}

func deliversToSchoolPortal(event Event) bool {
	if event.Type == TypeTest {
		return event.Portal == PortalSchool
	}
	def, ok := GetType(event.Type)
	return ok && OfferedInPortal(def, PortalSchool)
}

func (c *webPushChannel) sendAllSynchronously(ctx context.Context, event Event, payload []byte, subs []*deliveryModels.PushSubscription) error {
	ttl, urgency := pushOptionsForPriority(event.Priority)
	payloads := &portalPayloads{event: event, base: payload}
	var wg sync.WaitGroup
	var resultsMu sync.Mutex
	var errs []error
	succeeded := false
	for _, sub := range subs {
		wire, wireErr := payloads.forSubscription(sub)
		if wireErr != nil {
			resultsMu.Lock()
			errs = append(errs, wireErr)
			resultsMu.Unlock()
			continue
		}
		select {
		case c.sendSlots <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			if succeeded {
				return nil
			}
			if len(errs) > 0 {
				return errors.Join(errs...)
			}
			return ctx.Err()
		}

		wg.Add(1)
		go func(sub *deliveryModels.PushSubscription) {
			defer wg.Done()
			defer func() { <-c.sendSlots }()
			if err := c.sendOneSynchronously(ctx, event, wire, sub, ttl, urgency); err != nil {
				resultsMu.Lock()
				errs = append(errs, err)
				resultsMu.Unlock()
				return
			}
			resultsMu.Lock()
			succeeded = true
			resultsMu.Unlock()
		}(sub)
	}
	wg.Wait()
	if succeeded {
		// Claims are per guardian rather than per browser subscription. Once one
		// of the guardian's devices accepts the reminder, retrying failures would
		// duplicate it on that successful device.
		return nil
	}
	if len(errs) == 0 {
		return ctx.Err()
	}
	return errors.Join(errs...)
}

func (c *webPushChannel) sendOneSynchronously(ctx context.Context, event Event, payload []byte, sub *deliveryModels.PushSubscription, ttl int, urgency webpush.Urgency) error {
	if err := deliveryModels.ValidatePushEndpoint(sub.Endpoint); err != nil {
		return err
	}
	sendCtx, cancel := context.WithTimeout(ctx, pushSendTimeout)
	defer cancel()
	resp, err := c.sender.Send(sendCtx, &webpush.Subscription{Endpoint: sub.Endpoint, Keys: webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth}}, payload, &webpush.Options{Subscriber: c.vapid.webPushSubscriber(), VAPIDPublicKey: c.vapid.PublicKey, VAPIDPrivateKey: c.vapid.PrivateKey, TTL: ttl, Urgency: urgency})
	if err != nil {
		return err
	}
	if resp == nil {
		return errors.New("web push service returned no response")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone {
		if _, err := c.deleteExpiredSubscription(ctx, sub); err != nil {
			return err
		}
		return fmt.Errorf("web push subscription expired with status %d", resp.StatusCode)
	}
	return fmt.Errorf("web push service rejected notification with status %d", resp.StatusCode)
}

func (c *webPushChannel) deleteExpiredSubscription(ctx context.Context, sub *deliveryModels.PushSubscription) (bool, error) {
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
