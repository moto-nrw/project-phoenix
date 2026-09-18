package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Guardian relative access: invite a further guardian to a child, approve or
// reject parent-initiated requests, and revoke one account's access to one
// child. Shared by the staff "Erziehungsberechtigte" tab and the parents
// portal. Parent portal authority is relationship-scoped: the link's guardian
// role decides, never membership alone.

const (
	opGuardianInviteToStudent = "invite guardian to student"
	opGuardianInviteApprove   = "approve guardian invitation"
	opGuardianInviteReject    = "reject guardian invitation"
	opGuardianRevokeAccess    = "revoke guardian access"

	// defaultRelationshipType is used when an invite doesn't specify one.
	defaultRelationshipType = "guardian"
)

type resolvedProfile struct {
	profile domain.GuardianProfile
	created bool
}

// InviteToStudent resolves an email against existing data and either links an
// existing account to the child, (re)sends an invitation, or queues a parent
// request for staff approval. Tenant comes from context.
func (l *AccountLifecycle) InviteToStudent(ctx context.Context, req domain.InviteToStudentRequest) (*domain.InviteToStudentResult, error) {
	if req.StudentID <= 0 {
		return nil, failed(opGuardianInviteToStudent, fmt.Errorf("student ID is required"))
	}
	if req.CreatedBy <= 0 {
		return nil, failed(opGuardianInviteToStudent, fmt.Errorf("created_by is required"))
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		return nil, failed(opGuardianInviteToStudent, fmt.Errorf("email is required"))
	}

	tenantID := l.runtime.TenantID(ctx)

	// An existing restrictive contact link would survive the link-if-absent
	// untouched, leaving the invited account without parent_portal.access
	// (#2172). Detect it up front, with no side effects, and require an
	// explicit upgrade confirmation.
	roleUpgrade, restricted, err := l.detectRestrictedContact(ctx, req, email)
	if err != nil {
		return nil, err
	}
	if restricted != nil {
		return restricted, nil
	}

	resolved, err := l.findOrCreateProfileByEmail(ctx, email, req.FirstName, req.LastName, tenantID)
	if err != nil {
		return nil, err
	}
	profile := resolved.profile

	// A confirmed upgrade of an existing restricted contact overrides a
	// deliberate staff-set restriction. When a PARENT asks for it, the request
	// always goes through staff approval, even in direct invite mode.
	requireApproval := req.RequireApproval
	if roleUpgrade && req.RequestedByParentAccountID != nil {
		requireApproval = true
	}
	if requireApproval {
		return l.queuePendingApproval(ctx, req, profile, tenantID, resolved.created, roleUpgrade)
	}
	if err := l.attachExistingAccountByEmail(ctx, &profile, email); err != nil {
		return nil, err
	}
	return l.resolveInviteNow(ctx, req, profile, tenantID, resolved.created, roleUpgrade)
}

// detectRestrictedContact checks whether the invited email already belongs to
// a contact on this child whose link has a restrictive role (no portal
// access). Lookup-only.
func (l *AccountLifecycle) detectRestrictedContact(ctx context.Context, req domain.InviteToStudentRequest, email string) (bool, *domain.InviteToStudentResult, error) {
	existing, found, err := l.guardians.FindGuardianProfileByEmail(ctx, email)
	if err != nil {
		return false, nil, failed(opGuardianInviteToStudent, err)
	}
	if !found {
		return false, nil, nil
	}
	link, found, err := l.guardians.FindStudentGuardianLinkForUpdate(ctx, req.StudentID, existing.ID)
	if err != nil {
		return false, nil, failed(opGuardianInviteToStudent, err)
	}
	if !found {
		return false, nil, nil
	}
	switch l.guardians.GuardianRoleClass(link.GuardianRole) {
	case domain.GuardianRoleFull:
		return false, nil, nil
	case domain.GuardianRoleSocialWorker:
		// A social worker is a school-managed professional contact, never a
		// candidate for the legal-guardian upgrade; refuse the invite outright.
		return false, nil, failed(opGuardianInviteToStudent, domain.ErrInviteSocialWorkerManaged)
	}
	if !req.ConfirmRoleUpgrade {
		return false, &domain.InviteToStudentResult{
			Outcome:           domain.InviteOutcomeExistingContactRestricted,
			GuardianProfileID: existing.ID,
			ExistingRole:      link.GuardianRole,
		}, nil
	}
	return true, nil, nil
}

