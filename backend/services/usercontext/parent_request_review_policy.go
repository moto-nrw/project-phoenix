package usercontext

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
)

type parentRequestBoolResolver interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
}

type parentRequestGroupResolver interface {
	GetMyGroups(ctx context.Context) ([]*education.Group, error)
}

// ParentRequestReviewPolicy narrows review access behind the route-level
// permissions. Administrators remain school-wide; group leaders are opt-in.
type ParentRequestReviewPolicy struct {
	settings   parentRequestBoolResolver
	groups     parentRequestGroupResolver
	settingKey string
}

func NewParentRequestReviewPolicy(
	settings parentRequestBoolResolver,
	groups parentRequestGroupResolver,
	settingKey string,
) *ParentRequestReviewPolicy {
	return &ParentRequestReviewPolicy{settings: settings, groups: groups, settingKey: settingKey}
}

// StudentFilter resolves one immutable request-scoped filter. Resolving once
// keeps queue filtering O(1) per child and prevents list and decision checks
// from drifting.
func (p *ParentRequestReviewPolicy) StudentFilter(
	ctx context.Context,
	permissions []string,
) (func(*users.Student) bool, error) {
	schoolWide, groupIDs, err := p.resolveScope(ctx, permissions)
	if err != nil {
		return nil, err
	}
	if schoolWide {
		return func(student *users.Student) bool { return student != nil }, nil
	}
	return func(student *users.Student) bool {
		if student == nil || student.GroupID == nil {
			return false
		}
		_, ok := groupIDs[*student.GroupID]
		return ok
	}, nil
}

// Allows applies the same scope to a single decision.
func (p *ParentRequestReviewPolicy) Allows(
	ctx context.Context,
	permissions []string,
	student *users.Student,
) (bool, error) {
	filter, err := p.StudentFilter(ctx, permissions)
	if err != nil {
		return false, err
	}
	return filter(student), nil
}

// Review access levels reported by AccessLevel. They tell a client WHY its
// queue may be empty, which is the difference between "nothing to do" and
// "your school has not given you this".
const (
	ReviewAccessAdmin       = "admin"
	ReviewAccessGroupLeader = "group_leader"
	ReviewAccessNone        = "none"
)

// AccessLevel reports the caller's coarse review reach. It is the same
// decision StudentFilter makes, without the per-child part, so a client can
// explain an empty queue instead of only showing it.
func (p *ParentRequestReviewPolicy) AccessLevel(ctx context.Context, permissions []string) (string, error) {
	schoolWide, groupIDs, err := p.resolveScope(ctx, permissions)
	if err != nil {
		return "", err
	}
	if schoolWide {
		return ReviewAccessAdmin, nil
	}
	if len(groupIDs) > 0 {
		return ReviewAccessGroupLeader, nil
	}
	return ReviewAccessNone, nil
}

// resolveScope is the single policy evaluation shared by list/decision
// filtering and the lightweight navigation capability endpoint.
func (p *ParentRequestReviewPolicy) resolveScope(
	ctx context.Context,
	permissions []string,
) (bool, map[int64]struct{}, error) {
	if hasEffectiveAdminScope(ctx) || authorize.HasAdminWildcard(permissions) {
		return true, nil, nil
	}
	// users:absence alone never unlocks a read surface (#2232). Refusing here
	// keeps the queue consistent with the write gate instead of returning an
	// empty list that looks like "no work".
	if authorize.AbsenceReadPrerequisiteUnmet(permissions) {
		return false, nil, authorize.ErrAbsenceReadRequired
	}
	if !authorize.CanReviewExcusedAbsenceRequests(permissions) {
		return false, nil, nil
	}
	if p == nil || p.settings == nil || p.groups == nil || p.settingKey == "" {
		return false, nil, fmt.Errorf("parent request review policy is not configured")
	}
	enabled, err := p.settings.ResolveBool(ctx, p.settingKey)
	if err != nil {
		return false, nil, fmt.Errorf("resolve group-leader request review setting: %w", err)
	}
	if !enabled {
		return false, nil, nil
	}
	groups, err := p.groups.GetMyGroups(ctx)
	if err != nil {
		return false, nil, fmt.Errorf("resolve request review groups: %w", err)
	}
	groupIDs := make(map[int64]struct{}, len(groups))
	for _, group := range groups {
		if group != nil && group.ID > 0 {
			groupIDs[group.ID] = struct{}{}
		}
	}
	return false, groupIDs, nil
}

// hasEffectiveAdminScope reports whether the caller holds the admin role or a
// system-wide admin permission. Local copy of the helper #2645 removed from
// auth/authorize: services must not import the security-runtime contract, and
// sse_subscription.go in this package resolves it the same way.
func hasEffectiveAdminScope(ctx context.Context) bool {
	claims := jwt.ClaimsFromCtx(ctx)
	return claims.IsAdmin || authorize.HasAdminWildcard(jwt.PermissionsFromCtx(ctx))
}
