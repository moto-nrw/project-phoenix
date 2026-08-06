package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Operation names for AuthError wrapping (related-accounts flows).
const (
	opGuardianInviteToStudent = "invite guardian to student"
	opGuardianInviteApprove   = "approve guardian invitation"
	opGuardianInviteReject    = "reject guardian invitation"
	opGuardianRevokeAccess    = "revoke guardian access"
)

// defaultRelationshipType is used when an invite doesn't specify one. The
// students_guardians.relationship_type column accepts parent/guardian/relative/
// other; "guardian" is the neutral default for an invited further caregiver.
const defaultRelationshipType = "guardian"

// InviteToStudentRequest is the unified "invite an email to a child" input,
// used by both the staff "Erziehungsberechtigte" tab and the parents portal.
// The handler resolves RequireApproval from the tenant setting and performs
// the caller-type authorization before calling this method.
type InviteToStudentRequest struct {
	StudentID        int64
	Email            string
	FirstName        string // optional — only used when creating a brand-new profile
	LastName         string // optional
	RelationshipType string // defaults to "guardian"
	CreatedBy        int64  // inviting account (staff or parent)

	// RequestedByParentAccountID is set (non-nil) when a parent initiated the
	// invite via the parents portal; nil for staff-initiated invites.
	RequestedByParentAccountID *int64
	// RequireApproval queues the invite for staff approval instead of acting
	// immediately. The handler decides this from guardians.parent_invite_mode.
	RequireApproval bool
	// ConfirmRoleUpgrade confirms upgrading an existing restrictive contact
	// link (emergency_contact/pickup_only/custom) to legal_guardian. Without
	// it, such an invite returns InviteOutcomeExistingContactRestricted with
	// no side effects so the UI can ask first (#2172). A PARENT-initiated
	// confirmed upgrade is always queued for staff approval, even in direct
	// invite mode — see InviteToStudent.
	ConfirmRoleUpgrade bool
}

// InviteToStudentOutcome describes what the resolve logic did, so the UI can
// show the right confirmation ("eingeladen" vs "hinzugefügt" vs "wartet auf
// Freigabe").
type InviteToStudentOutcome string

const (
	// InviteOutcomeLinkedExistingAccount: the email already had an account;
	// the child was linked to it immediately (no invite email).
	InviteOutcomeLinkedExistingAccount InviteToStudentOutcome = "linked_existing_account"
	// InviteOutcomeAlreadyLinked: the account was already linked to this child.
	InviteOutcomeAlreadyLinked InviteToStudentOutcome = "already_linked"
	// InviteOutcomeInvited: a token invitation was created and emailed.
	InviteOutcomeInvited InviteToStudentOutcome = "invited"
	// InviteOutcomePendingApproval: a parent request was queued for staff.
	InviteOutcomePendingApproval InviteToStudentOutcome = "pending_approval"
	// InviteOutcomeExistingContactRestricted: the email belongs to an existing
	// contact on this child with a restrictive role and no portal access.
	// Nothing was changed; the caller must confirm the role upgrade
	// (ConfirmRoleUpgrade) and retry (#2172).
	InviteOutcomeExistingContactRestricted InviteToStudentOutcome = "existing_contact_restricted"
)

// InviteToStudentResult is returned by InviteToStudent.
type InviteToStudentResult struct {
	Outcome           InviteToStudentOutcome
	GuardianProfileID int64
	InvitationID      *int64 // nil for auto-link / already-linked outcomes
	// ExistingRole carries the current guardian role for the
	// existing_contact_restricted outcome; empty otherwise.
	ExistingRole string
}

type relatedAccountProfileResolution struct {
	profile *userModels.GuardianProfile
	created bool
}

// RevokeAccessRequest removes one account's access to one child by deleting the
// students_guardians link. The account, profile, and sibling links are left
// untouched.
type RevokeAccessRequest struct {
	StudentID         int64
	GuardianProfileID int64
	ActorAccountID    int64
	// ByParent is true for parents-portal removals. Parents may not remove the
	// primary guardian; staff may remove anyone.
	ByParent bool
}