// upgradeStudentLink promotes an existing restrictive link to legal_guardian
// (#2172). No-op when the link already carries a full guardian role; refused
// when it became a school-managed social-worker contact meanwhile.
func (l *AccountLifecycle) upgradeStudentLink(ctx context.Context, studentID, guardianProfileID int64, op string) error {
	link, found, err := l.guardians.FindStudentGuardianLinkForUpdate(ctx, studentID, guardianProfileID)
	if err != nil {
		return failed(op, err)
	}
	if !found {
		return failed(op, fmt.Errorf("student guardian relationship not found"))
	}
	switch l.guardians.GuardianRoleClass(link.GuardianRole) {
	case domain.GuardianRoleFull:
		return nil
	case domain.GuardianRoleSocialWorker:
		l.logger.Warn("guardian contact upgrade refused: social-worker link is school-managed",
			slog.Int64("student_id", studentID),
			slog.Int64("guardian_profile_id", guardianProfileID),
		)
		return failed(op, domain.ErrInviteSocialWorkerManaged)
	}
	if err := l.guardians.PromoteStudentGuardianLink(ctx, link.ID); err != nil {
		return failed(op, err)
	}
	l.logger.Info("guardian contact link upgraded to full access",
		slog.Int64("student_id", studentID),
		slog.Int64("guardian_profile_id", guardianProfileID),
	)
	return nil
}

// findOrCreateProfileByEmail returns the guardian profile for this email,
// creating a fresh contact record when none exists.
func (l *AccountLifecycle) findOrCreateProfileByEmail(ctx context.Context, email, firstName, lastName string, tenantID int64) (resolvedProfile, error) {
	existing, found, err := l.guardians.FindGuardianProfileByEmail(ctx, email)
	if err != nil {
		return resolvedProfile{}, failed(opGuardianInviteToStudent, err)
	}
	if found {
		return resolvedProfile{profile: existing}, nil
	}
	profile := domain.GuardianProfile{TenantID: tenantID, FirstName: strings.TrimSpace(firstName), LastName: strings.TrimSpace(lastName), Email: email}
	profile.ID, err = l.guardians.CreateGuardianProfile(ctx, profile)
	if err != nil {
		return resolvedProfile{}, failed(opGuardianInviteToStudent, err)
	}
	return resolvedProfile{profile: profile, created: true}, nil
}

// attachExistingAccountByEmail links the profile to the platform account
// that already owns the address: guardian role and school mapping through
// the module's own grant, then the profile link.
func (l *AccountLifecycle) attachExistingAccountByEmail(ctx context.Context, profile *domain.GuardianProfile, email string) error {
	if profile.HasAccount {
		return nil
	}
	account, err := l.sessions.FindAccountByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrAccountNotFound) {
			return nil
		}
		return failed(opGuardianInviteToStudent, err)
	}
	if _, err := l.sessions.GrantGuardianTenantAccess(ctx, account.ID); err != nil {
		return failed(opGuardianInviteToStudent, fmt.Errorf("link account to tenant: %w", err))
	}
	if err := l.guardians.LinkGuardianProfileToAccount(ctx, profile.ID, account.ID); err != nil {
		return failed(opGuardianInviteToStudent, fmt.Errorf("link guardian profile to account: %w", err))
	}
	accountID := account.ID
	profile.AccountID = &accountID
	profile.HasAccount = true
	return nil
}

