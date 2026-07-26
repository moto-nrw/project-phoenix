package notifications

import (
	"context"
	"fmt"
	"maps"
	"strconv"

	"github.com/moto-nrw/project-phoenix/realtime"
)

// sseChannel delivers notifications in-app over the existing SSE hub as
// realtime.EventNotification events. It reuses the Broadcaster untouched —
// the existing cache-invalidation events are a separate concern and keep
// flowing exactly as before (#1624 acceptance criterion).
type sseChannel struct {
	broadcaster realtime.Broadcaster
}

// NewSSEChannel wraps the shared SSE broadcaster as a notification channel.
func NewSSEChannel(broadcaster realtime.Broadcaster) Channel {
	return &sseChannel{broadcaster: broadcaster}
}

func (c *sseChannel) Name() string { return "sse" }

func (c *sseChannel) Deliver(ctx context.Context, event Event) error {
	_ = ctx // Broadcaster is synchronous fan-out to in-memory channels
	sseEvent := realtime.NewEvent(realtime.EventNotification, event.Audience.ActiveGroupID, realtime.EventData{
		Title:            &event.Title,
		Body:             &event.Body,
		DeepLink:         &event.DeepLink,
		Priority:         &event.Priority,
		NotificationType: &event.Type,
		NotificationData: maps.Clone(event.Data),
	})

	switch event.Audience.Scope {
	case ScopeTenant:
		return c.broadcaster.BroadcastToTenant(event.Audience.TenantID, sseEvent)
	case ScopeGuardian:
		return c.broadcaster.BroadcastToGuardian(event.Audience.TenantID, event.Audience.GuardianAccountID, sseEvent)
	case ScopeGroup:
		return c.broadcaster.BroadcastToGroup(event.Audience.TenantID, event.Audience.ActiveGroupID, sseEvent)
	default:
		return fmt.Errorf("sse channel cannot route audience scope %q (tenant %s)",
			event.Audience.Scope, strconv.FormatInt(event.Audience.TenantID, 10))
	}
}
