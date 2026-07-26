package realtime

import (
	"cmp"
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
	// IsAdmin mirrors the effective admin scope used by tenant handlers:
	// literal admins plus accounts with admin:* or *:* permissions.
	IsAdmin bool
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

// removeClient returns clients with the first occurrence of target removed,
// preserving the order of the others. The freed tail slot is niled out so the
// removed *Client (and its 32-buffered event channel) can be GC'd immediately
// instead of being retained in the backing array until the slot is overwritten.
func removeClient(clients []*Client, target *Client) []*Client {
	for i, c := range clients {
		if c == target {
			copy(clients[i:], clients[i+1:])
			clients[len(clients)-1] = nil
			return clients[:len(clients)-1]
		}
	}
	return clients
}

// Hub manages SSE client connections and broadcasts events
type Hub struct {
	clients      map[*Client]bool
	groupClients map[string][]*Client // composite tenant:group key -> subscribers
	// guardianClients indexes parent-portal clients by their guardian account id
	// (Client.UserID) and tenantClients indexes staff clients by tenant id, so
	// BroadcastParentMessage delivers in O(recipients) instead of scanning every
	// connection. Both are maintained alongside h.clients in Register /
	// RegisterParent / Unregister.
	guardianClients map[int64][]*Client
	tenantClients   map[int64][]*Client
	mu              sync.RWMutex
	logger          *slog.Logger
}

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (h *Hub) getLogger() *slog.Logger {
	return cmp.Or(h.logger, slog.Default())
}

// NewHub creates a new SSE hub
func NewHub(logger *slog.Logger) *Hub {
	return &Hub{
		clients:         make(map[*Client]bool),
		groupClients:    make(map[string][]*Client),
		guardianClients: make(map[int64][]*Client),
		tenantClients:   make(map[int64][]*Client),
		logger:          logger,
	}
}

// Register adds a STAFF (tenant-portal) client to the hub and subscribes them to
// the specified active groups.
//
// INVARIANT: guardian-portal connections MUST go through RegisterParent, never
// here. Registering a guardian via Register would index them in tenantClients and
// hand them staff-scoped fan-out (BroadcastToTenant, the staff copy of
// BroadcastParentMessage) — a cross-portal data leak. The two index sets
// (tenantClients vs guardianClients) are the enforcement: Unregister branches on
// client.IsParent, which only RegisterParent sets.
func (h *Hub) Register(client *Client, tenantID int64, activeGroupIDs []string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	client.TenantID = tenantID
	h.clients[client] = true
	// Index staff clients by tenant for O(recipients) parent-message fan-out.
	h.tenantClients[tenantID] = append(h.tenantClients[tenantID], client)

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
	// Index by guardian account id so BroadcastParentMessage reaches this
	// guardian's tabs in O(1) instead of scanning every connection.
	h.guardianClients[client.UserID] = append(h.guardianClients[client.UserID], client)

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

	// Remove from the guardian/tenant indexes so a closed channel can never be
	// reached by a later BroadcastParentMessage (a send on it would panic).
	if client.IsParent {
		h.guardianClients[client.UserID] = removeClient(h.guardianClients[client.UserID], client)
		if len(h.guardianClients[client.UserID]) == 0 {
			delete(h.guardianClients, client.UserID)
		}
	} else {
		h.tenantClients[client.TenantID] = removeClient(h.tenantClients[client.TenantID], client)
		if len(h.tenantClients[client.TenantID]) == 0 {
			delete(h.tenantClients, client.TenantID)
		}
	}

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

// BroadcastToTenant sends an event to every staff client whose
// Client.TenantID matches tenantID. It reads the tenantClients index
// (maintained in Register/Unregister) for O(recipients) delivery instead
// of scanning every connection. Parent (guardian-portal) clients carry
// TenantID 0 and live only in guardianClients, so they are correctly
// excluded — tenant-wide refreshes (settings invalidations, dashboard
// counts, arrival schedule) are staff-only data. Fire-and-forget: a full
// client channel is logged and skipped, not surfaced to the caller.
func (h *Hub) BroadcastToTenant(tenantID int64, event Event) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	recipients := 0
	droppedCount := 0
	for _, client := range h.tenantClients[tenantID] {
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

// BroadcastToTenantAdmins sends an event only to effective admins in one
// tenant. This is for user-visible payloads derived from an admin-wide data
// view; scoped caregivers must not receive counts or links for data outside
// their own reminder scope.
func (h *Hub) BroadcastToTenantAdmins(tenantID int64, event Event) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	recipients := 0
	droppedCount := 0
	for _, client := range h.tenantClients[tenantID] {
		if !client.IsAdmin {
			continue
		}
		recipients++
		select {
		case client.Channel <- event:
		default:
			droppedCount++
			h.getLogger().Warn("SSE client channel full, skipping admin broadcast",
				slog.Int64("user_id", client.UserID),
				slog.Int64("tenant_id", tenantID),
				slog.String("event_type", string(event.Type)),
			)
		}
	}
	h.getLogger().Debug("SSE event broadcast to tenant admins",
		slog.Int64("tenant_id", tenantID),
		slog.String("event_type", string(event.Type)),
		slog.Int("recipient_count", recipients),
	)
	observability.RecordSSEBroadcast(tenantID, string(event.Type), "tenant_admin", droppedCount)
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
	deliver := func(client *Client, payload Event) {
		recipients++
		select {
		case client.Channel <- payload:
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
	// The addressed guardian's own tabs get the full event (their own data); the
	// tenant's staff (whose access-filtered inboxes refetch) get a sanitized copy
	// with the child/guardian identity stripped — an unauthorized staffer must not
	// learn which child a thread concerns from raw SSE traffic. Both come straight
	// from the indexes, so this is O(recipients), not O(all connections).
	staffEvent := staffSafeParentMessage(event)
	for _, client := range h.guardianClients[guardianAccountID] {
		deliver(client, event)
	}
	for _, client := range h.tenantClients[tenantID] {
		deliver(client, staffEvent)
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

// BroadcastToGuardian wakes ONLY the addressed guardian's own portal clients —
// no staff copy at all. It exists for a message-INDEPENDENT guardian
// invalidation (EventParentChildUpdated) that staff must never receive: unlike
// BroadcastParentMessage, which also fans a sanitized copy out to every staff
// client in the tenant so their inbox refreshes, this carries no staff-relevant
// signal. Sending such an event via BroadcastParentMessage — once per guardian,
// as the child-update fan-out does — would push one redundant staff event PER
// GUARDIAN into every staff client's 32-slot channel, displacing real updates
// under a busy tenant. Fire-and-forget, same drop semantics as the others.
func (h *Hub) BroadcastToGuardian(tenantID, guardianAccountID int64, event Event) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	recipients := 0
	droppedCount := 0
	for _, client := range h.guardianClients[guardianAccountID] {
		recipients++
		select {
		case client.Channel <- event:
		default:
			droppedCount++
			h.getLogger().Warn("SSE client channel full, skipping broadcast-to-guardian",
				slog.Int64("user_id", client.UserID),
				slog.Int64("tenant_id", tenantID),
				slog.Int64("guardian_account_id", guardianAccountID),
				slog.String("event_type", string(event.Type)),
			)
		}
	}
	h.getLogger().Debug("SSE event broadcast to guardian",
		slog.Int64("tenant_id", tenantID),
		slog.Int64("guardian_account_id", guardianAccountID),
		slog.String("event_type", string(event.Type)),
		slog.Int("recipient_count", recipients),
	)
	observability.RecordSSEBroadcast(tenantID, string(event.Type), "guardian", droppedCount)
	return nil
}

// BroadcastToAll sends an event to every connected client regardless of group subscriptions.
func (h *Hub) BroadcastToAll(event Event) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	droppedCount := 0
	for client := range h.clients {
		// Parent (guardian-portal) clients are cross-tenant and addressed only
		// by BroadcastParentMessage. A tenant-wide refresh (dashboard counts,
		// arrival schedule) is staff-only data and must never fan out to every
		// connected guardian nationwide.
		if client.IsParent {
			continue
		}
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
