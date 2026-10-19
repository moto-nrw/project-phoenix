package enrollment

import (
	"context"
	"fmt"
)

// CreatedStudentRequestChildIDs identifies source applications for care-exit
// cleanup. Matched-only links deliberately do not establish the created-student
// ownership used by that workflow.
func (m *Module) CreatedStudentRequestChildIDs(ctx context.Context, studentIDs []int64) ([]int64, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.CreatedStudentRequestChildIDs(ctx, studentIDs)
}

// CountStudentReferencesForDeletion counts request children that retain a
// created or matched student link. The approved count is the number the
// student deletion removes; the total lets it still lock and recheck an open,
// rejected, or withdrawn reference before the student goes. A child
// referencing the student through both links counts once.
func (m *Module) CountStudentReferencesForDeletion(ctx context.Context, studentID int64) (approved, total int, err error) {
	if err = m.transactions.RequireTransaction(ctx); err != nil {
		return 0, 0, err
	}
	return m.engine.CountStudentReferencesForDeletion(ctx, studentID)
}

// DeleteApprovedEnrollmentsOfStudent deletes the approved enrollment children
// a child was created from or matched to (#3962). The student deletion calls
// it in its transaction before the child goes: an accepted enrollment has no
// window of its own and leaves with its child. A request left without
// children goes as a whole, with its guardians, offerings and late invites.
// Open, rejected and withdrawn children keep their own paths. It returns the
// number of deleted enrollment children.
func (m *Module) DeleteApprovedEnrollmentsOfStudent(ctx context.Context, studentID int64) (int, error) {
	deleted := 0
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		links, err := m.engine.CareExitApplicationLinks(txCtx, []int64{studentID})
		if err != nil {
			return err
		}
		for _, link := range links {
			if link.Status != ChildStatusApproved {
				continue
			}
			removed, err := m.deleteApprovedChild(txCtx, link.ID, studentID)
			if err != nil {
				return err
			}
			if removed {
				deleted++
			}
		}
		return nil
	})
	return deleted, err
}

// deleteApprovedChild locks the child's request and its children, then
// rechecks that the child is still approved and linked to the departing
// student. It deletes the child, or the whole request when it is the only
// child, only when that locked recheck succeeds.
func (m *Module) deleteApprovedChild(ctx context.Context, childID, studentID int64) (bool, error) {
	child, err := m.engine.ChildByID(ctx, childID)
	if err != nil {
		return false, fmt.Errorf("read approved enrollment child %d: %w", childID, err)
	}
	if _, err := m.engine.RequestByID(ctx, child.RequestID, true); err != nil {
		return false, fmt.Errorf("lock enrollment request %d: %w", child.RequestID, err)
	}
	children, err := m.engine.ChildrenForRequest(ctx, child.RequestID, true)
	if err != nil {
		return false, fmt.Errorf("lock enrollment request children %d: %w", child.RequestID, err)
	}
	var locked *RequestChild
	for _, candidate := range children {
		if candidate.ID == childID {
			locked = candidate
			break
		}
	}
	if locked == nil || !approvedReferenceOfStudent(locked, studentID) {
		return false, nil
	}
	if len(children) == 1 && children[0].ID == childID {
		return true, m.engine.DeleteRequestTree(ctx, child.RequestID)
	}
	return true, m.engine.DeleteRequestChildTree(ctx, child.RequestID, childID)
}

func approvedReferenceOfStudent(child *RequestChild, studentID int64) bool {
	return child.Status == ChildStatusApproved &&
		(child.CreatedStudentID != nil && *child.CreatedStudentID == studentID ||
			child.MatchedStudentID != nil && *child.MatchedStudentID == studentID)
}