// resolveInviteNow links/invites immediately (staff invites, or parent invites
// in "direct" mode). roleUpgrade applies a confirmed restrictive-contact
// upgrade right away.
func (l *AccountLifecycle) resolveInviteNow(ctx context.Context, req domain.InviteToStudentRequest, profile domain.GuardianProfile, tenantID int64, profileCreated, roleUpgrade bool) (*domain.InviteToStudentResult, error) {
	rel := req.RelationshipType
	if rel == "" {
		rel = defaultRelationshipType
	}
	linkCreated, err := l.ensureStudentLink(ctx, req.StudentID, profile.ID, rel, tenantID)
	if err != nil {
		return nil, err
	}
	if roleUpgrade {
		if err := l.upgradeStudentLink(ctx, req.StudentID, profile.ID, opGuardianInviteToStudent); err != nil {
			return nil, err
		}
	}

	// Existing account: access is granted by the link alone; no token needed.
	// The account holder still gets a mail pointing at the parents portal
	// login (#3320), otherwise nobody tells them the access exists.
	if profile.HasAccount {
		if err := l.closeSupersededApprovalRequests(ctx, profile.ID, req.StudentID); err != nil {
			return nil, err
		}
		l.delivery.EnqueueExistingAccountEmail(ctx, profile, l.delivery.SchoolName(ctx, tenantID))
		outcome := domain.InviteOutcomeLinkedExistingAccount
		if !linkCreated {
			outcome = domain.InviteOutcomeAlreadyLinked
		}
		l.logger.Info("guardian related-account linked to existing account",
			slog.Int64("student_id", req.StudentID),
			slog.Int64("guardian_profile_id", profile.ID),
			slog.Int64("created_by", req.CreatedBy),
			slog.String("outcome", string(outcome)),
		)
		return &domain.InviteToStudentResult{Outcome: outcome, GuardianProfileID: profile.ID}, nil
	}

	// No account yet: issue a token invitation for this child. The upgrade
	// (if any) was already applied above, so the row's flag stays false.
	invitation, err := l.createStudentInvitation(ctx, req, profile, tenantID, domain.GuardianInvitationApprovalNotRequired, profileCreated, false)
	if err != nil {
		return nil, err
	}
	l.delivery.EnqueueInvitationEmail(ctx, invitation, profile, l.delivery.SchoolName(ctx, tenantID))
	return &domain.InviteToStudentResult{
		Outcome:           domain.InviteOutcomeInvited,
		GuardianProfileID: profile.ID,
		InvitationID:      &invitation.ID,
	}, nil
}

// queuePendingApproval records a parent-initiated request awaiting staff
// approval. No link is created and no email is sent until staff approve.
func (l *AccountLifecycle) queuePendingApproval(ctx context.Context, req domain.InviteToStudentRequest, profile domain.GuardianProfile, tenantID int64, profileCreated, roleUpgrade bool) (*domain.InviteToStudentResult, error) {
	invitation, err := l.createStudentInvitation(ctx, req, profile, tenantID, domain.GuardianInvitationApprovalPending, profileCreated, roleUpgrade)
	if err != nil {
		return nil, err
	}
	l.logger.Info("guardian related-account invite queued for approval",
		slog.Int64("student_id", req.StudentID),
		slog.Int64("guardian_profile_id", profile.ID),
		slog.Int64("created_by", req.CreatedBy),
	)
	return &domain.InviteToStudentResult{
		Outcome:           domain.InviteOutcomePendingApproval,
		GuardianProfileID: profile.ID,
		InvitationID:      &invitation.ID,
	}, nil
}

// ensureStudentLink inserts the student-guardian link if it doesn't already
// exist. Returns true when a new link row was created.
func (l *AccountLifecycle) ensureStudentLink(ctx context.Context, studentID, guardianProfileID int64, relationshipType string, tenantID int64) (bool, error) {
	created, err := l.guardians.LinkStudentGuardianIfAbsent(ctx, domain.StudentGuardianLink{
		TenantID: tenantID, StudentID: studentID, GuardianProfileID: guardianProfileID,
		RelationshipType: relationshipType, EmergencyPriority: 1,
	})
	if err != nil {
		return false, failed(opGuardianInviteToStudent, err)
	}
	return created, nil
}

