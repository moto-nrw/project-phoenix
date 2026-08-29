package usercontext

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
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
	if authorize.HasEffectiveAdminScope(ctx) || authorize.HasAdminWildcard(permissions) {
		return func(student *users.Student) bool { return student != nil }, nil
	}
	if p == nil || p.settings == nil || p.groups == nil || p.settingKey == "" {
		return nil, fmt.Errorf("parent request review policy is not configured")
	}
	enabled, err := p.settings.ResolveBool(ctx, p.settingKey)
	if err != nil {
		return nil, fmt.Errorf("resolve group-leader request review setting: %w", err)
	}
	if !enabled {
		return func(*users.Student) bool { return false }, nil
	}
	groups, err := p.groups.GetMyGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve request review groups: %w", err)
	}
	groupIDs := make(map[int64]struct{}, len(groups))
	for _, group := range groups {
		if group != nil && group.ID > 0 {
			groupIDs[group.ID] = struct{}{}
		}
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
