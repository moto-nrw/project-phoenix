package auth

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gofrs/uuid"
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
)

// InviteToStudentResult is returned by InviteToStudent.
type InviteToStudentResult struct {
	Outcome           InviteToStudentOutcome
	GuardianProfileID int64
	InvitationID      *int64 // nil for auto-link / already-linked outcomes
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
	profile, err := s.findOrCreateProfileByEmail(ctx, email, req.FirstName, req.LastName, tenantID)
	if err != nil {
		return nil, err
	}

	if req.RequireApproval {
		return s.queuePendingApproval(ctx, req, profile, tenantID)
	}
	return s.resolveInviteNow(ctx, req, profile, tenantID)
}

// findOrCreateProfileByEmail returns the guardian profile for this email,
// creating a fresh contact record when none exists.
func (s *guardianInvitationService) findOrCreateProfileByEmail(ctx context.Context, email, firstName, lastName string, tenantID int64) (*userModels.GuardianProfile, error) {
	existing, err := s.guardianProfileRepo.FindByEmail(ctx, email)
	if err == nil && existing != nil {
		return existing, nil
	}
	// Both sql.ErrNoRows and the repo's "not found" string flow through here;
	// we don't distinguish — if the lookup finds nothing we create. A genuine
	// DB failure surfaces from the Create below. Mirrors the enrollment
	// decision service's find-or-create-by-email path.

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
	if err := s.guardianProfileRepo.Create(ctx, profile); err != nil {
		return nil, &AuthError{Op: opGuardianInviteToStudent, Err: err}
	}
	return profile, nil
}

