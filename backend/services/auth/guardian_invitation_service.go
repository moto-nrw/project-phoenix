package auth

import (
	"context"
	"time"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
)

// The guardian invitation lifecycle lives in modules/identityaccess (#2722).
// This service keeps the contract its callers established — the public
// accept page, the enrollment decisions and the parents portal — and
// delegates through the consumer-owned ports the composition root binds:
// the invitation flows and the relative access flows.

// Guardian invitation operation names, used in AuthError wrapping for
// callers that match on Op.
const (
	opGuardianInviteCreate   = "create guardian invitation"
	opGuardianInviteValidate = "validate guardian invitation"
	opGuardianInviteAccept   = "accept guardian invitation"
	opGuardianInviteResend   = "resend guardian invitation"
)

// GuardianTokenExpiryFallback applies when neither the registry setting nor
// the env var name a guardian token lifetime. 48 hours matches the staff
// invitation default and the registry default.
const GuardianTokenExpiryFallback = 48 * time.Hour

// GuardianInvitationRecord is one auth.guardian_invitations row as the owner
// reports it.
type GuardianInvitationRecord struct {
	ID                int64
	TenantID          int64
	Token             string
	GuardianProfileID int64
	CreatedBy         int64
	ExpiresAt         time.Time
	AcceptedAt        *time.Time
	StudentID         *int64
	ApprovalStatus    string
	CreatedAt         time.Time
}

// GuardianInvitationPreview is the public-safe view of a redeemable
// invitation, as the owner reports it.
type GuardianInvitationPreview struct {
	Email         string
	FirstName     string
	LastName      string
	ExpiresAt     time.Time
	SchoolName    string
	SchoolSlug    string
	SchoolLogoURL string
}

// GuardianInvitationAccount is the account an acceptance gives access to.
type GuardianInvitationAccount struct {
	ID    int64
	Email string
}

// GuardianInvitations is the consumer-owned port over the Identity & Access
// guardian invitation capability. The composition root binds it; every error
// already carries the AuthError envelope and the sentinels of this package.
type GuardianInvitations interface {
	CreateGuardianInvitation(ctx context.Context, guardianProfileID, createdBy int64) (GuardianInvitationRecord, error)
	ValidateGuardianInvitation(ctx context.Context, token string) (GuardianInvitationPreview, error)
	AcceptGuardianInvitation(ctx context.Context, token, password, confirmPassword string) (GuardianInvitationAccount, error)
	ResendGuardianInvitation(ctx context.Context, invitationID, actorAccountID int64) error
	GuardianInvitationSchoolSlug(ctx context.Context, token string) string
}

// NewGuardianInvitationService serves the retained contract over the owner's
// invitation and relative access capabilities. A nil port reports
// ErrAccountLifecycleUnavailable from every method.
func NewGuardianInvitationService(invitations GuardianInvitations, access GuardianRelativeAccess) GuardianInvitationService {
	return &guardianInvitationService{invitations: invitations, access: access}
}

type guardianInvitationService struct {
	invitations GuardianInvitations
	access      GuardianRelativeAccess
}

func (s *guardianInvitationService) owner(op string) (GuardianInvitations, error) {
	if s.invitations == nil {
		return nil, &AuthError{Op: op, Err: ErrAccountLifecycleUnavailable}
	}
	return s.invitations, nil
}

func (s *guardianInvitationService) Create(ctx context.Context, req GuardianInvitationCreateRequest) (*authModels.GuardianInvitation, error) {
	invitations, err := s.owner(opGuardianInviteCreate)
	if err != nil {
		return nil, err
	}
	record, err := invitations.CreateGuardianInvitation(ctx, req.GuardianProfileID, req.CreatedBy)
	if err != nil {
		return nil, err
	}
	return guardianInvitationModel(record), nil
}

// guardianInvitationModel rebuilds the persistence model the callers of this
// service still read.
func guardianInvitationModel(record GuardianInvitationRecord) *authModels.GuardianInvitation {
	invitation := &authModels.GuardianInvitation{
		Token: record.Token, GuardianProfileID: record.GuardianProfileID, CreatedBy: record.CreatedBy,
		ExpiresAt: record.ExpiresAt, AcceptedAt: record.AcceptedAt, StudentID: record.StudentID,
		ApprovalStatus: record.ApprovalStatus,
	}
	invitation.ID = record.ID
	invitation.CreatedAt = record.CreatedAt
	invitation.SetTenantID(record.TenantID)
	return invitation
}

