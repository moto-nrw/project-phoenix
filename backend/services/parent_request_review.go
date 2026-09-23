package services

import (
	"context"
	"errors"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

var errReviewPolicyNotConfigured = errors.New("parent request review policy is not configured")

// ParentRequestReviewPolicy is the one cross-domain decision about WHO may
// see and decide parent requests. The Identity & Access caller context
// decides the scope; this adapter applies it to the retained student rows
// the request services filter.
type ParentRequestReviewPolicy struct {
	reviews identityaccess.ParentRequestReviews
}

func NewParentRequestReviewPolicy(reviews identityaccess.ParentRequestReviews) *ParentRequestReviewPolicy {
	return &ParentRequestReviewPolicy{reviews: reviews}
}

// ReviewPermissions evaluates the route-level permission facts the review
// policy narrows.
func ReviewPermissions(permissions []string) identityaccess.ReviewPermissions {
	return identityaccess.ReviewPermissions{
		AdminWildcard:                securityruntime.HasAdminWildcard(permissions),
		AbsenceReadPrerequisiteUnmet: securityruntime.AbsenceReadPrerequisiteUnmet(permissions),
		CanReviewExcused:             securityruntime.CanReviewExcusedAbsenceRequests(permissions),
	}
}

// Scope reports the caller's reach: school-wide, or exactly the groups a
// group leader may review.
func (p *ParentRequestReviewPolicy) Scope(ctx context.Context, permissions []string) (bool, []int64, error) {
	if p == nil || p.reviews == nil {
		return false, nil, errReviewPolicyNotConfigured
	}
	return p.reviews.ReviewScope(ctx, permissions)
}

// AbsenceScope applies only to sick and excused parent requests.
func (p *ParentRequestReviewPolicy) AbsenceScope(ctx context.Context, permissions []string) (bool, []int64, error) {
	if p == nil || p.reviews == nil {
		return false, nil, errReviewPolicyNotConfigured
	}
	return p.reviews.AbsenceReviewScope(ctx, permissions)
}

// AccessLevel reports the union of the queues the caller can review.
func (p *ParentRequestReviewPolicy) AccessLevel(ctx context.Context, permissions []string) (string, error) {
	if p == nil || p.reviews == nil {
		return "", errReviewPolicyNotConfigured
	}
	return p.reviews.ReviewAccessLevel(ctx, permissions)
}

// StudentFilter resolves one immutable request-scoped filter. Resolving once
// keeps queue filtering O(1) per child and prevents list and decision checks
// from drifting.
func (p *ParentRequestReviewPolicy) StudentFilter(ctx context.Context, permissions []string) (func(*userModels.Student) bool, error) {
	schoolWide, groupIDs, err := p.Scope(ctx, permissions)
	if err != nil {
		return nil, err
	}
	if schoolWide {
		return func(student *userModels.Student) bool { return student != nil }, nil
	}
	groups := make(map[int64]struct{}, len(groupIDs))
	for _, id := range groupIDs {
		groups[id] = struct{}{}
	}
	return func(student *userModels.Student) bool {
		if student == nil || student.GroupID == nil {
			return false
		}
		_, ok := groups[*student.GroupID]
		return ok
	}, nil
}

// Allows applies the same scope to a single decision.
func (p *ParentRequestReviewPolicy) Allows(ctx context.Context, permissions []string, student *userModels.Student) (bool, error) {
	filter, err := p.StudentFilter(ctx, permissions)
	if err != nil {
		return false, err
	}
	return filter(student), nil
}
