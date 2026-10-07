package application

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

// Parent request review scopes of the tenant settings. The request scope
// inherits the group-leader switch, the absence scope the request scope.
const (
	reviewScopeInherit      = "inherit"
	reviewScopeAdmins       = "admins"
	reviewScopeGroupLeaders = "group_leaders"
	reviewScopeAllStaff     = "all_staff"
)

// ParentRequestReviewDependencies binds the parent request review policy.
// Permissions evaluates the route-level permission facts the policy narrows;
// AbsenceReadRequired is the error a caller holding users:absence without
// users:read is refused with.
type ParentRequestReviewDependencies struct {
	Permissions           func([]string) domain.ReviewPermissions
	Settings              ports.ReviewSettings
	GroupLeaderSettingKey string
	RequestSettingKey     string
	AbsenceSettingKey     string
	AbsenceReadRequired   error
}

// ParentRequestReview narrows parent request review behind the route-level
// permissions. Administrators stay school-wide; group leaders and the whole
// team are opt-in per school.
type ParentRequestReview struct {
	caller *CallerContext
	deps   ParentRequestReviewDependencies
}

func NewParentRequestReview(caller *CallerContext, deps ParentRequestReviewDependencies) (*ParentRequestReview, error) {
	if caller == nil || deps.Permissions == nil || deps.AbsenceReadRequired == nil {
		return nil, errors.New("identity access parent request review: dependencies are required")
	}
	return &ParentRequestReview{caller: caller, deps: deps}, nil
}

// Scope reports the caller's reach: school-wide, or exactly the groups a
// group leader may review. It is the single evaluation list filtering,
// decisions and the navigation capability share.
func (p *ParentRequestReview) Scope(ctx context.Context, permissions []string) (bool, []int64, error) {
	facts := p.deps.Permissions(permissions)
	if p.caller.principal(ctx).EffectiveAdmin() || facts.AdminWildcard {
		return true, nil, nil
	}
	// users:absence alone never unlocks a read surface (#2232). Refusing
	// keeps the queue consistent with the write gate instead of returning an
	// empty list that looks like "no work".
	if facts.AbsenceReadPrerequisiteUnmet {
		return false, nil, p.deps.AbsenceReadRequired
	}
	if !facts.CanReviewExcused {
		return false, nil, nil
	}
	if p.deps.Settings == nil || p.deps.GroupLeaderSettingKey == "" || p.deps.RequestSettingKey == "" {
		return false, nil, errors.New("parent request review policy is not configured")
	}
	scope, err := p.deps.Settings.ResolveString(ctx, p.deps.RequestSettingKey)
	if err != nil {
		return false, nil, fmt.Errorf("resolve parent request review scope: %w", err)
	}
	if scope == reviewScopeInherit {
		enabled, err := p.deps.Settings.ResolveBool(ctx, p.deps.GroupLeaderSettingKey)
		if err != nil {
			return false, nil, fmt.Errorf("resolve group-leader request review setting: %w", err)
		}
		scope = reviewScopeAdmins
		if enabled {
			scope = reviewScopeGroupLeaders
		}
	}
	return p.scopeOf(ctx, scope)
}

// scopeOf evaluates an explicit scope for a caller who already passed the
// route-level permission facts.
func (p *ParentRequestReview) scopeOf(ctx context.Context, scope string) (bool, []int64, error) {
	switch scope {
	case reviewScopeAdmins:
		return false, nil, nil
	case reviewScopeGroupLeaders:
		groups, err := p.reviewGroupIDs(ctx)
		if err != nil {
			return false, nil, fmt.Errorf("resolve request review groups: %w", err)
		}
		return false, groups, nil
	case reviewScopeAllStaff:
		return p.allStaffScope(ctx)
	default:
		return false, nil, fmt.Errorf("unknown parent request review scope %q", scope)
	}
}

// AbsenceScope applies only to sick and excused parent requests. Its
// inherit value follows Scope, the scope of the other request kinds.
func (p *ParentRequestReview) AbsenceScope(ctx context.Context, permissions []string) (bool, []int64, error) {
	facts := p.deps.Permissions(permissions)
	if p.caller.principal(ctx).EffectiveAdmin() || facts.AdminWildcard {
		return true, nil, nil
	}
	if facts.AbsenceReadPrerequisiteUnmet {
		return false, nil, p.deps.AbsenceReadRequired
	}
	if !facts.CanReviewExcused {
		return false, nil, nil
	}
	if p.deps.Settings == nil || p.deps.AbsenceSettingKey == "" {
		return false, nil, errors.New("parent absence review policy is not configured")
	}
	scope, err := p.deps.Settings.ResolveString(ctx, p.deps.AbsenceSettingKey)
	if err != nil {
		return false, nil, fmt.Errorf("resolve parent absence review scope: %w", err)
	}
	if scope == reviewScopeInherit {
		return p.Scope(ctx, permissions)
	}
	return p.scopeOf(ctx, scope)
}

// allStaffScope lets every verified staff member of the token's own school
// review, but no platform, parent or cross-school token.
func (p *ParentRequestReview) allStaffScope(ctx context.Context) (bool, []int64, error) {
	caller := p.caller.principal(ctx)
	if caller.AccountID <= 0 || caller.ClaimsTenantID <= 0 || caller.ClaimsTenantID != caller.TenantID ||
		(caller.Scope != "" && caller.Scope != "tenant" && caller.Scope != "org") {
		return false, nil, nil
	}
	staffID, err := p.caller.StaffID(ctx)
	if err != nil {
		return false, nil, fmt.Errorf("resolve team reviewer staff: %w", err)
	}
	return staffID > 0, nil, nil
}

// AccessLevel reports the union of the queues the caller can review: why a
// queue may be empty, not what is in it. Each queue still applies its own
// scope to rows, notes, counts and decisions. Only administrators report
// admin; a school-wide team scope reports team.
func (p *ParentRequestReview) AccessLevel(ctx context.Context, permissions []string) (string, error) {
	if p.caller.principal(ctx).EffectiveAdmin() || p.deps.Permissions(permissions).AdminWildcard {
		return domain.ReviewAccessAdmin, nil
	}
	schoolWide, groups, err := p.Scope(ctx, permissions)
	if err != nil {
		return "", err
	}
	if schoolWide {
		return domain.ReviewAccessTeam, nil
	}
	absenceWide, absenceGroups, err := p.AbsenceScope(ctx, permissions)
	if err != nil {
		return "", err
	}
	if absenceWide {
		return domain.ReviewAccessTeam, nil
	}
	if len(groups) > 0 || len(absenceGroups) > 0 {
		return domain.ReviewAccessGroupLeader, nil
	}
	return domain.ReviewAccessNone, nil
}

// reviewGroupIDs returns the caller's groups sorted and without duplicates.
func (p *ParentRequestReview) reviewGroupIDs(ctx context.Context) ([]int64, error) {
	groups, err := p.caller.MyGroupIDs(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(groups))
	for _, id := range groups {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return slices.Compact(ids), nil
}
