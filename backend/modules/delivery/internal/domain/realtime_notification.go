package domain

import (
	"encoding/json"
	"time"
)

// RealtimeNotificationLifetime is how long another API process may load a
// stored in-app notification (#5504). The relay is lossy and immediate; a
// signal that arrives later is stale, and a replayed one must show nothing.
const RealtimeNotificationLifetime = 2 * time.Minute

// The audiences of a stored notification, the scopes of the notification
// router.
const (
	RealtimeScopeTenant   = "tenant"
	RealtimeScopeAdmin    = "admin"
	RealtimeScopeGroup    = "group"
	RealtimeScopeStaff    = "staff"
	RealtimeScopeGuardian = "guardian"
)

// The recipients a relayed notification is rechecked for when it is shown:
// staff and school-portal accounts and guardians, with the delivery-time rules
// of Web Push.
const (
	RealtimeRecipientStaff    = "staff"
	RealtimeRecipientSchool   = "school"
	RealtimeRecipientGuardian = "guardian"
)

// RealtimeNotification is the stored copy of one in-app notification: its
// school, the recipients the producing process resolved, and the
// display-safe SSE payload.
type RealtimeNotification struct {
	ID            int64
	TenantID      int64
	Scope         string
	ActiveGroupID string
	// AccountIDs are the addressed staff accounts or the guardian accounts
	// that passed the child-access check.
	AccountIDs   []int64
	StaffPortal  bool
	SchoolPortal bool
	Content      json.RawMessage
}