// createStudentInvitation persists a guardian_invitations row carrying the
// student and (for parent-initiated invites) the requesting account, reusing
// an open invitation for the same child and profile.
func (l *AccountLifecycle) createStudentInvitation(ctx context.Context, req domain.InviteToStudentRequest, profile domain.GuardianProfile, tenantID int64, approvalStatus string, profileCreated, roleUpgrade bool) (domain.GuardianInvitation, error) {
	studentID := req.StudentID
	open, err := l.openStudentInvitations(ctx, profile.ID, studentID, time.Now())
	if err != nil {
		return domain.GuardianInvitation{}, failed(opGuardianInviteToStudent, err)
	}
	if len(open) > 0 {
		openInvitation := open[0]
		if openInvitation.ApprovalStatus == domain.GuardianInvitationApprovalPending &&
			approvalStatus == domain.GuardianInvitationApprovalNotRequired {
			openInvitation.ApprovalStatus = domain.GuardianInvitationApprovalNotRequired
			openInvitation.ApprovedBy = nil
			openInvitation.ApprovedAt = nil
			openInvitation.ExpiresAt = time.Now().Add(l.delivery.InvitationExpiry(ctx))
			openInvitation.EmailSentAt = nil
			openInvitation.EmailError = nil
			if err := l.invitations.UpdateGuardianInvitation(ctx, openInvitation); err != nil {
				return domain.GuardianInvitation{}, failed(opGuardianInviteToStudent, err)
			}
		}
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
			openInvitation.ExpiresAt = time.Now().Add(l.delivery.InvitationExpiry(ctx))
			openInvitation.EmailSentAt = nil
			openInvitation.EmailError = nil
			if req.RequestedByParentAccountID != nil {
				openInvitation.RequestedByAccountID = req.RequestedByParentAccountID
			}
			if err := l.invitations.UpdateGuardianInvitation(ctx, openInvitation); err != nil {
				return domain.GuardianInvitation{}, failed(opGuardianInviteToStudent, err)
			}
		}
		// A re-invite that confirmed a restrictive-contact upgrade must stick
		// the flag onto the reused row, or the approval would silently skip
		// the upgrade (#2172).
		if roleUpgrade && !openInvitation.RoleUpgrade {
			openInvitation.RoleUpgrade = true
			if err := l.invitations.UpdateGuardianInvitation(ctx, openInvitation); err != nil {
				return domain.GuardianInvitation{}, failed(opGuardianInviteToStudent, err)
			}
		}
		return openInvitation, nil
	}
	invitation, err := l.invitations.InsertGuardianInvitation(ctx, domain.GuardianInvitation{
		TenantID:                    tenantID,
		Token:                       uuid.Must(uuid.NewV4()).String(),
		GuardianProfileID:           profile.ID,
		CreatedBy:                   req.CreatedBy,
		ExpiresAt:                   time.Now().Add(l.delivery.InvitationExpiry(ctx)),
		StudentID:                   &studentID,
		RequestedByAccountID:        req.RequestedByParentAccountID,
		ApprovalStatus:              approvalStatus,
		ProfileCreatedForInvitation: profileCreated,
		RoleUpgrade:                 roleUpgrade,
	})
	if err != nil {
		return domain.GuardianInvitation{}, failed(opGuardianInviteToStudent, err)
	}
	return invitation, nil
}

