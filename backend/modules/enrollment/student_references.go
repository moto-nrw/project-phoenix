package enrollment

import (
	"context"
	"fmt"
)

// CreatedStudentRequestChildIDs identifies source applications for care-exit
// cleanup. Matched-only links deliberately do not establish the created-student
// ownership used by that workflow.
func (m *Module) CreatedStudentRequestChildIDs(ctx context.Context, studentIDs []int64) ([]int64, error) {
	var ids []int64
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		ids, err = m.engine.CreatedStudentRequestChildIDs(txCtx, studentIDs)
		return err
	})
	return ids, err
}

// CountStudentReferences counts request children that retain a created or matched
// student link. A child referencing the student through both links counts once.
func (m *Module) CountStudentReferences(ctx context.Context, studentID int64) (int, error) {
	var count int
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		count, err = m.engine.CountStudentReferences(txCtx, studentID)
		return err
	})
	return count, err
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
			if err := m.deleteApprovedChild(txCtx, link.ID); err != nil {
				return err
			}
			deleted++
		}
		return nil
	})
	return deleted, err
}

// deleteApprovedChild locks the child's request and its children, then
// deletes the child, or the whole request when it is the only child.
func (m *Module) deleteApprovedChild(ctx context.Context, childID int64) error {
	child, err := m.engine.ChildByID(ctx, childID)
	if err != nil {
		return fmt.Errorf("read approved enrollment child %d: %w", childID, err)
	}
	if _, err := m.engine.RequestByID(ctx, child.RequestID, true); err != nil {
		return fmt.Errorf("lock enrollment request %d: %w", child.RequestID, err)
	}
	children, err := m.engine.ChildrenForRequest(ctx, child.RequestID, true)
	if err != nil {
		return fmt.Errorf("lock enrollment request children %d: %w", child.RequestID, err)
	}
	if len(children) == 1 && children[0].ID == childID {
		return m.engine.DeleteRequestTree(ctx, child.RequestID)
	}
	return m.engine.DeleteRequestChildTree(ctx, child.RequestID, childID)
}
