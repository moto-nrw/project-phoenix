package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/moto-nrw/project-phoenix/models/iot"
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

// webPushChannel delivers notifications as Web Push messages (#2003) to the
// devices registered in iot.push_subscriptions. Delivery is fire-and-forget:
// per-subscription failures are logged, expired subscriptions (HTTP 404/410)
// are pruned, and Deliver only returns an error for whole-audience failures.
type webPushChannel struct {
	db            *bun.DB
	repo          iot.PushSubscriptionRepository
	vapid         VAPIDConfig
	sender        pushSender
	logger        *slog.Logger
	tenantRuntime *tenant.UnitOfWork
	// Shared across deliveries so concurrent notification batches cannot each
	// consume maxConcurrentPushSends outbound connections.
	sendSlots chan struct{}
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
func NewWebPushChannel(db *bun.DB, repo iot.PushSubscriptionRepository, vapid VAPIDConfig, logger *slog.Logger) Channel {
	return &webPushChannel{
		db:        db,
		repo:      repo,
		vapid:     vapid,
		sender:    webpushGoSender{client: newWebPushHTTPClient()},
		logger:    logger,
		sendSlots: make(chan struct{}, maxConcurrentPushSends),
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
	ctx = c.withTenantRuntime(ctx)
	if !c.vapid.Configured() {
		c.getLogger().Debug("web push channel has no VAPID keys configured, skipping delivery",
			"notification_type", event.Type,
			"tenant_id", event.Audience.TenantID,
		)
		return nil
	}

	payload, err := marshalPushPayload(event)
	if err != nil {
		return err
	}

	// Read subscriptions under RLS, then commit before any network request.
	// The bounded send batch runs asynchronously so push-service latency never
	// delays the request that produced the notification.
	var subs []*iot.PushSubscription
	err = tenant.WithTenantTx(ctx, c.db, event.Audience.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		resolved, err := c.resolveEventSubscriptions(txCtx, event)
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
	dispatchCtx = tenant.ContextWithoutTransaction(dispatchCtx)
	dispatchCtx = tenant.ContextWithoutAfterCommitHooks(dispatchCtx)
	go func() {
		c.sendAll(dispatchCtx, event, payload, subs)
	}()
	return nil
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
	var subs []*iot.PushSubscription
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

// DeliverBatch resolves the devices of every recipient in ONE transaction and
// then sends each recipient their own payload.
//
// Looping Deliver would open one tenant transaction per event, each with its
// own SET LOCAL ROLE round trip. With a per-minute tick addressing every staff
// member of a school that is the difference between a handful of transactions
// and one per person.
//
// Only staff-scoped events are grouped; anything else falls back to Deliver,
// because the other scopes resolve their devices from the scope alone and gain
// nothing from batching.
func (c *webPushChannel) DeliverBatch(ctx context.Context, events []Event) error {
	ctx = c.withTenantRuntime(ctx)
	if !c.vapid.Configured() || len(events) == 0 {
		return nil
	}

	staffEvents := make([]Event, 0, len(events))
	for _, event := range events {
		if event.Audience.Scope == ScopeStaff {
			staffEvents = append(staffEvents, event)
			continue
		}
		if err := c.Deliver(ctx, event); err != nil {
			c.getLogger().Error("web push delivery failed inside batch",
				"notification_type", event.Type,
				"tenant_id", event.Audience.TenantID,
				"error", err.Error(),
			)
		}
	}
	if len(staffEvents) == 0 {
		return nil
	}

	tenantID := staffEvents[0].Audience.TenantID
	recipientSet := make(map[int64]struct{})
	for _, event := range staffEvents {
		for _, accountID := range event.Audience.StaffAccountIDs {
			recipientSet[accountID] = struct{}{}
		}
	}
	recipients := make([]int64, 0, len(recipientSet))
	for accountID := range recipientSet {
		recipients = append(recipients, accountID)
	}

	// Read under RLS, then commit before any network request — the same
	// ordering Deliver keeps, for the same reason.
	var staffSubs, schoolSubs []*iot.PushSubscription
	err := tenant.WithTenantTx(ctx, c.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		if slices.ContainsFunc(staffEvents, deliversToStaffPortal) {
			resolved, err := c.repo.FindForStaffAccounts(txCtx, recipients)
			if err != nil {
				return err
			}
			staffSubs = resolved
		}
		if slices.ContainsFunc(staffEvents, deliversToSchoolPortal) {
			resolved, err := c.repo.FindForSchoolAccounts(txCtx, recipients)
			if err != nil {
				return err
			}
			schoolSubs = resolved
		}
		return nil
	})
	if err != nil || (len(staffSubs) == 0 && len(schoolSubs) == 0) {
		return err
	}

	staffByAccount := make(map[int64][]*iot.PushSubscription, len(recipients))
	for _, sub := range staffSubs {
		staffByAccount[sub.AccountID] = append(staffByAccount[sub.AccountID], sub)
	}
	schoolByAccount := make(map[int64][]*iot.PushSubscription, len(recipients))
	for _, sub := range schoolSubs {
		schoolByAccount[sub.AccountID] = append(schoolByAccount[sub.AccountID], sub)
	}

	dispatchCtx := context.WithoutCancel(ctx)
	dispatchCtx = tenant.ContextWithoutTransaction(dispatchCtx)
	dispatchCtx = tenant.ContextWithoutAfterCommitHooks(dispatchCtx)
	for _, event := range staffEvents {
		targets := make([]*iot.PushSubscription, 0, len(event.Audience.StaffAccountIDs))
		toStaff, toSchool := deliversToStaffPortal(event), deliversToSchoolPortal(event)
		for _, accountID := range event.Audience.StaffAccountIDs {
			if toStaff {
				targets = append(targets, staffByAccount[accountID]...)
			}
			if toSchool {
				targets = append(targets, schoolByAccount[accountID]...)
			}
		}
		targets = dedupeSubscriptionsByEndpoint(targets)
		if len(targets) == 0 {
			continue
		}
		payload, marshalErr := marshalPushPayload(event)
		if marshalErr != nil {
			c.getLogger().Error("skipping web push event with unusable payload",
				"notification_type", event.Type,
				"tenant_id", event.Audience.TenantID,
				"error", marshalErr.Error(),
			)
			continue
		}
		go func(event Event, payload []byte, targets []*iot.PushSubscription) {
			c.sendAll(dispatchCtx, event, payload, targets)
		}(event, payload, targets)
	}
	return nil
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

func (p *portalPayloads) forSubscription(sub *iot.PushSubscription) ([]byte, error) {
	if sub.Portal != iot.PushPortalSchool {
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
func (c *webPushChannel) resolveEventSubscriptions(ctx context.Context, event Event) ([]*iot.PushSubscription, error) {
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
		var subs []*iot.PushSubscription
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
func dedupeSubscriptionsByEndpoint(subs []*iot.PushSubscription) []*iot.PushSubscription {
	position := make(map[string]int, len(subs))
	deduped := make([]*iot.PushSubscription, 0, len(subs))
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
func newerRegistration(candidate, incumbent *iot.PushSubscription) bool {
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

// sendAll pushes the payload to every subscription. Per-subscription errors
// never abort the loop; 404/410 responses prune the dead subscription.
func (c *webPushChannel) sendAll(ctx context.Context, event Event, payload []byte, subs []*iot.PushSubscription) {
	ttl, urgency := pushOptionsForPriority(event.Priority)
	payloads := &portalPayloads{event: event, base: payload}
	var wg sync.WaitGroup

	for _, sub := range subs {
		wire, err := payloads.forSubscription(sub)
		if err != nil {
			c.getLogger().Error("skipping web push with unusable school payload",
				"notification_type", event.Type,
				"tenant_id", event.Audience.TenantID,
				"error", err.Error(),
			)
			continue
		}
		select {
		case c.sendSlots <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-c.sendSlots }()
			c.sendOne(ctx, event, wire, sub, ttl, urgency)
		}()
	}
	wg.Wait()
}

func (c *webPushChannel) sendAllSynchronously(ctx context.Context, event Event, payload []byte, subs []*iot.PushSubscription) error {
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
		go func(sub *iot.PushSubscription) {
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

func (c *webPushChannel) sendOneSynchronously(ctx context.Context, event Event, payload []byte, sub *iot.PushSubscription, ttl int, urgency webpush.Urgency) error {
	if err := iot.ValidatePushEndpoint(sub.Endpoint); err != nil {
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
