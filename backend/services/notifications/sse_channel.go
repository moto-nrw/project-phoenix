package notifications

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"strconv"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// GuardianChildAccessRepository answers, at delivery time, which of an
// already-resolved guardian audience still hold a parent_portal.* permission for
// at least one of the children an event is about.
type GuardianChildAccessRepository interface {
	FilterAccountsWithStudentAccess(ctx context.Context, guardianAccountIDs, studentIDs []int64, tenantID int64, permission string) ([]int64, error)
}

// sseChannel delivers notifications in-app over the existing SSE hub as
// realtime.EventNotification events. It reuses the Broadcaster untouched —
// the existing cache-invalidation events are a separate concern and keep
// flowing exactly as before (#1624 acceptance criterion).
type sseChannel struct {
	broadcaster   realtime.Broadcaster
	db            *bun.DB
	guardians     GuardianChildAccessRepository
	logger        *slog.Logger
	tenantRuntime *tenant.Runtime
}

func (c *sseChannel) SetTenantRuntime(runtime tenant.Runtime) {
	c.tenantRuntime = &runtime
}

func (c *sseChannel) withTenantRuntime(ctx context.Context) context.Context {
	if c.tenantRuntime == nil {
		return ctx
	}
	return tenant.WithRuntime(ctx, *c.tenantRuntime)
}

// SSEChannelOption configures optional dependencies of the SSE channel.
type SSEChannelOption func(*sseChannel)

// WithGuardianChildAccess makes the channel re-read users.students_guardians
// before a guardian fan-out, keeping only the recipients who still hold
// parent_portal.access for at least one of Audience.StudentIDs.
//
// It is the SSE counterpart of the same question the Web Push channel asks in
// its device lookup, and for the same reason: the recipient list was decided in
// the producer's transaction, delivery runs in a later one, and an open parent
// session would otherwise be handed the title, body and deep link of an event
// about a child whose access was revoked in between. Wired in production; a
// channel built without it drops student-scoped guardian events rather than
// deliver them unchecked.
func WithGuardianChildAccess(db *bun.DB, guardians GuardianChildAccessRepository, logger *slog.Logger) SSEChannelOption {
	return func(c *sseChannel) {
		c.db = db
		c.guardians = guardians
		c.logger = logger
	}
}

// NewSSEChannel wraps the shared SSE broadcaster as a notification channel.
func NewSSEChannel(broadcaster realtime.Broadcaster, opts ...SSEChannelOption) Channel {
	channel := &sseChannel{broadcaster: broadcaster}
	for _, opt := range opts {
		opt(channel)
	}
	return channel
}

func (c *sseChannel) Name() string { return "sse" }

func (c *sseChannel) getLogger() *slog.Logger {
	if c.logger == nil {
		return slog.Default()
	}
	return c.logger
}

func (c *sseChannel) Deliver(ctx context.Context, event Event) error {
	ctx = c.withTenantRuntime(ctx)
	sseEvent := realtime.NewEvent(realtime.EventNotification, event.Audience.ActiveGroupID, realtime.EventData{
		Title:            &event.Title,
		Body:             &event.Body,
		DeepLink:         &event.DeepLink,
		SchoolDeepLink:   optionalString(event.SchoolDeepLink),
		Priority:         &event.Priority,
		NotificationType: &event.Type,
		NotificationData: maps.Clone(event.Data),
	})

	switch event.Audience.Scope {
	case ScopeTenant:
		return c.broadcaster.BroadcastToTenant(event.Audience.TenantID, sseEvent)
	case ScopeAdmin:
		return c.broadcaster.BroadcastToTenantAdmins(event.Audience.TenantID, sseEvent)
	case ScopeGuardian:
		recipients, err := c.permittedGuardians(ctx, event.Audience)
		if err != nil {
			return err
		}
		for _, accountID := range recipients {
			if err := c.broadcaster.BroadcastToGuardian(event.Audience.TenantID, accountID, sseEvent); err != nil {
				return err
			}
		}
		return nil
	case ScopeGroup:
		return c.broadcaster.BroadcastToGroup(event.Audience.TenantID, event.Audience.ActiveGroupID, sseEvent)
	case ScopeStaff:
		accountIDs := staffAccountIDs(event.Audience)
		if deliversToStaffPortal(event) {
			if err := c.broadcaster.BroadcastToStaffAccounts(event.Audience.TenantID, accountIDs, sseEvent); err != nil {
				return err
			}
		}
		if deliversToSchoolPortal(event) {
			return c.broadcaster.BroadcastToSchoolAccounts(event.Audience.TenantID, accountIDs, sseEvent)
		}
		return nil
	default:
		return fmt.Errorf("sse channel cannot route audience scope %q (tenant %s)",
			event.Audience.Scope, strconv.FormatInt(event.Audience.TenantID, 10))
	}
}

// permittedGuardians narrows the resolved recipients to the accounts that still
// see at least one of the children the event is about. An audience naming no
// child (a broadcast announcement) is gated by its own producer and passes
// through unchanged.
func (c *sseChannel) permittedGuardians(ctx context.Context, audience Audience) ([]int64, error) {
	recipients := guardianAccountIDs(audience)
	if len(recipients) == 0 || len(audience.StudentIDs) == 0 {
		return recipients, nil
	}
	if c.db == nil || c.guardians == nil {
		// Fail closed: an event that names children is deliverable only where the
		// question "does this account still see them" can actually be asked.
		c.getLogger().Warn("sse channel cannot re-check guardian child access, dropping student-scoped notification",
			"tenant_id", audience.TenantID,
			"recipient_count", len(recipients),
		)
		return nil, nil
	}

	var permitted []int64
	err := tenant.WithTenantTx(ctx, c.db, audience.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		allowed, err := c.guardians.FilterAccountsWithStudentAccess(
			txCtx, recipients, audience.StudentIDs, audience.TenantID, authorize.GuardianPermissionPortalAccess)
		if err != nil {
			return err
		}
		permitted = allowed
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("re-checking guardian child access for tenant %s: %w",
			strconv.FormatInt(audience.TenantID, 10), err)
	}
	return permitted, nil
}

// optionalString maps "" to nil so the SSE payload omits the field.
func optionalString(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}