// InviteToStudent resolves an email against existing data and either links an
// existing account to the child, (re)sends an invitation, or queues a parent
// request for staff approval. Tenant comes from context.
func (s *guardianInvitationService) InviteToStudent(ctx context.Context, req InviteToStudentRequest) (*InviteToStudentResult, error) {
	if req.StudentID <= 0 {
		return nil, &AuthError{Op: opGuardianInviteToStudent, Err: fmt.Errorf("student ID is required")}
	}
	if req.CreatedBy <= 0 {
		return nil, &AuthError{Op: opGuardianInviteToStudent, Err: fmt.Errorf("created_by is required")}
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		return nil, &AuthError{Op: opGuardianInviteToStudent, Err: fmt.Errorf("email is required")}
	}

	tenantID := tenant.FromContext(ctx)

	// An existing restrictive contact link (emergency_contact/pickup_only/
	// custom) would survive LinkIfNotExists untouched, leaving the invited
	// account without parent_portal.access (#2172). Detect it up front — with
	// no side effects — and require an explicit upgrade confirmation.
	roleUpgrade, restricted, err := s.detectRestrictedContact(ctx, req, email)
	if err != nil {
		return nil, err
	}
	if restricted != nil {
		return restricted, nil
	}

	resolved, err := s.findOrCreateProfileByEmail(ctx, email, req.FirstName, req.LastName, tenantID)
	if err != nil {
		return nil, err
	}
	profile := resolved.profile
	if profile == nil {
		return nil, &AuthError{Op: opGuardianInviteToStudent, Err: fmt.Errorf("guardian profile resolution failed")}
	}

	// A confirmed upgrade of an existing restricted contact overrides a
	// deliberate staff-set restriction (e.g. a custody-motivated pickup_only
	// or emergency_contact role). When a PARENT asks for it, the request
	// always goes through staff approval — even in direct invite mode.
	// Staff-initiated invites keep applying immediately, and parents inviting
	// brand-new contacts in direct mode are unaffected.
	requireApproval := req.RequireApproval
	if roleUpgrade && req.RequestedByParentAccountID != nil {
		requireApproval = true
	}
	if requireApproval {
		return s.queuePendingApproval(ctx, req, profile, tenantID, resolved.created, roleUpgrade)
	}
	if err := s.attachExistingAccountByEmail(ctx, profile, email, tenantID); err != nil {
		return nil, err
	}
	return s.resolveInviteNow(ctx, req, profile, tenantID, resolved.created, roleUpgrade)
}

// detectRestrictedContact checks whether the invited email already belongs to a
// contact on this child whose link has a restrictive role (no portal access).
// Returns (roleUpgrade=true, nil, nil) when the caller confirmed the upgrade,
// or a ready-made existing_contact_restricted result when confirmation is still
// missing. Lookup-only — no rows are created or modified.
func (s *guardianInvitationService) detectRestrictedContact(ctx context.Context, req InviteToStudentRequest, email string) (bool, *InviteToStudentResult, error) {
	existing, err := s.GuardianProfileRepo.FindByEmail(ctx, email)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil, nil
		}
		return false, nil, &AuthError{Op: opGuardianInviteToStudent, Err: err}
	}
	if existing == nil {
		return false, nil, nil
	}
	link, err := s.StudentGuardianRepo.FindByStudentAndGuardianForUpdate(ctx, req.StudentID, existing.ID)
	if err != nil {
		if errors.Is(err, userModels.ErrStudentGuardianNotFound) {
			return false, nil, nil
		}
		return false, nil, &AuthError{Op: opGuardianInviteToStudent, Err: err}
	}
	if authorize.IsFullGuardianRole(link.GuardianRole) {
		return false, nil, nil
	}
	// A social worker is a school-managed professional contact, never a
	// candidate for the legal-guardian upgrade — the parent edit paths reject
	// every parent write to such a link (ErrGuardianSocialWorkerManaged), and
	// the invite flow must not open a side door. Refuse the invite outright,
	// with or without confirmation.
	if authorize.NormalizeGuardianRole(link.GuardianRole) == authorize.GuardianRoleSocialWorker {
		return false, nil, &AuthError{Op: opGuardianInviteToStudent, Err: ErrInviteSocialWorkerManaged}
	}
	if !req.ConfirmRoleUpgrade {
		return false, &InviteToStudentResult{
			Outcome:           InviteOutcomeExistingContactRestricted,
			GuardianProfileID: existing.ID,
			ExistingRole:      link.GuardianRole,
		}, nil
	}
	return true, nil, nil
}

