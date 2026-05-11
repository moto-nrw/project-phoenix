package realtime

// Broadcaster defines the interface for broadcasting events to SSE clients.
// Services use this interface to emit events without depending on the Hub implementation.
type Broadcaster interface {
	// BroadcastToGroup sends an event to all clients subscribed to the given active group ID.
	// This is a fire-and-forget operation - errors are logged but don't affect service execution.
	BroadcastToGroup(tenantID int64, activeGroupID string, event Event) error

	// BroadcastToTenant sends an event to every client whose Client.TenantID
	// matches the supplied tenantID, regardless of group subscriptions.
	// Used for tenant-wide invalidations (e.g. tenant_settings_changed)
	// where a setting flip on one school must reach that school's tabs but
	// NOT the rest of the platform's connected clients. Fire-and-forget.
	BroadcastToTenant(tenantID int64, event Event) error

	// BroadcastToAll sends an event to every connected client regardless of group subscriptions.
	// Used for global dashboard count refreshes. Fire-and-forget.
	BroadcastToAll(event Event) error

	// BroadcastToTenant sends an event to every connected client for one tenant,
	// regardless of group subscriptions. Used for tenant-wide refresh signals.
	BroadcastToTenant(tenantID int64, event Event) error
}
