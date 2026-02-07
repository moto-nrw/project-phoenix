package realtime

import (
	"log/slog"
	"sync"
)

// Client represents a single SSE client connection
type Client struct {
	Channel          chan Event      // Channel to send events to this client
	UserID           int64           // User ID for audit logging
	SubscribedTopics map[string]bool // topic -> subscribed (e.g., "group:123", "settings:system")
}

// Hub manages SSE client connections and broadcasts events
// Topics can be anything: "group:123" for active groups, "settings:system" for settings, etc.
type Hub struct {
	clients      map[*Client]bool
	topicClients map[string][]*Client // topic -> subscribers
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
		topicClients: make(map[string][]*Client),
		logger:       logger,
	}
}

// Register adds a client to the hub and subscribes them to specified topics
// Topics can be: "group:123" for active groups, "settings:system", "settings:user:42", etc.
func (h *Hub) Register(client *Client, topics []string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client] = true

	// Subscribe client to each topic
	for _, topic := range topics {
		h.topicClients[topic] = append(h.topicClients[topic], client)
		client.SubscribedTopics[topic] = true
	}

	h.getLogger().Info("SSE client connected",
		slog.Int64("user_id", client.UserID),
		slog.Any("subscribed_topics", topics),
		slog.Int("total_clients", len(h.clients)),
	)
}

// Unregister removes a client from the hub and all topic subscriptions
func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.clients[client] {
		return // Client not registered
	}

	delete(h.clients, client)

	// Remove from all topic subscriptions
	for topic := range client.SubscribedTopics {
		clients := h.topicClients[topic]
		for i, c := range clients {
			if c == client {
				// Remove this client from the topic's subscriber list
				h.topicClients[topic] = append(clients[:i], clients[i+1:]...)
				break
			}
		}

		// Clean up empty topic lists
		if len(h.topicClients[topic]) == 0 {
			delete(h.topicClients, topic)
		}
	}

	close(client.Channel)

	h.getLogger().Info("SSE client disconnected",
		slog.Int64("user_id", client.UserID),
		slog.Int("total_clients", len(h.clients)),
	)
}

// BroadcastToTopic sends an event to all clients subscribed to the specified topic
// This is a fire-and-forget operation - errors don't affect service execution
func (h *Hub) BroadcastToTopic(topic string, event Event) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients := h.topicClients[topic]
	if len(clients) == 0 {
		// No subscribers for this topic - not an error
		h.getLogger().Debug("no SSE subscribers for topic",
			slog.String("topic", topic),
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
				slog.String("topic", topic),
				slog.String("event_type", string(event.Type)),
			)
		}
	}

	h.getLogger().Debug("SSE event broadcast",
		slog.String("topic", topic),
		slog.String("event_type", string(event.Type)),
		slog.Int("recipient_count", len(clients)),
		slog.Int("successful", successCount),
	)

	return nil
}

// BroadcastToGroup is an alias for BroadcastToTopic for backward compatibility
// Deprecated: Use BroadcastToTopic with topic format "group:{id}" instead
func (h *Hub) BroadcastToGroup(activeGroupID string, event Event) error {
	return h.BroadcastToTopic(activeGroupID, event)
}

// GetClientCount returns the total number of connected clients (for monitoring)
func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// GetTopicSubscriberCount returns the number of clients subscribed to a specific topic
func (h *Hub) GetTopicSubscriberCount(topic string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.topicClients[topic])
}

// GetGroupSubscriberCount is an alias for GetTopicSubscriberCount for backward compatibility
// Deprecated: Use GetTopicSubscriberCount instead
func (h *Hub) GetGroupSubscriberCount(activeGroupID string) int {
	return h.GetTopicSubscriberCount(activeGroupID)
}

// SubscribeToTopic adds a topic subscription for an already registered client
func (h *Hub) SubscribeToTopic(client *Client, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.clients[client] {
		return // Client not registered
	}

	if client.SubscribedTopics[topic] {
		return // Already subscribed
	}

	h.topicClients[topic] = append(h.topicClients[topic], client)
	client.SubscribedTopics[topic] = true
}

// UnsubscribeFromTopic removes a topic subscription for a client
func (h *Hub) UnsubscribeFromTopic(client *Client, topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !client.SubscribedTopics[topic] {
		return // Not subscribed
	}

	delete(client.SubscribedTopics, topic)

	clients := h.topicClients[topic]
	for i, c := range clients {
		if c == client {
			h.topicClients[topic] = append(clients[:i], clients[i+1:]...)
			break
		}
	}

	if len(h.topicClients[topic]) == 0 {
		delete(h.topicClients, topic)
	}
}
