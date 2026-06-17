package realtime

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/moto-nrw/project-phoenix/observability"
)

// Client represents a single SSE client connection
type Client struct {
	Channel          chan Event      // Channel to send events to this client
	UserID           int64           // User ID for audit logging
	TenantID         int64           // Tenant ID for multi-tenancy isolation
	SubscribedGroups map[string]bool // composite key (tenantID:groupID) -> subscribed
	// IsParent marks a guardian-portal connection. Parent clients are
	// cross-tenant and routed by their own account id (UserID): they never
	// match tenant/group broadcasts, only BroadcastParentMessage addressed to
	// their guardian account.
	IsParent bool
}

// tenantGroupKey builds a composite map key for tenant-isolated group lookups.
func tenantGroupKey(tenantID int64, groupID string) string {
	return fmt.Sprintf("%d:%s", tenantID, groupID)
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
func (h *Hub) Register(client *Client, tenantID int64, activeGroupIDs []string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	client.TenantID = tenantID
	h.clients[client] = true

	// Subscribe client to each active group using composite keys
	for _, groupID := range activeGroupIDs {
		key := tenantGroupKey(tenantID, groupID)
		h.groupClients[key] = append(h.groupClients[key], client)
		client.SubscribedGroups[key] = true
	}

	h.getLogger().Info("SSE client connected",
		slog.Int64("user_id", client.UserID),
		slog.Int64("tenant_id", tenantID),
		slog.Any("subscribed_groups", activeGroupIDs),
		slog.Int("total_clients", len(h.clients)),
	)
	observability.RecordSSEConnection(tenantID, "connected")
}

// RegisterParent adds a guardian-portal client. It is identified by its own
// account id (Client.UserID) and woken only by BroadcastParentMessage
// addressed to that guardian account. TenantID stays 0 so tenant/group
// broadcasts never reach it.
func (h *Hub) RegisterParent(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	client.IsParent = true
	h.clients[client] = true

	h.getLogger().Info("SSE parent client connected",
		slog.Int64("user_id", client.UserID),
		slog.Int("total_clients", len(h.clients)),
	)
	observability.RecordSSEConnection(0, "connected")
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
		slog.Int64("tenant_id", client.TenantID),
		slog.Int("total_clients", len(h.clients)),
	)
	observability.RecordSSEConnection(client.TenantID, "disconnected")
}

// BroadcastToGroup sends an event to all clients subscribed to the specified active group
// This is a fire-and-forget operation - errors don't affect service execution
func (h *Hub) BroadcastToGroup(tenantID int64, activeGroupID string, event Event) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	key := tenantGroupKey(tenantID, activeGroupID)
	clients := h.groupClients[key]
	if len(clients) == 0 {
		// No subscribers for this group - not an error
		h.getLogger().Debug("no SSE subscribers for group",
			slog.String("active_group_id", activeGroupID),
			slog.Int64("tenant_id", tenantID),
			slog.String("event_type", string(event.Type)),
		)
		return nil
	}

	// Send event to all subscribed clients
	successCount := 0
	droppedCount := 0
	for _, client := range clients {
		select {
		case client.Channel <- event:
			successCount++
		default:
			droppedCount++
			// Client's channel is full - skip this client
			h.getLogger().Warn("SSE client channel full, skipping event",
				slog.Int64("user_id", client.UserID),
				slog.String("active_group_id", activeGroupID),
				slog.Int64("tenant_id", tenantID),
				slog.String("event_type", string(event.Type)),
			)
		}
	}

	h.getLogger().Debug("SSE event broadcast",
		slog.String("active_group_id", activeGroupID),
		slog.Int64("tenant_id", tenantID),
		slog.String("event_type", string(event.Type)),
		slog.Int("recipient_count", len(clients)),
		slog.Int("successful", successCount),
	)
	observability.RecordSSEBroadcast(tenantID, string(event.Type), "group", droppedCount)

	return nil
}

