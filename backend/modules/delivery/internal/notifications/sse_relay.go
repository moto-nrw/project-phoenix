package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	deliveryPorts "github.com/moto-nrw/project-phoenix/modules/delivery/internal/ports"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Notifications cross API processes by ID (#5504). The PostgreSQL relay is
// not authenticated, so the producing process stores the SSE payload with
// its recipients and relays only the row's ID with school and address. The
// receiving API loads the row in that school's transaction and shows it only
// to recipients both the signal and the row name. A forged, foreign, expired
// or differently addressed ID shows nothing.

// Optional relay persistence must not hold up local fan-out on a busy pool or
// locked insert. The caller's earlier deadline still takes precedence.
const notificationStoreTimeout = 250 * time.Millisecond

// relayedNotificationContent keeps authorization metadata only in the stored
// JSON, never in the signal or browser payload. A missing child scope denotes
// a legacy row, which cannot safely authorize a guardian notification.
type relayedNotificationContent struct {
	realtime.EventData
	StudentIDs *[]int64 `json:"relay_student_ids,omitempty"`
}

// relayingHub is the process hub that relays signals to other API processes
// and shows the notifications they relay.
type relayingHub interface {
	RelayNotifications(realtime.NotificationContent) bool
}

// withRelayedContent makes the channel store notifications for other API
// processes and lets the hub load theirs. A hub that does not relay keeps
// every notification in its process, as before.
func withRelayedContent(store deliveryPorts.RealtimeNotificationStore) SSEChannelOption {
	return func(c *sseChannel) {
		hub, ok := c.broadcaster.(relayingHub)
		if store == nil || !ok {
			return
		}
		c.relayStore = store
		if !hub.RelayNotifications(c.loadRelayed) {
			c.relayStore = nil
		}
	}
}

// relayable stores the notification so other API processes can load it, and
// marks the event with the stored ID. Without a stored copy the notification
// stays in this process; local delivery never depends on the relay.
func (c *sseChannel) relayable(ctx context.Context, event realtime.Event, stored domain.RealtimeNotification, studentIDs []int64) realtime.Event {
	if c.relayStore == nil {
		return event
	}
	ctx, cancel := context.WithTimeout(ctx, notificationStoreTimeout)
	defer cancel()
	payload := relayedNotificationContent{EventData: event.Data}
	if stored.Scope == domain.RealtimeScopeGuardian {
		if studentIDs == nil {
			studentIDs = []int64{}
		}
		payload.StudentIDs = &studentIDs
	}
	content, err := json.Marshal(payload)
	if err == nil {
		stored.Content = content
		stored.ActiveGroupID = event.ActiveGroupID
		var id int64
		err = tenant.WithTenantTx(ctx, c.db, stored.TenantID, func(txCtx context.Context, _ bun.Tx) error {
			var saveErr error
			id, saveErr = c.relayStore.SaveRealtimeNotification(txCtx, stored)
			return saveErr
		})
		if err == nil {
			return event.WithNotificationID(id)
		}
	}
	c.getLogger().Warn("notification not stored for other API processes; delivered in this process only",
		"tenant_id", stored.TenantID,
		"scope", stored.Scope,
		"error", err,
	)
	return event
}

// loadRelayed is the hub's NotificationContent: it reads the notification in
// the school's transaction and narrows the signal's address to the
// recipients stored with it.
func (c *sseChannel) loadRelayed(ctx context.Context, schoolID, id int64, address realtime.NotificationAddress) (realtime.Event, realtime.NotificationAddress, bool, error) {
	ctx = c.withTenantRuntime(ctx)
	var stored domain.RealtimeNotification
	found := false
	err := tenant.WithTenantTx(ctx, c.db, schoolID, func(txCtx context.Context, _ bun.Tx) error {
		var loadErr error
		stored, found, loadErr = c.relayStore.LoadRealtimeNotification(txCtx, schoolID, id, time.Now().Add(-domain.RealtimeNotificationLifetime))
		return loadErr
	})
	if err != nil {
		return realtime.Event{}, realtime.NotificationAddress{}, false, fmt.Errorf("loading relayed notification for tenant %d: %w", schoolID, err)
	}
	if !found || stored.TenantID != schoolID {
		return realtime.Event{}, realtime.NotificationAddress{}, false, nil
	}
	permitted, ok := storedRecipients(stored, address)
	if !ok {
		return realtime.Event{}, realtime.NotificationAddress{}, false, nil
	}
	var data relayedNotificationContent
	if err := json.Unmarshal(stored.Content, &data); err != nil {
		return realtime.Event{}, realtime.NotificationAddress{}, false, fmt.Errorf("decoding relayed notification for tenant %d: %w", schoolID, err)
	}
	if permitted.Audience == realtime.AudienceGuardian {
		if data.StudentIDs == nil {
			return realtime.Event{}, realtime.NotificationAddress{}, false, nil
		}
		recipients, err := c.permittedGuardians(ctx, Audience{
			TenantID: schoolID, Scope: ScopeGuardian,
			GuardianAccountIDs: []int64{permitted.GuardianID}, StudentIDs: *data.StudentIDs,
		})
		if err != nil || !slices.Contains(recipients, permitted.GuardianID) {
			return realtime.Event{}, realtime.NotificationAddress{}, false, err
		}
	}
	return realtime.NewEvent(realtime.EventNotification, stored.ActiveGroupID, data.EventData), permitted, true, nil
}

// storedRecipients keeps the part of the signal's address the stored
// notification names: the same audience and group, and only the accounts
// or the guardian it was resolved for.
func storedRecipients(stored domain.RealtimeNotification, address realtime.NotificationAddress) (realtime.NotificationAddress, bool) {
	permitted := realtime.NotificationAddress{Audience: address.Audience}
	switch address.Audience {
	case realtime.AudienceTenant:
		return permitted, stored.Scope == domain.RealtimeScopeTenant
	case realtime.AudienceAdmins:
		return permitted, stored.Scope == domain.RealtimeScopeAdmin
	case realtime.AudienceGroup:
		permitted.Topic = address.Topic
		return permitted, stored.Scope == domain.RealtimeScopeGroup && address.Topic != "" &&
			address.Topic == realtime.NormalizeTopic(stored.ActiveGroupID)
	case realtime.AudienceStaffAccounts, realtime.AudienceSchoolAccounts:
		portal := stored.StaffPortal
		if address.Audience == realtime.AudienceSchoolAccounts {
			portal = stored.SchoolPortal
		}
		if stored.Scope != domain.RealtimeScopeStaff || !portal {
			return permitted, false
		}
		for _, accountID := range address.AccountIDs {
			if slices.Contains(stored.AccountIDs, accountID) {
				permitted.AccountIDs = append(permitted.AccountIDs, accountID)
			}
		}
		return permitted, len(permitted.AccountIDs) > 0
	case realtime.AudienceGuardian:
		permitted.GuardianID = address.GuardianID
		return permitted, stored.Scope == domain.RealtimeScopeGuardian && address.GuardianID > 0 &&
			slices.Contains(stored.AccountIDs, address.GuardianID)
	default:
		return permitted, false
	}
}