// upgradeStudentLink promotes an existing restrictive students_guardians link
// to legal_guardian, recomputing the permission set server-side (#2172).
// No-op when the link already carries a full guardian role.
func (s *guardianInvitationService) upgradeStudentLink(ctx context.Context, studentID, guardianProfileID int64, op string) error {
	link, err := s.StudentGuardianRepo.FindByStudentAndGuardianForUpdate(ctx, studentID, guardianProfileID)
	if err != nil {
		return &AuthError{Op: op, Err: err}
	}
	if authorize.IsFullGuardianRole(link.GuardianRole) {
		return nil
	}
	// Defense in depth: a persisted RoleUpgrade flag must not promote a link
	// that became a social-worker contact between request and approval.
	if authorize.NormalizeGuardianRole(link.GuardianRole) == authorize.GuardianRoleSocialWorker {
		s.getLogger().Warn("guardian contact upgrade skipped: social-worker link is school-managed",
			slog.Int64("student_id", studentID),
			slog.Int64("guardian_profile_id", guardianProfileID),
		)
		return nil
	}
	authorize.ApplyStudentGuardianRole(link, authorize.GuardianRoleLegalGuardian)
	if err := s.StudentGuardianRepo.Update(ctx, link); err != nil {
		return &AuthError{Op: op, Err: err}
	}
	s.getLogger().Info("guardian contact link upgraded to full access",
		slog.Int64("student_id", studentID),
		slog.Int64("guardian_profile_id", guardianProfileID),
	)
	return nil
}