// closeSupersededApprovalRequests resolves parent-initiated requests for this
// child that a direct link just overtook: the account now has access, so a
// still-pending row would linger in the staff queue forever.
func (l *AccountLifecycle) closeSupersededApprovalRequests(ctx context.Context, guardianProfileID, studentID int64) error {
	now := time.Now()
	open, err := l.openStudentInvitations(ctx, guardianProfileID, studentID, now)
	if err != nil {
		return failed(opGuardianInviteToStudent, err)
	}
	for _, inv := range open {
		if !inv.IsPendingApproval() {
			continue
		}
		inv.ApprovalStatus = domain.GuardianInvitationApprovalNotRequired
		inv.ApprovedBy = nil
		inv.ApprovedAt = nil
		accepted := now
		inv.AcceptedAt = &accepted
		if err := l.invitations.UpdateGuardianInvitation(ctx, inv); err != nil {
			return failed(opGuardianInviteToStudent, err)
		}
		l.logger.Info("guardian approval request closed: access granted directly",
			slog.Int64("invitation_id", inv.ID),
			slog.Int64("student_id", studentID),
			slog.Int64("guardian_profile_id", guardianProfileID),
		)
	}
	return nil
}

// ApproveInvitation resolves a pending parent-initiated request: it links the
// child and either grants access to an existing account or dispatches the
// invitation email. A promised role upgrade that can no longer be applied
// aborts the approval: the request stays pending so staff can reject it.
func (l *AccountLifecycle) ApproveInvitation(ctx context.Context, invitationID, approverAccountID int64) error {
	invitation, err := l.pendingInvitation(ctx, opGuardianInviteApprove, invitationID)
	if err != nil {
		return err
	}

	profile, found, err := l.guardians.FindGuardianProfile(ctx, invitation.GuardianProfileID)
	if err != nil {
		return failed(opGuardianInviteApprove, err)
	}
	if !found {
		return failed(opGuardianInviteApprove, fmt.Errorf("guardian profile not found"))
	}
	if !profile.HasAccount {
		if email := strings.ToLower(strings.TrimSpace(profile.Email)); email != "" {
			if err := l.attachExistingAccountByEmail(ctx, &profile, email); err != nil {
				return err
			}
		}
	}

	if invitation.StudentID != nil {
		if _, err := l.ensureStudentLink(ctx, *invitation.StudentID, profile.ID, defaultRelationshipType, invitation.TenantID); err != nil {
			return err
		}
		if invitation.RoleUpgrade {
			if err := l.upgradeStudentLink(ctx, *invitation.StudentID, profile.ID, opGuardianInviteApprove); err != nil {
				return err
			}
		}
	}

	now := time.Now()
	invitation.ApprovalStatus = domain.GuardianInvitationApprovalApproved
	invitation.ApprovedBy = &approverAccountID
	invitation.ApprovedAt = &now

	// Existing account: access is granted by the link; close the invitation.
	if profile.HasAccount {
		invitation.AcceptedAt = &now
		if err := l.invitations.UpdateGuardianInvitation(ctx, invitation); err != nil {
			return failed(opGuardianInviteApprove, err)
		}
		l.logger.Info("guardian invitation approved (existing account)",
			slog.Int64("invitation_id", invitation.ID),
			slog.Int64("approver_account_id", approverAccountID),
		)
		l.delivery.EnqueueExistingAccountEmail(ctx, profile, l.delivery.SchoolName(ctx, invitation.TenantID))
		return nil
	}

	// No account yet: refresh expiry, clear stale email tracking, dispatch.
	invitation.ExpiresAt = now.Add(l.delivery.InvitationExpiry(ctx))
	invitation.EmailSentAt = nil
	invitation.EmailError = nil
	if err := l.invitations.UpdateGuardianInvitation(ctx, invitation); err != nil {
		return failed(opGuardianInviteApprove, err)
	}
	l.logger.Info("guardian invitation approved",
		slog.Int64("invitation_id", invitation.ID),
		slog.Int64("approver_account_id", approverAccountID),
	)
	l.delivery.EnqueueInvitationEmail(ctx, invitation, profile, l.delivery.SchoolName(ctx, invitation.TenantID))
	return nil
}

