package realtime

// Broadcaster defines the interface for broadcasting events to SSE clients.
// Services use this interface to emit events without depending on the Hub implementation.
type Broadcaster interface {
	// BroadcastToGroup sends an event to all clients subscribed to the given active group ID.
	// This is a fire-and-forget operation - errors are logged but don't affect service execution.
	BroadcastToGroup(activeGroupID string, event Event) error
}
