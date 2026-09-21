package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// HTTP statuses a rejected live-update subscription maps to.
const (
	statusUnauthorized = 401
	statusForbidden    = 403
)

// Navigation returns the identity projection with every available
// navigation section. Group-derived sections are optional: omitting one on a
// failed read is safer than failing unrelated navigation entirely.
func (c *CallerContext) Navigation(ctx context.Context) (domain.CallerNavigation, error) {
	unavailable := make([]string, 0, 3)

	groups, err := c.MyGroupIDs(ctx)
	if err != nil {
		c.logger().Warn("navigation context groups unavailable",
			slog.String("error", err.Error()),
		)
		unavailable = append(unavailable, "educational_groups")
	}
	substituted, err := c.SubstitutedGroupIDs(ctx)
	if err != nil {
		c.logger().Warn("navigation context substitutions unavailable",
			slog.String("error", err.Error()),
		)
		substituted = make(map[int64]bool)
		unavailable = append(unavailable, "substitutions")
	}
	supervised, err := c.MySupervisedSessionIDs(ctx)
	if err != nil {
		c.logger().Warn("navigation context supervision unavailable",
			slog.String("error", err.Error()),
		)
		supervised = []int64{}
		unavailable = append(unavailable, "supervised_groups")
	}
	staffID, err := c.StaffID(ctx)
	if err != nil && !isNotStaff(err) {
		return domain.CallerNavigation{}, err
	}

	educational := make([]domain.CallerGroup, 0, len(groups))
	for _, id := range groups {
		educational = append(educational, domain.CallerGroup{ID: id, ViaSubstitution: substituted[id]})
	}
	return domain.CallerNavigation{
		Groups:               educational,
		SupervisedSessionIDs: supervised,
		StaffID:              staffID,
		Incomplete:           len(unavailable) > 0,
		UnavailableSections:  unavailable,
	}, nil
}

// SSESubscription resolves the live-update topics of the caller. Effective
// admins may have no staff record: they still receive broadcast events and,
// when the school-wide overview covers them, every open session's events.
// Everyone else without a staff record is rejected with an SSESetupError.
func (c *CallerContext) SSESubscription(ctx context.Context) (domain.SSESubscription, error) {
	caller := c.principal(ctx)
	staffID, message, status := c.subscriptionStaff(ctx)
	if staffID == 0 && !caller.EffectiveAdmin() {
		return domain.SSESubscription{}, &domain.SSESetupError{Message: message, Status: status}
	}
	return c.buildSubscription(ctx, caller, staffID)
}

// subscriptionStaff resolves the caller's staff member through the memoized
// chain, so the educational-group read below does not walk it again.
func (c *CallerContext) subscriptionStaff(ctx context.Context) (int64, string, int) {
	if _, err := c.Person(ctx); err != nil {
		return 0, "Account not found", statusUnauthorized
	}
	staffID, err := c.StaffID(ctx)
	if err != nil {
		return 0, "User is not a staff member", statusForbidden
	}
	return staffID, "", 0
}

func (c *CallerContext) buildSubscription(ctx context.Context, caller domain.Caller, staffID int64) (domain.SSESubscription, error) {
	sessions, err := c.subscriptionSessions(ctx, caller, staffID)
	if err != nil {
		return domain.SSESubscription{}, err
	}
	subscription := domain.SSESubscription{
		StaffID:        staffID,
		ActiveGroupIDs: make([]string, 0, len(sessions)),
		EduTopics:      make([]string, 0),
		AllTopics:      make([]string, 0),
	}
	seen := make(map[string]struct{})
	addTopic := func(topic string) {
		if _, exists := seen[topic]; exists {
			return
		}
		seen[topic] = struct{}{}
		subscription.AllTopics = append(subscription.AllTopics, topic)
	}
	for _, id := range sessions {
		topic := strconv.FormatInt(id, 10)
		subscription.ActiveGroupIDs = append(subscription.ActiveGroupIDs, topic)
		addTopic(topic)
	}

	groups, err := c.MyGroupIDs(ctx)
	if err != nil {
		c.logger().Warn("failed to load educational groups for SSE subscription",
			slog.String("error", err.Error()),
			slog.Int64("staff_id", staffID),
		)
		return subscription, nil
	}
	subscription.EduTopics = make([]string, 0, len(groups))
	for _, id := range groups {
		topic := fmt.Sprintf("edu:%d", id)
		subscription.EduTopics = append(subscription.EduTopics, topic)
		addTopic(topic)
	}
	return subscription, nil
}

// subscriptionSessions returns the room sessions to subscribe to. Callers
// covered by the school-wide overview scope (#2380) get every open session,
// the same rule the HTTP endpoints use, so a client never sees a block in a
// list whose live updates it is not subscribed to; open sessions without a
// current supervisor are included. Everyone else keeps the sessions they
// supervise.
func (c *CallerContext) subscriptionSessions(ctx context.Context, caller domain.Caller, staffID int64) ([]int64, error) {
	topics := c.deps.LiveTopics
	if topics == nil {
		return nil, errors.New("SSE active service is not configured")
	}
	if c.deps.Overview != nil {
		broad, err := c.deps.Overview.HasOperationalOverview(ctx, c, caller.SchoolScope, caller.EffectiveAdmin())
		if err != nil {
			c.logger().Warn("operational overview scope check failed for SSE, falling back to staff supervisions",
				slog.String("error", err.Error()),
				slog.Int64("staff_id", staffID),
			)
		} else if broad {
			ids, err := topics.OpenSessionIDs(ctx)
			if err != nil {
				c.logger().Error("failed to list active groups for school-wide SSE",
					slog.String("error", err.Error()),
					slog.Int64("staff_id", staffID),
				)
				return nil, err
			}
			return ids, nil
		}
	}
	ids, err := topics.StaffSessionIDs(ctx, staffID)
	if err != nil {
		c.logger().Error("failed to get staff active supervisions for SSE",
			slog.String("error", err.Error()),
			slog.Int64("staff_id", staffID),
		)
		return nil, err
	}
	return ids, nil
}