// findOrCreateProfileByEmail returns the guardian profile for this email,
// creating a fresh contact record when none exists.
func (s *guardianInvitationService) findOrCreateProfileByEmail(ctx context.Context, email, firstName, lastName string, tenantID int64) (*relatedAccountProfileResolution, error) {
	existing, err := s.GuardianProfileRepo.FindByEmail(ctx, email)
	if err == nil && existing != nil {
		return &relatedAccountProfileResolution{profile: existing}, nil
	}
	if err != nil && !isNotFoundError(err) {
		return nil, &AuthError{Op: opGuardianInviteToStudent, Err: err}
	}

	emailCopy := email
	profile := &userModels.GuardianProfile{
		FirstName:              strings.TrimSpace(firstName),
		LastName:               strings.TrimSpace(lastName),
		Email:                  &emailCopy,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	if tenantID > 0 {
		profile.SetTenantID(tenantID)
	}
	if err := profile.Validate(); err != nil {
		return nil, &AuthError{Op: opGuardianInviteToStudent, Err: err}
	}
	if err := s.GuardianProfileRepo.Create(ctx, profile); err != nil {
		return nil, &AuthError{Op: opGuardianInviteToStudent, Err: err}
	}
	return &relatedAccountProfileResolution{profile: profile, created: true}, nil
}

func (s *guardianInvitationService) attachExistingAccountByEmail(ctx context.Context, profile *userModels.GuardianProfile, email string, tenantID int64) error {
	if profile.HasAccount || s.AccountRepo == nil {
		return nil
	}
	account, err := s.AccountRepo.FindByEmail(ctx, email)
	if err != nil {
		if isNotFoundError(err) {
			return nil
		}
		return &AuthError{Op: opGuardianInviteToStudent, Err: err}
	}
	if account == nil {
		return nil
	}
	if err := s.linkProfileToAccount(ctx, profile, account.ID, tenantID, opGuardianInviteToStudent); err != nil {
		return err
	}
	profile.AccountID = &account.ID
	profile.HasAccount = true
	return nil
}

// resolveInviteNow links/invites immediately (staff invites, or parent invites
// in "direct" mode). roleUpgrade applies a confirmed restrictive-contact
// upgrade right away, so access does not depend on a later approval.
func (s *guardianInvitationService) resolveInviteNow(ctx context.Context, req InviteToStudentRequest, profile *userModels.GuardianProfile, tenantID int64, profileCreated bool, roleUpgrade bool) (*InviteToStudentResult, error) {
	rel := req.RelationshipType
	if rel == "" {
		rel = defaultRelationshipType
	}
	linkCreated, err := s.ensureStudentLink(ctx, req.StudentID, profile.ID, rel, tenantID)
	if err != nil {
		return nil, err
	}
	if roleUpgrade {
		if err := s.upgradeStudentLink(ctx, req.StudentID, profile.ID, opGuardianInviteToStudent); err != nil {
			return nil, err
		}
	}

	// Existing account → access is granted by the link alone; no token needed.
	if profile.HasAccount {
		outcome := InviteOutcomeLinkedExistingAccount
		if !linkCreated {
			outcome = InviteOutcomeAlreadyLinked
		}
		s.getLogger().Info("guardian related-account linked to existing account",
			slog.Int64("student_id", req.StudentID),
			slog.Int64("guardian_profile_id", profile.ID),
			slog.Int64("created_by", req.CreatedBy),
			slog.String("outcome", string(outcome)),
		)
		return &InviteToStudentResult{Outcome: outcome, GuardianProfileID: profile.ID}, nil
	}

	// No account yet → issue a token invitation for this child. The upgrade
	// (if any) was already applied above, so the row's flag stays false.
	invitation, err := s.createStudentInvitation(ctx, req, profile, tenantID, authModels.GuardianInvitationApprovalNotRequired, profileCreated, false)
	if err != nil {
		return nil, err
	}
	schoolName := s.lookupSchoolName(ctx, tenantID)
	s.enqueueEmail(ctx, invitation, profile, schoolName)
	return &InviteToStudentResult{
		Outcome:           InviteOutcomeInvited,
		GuardianProfileID: profile.ID,
		InvitationID:      &invitation.ID,
	}, nil
}

// queuePendingApproval records a parent-initiated request awaiting staff
// approval. No link is created and no email is sent until a staff member
// approves it. roleUpgrade is persisted on the invitation so the approval
// step knows it must also upgrade the existing restrictive link.
func (s *guardianInvitationService) queuePendingApproval(ctx context.Context, req InviteToStudentRequest, profile *userModels.GuardianProfile, tenantID int64, profileCreated bool, roleUpgrade bool) (*InviteToStudentResult, error) {
	invitation, err := s.createStudentInvitation(ctx, req, profile, tenantID, authModels.GuardianInvitationApprovalPending, profileCreated, roleUpgrade)
	if err != nil {
		return nil, err
	}
	s.getLogger().Info("guardian related-account invite queued for approval",
		slog.Int64("student_id", req.StudentID),
		slog.Int64("guardian_profile_id", profile.ID),
		slog.Int64("created_by", req.CreatedBy),
	)
	return &InviteToStudentResult{
		Outcome:           InviteOutcomePendingApproval,
		GuardianProfileID: profile.ID,
		InvitationID:      &invitation.ID,
	}, nil
}

// ensureStudentLink inserts the student↔guardian link if it doesn't already
// exist. Returns true when a new link row was created.
func (s *guardianInvitationService) ensureStudentLink(ctx context.Context, studentID, guardianProfileID int64, relationshipType string, tenantID int64) (bool, error) {
	rel := &userModels.StudentGuardian{
		StudentID:         studentID,
		GuardianProfileID: guardianProfileID,
		RelationshipType:  relationshipType,
		EmergencyPriority: 1,
	}
	if tenantID > 0 {
		rel.SetTenantID(tenantID)
	}
	created, err := s.StudentGuardianRepo.LinkIfNotExists(ctx, rel)
	if err != nil {
		return false, &AuthError{Op: opGuardianInviteToStudent, Err: err}
	}
	return created, nil
}

// createStudentInvitation persists a guardian_invitations row carrying the
// student and (for parent-initiated invites) the requesting account.
func (s *guardianInvitationService) createStudentInvitation(ctx context.Context, req InviteToStudentRequest, profile *userModels.GuardianProfile, tenantID int64, approvalStatus string, profileCreated bool, roleUpgrade bool) (*authModels.GuardianInvitation, error) {
	studentID := req.StudentID
	openInvitation, err := s.findOpenStudentInvitation(ctx, profile.ID, studentID)
	if err != nil {
		return nil, err
	}
	if openInvitation != nil {
		if openInvitation.ApprovalStatus == authModels.GuardianInvitationApprovalPending &&
			approvalStatus == authModels.GuardianInvitationApprovalNotRequired {
			now := time.Now()
			openInvitation.ApprovalStatus = authModels.GuardianInvitationApprovalNotRequired
			openInvitation.ApprovedBy = nil
			openInvitation.ApprovedAt = nil
			openInvitation.ExpiresAt = now.Add(s.resolveTokenExpiry(ctx))
			openInvitation.EmailSentAt = nil
			openInvitation.EmailError = nil
			if err := s.InvitationRepo.Update(ctx, openInvitation); err != nil {
				return nil, &AuthError{Op: opGuardianInviteToStudent, Err: err}
			}
		}
		// A re-invite that confirmed a restrictive-contact upgrade must stick
		// the flag onto the reused row, or the approval would silently skip
		// the upgrade (#2172).
		if roleUpgrade && !openInvitation.RoleUpgrade {
			openInvitation.RoleUpgrade = true
			if err := s.InvitationRepo.Update(ctx, openInvitation); err != nil {
				return nil, &AuthError{Op: opGuardianInviteToStudent, Err: err}
			}
		}
		return openInvitation, nil
	}
	invitation := &authModels.GuardianInvitation{
		Token:                       uuid.Must(uuid.NewV4()).String(),
		GuardianProfileID:           profile.ID,
		CreatedBy:                   req.CreatedBy,
		ExpiresAt:                   time.Now().Add(s.resolveTokenExpiry(ctx)),
		StudentID:                   &studentID,
		RequestedByAccountID:        req.RequestedByParentAccountID,
		ApprovalStatus:              approvalStatus,
		ProfileCreatedForInvitation: profileCreated,
		RoleUpgrade:                 roleUpgrade,
	}
	invitation.SetTenantID(tenantID)
	if err := s.InvitationRepo.Create(ctx, invitation); err != nil {
		return nil, &AuthError{Op: opGuardianInviteToStudent, Err: err}
	}
	return invitation, nil
}

func (s *guardianInvitationService) findOpenStudentInvitation(ctx context.Context, guardianProfileID, studentID int64) (*authModels.GuardianInvitation, error) {
	invitations, err := s.openStudentInvitations(ctx, guardianProfileID, studentID, time.Now())
	if err != nil {
		return nil, &AuthError{Op: opGuardianInviteToStudent, Err: err}
	}
	if len(invitations) == 0 {
		return nil, nil
	}
	return invitations[0], nil
}

// ApproveInvitation resolves a pending parent-initiated request: it links the
// child and either grants access to an existing account or dispatches the
// invitation email.
func (s *guardianInvitationService) ApproveInvitation(ctx context.Context, invitationID int64, approverAccountID int64) error {
	invitation, err := s.InvitationRepo.FindByID(ctx, invitationID)
	if err != nil {
		if isNotFoundError(err) {
			return &AuthError{Op: opGuardianInviteApprove, Err: ErrInvitationNotFound}
		}
		return &AuthError{Op: opGuardianInviteApprove, Err: err}
	}
	if !invitation.IsPendingApproval() {
		return &AuthError{Op: opGuardianInviteApprove, Err: fmt.Errorf("invitation is not awaiting approval")}
	}
	if guardianApprovalExpired(invitation, time.Now()) {
		return &AuthError{Op: opGuardianInviteApprove, Err: ErrInvitationExpired}
	}

	profile, err := s.GuardianProfileRepo.FindByID(ctx, invitation.GuardianProfileID)
	if err != nil {
		return &AuthError{Op: opGuardianInviteApprove, Err: err}
	}
	if profile != nil && !profile.HasAccount && profile.Email != nil {
		email := strings.ToLower(strings.TrimSpace(*profile.Email))
		if email != "" {
			if err := s.attachExistingAccountByEmail(ctx, profile, email, invitation.TenantID); err != nil {
				return err
			}
		}
	}

	rel := defaultRelationshipType
	if invitation.StudentID != nil {
		if _, err := s.ensureStudentLink(ctx, *invitation.StudentID, profile.ID, rel, invitation.TenantID); err != nil {
			return err
		}
		if invitation.RoleUpgrade {
			if err := s.upgradeStudentLink(ctx, *invitation.StudentID, profile.ID, opGuardianInviteApprove); err != nil {
				return err
			}
		}
	}

	now := time.Now()
	invitation.ApprovalStatus = authModels.GuardianInvitationApprovalApproved
	invitation.ApprovedBy = &approverAccountID
	invitation.ApprovedAt = &now

	// Existing account: access is granted by the link; close the invitation.
	if profile.HasAccount {
		invitation.AcceptedAt = &now
		if err := s.InvitationRepo.Update(ctx, invitation); err != nil {
			return &AuthError{Op: opGuardianInviteApprove, Err: err}
		}
		s.getLogger().Info("guardian invitation approved (existing account)",
			slog.Int64("invitation_id", invitation.ID),
			slog.Int64("approver_account_id", approverAccountID),
		)
		return nil
	}

	// No account yet: refresh expiry, clear stale email tracking, dispatch.
	invitation.ExpiresAt = now.Add(s.resolveTokenExpiry(ctx))
	invitation.EmailSentAt = nil
	invitation.EmailError = nil
	if err := s.InvitationRepo.Update(ctx, invitation); err != nil {
		return &AuthError{Op: opGuardianInviteApprove, Err: err}
	}

	s.getLogger().Info("guardian invitation approved",
		slog.Int64("invitation_id", invitation.ID),
		slog.Int64("approver_account_id", approverAccountID),
	)
	schoolName := s.lookupSchoolName(ctx, invitation.TenantID)
	s.enqueueEmail(ctx, invitation, profile, schoolName)
	return nil
}

// RejectInvitation marks a pending parent-initiated request as rejected. No
// access is granted. A profile created solely for this request (no account, no
// other child links) is cleaned up.
func (s *guardianInvitationService) RejectInvitation(ctx context.Context, invitationID int64, approverAccountID int64) error {
	invitation, err := s.InvitationRepo.FindByID(ctx, invitationID)
	if err != nil {
		if isNotFoundError(err) {
			return &AuthError{Op: opGuardianInviteReject, Err: ErrInvitationNotFound}
		}
		return &AuthError{Op: opGuardianInviteReject, Err: err}
	}
	if !invitation.IsPendingApproval() {
		return &AuthError{Op: opGuardianInviteReject, Err: fmt.Errorf("invitation is not awaiting approval")}
	}
	if guardianApprovalExpired(invitation, time.Now()) {
		return &AuthError{Op: opGuardianInviteReject, Err: ErrInvitationExpired}
	}

	now := time.Now()
	invitation.ApprovalStatus = authModels.GuardianInvitationApprovalRejected
	invitation.ApprovedBy = &approverAccountID
	invitation.ApprovedAt = &now
	if err := s.InvitationRepo.Update(ctx, invitation); err != nil {
		return &AuthError{Op: opGuardianInviteReject, Err: err}
	}

	s.cleanupOrphanProfile(ctx, invitation)

	s.getLogger().Info("guardian invitation rejected",
		slog.Int64("invitation_id", invitation.ID),
		slog.Int64("approver_account_id", approverAccountID),
	)
	return nil
}

// PendingInvitationStudentID returns the child targeted by a pending approval
// request. Staff handlers use it for the same per-student authorization gate
// as direct relationship mutations before approve/reject mutates anything.
func (s *guardianInvitationService) PendingInvitationStudentID(ctx context.Context, invitationID int64) (int64, error) {
	invitation, err := s.InvitationRepo.FindByID(ctx, invitationID)
	if err != nil {
		if isNotFoundError(err) {
			return 0, &AuthError{Op: opGuardianInviteApprove, Err: ErrInvitationNotFound}
		}
		return 0, &AuthError{Op: opGuardianInviteApprove, Err: err}
	}
	if !invitation.IsPendingApproval() {
		return 0, &AuthError{Op: opGuardianInviteApprove, Err: fmt.Errorf("invitation is not awaiting approval")}
	}
	if guardianApprovalExpired(invitation, time.Now()) {
		return 0, &AuthError{Op: opGuardianInviteApprove, Err: ErrInvitationExpired}
	}
	if invitation.StudentID == nil || *invitation.StudentID <= 0 {
		return 0, &AuthError{Op: opGuardianInviteApprove, Err: fmt.Errorf("invitation is missing student_id")}
	}
	return *invitation.StudentID, nil
}

// cleanupOrphanProfile deletes a guardian profile that has no account and no
// remaining child links — i.e. one created only to back a now-rejected
// request. Best-effort; failures are logged, not surfaced.
func (s *guardianInvitationService) cleanupOrphanProfile(ctx context.Context, invitation *authModels.GuardianInvitation) {
	if invitation == nil || !invitation.ProfileCreatedForInvitation {
		return
	}
	guardianProfileID := invitation.GuardianProfileID
	profile, err := s.GuardianProfileRepo.FindByID(ctx, guardianProfileID)
	if err != nil || profile == nil || profile.HasAccount {
		return
	}
	links, err := s.StudentGuardianRepo.FindByGuardianProfileID(ctx, guardianProfileID)
	if err != nil {
		s.getLogger().Warn("guardian invitation reject: orphan-profile link check failed",
			slog.Int64("guardian_profile_id", guardianProfileID),
			slog.String("error", err.Error()))
		return
	}
	if len(links) > 0 {
		return
	}
	invitations, err := s.InvitationRepo.FindByGuardianProfileID(ctx, guardianProfileID)
	if err != nil {
		s.getLogger().Warn("guardian invitation reject: orphan-profile invitation check failed",
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
	if err := s.GuardianProfileRepo.Delete(ctx, guardianProfileID); err != nil {
		s.getLogger().Warn("guardian invitation reject: orphan-profile cleanup failed",
			slog.Int64("guardian_profile_id", guardianProfileID),
			slog.String("error", err.Error()))
	}
}

// guardianApprovalExpired reports whether a pending approval request has passed
// its expiry. Expired requests must not be approved or rejected into a new
// state (which would link a child or mutate the row) — cleanup removes them.
func guardianApprovalExpired(inv *authModels.GuardianInvitation, now time.Time) bool {
	return !inv.ExpiresAt.IsZero() && !inv.ExpiresAt.After(now)
}

func guardianInvitationNonFinal(inv *authModels.GuardianInvitation, now time.Time) bool {
	if inv == nil || inv.AcceptedAt != nil {
		return false
	}
	if inv.ApprovalStatus == authModels.GuardianInvitationApprovalRejected {
		return false
	}
	return inv.ExpiresAt.IsZero() || inv.ExpiresAt.After(now)
}

func (s *guardianInvitationService) openStudentInvitations(ctx context.Context, guardianProfileID, studentID int64, now time.Time) ([]*authModels.GuardianInvitation, error) {
	invitations, err := s.InvitationRepo.FindByGuardianProfileID(ctx, guardianProfileID)
	if err != nil {
		return nil, err
	}
	open := make([]*authModels.GuardianInvitation, 0, len(invitations))
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

func (s *guardianInvitationService) expireInvitations(ctx context.Context, invitations []*authModels.GuardianInvitation, now time.Time) error {
	for _, inv := range invitations {
		inv.ExpiresAt = now
		if inv.ApprovalStatus == authModels.GuardianInvitationApprovalPending {
			inv.ApprovalStatus = authModels.GuardianInvitationApprovalRejected
		}
		if err := s.InvitationRepo.Update(ctx, inv); err != nil {
			return err
		}
	}
	return nil
}

// PendingApprovalView is the staff-facing, name-resolved projection of a
// parent-initiated invitation awaiting approval. Backs the approval queue so
// staff see who is being invited, to which child, and by whom — not raw IDs.
type PendingApprovalView struct {
	InvitationID      int64
	GuardianProfileID int64
	GuardianName      string
	GuardianEmail     string
	StudentID         int64
	StudentName       string
	RequestedByEmail  string
	CreatedAt         time.Time
	ExpiresAt         time.Time
	// RoleUpgrade marks a request whose approval also upgrades an existing
	// restrictive contact link to full portal access (#2172).
	RoleUpgrade bool
}

// ListPendingApprovalsDetailed returns the approval queue with guardian, child,
// and requester names resolved. The queue is short (one row per outstanding
// request), so the per-row lookups are acceptable.
func (s *guardianInvitationService) ListPendingApprovalsDetailed(ctx context.Context) ([]*PendingApprovalView, error) {
	invitations, err := s.InvitationRepo.FindPendingApproval(ctx)
	if err != nil {
		return nil, &AuthError{Op: opGuardianInviteApprove, Err: err}
	}

	views := make([]*PendingApprovalView, 0, len(invitations))
	for _, inv := range invitations {
		view := &PendingApprovalView{
			InvitationID:      inv.ID,
			GuardianProfileID: inv.GuardianProfileID,
			CreatedAt:         inv.CreatedAt,
			ExpiresAt:         inv.ExpiresAt,
			RoleUpgrade:       inv.RoleUpgrade,
		}
		s.fillGuardianFields(ctx, inv.GuardianProfileID, view)
		if inv.StudentID != nil {
			view.StudentID = *inv.StudentID
			view.StudentName = s.resolveStudentName(ctx, *inv.StudentID)
		}
		if inv.RequestedByAccountID != nil {
			if acc, accErr := s.AccountRepo.FindByID(ctx, *inv.RequestedByAccountID); accErr == nil && acc != nil {
				view.RequestedByEmail = acc.Email
			}
		}
		views = append(views, view)
	}
	return views, nil
}

// fillGuardianFields populates the guardian name + email on the view.
func (s *guardianInvitationService) fillGuardianFields(ctx context.Context, guardianProfileID int64, view *PendingApprovalView) {
	profile, err := s.GuardianProfileRepo.FindByID(ctx, guardianProfileID)
	if err != nil || profile == nil {
		return
	}
	view.GuardianName = strings.TrimSpace(profile.GetFullName())
	if profile.Email != nil {
		view.GuardianEmail = strings.TrimSpace(*profile.Email)
	}
}

// resolveStudentName resolves a child's display name via student → person.
// Best-effort: returns "" when the lookup fails.
func (s *guardianInvitationService) resolveStudentName(ctx context.Context, studentID int64) string {
	if s.StudentRepo == nil {
		return ""
	}
	student, err := s.StudentRepo.FindByID(ctx, studentID)
	if err != nil || student == nil {
		return ""
	}
	person, err := s.PersonRepo.FindByID(ctx, student.PersonID)
	if err != nil || person == nil {
		return ""
	}
	return strings.TrimSpace(person.FirstName + " " + person.LastName)
}

// RevokeAccess removes one account's link to one child. Parents may not remove
// the primary guardian; staff may remove anyone. The account/profile and
// sibling links are untouched.
func (s *guardianInvitationService) RevokeAccess(ctx context.Context, req RevokeAccessRequest) error {
	if req.StudentID <= 0 || req.GuardianProfileID <= 0 {
		return &AuthError{Op: opGuardianRevokeAccess, Err: fmt.Errorf("student and guardian profile IDs are required")}
	}

	links, err := s.StudentGuardianRepo.FindByStudentID(ctx, req.StudentID)
	if err != nil {
		return &AuthError{Op: opGuardianRevokeAccess, Err: err}
	}
	var link *userModels.StudentGuardian
	for _, l := range links {
		if l.GuardianProfileID == req.GuardianProfileID {
			link = l
			break
		}
	}
	if link == nil {
		return &AuthError{Op: opGuardianRevokeAccess, Err: fmt.Errorf("account is not linked to this child")}
	}
	if req.ByParent && link.IsPrimary {
		return &AuthError{Op: opGuardianRevokeAccess, Err: ErrCannotRemovePrimaryGuardian}
	}
	deleteLink := true
	if req.ByParent {
		profile, err := s.GuardianProfileRepo.FindByID(ctx, req.GuardianProfileID)
		if err != nil {
			return &AuthError{Op: opGuardianRevokeAccess, Err: err}
		}
		if profile == nil {
			return &AuthError{Op: opGuardianRevokeAccess, Err: fmt.Errorf("guardian profile not found")}
		}
		// The parent flow manages *another* account's access. A parent must not
		// be able to revoke their own link to the child (passing their own
		// guardianProfileId) — that could lock them out. Staff (ByParent=false)
		// may remove anyone.
		if profile.AccountID != nil && *profile.AccountID == req.ActorAccountID {
			return &AuthError{Op: opGuardianRevokeAccess, Err: ErrCannotRemoveOwnAccess}
		}
		if !profile.HasAccount {
			now := time.Now()
			openInvitations, err := s.openStudentInvitations(ctx, req.GuardianProfileID, req.StudentID, now)
			if err != nil {
				return &AuthError{Op: opGuardianRevokeAccess, Err: err}
			}
			if len(openInvitations) == 0 {
				return &AuthError{Op: opGuardianRevokeAccess, Err: ErrCannotRemoveStaffManagedGuardian}
			}
			if err := s.expireInvitations(ctx, openInvitations, now); err != nil {
				return &AuthError{Op: opGuardianRevokeAccess, Err: err}
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
		s.getLogger().Info("guardian pending invite cancelled without deleting staff-managed contact",
			slog.Int64("student_id", req.StudentID),
			slog.Int64("guardian_profile_id", req.GuardianProfileID),
			slog.Int64("actor_account_id", req.ActorAccountID),
			slog.Bool("by_parent", req.ByParent),
		)
		return nil
	}

	if err := s.StudentGuardianRepo.Delete(ctx, link.ID); err != nil {
		return &AuthError{Op: opGuardianRevokeAccess, Err: err}
	}

	s.getLogger().Info("guardian access revoked",
		slog.Int64("student_id", req.StudentID),
		slog.Int64("guardian_profile_id", req.GuardianProfileID),
		slog.Int64("actor_account_id", req.ActorAccountID),
		slog.Bool("by_parent", req.ByParent),
	)
	return nil
}