// RejectInvitation marks a pending parent-initiated request as rejected. No
// access is granted. A profile created solely for this request is cleaned up.
func (l *AccountLifecycle) RejectInvitation(ctx context.Context, invitationID, approverAccountID int64) error {
	invitation, err := l.pendingInvitation(ctx, opGuardianInviteReject, invitationID)
	if err != nil {
		return err
	}
	now := time.Now()
	invitation.ApprovalStatus = domain.GuardianInvitationApprovalRejected
	invitation.ApprovedBy = &approverAccountID
	invitation.ApprovedAt = &now
	if err := l.invitations.UpdateGuardianInvitation(ctx, invitation); err != nil {
		return failed(opGuardianInviteReject, err)
	}

	l.cleanupOrphanProfile(ctx, invitation)

	l.logger.Info("guardian invitation rejected",
		slog.Int64("invitation_id", invitation.ID),
		slog.Int64("approver_account_id", approverAccountID),
	)
	return nil
}

// PendingInvitationStudentID returns the child targeted by a pending approval
// request, for the per-student authorization gate staff handlers apply
// before approve/reject mutates anything.
func (l *AccountLifecycle) PendingInvitationStudentID(ctx context.Context, invitationID int64) (int64, error) {
	invitation, err := l.pendingInvitation(ctx, opGuardianInviteApprove, invitationID)
	if err != nil {
		return 0, err
	}
	if invitation.StudentID == nil || *invitation.StudentID <= 0 {
		return 0, failed(opGuardianInviteApprove, fmt.Errorf("invitation is missing student_id"))
	}
	return *invitation.StudentID, nil
}

// pendingInvitation loads an invitation that is awaiting approval and has not
// expired. Expired requests must not be approved or rejected into a new
// state; cleanup removes them.
func (l *AccountLifecycle) pendingInvitation(ctx context.Context, op string, invitationID int64) (domain.GuardianInvitation, error) {
	invitation, found, err := l.invitations.FindGuardianInvitation(ctx, invitationID)
	if err != nil {
		return domain.GuardianInvitation{}, failed(op, err)
	}
	if !found {
		return domain.GuardianInvitation{}, failed(op, domain.ErrGuardianInvitationNotFound)
	}
	if !invitation.IsPendingApproval() {
		return domain.GuardianInvitation{}, failed(op, fmt.Errorf("invitation is not awaiting approval"))
	}
	if guardianApprovalExpired(invitation, time.Now()) {
		return domain.GuardianInvitation{}, failed(op, domain.ErrGuardianInvitationExpired)
	}
	return invitation, nil
}

// cleanupOrphanProfile deletes a guardian profile that has no account and no
// remaining child links, i.e. one created only to back a now-rejected
// request. Best-effort; failures are logged, not surfaced.
func (l *AccountLifecycle) cleanupOrphanProfile(ctx context.Context, invitation domain.GuardianInvitation) {
	if !invitation.ProfileCreatedForInvitation {
		return
	}
	guardianProfileID := invitation.GuardianProfileID
	profile, found, err := l.guardians.FindGuardianProfile(ctx, guardianProfileID)
	if err != nil || !found || profile.HasAccount {
		return
	}
	links, err := l.guardians.ListStudentGuardianLinksByProfile(ctx, guardianProfileID)
	if err != nil {
		l.logger.Warn("guardian invitation reject: orphan-profile link check failed",
			slog.Int64("guardian_profile_id", guardianProfileID),
			slog.String("error", err.Error()))
		return
	}
	if len(links) > 0 {
		return
	}
	invitations, err := l.invitations.ListGuardianInvitationsByProfile(ctx, guardianProfileID)
	if err != nil {
		l.logger.Warn("guardian invitation reject: orphan-profile invitation check failed",
			slog.Int64("guardian_profile_id", guardianProfileID),
			slog.String("error", err.Error()))
		return
	}
	now := time.Now()
	for _, inv := range invitations {
		if inv.ID == invitation.ID {
			continue
		}
		if guardianInvitationNonFinal(inv, now) {
			return
		}
	}
	if err := l.guardians.DeleteGuardianProfile(ctx, guardianProfileID); err != nil {
		l.logger.Warn("guardian invitation reject: orphan-profile cleanup failed",
			slog.Int64("guardian_profile_id", guardianProfileID),
			slog.String("error", err.Error()))
	}
}

