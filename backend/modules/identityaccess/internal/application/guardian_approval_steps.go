package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (l *AccountLifecycle) attachApprovalAccount(ctx context.Context, profile *domain.GuardianProfile) error {
	if !profile.HasAccount {
		if email := strings.ToLower(strings.TrimSpace(profile.Email)); email != "" {
			if err := l.attachExistingAccountByEmail(ctx, profile, email); err != nil {
				return err
			}
		}
	}

	return nil
}

func (l *AccountLifecycle) approvalStudentNames(ctx context.Context, studentIDs []int64) (map[int64]domain.Student, map[int64]domain.PersonName) {
	students := map[int64]domain.Student{}
	if len(studentIDs) > 0 {
		loaded, err := l.guardians.FindStudents(ctx, studentIDs)
		if err != nil {
			l.logger.Error("failed to hydrate pending guardian approvals",
				"relation", "students",
				"error", err,
			)
		} else {
			students = loaded
		}
	}
	personIDs := make([]int64, 0, len(students))
	for _, student := range students {
		personIDs = append(personIDs, student.PersonID)
	}
	persons := map[int64]domain.PersonName{}
	if len(personIDs) > 0 {
		loaded, err := l.guardians.FindPersonNamesByIDs(ctx, personIDs)
		if err != nil {
			l.logger.Error("failed to hydrate pending guardian approvals",
				"relation", "persons",
				"error", err,
			)
		} else {
			persons = loaded
		}
	}
	return students, persons
}

func (l *AccountLifecycle) approvalRequesterEmails(ctx context.Context, accountIDs []int64) map[int64]string {
	emails := map[int64]string{}
	if len(accountIDs) > 0 {
		loaded, _, err := l.store.ListAccountEmails(ctx, accountIDs)
		if err != nil {
			l.logger.Error("failed to hydrate pending guardian approvals",
				"relation", "requester_emails",
				"error", err,
			)
		} else {
			emails = loaded
		}
	}

	return emails
}

func pendingApprovalView(inv domain.GuardianInvitation, profiles map[int64]domain.GuardianProfile, students map[int64]domain.Student, persons map[int64]domain.PersonName, emails map[int64]string) domain.PendingApprovalView {
	view := domain.PendingApprovalView{
		InvitationID: inv.ID, GuardianProfileID: inv.GuardianProfileID,
		CreatedAt: inv.CreatedAt, ExpiresAt: inv.ExpiresAt, RoleUpgrade: inv.RoleUpgrade,
	}
	if profile, ok := profiles[inv.GuardianProfileID]; ok {
		view.GuardianName = profile.FullName()
		view.GuardianEmail = strings.TrimSpace(profile.Email)
	}
	if inv.StudentID != nil {
		view.StudentID = *inv.StudentID
		if student, ok := students[*inv.StudentID]; ok {
			if person, ok := persons[student.PersonID]; ok {
				view.StudentName = strings.TrimSpace(person.FirstName + " " + person.LastName)
			}
		}
	}
	if inv.RequestedByAccountID != nil {
		view.RequestedByEmail = emails[*inv.RequestedByAccountID]
	}
	return view
}

func (l *AccountLifecycle) requeueRoleUpgrade(ctx context.Context, req domain.InviteToStudentRequest, openInvitation domain.GuardianInvitation, approvalStatus string, roleUpgrade bool) (domain.GuardianInvitation, error) {
	// Reverse transition, ONLY for a confirmed role upgrade: the upgrade
	// requires approval-gating, but the open invitation was created
	// without it. Re-queue it as pending so the staff queue surfaces it
	// and the outstanding token is frozen until staff decide. The
	// roleUpgrade gate is load-bearing: a plain duplicate re-invite must
	// not revert an already-resolved invitation.
	if roleUpgrade &&
		approvalStatus == domain.GuardianInvitationApprovalPending &&
		openInvitation.ApprovalStatus != domain.GuardianInvitationApprovalPending {
		openInvitation.ApprovalStatus = domain.GuardianInvitationApprovalPending
		openInvitation.ApprovedBy = nil
		openInvitation.ApprovedAt = nil
		// Restart the window and drop the stale email tracking, exactly
		// like the pending -> not_required branch.
		expiry, err := l.delivery.InvitationExpiry(ctx)
		if err != nil {
			return domain.GuardianInvitation{}, failed(opGuardianInviteToStudent, err)
		}
		openInvitation.ExpiresAt = time.Now().Add(expiry)
		openInvitation.EmailSentAt = nil
		openInvitation.EmailError = nil
		if req.RequestedByParentAccountID != nil {
			openInvitation.RequestedByAccountID = req.RequestedByParentAccountID
		}
		if err := l.invitations.UpdateGuardianInvitation(ctx, openInvitation); err != nil {
			return domain.GuardianInvitation{}, failed(opGuardianInviteToStudent, err)
		}
	}
	return openInvitation, nil
}

func (l *AccountLifecycle) parentRevokeDeletesLink(ctx context.Context, req domain.RevokeAccessRequest) (bool, error) {
	deleteLink := true
	if req.ByParent {
		profile, found, err := l.guardians.FindGuardianProfile(ctx, req.GuardianProfileID)
		if err != nil {
			return false, failed(opGuardianRevokeAccess, err)
		}
		if !found {
			return false, failed(opGuardianRevokeAccess, fmt.Errorf("guardian profile not found"))
		}
		// The parent flow manages ANOTHER account's access. A parent must not
		// be able to revoke their own link to the child.
		if profile.AccountID != nil && *profile.AccountID == req.ActorAccountID {
			return false, failed(opGuardianRevokeAccess, identityaccess.ErrCannotRemoveOwnAccess)
		}
		return l.cancelPendingStudentInvitations(ctx, req, profile)
	}
	return deleteLink, nil
}

func (l *AccountLifecycle) cancelPendingStudentInvitations(ctx context.Context, req domain.RevokeAccessRequest, profile domain.GuardianProfile) (bool, error) {
	deleteLink := true
	if !profile.HasAccount {
		now := time.Now()
		openInvitations, err := l.openStudentInvitations(ctx, req.GuardianProfileID, req.StudentID, now)
		if err != nil {
			return false, failed(opGuardianRevokeAccess, err)
		}
		if len(openInvitations) == 0 {
			return false, failed(opGuardianRevokeAccess, identityaccess.ErrCannotRemoveStaffManagedGuardian)
		}
		if err := l.expireInvitations(ctx, openInvitations, now); err != nil {
			return false, failed(opGuardianRevokeAccess, err)
		}
		deleteLink = false
		for _, inv := range openInvitations {
			if inv.ProfileCreatedForInvitation {
				deleteLink = true
				break
			}
		}
	}
	return deleteLink, nil
}
