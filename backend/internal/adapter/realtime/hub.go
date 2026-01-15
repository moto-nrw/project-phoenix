// Package realtime provides Server-Sent Events (SSE) infrastructure for real-time notifications.
// This package is an adapter that implements the port.Broadcaster interface.
package realtime

import (
	"sync"

	"github.com/moto-nrw/project-phoenix/internal/core/port"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
)

// Client represents a single SSE client connection
type Client struct {
	Channel          chan port.Event // Channel to send events to this client
	UserID           int64           // User ID for audit logging
	SubscribedGroups map[string]bool // active_group_id -> subscribed
}

// Hub manages SSE client connections and broadcasts events.
// Implements port.Broadcaster interface.
type Hub struct {
	clients      map[*Client]bool
	groupClients map[string][]*Client // active_group_id -> subscribers
	mu           sync.RWMutex
}

// NewHub creates a new SSE hub
func NewHub() *Hub {
	return &Hub{
		clients:      make(map[*Client]bool),
		groupClients: make(map[string][]*Client),
	}
}

// Ensure Hub implements port.Broadcaster
var _ port.Broadcaster = (*Hub)(nil)

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

	// Audit logging for GDPR compliance (defensive nil check)
	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]any{
			"user_id":           client.UserID,
			"subscribed_groups": activeGroupIDs,
			"total_clients":     len(h.clients),
		}).Info("SSE client connected")
	}
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

	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]any{
			"user_id":       client.UserID,
			"total_clients": len(h.clients),
		}).Info("SSE client disconnected")
	}
}

// BroadcastToGroup sends an event to all clients subscribed to the specified active group.
// This is a fire-and-forget operation - errors don't affect service execution.
// Implements port.Broadcaster interface.
func (h *Hub) BroadcastToGroup(activeGroupID string, event port.Event) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients := h.groupClients[activeGroupID]
	if len(clients) == 0 {
		// No subscribers for this group - not an error
		if logger.Logger != nil {
			logger.Logger.WithFields(map[string]any{
				"active_group_id": activeGroupID,
				"event_type":      string(event.Type),
			}).Debug("No SSE subscribers for group")
		}
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
			if logger.Logger != nil {
				logger.Logger.WithFields(map[string]any{
					"user_id":         client.UserID,
					"active_group_id": activeGroupID,
					"event_type":      string(event.Type),
				}).Warn("SSE client channel full, skipping event")
			}
		}
	}

	if logger.Logger != nil {
		logger.Logger.WithFields(map[string]any{
			"active_group_id": activeGroupID,
			"event_type":      string(event.Type),
			"recipient_count": len(clients),
			"successful":      successCount,
		}).Debug("SSE event broadcast")
	}

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
