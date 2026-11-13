//go:build testhooks

package owners

import (
	"context"
	"errors"

	parentportal "github.com/moto-ogs/project-phoenix/workflows/parentportal"
)

// The hooks below let the root's route tests of the child write guard (#6529)
// park a parent write at a chosen point and stand in for owner commands whose
// own preconditions their owners test. They bind the workflow's ports and
// add no behaviour of their own.

// ParentChildReadsAfterCheck calls park after each decided child lookup, the
// point between the check before the unit of work and the unit itself.
func ParentChildReadsAfterCheck(inner parentportal.ChildReads, park func()) parentportal.ChildReads {
	return parkedParentChildReads{ChildReads: inner, park: park}
}

type parkedParentChildReads struct {
	parentportal.ChildReads
	park func()
}

func (r parkedParentChildReads) FindForAccount(ctx context.Context, accountID, studentID int64) (*parentportal.ChildSummary, error) {
	child, err := r.ChildReads.FindForAccount(ctx, accountID, studentID)
	r.park()
	return child, err
}

// ParentRelationshipLockHook calls afterLock with the write's context once
// the guard holds the relationship row.
func ParentRelationshipLockHook(inner parentportal.StudentGuardianReads, afterLock func(context.Context)) parentportal.StudentGuardianReads {
	return parentportal.StudentGuardianReadsAfterLock(inner, afterLock)
}

// OwnerWriteMark writes one row of the child in the request's transaction.
type OwnerWriteMark func(ctx context.Context, studentID, accountID int64) error

// MarkingParentOfferings keeps Enrollment's reads and marks the request
// commands instead of running them.
func MarkingParentOfferings(inner parentportal.OfferingChangeRequests, mark OwnerWriteMark) parentportal.OfferingChangeRequests {
	return markingParentOfferings{OfferingChangeRequests: inner, mark: mark}
}

type markingParentOfferings struct {
	parentportal.OfferingChangeRequests
	mark OwnerWriteMark
}

func (m markingParentOfferings) SubmitOfferingChange(ctx context.Context, input parentportal.CreateOfferingChangeInputSnapshot) (*parentportal.OfferingChangeRequestSnapshot, error) {
	return &parentportal.OfferingChangeRequestSnapshot{}, m.mark(ctx, input.StudentID, input.AccountID)
}

func (m markingParentOfferings) Edit(ctx context.Context, _ int64, input parentportal.CreateOfferingChangeInputSnapshot, _ string) (*parentportal.OfferingChangeRequestSnapshot, error) {
	return &parentportal.OfferingChangeRequestSnapshot{}, m.mark(ctx, input.StudentID, input.AccountID)
}

func (m markingParentOfferings) CreateCourseRequest(ctx context.Context, input parentportal.CreateCourseRequestInputSnapshot) (*parentportal.OfferingChangeRequestSnapshot, error) {
	return &parentportal.OfferingChangeRequestSnapshot{}, m.mark(ctx, input.StudentID, input.AccountID)
}

func (m markingParentOfferings) WithdrawCourseRequest(ctx context.Context, _ int64, accountID, studentID int64) error {
	return m.mark(ctx, studentID, accountID)
}

// MarkingParentGuardianAccess stands for Identity & Access's relative-access
// flows.
func MarkingParentGuardianAccess(mark OwnerWriteMark) parentportal.GuardianAccess {
	return markingParentGuardianAccess{mark: mark}
}

type markingParentGuardianAccess struct{ mark OwnerWriteMark }

func (m markingParentGuardianAccess) InviteToStudent(ctx context.Context, request parentportal.GuardianInviteRequest) (parentportal.GuardianInviteOutcome, error) {
	return parentportal.GuardianInviteOutcome{Outcome: "invited"}, m.mark(ctx, request.StudentID, request.RequestedByAccountID)
}

func (m markingParentGuardianAccess) RevokeAccess(ctx context.Context, revocation parentportal.GuardianAccessRevocation) error {
	return m.mark(ctx, revocation.StudentID, revocation.ActorAccountID)
}

// ParentConferenceBooking runs write as the calendar's booking of the
// Elternsprechtag, in the unit the portal guards; the booking route belongs to
// the calendar's resource.
func ParentConferenceBooking(portal *parentportal.Portal) func(ctx context.Context, accountID, studentID int64, write func(context.Context) error) error {
	return func(ctx context.Context, accountID, studentID int64, write func(context.Context) error) error {
		return portal.WithParentConferenceBooking(ctx, accountID, studentID, func(txCtx context.Context, _ *parentportal.Child) error {
			return write(txCtx)
		})
	}
}

// IsParentChildRefusal reports the refusals of the child check and its guard.
func IsParentChildRefusal(err error) bool {
	return errors.Is(err, parentportal.ErrChildNotLinked) || errors.Is(err, parentportal.ErrGuardianPermissionDenied)
}