func (s *guardianInvitationService) Validate(ctx context.Context, token string) (*GuardianInvitationValidation, error) {
	invitations, err := s.owner(opGuardianInviteValidate)
	if err != nil {
		return nil, err
	}
	preview, err := invitations.ValidateGuardianInvitation(ctx, token)
	if err != nil {
		return nil, err
	}
	return &GuardianInvitationValidation{
		Email: preview.Email, FirstName: preview.FirstName, LastName: preview.LastName,
		ExpiresAt: preview.ExpiresAt, SchoolName: preview.SchoolName, TenantSlug: preview.SchoolSlug,
		SchoolLogoURL: preview.SchoolLogoURL,
	}, nil
}

func (s *guardianInvitationService) Accept(ctx context.Context, token string, data GuardianInvitationAcceptData) (*authModels.Account, error) {
	invitations, err := s.owner(opGuardianInviteAccept)
	if err != nil {
		return nil, err
	}
	account, err := invitations.AcceptGuardianInvitation(ctx, token, data.Password, data.ConfirmPassword)
	if err != nil {
		return nil, err
	}
	created := &authModels.Account{Email: account.Email, Active: true}
	created.ID = account.ID
	return created, nil
}

func (s *guardianInvitationService) Resend(ctx context.Context, invitationID int64, actorAccountID int64) error {
	invitations, err := s.owner(opGuardianInviteResend)
	if err != nil {
		return err
	}
	return invitations.ResendGuardianInvitation(ctx, invitationID, actorAccountID)
}

// GetTenantSlugForToken resolves the tenant slug for a guardian invitation
// token. Best-effort: returns "" on any error or for deleted schools
// (issue #584).
func (s *guardianInvitationService) GetTenantSlugForToken(ctx context.Context, token string) string {
	if s.invitations == nil {
		return ""
	}
	return s.invitations.GuardianInvitationSchoolSlug(ctx, token)
}

func (s *guardianInvitationService) relativeAccess(op string) (GuardianRelativeAccess, error) {
	if s.access == nil {
		return nil, &AuthError{Op: op, Err: ErrAccountLifecycleUnavailable}
	}
	return s.access, nil
}

// The related-accounts methods delegate to the Identity & Access port so the
// staff tab and the parents portal keep their contract while the flows live
// in the owner module.

func (s *guardianInvitationService) InviteToStudent(ctx context.Context, req InviteToStudentRequest) (*InviteToStudentResult, error) {
	access, err := s.relativeAccess("invite guardian to student")
	if err != nil {
		return nil, err
	}
	return access.InviteToStudent(ctx, req)
}

func (s *guardianInvitationService) ApproveInvitation(ctx context.Context, invitationID int64, approverAccountID int64) error {
	access, err := s.relativeAccess("approve guardian invitation")
	if err != nil {
		return err
	}
	return access.ApproveInvitation(ctx, invitationID, approverAccountID)
}

func (s *guardianInvitationService) RejectInvitation(ctx context.Context, invitationID int64, approverAccountID int64) error {
	access, err := s.relativeAccess("reject guardian invitation")
	if err != nil {
		return err
	}
	return access.RejectInvitation(ctx, invitationID, approverAccountID)
}

func (s *guardianInvitationService) PendingInvitationStudentID(ctx context.Context, invitationID int64) (int64, error) {
	access, err := s.relativeAccess("approve guardian invitation")
	if err != nil {
		return 0, err
	}
	return access.PendingInvitationStudentID(ctx, invitationID)
}

func (s *guardianInvitationService) ListPendingApprovalsDetailed(ctx context.Context) ([]*PendingApprovalView, error) {
	access, err := s.relativeAccess("approve guardian invitation")
	if err != nil {
		return nil, err
	}
	return access.ListPendingApprovalsDetailed(ctx)
}

func (s *guardianInvitationService) RevokeAccess(ctx context.Context, req RevokeAccessRequest) error {
	access, err := s.relativeAccess("revoke guardian access")
	if err != nil {
		return err
	}
	return access.RevokeAccess(ctx, req)
}
