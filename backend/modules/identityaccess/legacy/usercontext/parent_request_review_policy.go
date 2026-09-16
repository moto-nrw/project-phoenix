package usercontext

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type parentRequestSettingsResolver interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
	ResolveString(ctx context.Context, key string) (string, error)
}

type parentRequestGroupResolver interface {
	GetMyGroups(ctx context.Context) ([]*education.Group, error)
	GetCurrentStaff(ctx context.Context) (*users.Staff, error)
}

// ParentRequestReviewPolicy narrows review access behind the route-level
// permissions. Administrators remain school-wide; group leaders are opt-in.
type ParentRequestReviewPolicy struct {
	settings          parentRequestSettingsResolver
	groups            parentRequestGroupResolver
	settingKey        string
	absenceSettingKey string
}

func NewParentRequestReviewPolicy(
	settings parentRequestSettingsResolver,
	groups parentRequestGroupResolver,
	settingKey string,
	absenceSettingKey string,
) *ParentRequestReviewPolicy {
	return &ParentRequestReviewPolicy{settings: settings, groups: groups, settingKey: settingKey, absenceSettingKey: absenceSettingKey}
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

// Scope reports the caller's reach as data: school-wide, or exactly the
// group ids a group leader may review. Owners that keep their own student
// projection apply it themselves instead of handing this package a row.
func (p *ParentRequestReviewPolicy) Scope(ctx context.Context, permissions []string) (schoolWide bool, groupIDs []int64, err error) {
	schoolWide, groups, err := p.resolveScope(ctx, permissions)
	if err != nil {
		return false, nil, err
	}
	groupIDs = make([]int64, 0, len(groups))
	for id := range groups {
		groupIDs = append(groupIDs, id)
	}
	slices.Sort(groupIDs)
	return schoolWide, groupIDs, nil
}

// AbsenceScope applies only to sick/excused parent requests. Other request
// kinds continue to use Scope, including their original group-leader switch.
func (p *ParentRequestReviewPolicy) AbsenceScope(ctx context.Context, permissions []string) (bool, []int64, error) {
	if hasEffectiveAdminScope(ctx) || authorize.HasAdminWildcard(permissions) {
		return true, nil, nil
	}
	if authorize.AbsenceReadPrerequisiteUnmet(permissions) {
		return false, nil, authorize.ErrAbsenceReadRequired
	}
	if !authorize.CanReviewExcusedAbsenceRequests(permissions) {
		return false, nil, nil
	}
	if p == nil || p.settings == nil || p.groups == nil || p.absenceSettingKey == "" {
		return false, nil, fmt.Errorf("parent absence review policy is not configured")
	}
	scope, err := p.settings.ResolveString(ctx, p.absenceSettingKey)
	if err != nil {
		return false, nil, fmt.Errorf("resolve parent absence review scope: %w", err)
	}
	switch scope {
	case "inherit":
		return p.Scope(ctx, permissions)
	case "admins":
		return false, nil, nil
	case "group_leaders":
		groups, err := p.groups.GetMyGroups(ctx)
		if err != nil {
			return false, nil, fmt.Errorf("resolve absence review groups: %w", err)
		}
		ids := make([]int64, 0, len(groups))
		for _, group := range groups {
			if group != nil && group.ID > 0 {
				ids = append(ids, group.ID)
			}
		}
		slices.Sort(ids)
		return false, slices.Compact(ids), nil
	case "all_staff":
		claims := jwt.ClaimsFromCtx(ctx)
		if claims.ID <= 0 || claims.TenantID <= 0 || claims.TenantID != tenant.FromContext(ctx) ||
			(claims.Scope != "" && claims.Scope != "tenant" && claims.Scope != "org") {
			return false, nil, nil
		}
		staff, err := p.groups.GetCurrentStaff(ctx)
		if err != nil {
			return false, nil, fmt.Errorf("resolve absence reviewer staff: %w", err)
		}
		return staff != nil && staff.ID > 0 && staff.TenantID == claims.TenantID, nil, nil
	default:
		return false, nil, fmt.Errorf("unknown parent absence review scope %q", scope)
	}
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
	ReviewAccessTeam        = "team"
	ReviewAccessGroupLeader = "group_leader"
	ReviewAccessNone        = "none"
)

// AccessLevel reports the union of the queues the caller can review. Each
// queue still applies its own scope to rows, notes, counts and decisions.
func (p *ParentRequestReviewPolicy) AccessLevel(ctx context.Context, permissions []string) (string, error) {
	schoolWide, groupIDs, err := p.resolveScope(ctx, permissions)
	if err != nil {
		return "", err
	}
	if schoolWide {
		return ReviewAccessAdmin, nil
	}
	absenceWide, absenceGroups, err := p.AbsenceScope(ctx, permissions)
	if err != nil {
		return "", err
	}
	if absenceWide {
		return ReviewAccessTeam, nil
	}
	if len(groupIDs) > 0 || len(absenceGroups) > 0 {
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
