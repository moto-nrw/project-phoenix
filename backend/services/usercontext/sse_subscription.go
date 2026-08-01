package usercontext

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// SSESettingsResolver resolves the tenant-scoped boolean settings the SSE
// subscription policy depends on. Implemented by config.SettingsService.
// Optional — when nil, the admin-supervision-overview path is skipped and the
// resolver falls back to the staff member's own supervised groups.
type SSESettingsResolver interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
}

// SSESubscription is the resolved topic set for an SSE client: the staff
// member (0 for effective admins without a staff record), the active-group
// topics, the derived educational-group topics, and the deduplicated union.
type SSESubscription struct {
	StaffID        int64
	ActiveGroupIDs []string
	EduTopics      []string
	AllTopics      []string
}

// SSESetupError carries an HTTP status out of subscription resolution so the
// SSE handler can surface the correct code (401 for a missing account, 403 for
// a non-staff, non-admin user) without string matching.
type SSESetupError struct {
	Message string
	Status  int
}

func (e *SSESetupError) Error() string { return fmt.Sprintf("SSE setup: %s", e.Message) }

// ResolveSSESubscription resolves the full topic subscription for the
// authenticated caller. Effective admins may not have a staff record — that's
// OK, they still receive BroadcastToAll events and (when
// admin_supervision_overview is enabled) every active group's events.
// Non-admins without a staff record are rejected with an SSESetupError.
func (s *userContextService) ResolveSSESubscription(ctx context.Context) (*SSESubscription, error) {
	isAdmin := authorize.HasEffectiveAdminScope(ctx)

	staff, errMsg, code := s.resolveSSEStaff(ctx)
	if staff == nil && !isAdmin {
		return nil, &SSESetupError{Message: errMsg, Status: code}
	}

	var staffID int64
	if staff != nil {
		staffID = staff.ID
	}

	return s.buildSSESubscription(ctx, staffID)
}

// resolveSSEStaff resolves the staff member linked to the authenticated
// account. Returns an error message and HTTP status code on failure.
func (s *userContextService) resolveSSEStaff(ctx context.Context) (*users.Staff, string, int) {
	claims := jwt.ClaimsFromCtx(ctx)

	person, err := s.personRepo.FindByAccountID(ctx, int64(claims.ID))
	if err != nil || person == nil {
		return nil, "Account not found", http.StatusUnauthorized
	}

	staff, err := s.staffRepo.FindByPersonID(ctx, person.ID)
	if err != nil || staff == nil {
		return nil, "User is not a staff member", http.StatusForbidden
	}

	return staff, "", 0
}

// buildSSESubscription builds the topic set (active groups + derived
// educational groups) for the resolved staff member.
func (s *userContextService) buildSSESubscription(ctx context.Context, staffID int64) (*SSESubscription, error) {
	supervisions, err := s.resolveSSESupervisions(ctx, staffID)
	if err != nil {
		return nil, err
	}

	activeGroupIDs := make([]string, 0, len(supervisions))
	eduTopics := make([]string, 0)
	allTopics := make([]string, 0)
	topicSet := make(map[string]struct{})

	addTopic := func(topic string) {
		if topic == "" {
			return
		}
		if _, exists := topicSet[topic]; exists {
			return
		}
		topicSet[topic] = struct{}{}
		allTopics = append(allTopics, topic)
	}

	for _, supervision := range supervisions {
		groupTopic := strconv.FormatInt(supervision.GroupID, 10)
		activeGroupIDs = append(activeGroupIDs, groupTopic)
		addTopic(groupTopic)
	}

	eduGroups, err := s.GetMyGroups(ctx)
	if err != nil {
		s.getLogger().Warn("failed to load educational groups for SSE subscription",
			slog.String("error", err.Error()),
			slog.Int64("staff_id", staffID),
		)
	} else {
		eduTopics = make([]string, 0, len(eduGroups))
		for _, group := range eduGroups {
			topic := fmt.Sprintf("edu:%d", group.ID)
			eduTopics = append(eduTopics, topic)
			addTopic(topic)
		}
	}

	return &SSESubscription{
		StaffID:        staffID,
		ActiveGroupIDs: activeGroupIDs,
		EduTopics:      eduTopics,
		AllTopics:      allTopics,
	}, nil
}

// resolveSSESupervisions returns the supervisions to subscribe to. For admins
// with admin_supervision_overview enabled, returns a synthetic entry for every
// currently active group — aligned with the HTTP endpoint
// /api/active/supervisors/all which also enumerates active.groups directly.
// Using ListActiveGroups (rather than the supervisor rows) ensures unclaimed
// active groups (e.g. Schulhof without a current supervisor) still receive
// live events. For regular staff, returns only their own supervised groups.
func (s *userContextService) resolveSSESupervisions(ctx context.Context, staffID int64) ([]*active.GroupSupervisor, error) {
	if s.sseActiveSvc == nil {
		return nil, errors.New("SSE active service is not configured")
	}

	if authorize.HasEffectiveAdminScope(ctx) && s.sseSettings != nil {
		enabled, err := s.sseSettings.ResolveBool(ctx, configModel.KeyAdminSupervisionOverview)
		if err != nil {
			s.getLogger().Warn("admin_supervision_overview setting check failed for SSE, falling back to staff supervisions",
				slog.String("error", err.Error()),
				slog.Int64("staff_id", staffID),
			)
		} else if enabled {
			groups, err := s.sseActiveSvc.ListActiveGroups(ctx, base.NewQueryOptions())
			if err != nil {
				s.getLogger().Error("failed to list active groups for admin SSE",
					slog.String("error", err.Error()),
					slog.Int64("staff_id", staffID),
				)
				return nil, err
			}
			// Synthesise GroupSupervisor records so the topic-building loop can
			// reuse GroupID without special-casing admin paths.
			synthetic := make([]*active.GroupSupervisor, 0, len(groups))
			for _, g := range groups {
				if g.IsActive() {
					synthetic = append(synthetic, &active.GroupSupervisor{
						GroupID: g.ID,
					})
				}
			}
			return synthetic, nil
		}
	}

	supervisions, err := s.sseActiveSvc.GetStaffActiveSupervisions(ctx, staffID)
	if err != nil {
		s.getLogger().Error("failed to get staff active supervisions for SSE",
			slog.String("error", err.Error()),
			slog.Int64("staff_id", staffID),
		)
		return nil, err
	}
	return supervisions, nil
}