func guardianApprovalExpired(inv domain.GuardianInvitation, now time.Time) bool {
	return !inv.ExpiresAt.IsZero() && !inv.ExpiresAt.After(now)
}

func guardianInvitationNonFinal(inv domain.GuardianInvitation, now time.Time) bool {
	if inv.AcceptedAt != nil {
		return false
	}
	if inv.ApprovalStatus == domain.GuardianInvitationApprovalRejected {
		return false
	}
	return inv.ExpiresAt.IsZero() || inv.ExpiresAt.After(now)
}

func (l *AccountLifecycle) openStudentInvitations(ctx context.Context, guardianProfileID, studentID int64, now time.Time) ([]domain.GuardianInvitation, error) {
	invitations, err := l.invitations.ListGuardianInvitationsByProfile(ctx, guardianProfileID)
	if err != nil {
		return nil, err
	}
	open := make([]domain.GuardianInvitation, 0, len(invitations))
	for _, inv := range invitations {
		if inv.StudentID == nil || *inv.StudentID != studentID {
			continue
		}
		if guardianInvitationNonFinal(inv, now) {
			open = append(open, inv)
		}
	}
	return open, nil
}

func (l *AccountLifecycle) expireInvitations(ctx context.Context, invitations []domain.GuardianInvitation, now time.Time) error {
	for _, inv := range invitations {
		inv.ExpiresAt = now
		if inv.ApprovalStatus == domain.GuardianInvitationApprovalPending {
			inv.ApprovalStatus = domain.GuardianInvitationApprovalRejected
		}
		if err := l.invitations.UpdateGuardianInvitation(ctx, inv); err != nil {
			return err
		}
	}
	return nil
}

// ListPendingApprovalsDetailed returns the approval queue with guardian,
// child, and requester names resolved. Name resolution is best-effort: a
// failed batch read logs and leaves the names empty.
func (l *AccountLifecycle) ListPendingApprovalsDetailed(ctx context.Context) ([]domain.PendingApprovalView, error) {
	invitations, err := l.invitations.ListPendingGuardianApprovals(ctx)
	if err != nil {
		return nil, failed(opGuardianInviteApprove, err)
	}
	if len(invitations) == 0 {
		return []domain.PendingApprovalView{}, nil
	}

	profileIDs := make([]int64, 0, len(invitations))
	studentIDs := make([]int64, 0, len(invitations))
	accountIDs := make([]int64, 0, len(invitations))
	for _, invitation := range invitations {
		profileIDs = append(profileIDs, invitation.GuardianProfileID)
		if invitation.StudentID != nil {
			studentIDs = append(studentIDs, *invitation.StudentID)
		}
		if invitation.RequestedByAccountID != nil {
			accountIDs = append(accountIDs, *invitation.RequestedByAccountID)
		}
	}

	profiles, err := l.guardians.FindGuardianProfiles(ctx, profileIDs)
	if err != nil {
		l.logger.Error("failed to hydrate pending guardian approvals",
			"relation", "guardian_profiles",
			"error", err,
		)
		profiles = map[int64]domain.GuardianProfile{}
	}
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

	views := make([]domain.PendingApprovalView, 0, len(invitations))
	for _, inv := range invitations {
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
		views = append(views, view)
	}
	return views, nil
}

// RevokeAccess removes one account's link to one child. Parents may not
// remove the primary guardian; staff may remove anyone. The account/profile
// and sibling links are untouched.
func (l *AccountLifecycle) RevokeAccess(ctx context.Context, req domain.RevokeAccessRequest) error {
	return l.runtime.WithTenantTx(ctx, l.runtime.TenantID(ctx), func(txCtx context.Context) error {
		if err := l.guardians.LockStudent(txCtx, req.StudentID); err != nil {
			return failed(opGuardianRevokeAccess, err)
		}
		return l.revokeAccess(txCtx, req)
	})
}