// resolveInviteNow links/invites immediately (staff invites, or parent invites
// in "direct" mode).
func (s *guardianInvitationService) resolveInviteNow(ctx context.Context, req InviteToStudentRequest, profile *userModels.GuardianProfile, tenantID int64) (*InviteToStudentResult, error) {
	rel := req.RelationshipType
	if rel == "" {
		rel = defaultRelationshipType
	}
	linkCreated, err := s.ensureStudentLink(ctx, req.StudentID, profile.ID, rel, tenantID)
	if err != nil {
		return nil, err
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

	// No account yet → issue a token invitation for this child.
	invitation, err := s.createStudentInvitation(ctx, req, profile, tenantID, authModels.GuardianInvitationApprovalNotRequired)
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
// approves it.
func (s *guardianInvitationService) queuePendingApproval(ctx context.Context, req InviteToStudentRequest, profile *userModels.GuardianProfile, tenantID int64) (*InviteToStudentResult, error) {
	invitation, err := s.createStudentInvitation(ctx, req, profile, tenantID, authModels.GuardianInvitationApprovalPending)
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
	created, err := s.studentGuardianRepo.LinkIfNotExists(ctx, rel)
	if err != nil {
		return false, &AuthError{Op: opGuardianInviteToStudent, Err: err}
	}
	return created, nil
}

// createStudentInvitation persists a guardian_invitations row carrying the
// student and (for parent-initiated invites) the requesting account.
func (s *guardianInvitationService) createStudentInvitation(ctx context.Context, req InviteToStudentRequest, profile *userModels.GuardianProfile, tenantID int64, approvalStatus string) (*authModels.GuardianInvitation, error) {
	studentID := req.StudentID
	invitation := &authModels.GuardianInvitation{
		Token:                uuid.Must(uuid.NewV4()).String(),
		GuardianProfileID:    profile.ID,
		CreatedBy:            req.CreatedBy,
		ExpiresAt:            time.Now().Add(s.resolveTokenExpiry(ctx)),
		StudentID:            &studentID,
		RequestedByAccountID: req.RequestedByParentAccountID,
		ApprovalStatus:       approvalStatus,
	}
	invitation.SetTenantID(tenantID)
	if err := s.invitationRepo.Create(ctx, invitation); err != nil {
		return nil, &AuthError{Op: opGuardianInviteToStudent, Err: err}
	}
	return invitation, nil
}

// ApproveInvitation resolves a pending parent-initiated request: it links the
// child and either grants access to an existing account or dispatches the
// invitation email.
func (s *guardianInvitationService) ApproveInvitation(ctx context.Context, invitationID int64, approverAccountID int64) error {
	invitation, err := s.invitationRepo.FindByID(ctx, invitationID)
	if err != nil {
		if isNotFoundError(err) {
			return &AuthError{Op: opGuardianInviteApprove, Err: ErrInvitationNotFound}
		}
		return &AuthError{Op: opGuardianInviteApprove, Err: err}
	}
	if !invitation.IsPendingApproval() {
		return &AuthError{Op: opGuardianInviteApprove, Err: fmt.Errorf("invitation is not awaiting approval")}
	}

	profile, err := s.guardianProfileRepo.FindByID(ctx, invitation.GuardianProfileID)
	if err != nil {
		return &AuthError{Op: opGuardianInviteApprove, Err: err}
	}

	rel := defaultRelationshipType
	if invitation.StudentID != nil {
		if _, err := s.ensureStudentLink(ctx, *invitation.StudentID, profile.ID, rel, invitation.TenantID); err != nil {
			return err
		}
	}

	now := time.Now()
	invitation.ApprovalStatus = authModels.GuardianInvitationApprovalApproved
	invitation.ApprovedBy = &approverAccountID
	invitation.ApprovedAt = &now

	// Existing account: access is granted by the link; close the invitation.
	if profile.HasAccount {
		invitation.AcceptedAt = &now
		if err := s.invitationRepo.Update(ctx, invitation); err != nil {
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
	if err := s.invitationRepo.Update(ctx, invitation); err != nil {
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
	invitation, err := s.invitationRepo.FindByID(ctx, invitationID)
	if err != nil {
		if isNotFoundError(err) {
			return &AuthError{Op: opGuardianInviteReject, Err: ErrInvitationNotFound}
		}
		return &AuthError{Op: opGuardianInviteReject, Err: err}
	}
	if !invitation.IsPendingApproval() {
		return &AuthError{Op: opGuardianInviteReject, Err: fmt.Errorf("invitation is not awaiting approval")}
	}

	now := time.Now()
	invitation.ApprovalStatus = authModels.GuardianInvitationApprovalRejected
	invitation.ApprovedBy = &approverAccountID
	invitation.ApprovedAt = &now
	if err := s.invitationRepo.Update(ctx, invitation); err != nil {
		return &AuthError{Op: opGuardianInviteReject, Err: err}
	}

	s.cleanupOrphanProfile(ctx, invitation.GuardianProfileID)

	s.getLogger().Info("guardian invitation rejected",
		slog.Int64("invitation_id", invitation.ID),
		slog.Int64("approver_account_id", approverAccountID),
	)
	return nil
}

// cleanupOrphanProfile deletes a guardian profile that has no account and no
// remaining child links — i.e. one created only to back a now-rejected
// request. Best-effort; failures are logged, not surfaced.
func (s *guardianInvitationService) cleanupOrphanProfile(ctx context.Context, guardianProfileID int64) {
	profile, err := s.guardianProfileRepo.FindByID(ctx, guardianProfileID)
	if err != nil || profile == nil || profile.HasAccount {
		return
	}
	links, err := s.studentGuardianRepo.FindByGuardianProfileID(ctx, guardianProfileID)
	if err != nil {
		s.getLogger().Warn("guardian invitation reject: orphan-profile link check failed",
			slog.Int64("guardian_profile_id", guardianProfileID),
			slog.String("error", err.Error()))
		return
	}
	if len(links) > 0 {
		return
	}
	if err := s.guardianProfileRepo.Delete(ctx, guardianProfileID); err != nil {
		s.getLogger().Warn("guardian invitation reject: orphan-profile cleanup failed",
			slog.Int64("guardian_profile_id", guardianProfileID),
			slog.String("error", err.Error()))
	}
}

// ListPendingApprovals returns parent-initiated invitations awaiting staff
// approval for the current tenant.
func (s *guardianInvitationService) ListPendingApprovals(ctx context.Context) ([]*authModels.GuardianInvitation, error) {
	invitations, err := s.invitationRepo.FindPendingApproval(ctx)
	if err != nil {
		return nil, &AuthError{Op: opGuardianInviteApprove, Err: err}
	}
	return invitations, nil
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
}

// ListPendingApprovalsDetailed returns the approval queue with guardian, child,
// and requester names resolved. The queue is short (one row per outstanding
// request), so the per-row lookups are acceptable.
func (s *guardianInvitationService) ListPendingApprovalsDetailed(ctx context.Context) ([]*PendingApprovalView, error) {
	invitations, err := s.invitationRepo.FindPendingApproval(ctx)
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
		}
		s.fillGuardianFields(ctx, inv.GuardianProfileID, view)
		if inv.StudentID != nil {
			view.StudentID = *inv.StudentID
			view.StudentName = s.resolveStudentName(ctx, *inv.StudentID)
		}
		if inv.RequestedByAccountID != nil {
			if acc, accErr := s.accountRepo.FindByID(ctx, *inv.RequestedByAccountID); accErr == nil && acc != nil {
				view.RequestedByEmail = acc.Email
			}
		}
		views = append(views, view)
	}
	return views, nil
}

// fillGuardianFields populates the guardian name + email on the view.
func (s *guardianInvitationService) fillGuardianFields(ctx context.Context, guardianProfileID int64, view *PendingApprovalView) {
	profile, err := s.guardianProfileRepo.FindByID(ctx, guardianProfileID)
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
	if s.studentRepo == nil {
		return ""
	}
	student, err := s.studentRepo.FindByID(ctx, studentID)
	if err != nil || student == nil {
		return ""
	}
	person, err := s.personRepo.FindByID(ctx, student.PersonID)
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

	links, err := s.studentGuardianRepo.FindByStudentID(ctx, req.StudentID)
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

	if err := s.studentGuardianRepo.Delete(ctx, link.ID); err != nil {
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
