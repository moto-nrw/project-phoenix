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

// relayedContent shows the notifications other API processes relay by ID
// (#5504). It needs neither the sending channel nor another owner: the stored
// copy, the stored recipients and Delivery's recipient projection answer,
// inside the school's transaction, what to show to whom.
type relayedContent struct {
	runtime tenant.UnitOfWork
	db      *bun.DB
	store   deliveryPorts.RealtimeNotificationStore
}

// NewRelayedContent builds the content lookup a relaying hub receives at
// construction.
func NewRelayedContent(runtime tenant.UnitOfWork, db *bun.DB, store deliveryPorts.RealtimeNotificationStore) (realtime.NotificationContent, error) {
	if err := requireTenantRuntime("relayed notification content", runtime, db != nil && store != nil); err != nil {
		return nil, err
	}
	return relayedContent{runtime: runtime, db: db, store: store}.load, nil
}

// load reads the notification in the school's transaction, narrows the
// signal's address to the recipients stored with it, and keeps only those
// that still may see it now: an active member of the school with the role
// of the addressed portal, a guardian also with parent_portal.access to one
// of the children the notification is about.
func (r relayedContent) load(ctx context.Context, schoolID, id int64, address realtime.NotificationAddress) (realtime.Event, realtime.NotificationAddress, bool, error) {
	var event realtime.Event
	var permitted realtime.NotificationAddress
	found := false
	err := tenant.WithTenantTx(tenant.WithUnitOfWork(ctx, r.runtime), r.db, schoolID, func(txCtx context.Context, _ bun.Tx) error {
		stored, ok, err := r.store.LoadRealtimeNotification(txCtx, schoolID, id, time.Now().Add(-domain.RealtimeNotificationLifetime))
		if err != nil || !ok || stored.TenantID != schoolID {
			return err
		}
		if permitted, ok = storedRecipients(stored, address); !ok {
			return nil
		}
		var data relayedNotificationContent
		if err := json.Unmarshal(stored.Content, &data); err != nil {
			return fmt.Errorf("decoding relayed notification: %w", err)
		}
		if permitted, ok, err = r.currentRecipients(txCtx, schoolID, permitted, data.StudentIDs); err != nil || !ok {
			return err
		}
		event, found = realtime.NewEvent(realtime.EventNotification, stored.ActiveGroupID, data.EventData), true
		return nil
	})
	if err != nil {
		return realtime.Event{}, realtime.NotificationAddress{}, false, fmt.Errorf("loading relayed notification for tenant %d: %w", schoolID, err)
	}
	if !found {
		return realtime.Event{}, realtime.NotificationAddress{}, false, nil
	}
	return event, permitted, true, nil
}

// currentRecipients rechecks account-addressed notifications with the
// delivery-time rules of Web Push. Tenant, admin and group notifications
// reach the connected tabs of the school, as in the producing process.
func (r relayedContent) currentRecipients(ctx context.Context, schoolID int64, permitted realtime.NotificationAddress, studentIDs *[]int64) (realtime.NotificationAddress, bool, error) {
	recipient, accountIDs := "", permitted.AccountIDs
	switch permitted.Audience {
	case realtime.AudienceStaffAccounts:
		recipient = domain.RealtimeRecipientStaff
	case realtime.AudienceSchoolAccounts:
		recipient = domain.RealtimeRecipientSchool
	case realtime.AudienceGuardian:
		// A guardian copy names its children; one without them cannot be
		// authorized.
		if studentIDs == nil {
			return permitted, false, nil
		}
		recipient, accountIDs = domain.RealtimeRecipientGuardian, []int64{permitted.GuardianID}
	default:
		return permitted, true, nil
	}
	var children []int64
	if studentIDs != nil {
		children = *studentIDs
	}
	current, err := r.store.CurrentRealtimeRecipients(ctx, schoolID, recipient, accountIDs, children)
	if err != nil {
		return permitted, false, err
	}
	if permitted.Audience == realtime.AudienceGuardian {
		return permitted, slices.Contains(current, permitted.GuardianID), nil
	}
	permitted.AccountIDs = slices.DeleteFunc(slices.Clone(permitted.AccountIDs), func(accountID int64) bool {
		return !slices.Contains(current, accountID)
	})
	return permitted, len(permitted.AccountIDs) > 0, nil
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