func (l *AccountLifecycle) revokeAccess(ctx context.Context, req domain.RevokeAccessRequest) error {
	if req.StudentID <= 0 || req.GuardianProfileID <= 0 {
		return failed(opGuardianRevokeAccess, fmt.Errorf("student and guardian profile IDs are required"))
	}

	links, err := l.guardians.ListStudentGuardianLinksByStudent(ctx, req.StudentID)
	if err != nil {
		return failed(opGuardianRevokeAccess, err)
	}
	var link *domain.StudentGuardianLink
	for i := range links {
		if links[i].GuardianProfileID == req.GuardianProfileID {
			link = &links[i]
			break
		}
	}
	if link == nil {
		return failed(opGuardianRevokeAccess, fmt.Errorf("account is not linked to this child"))
	}
	if req.ByParent && link.IsPrimary {
		return failed(opGuardianRevokeAccess, domain.ErrCannotRemovePrimaryGuardian)
	}
	// Checked under the student lock taken by RevokeAccess, so a payer
	// assigned concurrently cannot slip past.
	if link.IsPayer && !req.MayClearPayer {
		return failed(opGuardianRevokeAccess, domain.ErrCannotRemovePayerGuardian)
	}
	deleteLink := true
	if req.ByParent {
		profile, found, err := l.guardians.FindGuardianProfile(ctx, req.GuardianProfileID)
		if err != nil {
			return failed(opGuardianRevokeAccess, err)
		}
		if !found {
			return failed(opGuardianRevokeAccess, fmt.Errorf("guardian profile not found"))
		}
		// The parent flow manages ANOTHER account's access. A parent must not
		// be able to revoke their own link to the child.
		if profile.AccountID != nil && *profile.AccountID == req.ActorAccountID {
			return failed(opGuardianRevokeAccess, domain.ErrCannotRemoveOwnAccess)
		}
		if !profile.HasAccount {
			now := time.Now()
			openInvitations, err := l.openStudentInvitations(ctx, req.GuardianProfileID, req.StudentID, now)
			if err != nil {
				return failed(opGuardianRevokeAccess, err)
			}
			if len(openInvitations) == 0 {
				return failed(opGuardianRevokeAccess, domain.ErrCannotRemoveStaffManagedGuardian)
			}
			if err := l.expireInvitations(ctx, openInvitations, now); err != nil {
				return failed(opGuardianRevokeAccess, err)
			}
			deleteLink = false
			for _, inv := range openInvitations {
				if inv.ProfileCreatedForInvitation {
					deleteLink = true
					break
				}
			}
		}
	}
	if !deleteLink {
		l.logger.Info("guardian pending invite cancelled without deleting staff-managed contact",
			slog.Int64("student_id", req.StudentID),
			slog.Int64("guardian_profile_id", req.GuardianProfileID),
			slog.Int64("actor_account_id", req.ActorAccountID),
			slog.Bool("by_parent", req.ByParent),
		)
		return nil
	}
	if link.IsPayer {
		if req.ActorAccountID <= 0 {
			return failed(opGuardianRevokeAccess, fmt.Errorf("refusing to remove payer without a financial audit"))
		}
		if err := l.financial.RecordPayerRemoved(ctx, link.GuardianProfileID, link.StudentID, req.ActorAccountID); err != nil {
			return failed(opGuardianRevokeAccess, fmt.Errorf("write payer removal audit: %w", err))
		}
	}

	if err := l.guardians.DeleteStudentGuardianLink(ctx, link.ID); err != nil {
		return failed(opGuardianRevokeAccess, err)
	}

	l.logger.Info("guardian access revoked",
		slog.Int64("student_id", req.StudentID),
		slog.Int64("guardian_profile_id", req.GuardianProfileID),
		slog.Int64("actor_account_id", req.ActorAccountID),
		slog.Bool("by_parent", req.ByParent),
	)
	return nil
}
