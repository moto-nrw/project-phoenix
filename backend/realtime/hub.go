package realtime

import (
	"log/slog"
	"sync"
)

// Client represents a single SSE client connection
type Client struct {
	Channel          chan Event      // Channel to send events to this client
	UserID           int64           // User ID for audit logging
	SubscribedGroups map[string]bool // active_group_id -> subscribed
}

// Hub manages SSE client connections and broadcasts events
type Hub struct {
	clients      map[*Client]bool
	groupClients map[string][]*Client // active_group_id -> subscribers
	mu           sync.RWMutex
	logger       *slog.Logger
}

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (h *Hub) getLogger() *slog.Logger {
	if h.logger != nil {
		return h.logger
	}
	return slog.Default()
}

// NewHub creates a new SSE hub
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients:      make(map[*Client]bool),
		groupClients: make(map[string][]*Client),
		logger:       logger,
	}
}

// Register adds a client to the hub and subscribes them to specified active groups
func (h *Hub) Register(client *Client, activeGroupIDs []string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client] = true

	// Subscribe client to each active group
	for _, groupID := range activeGroupIDs {
		h.groupClients[groupID] = append(h.groupClients[groupID], client)
		client.SubscribedGroups[groupID] = true
	}

	h.getLogger().Info("SSE client connected",
		slog.Int64("user_id", client.UserID),
		slog.Any("subscribed_groups", activeGroupIDs),
		slog.Int("total_clients", len(h.clients)),
	)
}

// Unregister removes a client from the hub and all group subscriptions
func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.clients[client] {
		return // Client not registered
	}

	delete(h.clients, client)

	// Remove from all group subscriptions
	for groupID := range client.SubscribedGroups {
		clients := h.groupClients[groupID]
		for i, c := range clients {
			if c == client {
				// Remove this client from the group's subscriber list
				h.groupClients[groupID] = append(clients[:i], clients[i+1:]...)
				break
			}
		}

		// Clean up empty group lists
		if len(h.groupClients[groupID]) == 0 {
			delete(h.groupClients, groupID)
		}
	}

	close(client.Channel)

	h.getLogger().Info("SSE client disconnected",
		slog.Int64("user_id", client.UserID),
		slog.Int("total_clients", len(h.clients)),
	)
}

// BroadcastToGroup sends an event to all clients subscribed to the specified active group
// This is a fire-and-forget operation - errors don't affect service execution
func (h *Hub) BroadcastToGroup(activeGroupID string, event Event) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients := h.groupClients[activeGroupID]
	if len(clients) == 0 {
		// No subscribers for this group - not an error
		h.getLogger().Debug("no SSE subscribers for group",
			slog.String("active_group_id", activeGroupID),
			slog.String("event_type", string(event.Type)),
		)
		return nil
	}

	// Send event to all subscribed clients
	successCount := 0
	for _, client := range clients {
		select {
		case client.Channel <- event:
			successCount++
		default:
			// Client's channel is full - skip this client
			h.getLogger().Warn("SSE client channel full, skipping event",
				slog.Int64("user_id", client.UserID),
				slog.String("active_group_id", activeGroupID),
				slog.String("event_type", string(event.Type)),
			)
		}
	}

	h.getLogger().Debug("SSE event broadcast",
		slog.String("active_group_id", activeGroupID),
		slog.String("event_type", string(event.Type)),
		slog.Int("recipient_count", len(clients)),
		slog.Int("successful", successCount),
	)

	return nil
}

// GetClientCount returns the total number of connected clients (for monitoring)
func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// GetGroupSubscriberCount returns the number of clients subscribed to a specific group
func (h *Hub) GetGroupSubscriberCount(activeGroupID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.groupClients[activeGroupID])
}
