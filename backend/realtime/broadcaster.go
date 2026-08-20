package realtime

// Broadcaster defines the interface for broadcasting events to SSE clients.
// Services use this interface to emit events without depending on the Hub implementation.
type Broadcaster interface {
	// BroadcastToGroup sends an event to all clients subscribed to the given active group ID.
	// This is a fire-and-forget operation - errors are logged but don't affect service execution.
	BroadcastToGroup(tenantID int64, activeGroupID string, event Event) error

	// BroadcastToGroups sends one identical event to the union of subscribers
	// for the supplied topics. A client subscribed to more than one topic is
	// notified once. Callers with topic-specific payloads must keep using
	// BroadcastToGroup separately.
	BroadcastToGroups(tenantID int64, topics []string, event Event) error

	// BroadcastToTenant sends an event to every client whose Client.TenantID
	// matches the supplied tenantID, regardless of group subscriptions.
	// Used for tenant-wide invalidations (e.g. tenant_settings_changed,
	// active_supervision_changed) where a signal must reach all of one
	// school's tabs but NOT the rest of the platform's connected clients.
	// Fire-and-forget.
	BroadcastToTenant(tenantID int64, event Event) error

	// BroadcastToTenantAdmins sends an event only to connected staff clients
	// whose effective permissions grant admin scope.
	// Fire-and-forget.
	BroadcastToTenantAdmins(tenantID int64, event Event) error

	// BroadcastToStaffAccounts wakes only the addressed staff accounts' clients
	// within one tenant. This is the transport for personal notifications,
	// where each recipient gets their own payload. Fire-and-forget.
	BroadcastToStaffAccounts(tenantID int64, accountIDs []int64, event Event) error

	// BroadcastToAll sends an event to every connected client regardless of group subscriptions.
	// Used for global dashboard count refreshes. Fire-and-forget.
	BroadcastToAll(event Event) error

	// BroadcastParentMessage routes a parent-OGS messaging trigger precisely:
	// the addressed guardian's portal client (matched on account id) receives
	// it, while staff clients in the tenant receive it so their access-filtered
	// inbox refreshes. This keeps one family's messages from waking another
	// family's app. Fire-and-forget.
	BroadcastParentMessage(tenantID, guardianAccountID int64, event Event) error

	// BroadcastToGuardian wakes ONLY the addressed guardian's own portal clients
	// — no staff copy. Used for a message-INDEPENDENT guardian invalidation
	// (parent_child_updated) that staff must never receive; fanning it through
	// BroadcastParentMessage once per guardian would flood staff channels with
	// redundant copies. Fire-and-forget.
	BroadcastToGuardian(tenantID, guardianAccountID int64, event Event) error
}