// BroadcastToTenant sends an event to every connected client whose
// Client.TenantID matches tenantID. Walks the full client map under a
// read lock — O(N) in connected clients, but the alternative
// (maintaining a tenant→clients index) is bookkeeping that has to stay
// in sync with Register/Unregister and isn't worth it for the use case
// (tenant-wide settings invalidations are rare). Errors propagate the
// same fire-and-forget semantics as BroadcastToAll: a full client
// channel is logged and skipped, not surfaced to the caller.
func (h *Hub) BroadcastToTenant(tenantID int64, event Event) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	recipients := 0
	droppedCount := 0
	for client := range h.clients {
		if client.TenantID != tenantID {
			continue
		}
		recipients++
		select {
		case client.Channel <- event:
		default:
			droppedCount++
			h.getLogger().Warn("SSE client channel full, skipping broadcast-to-tenant",
				slog.Int64("user_id", client.UserID),
				slog.Int64("tenant_id", tenantID),
				slog.String("event_type", string(event.Type)),
			)
		}
	}
	h.getLogger().Debug("SSE event broadcast to tenant",
		slog.Int64("tenant_id", tenantID),
		slog.String("event_type", string(event.Type)),
		slog.Int("recipient_count", recipients),
	)
	observability.RecordSSEBroadcast(tenantID, string(event.Type), "tenant", droppedCount)
	return nil
}

// BroadcastParentMessage routes a parent-OGS messaging trigger by recipient:
// the addressed guardian's portal client (matched on account id) is woken so
// only that one family sees the activity, while staff clients in the tenant are
// woken so their access-filtered inbox refreshes (staff visibility spans admins
// / all-staff / supervisors, so a tenant-wide staff wake is the correct,
// no-miss granularity — the refetch is server-side access-filtered).
// Fire-and-forget, same drop semantics as the other broadcasts.
func (h *Hub) BroadcastParentMessage(tenantID, guardianAccountID int64, event Event) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	recipients := 0
	droppedCount := 0
	for client := range h.clients {
		var deliver bool
		if client.IsParent {
			deliver = client.UserID == guardianAccountID
		} else {
			deliver = client.TenantID == tenantID
		}
		if !deliver {
			continue
		}
		recipients++
		select {
		case client.Channel <- event:
		default:
			droppedCount++
			h.getLogger().Warn("SSE client channel full, skipping parent-message broadcast",
				slog.Int64("user_id", client.UserID),
				slog.Int64("tenant_id", tenantID),
				slog.Int64("guardian_account_id", guardianAccountID),
				slog.String("event_type", string(event.Type)),
			)
		}
	}
	h.getLogger().Debug("SSE parent-message broadcast",
		slog.Int64("tenant_id", tenantID),
		slog.Int64("guardian_account_id", guardianAccountID),
		slog.String("event_type", string(event.Type)),
		slog.Int("recipient_count", recipients),
	)
	observability.RecordSSEBroadcast(tenantID, string(event.Type), "parent_message", droppedCount)
	return nil
}

// BroadcastToAll sends an event to every connected client regardless of group subscriptions.
func (h *Hub) BroadcastToAll(event Event) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	droppedCount := 0
	for client := range h.clients {
		select {
		case client.Channel <- event:
		default:
			droppedCount++
			h.getLogger().Warn("SSE client channel full, skipping broadcast-to-all",
				slog.Int64("user_id", client.UserID),
				slog.String("event_type", string(event.Type)),
			)
		}
	}
	observability.RecordSSEBroadcast(0, string(event.Type), "all", droppedCount)
	return nil
}

// GetClientCount returns the total number of connected clients (for monitoring)
func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// GetGroupSubscriberCount returns the number of clients subscribed to a specific group
func (h *Hub) GetGroupSubscriberCount(tenantID int64, activeGroupID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.groupClients[tenantGroupKey(tenantID, activeGroupID)])
}

func (h *Hub) SnapshotStats() observability.SSEStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	stats := observability.SSEStats{ClientsByTenant: make(map[int64]int)}
	for client := range h.clients {
		stats.ClientsByTenant[client.TenantID]++
	}
	return stats
}
