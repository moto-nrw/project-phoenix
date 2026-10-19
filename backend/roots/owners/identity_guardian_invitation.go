package owners

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// GuardianEnrollmentClaims is the narrow contract the acceptance needs from
// the parent enrollment storage.
type GuardianEnrollmentClaims interface {
	BackfillGuardianAccountID(ctx context.Context, accountID int64, email string) (int, error)
}

// GuardianInvitationDependencies binds the delivery and the enrollment claim
// the lifecycle flows consume: the settings that hold the tenant's token
// lifetime, the school read for the mail subject, the outbox, the parents
// portal origin of the accept and login links (a guardian signs in on the
// parents portal, never on the staff frontend) and the deployment's
// fallback expiry. A root composed without the guardian mail passes
// UnsentGuardianMail{} to the lifecycle instead.
func GuardianInvitationDependencies(settings settingsCompose.SettingsService, schools organizationtenancy.Query, outbox MailResultOutbox, enrollments GuardianEnrollmentClaims, parentsURL string, fallbackExpiry time.Duration, logger *slog.Logger) (identityPorts.GuardianInvitationDelivery, identityPorts.GuardianEnrollments, error) {
	if schools == nil || outbox == nil || enrollments == nil {
		return nil, nil, errors.New("identity access composition: the guardian invitation seams need the school directory, the e-mail outbox and the enrollment claim")
	}
	if logger == nil {
		return nil, nil, errors.New("identity access composition: the guardian invitation seams need a logger")
	}
	return guardianInvitationDelivery{
		settings: settings, schools: schools, outbox: outbox,
		parentsURL: parentsURL, fallbackExpiry: fallbackExpiry, logger: logger,
	}, identityaccessCompose.NewSavepointGuardianEnrollments(enrollments), nil
}

// guardianInvitationDelivery holds the settings, school read, outbox,
// parents portal origin, fallback expiry and logger of the guardian mails.
type guardianInvitationDelivery struct {
	settings       settingsCompose.SettingsService
	schools        schoolFinder
	outbox         MailResultOutbox
	parentsURL     string
	fallbackExpiry time.Duration
	logger         *slog.Logger
}

// InvitationExpiry reads the school's token lifetime (tenant override, then
// registry default) and lets the owner fall back to the deployment's.
func (d guardianInvitationDelivery) InvitationExpiry(ctx context.Context) time.Duration {
	hours := settingsCompose.ResolveIntOrDefault(ctx, d.settings, configModels.KeyGuardianInvitationTokenExpiryHours, 0, d.logger)
	return identityaccess.GuardianInvitationExpiry(hours, d.fallbackExpiry)
}

// SchoolName resolves the school for the mail subject. Best-effort: an
// unreachable or deleted school leaves it out.
func (d guardianInvitationDelivery) SchoolName(ctx context.Context, tenantID int64) string {
	if tenantID <= 0 {
		return ""
	}
	school, found, err := LookupSchool(ctx, d.schools, tenantID)
	if err != nil || !found || school.IsDeleted() {
		return ""
	}
	return school.Name
}

func (d guardianInvitationDelivery) mailer() GuardianInvitationMailer {
	return NewGuardianInvitationMailer(GuardianInvitationMailerConfig{
		Outbox: d.outbox, FrontendURL: d.parentsURL, Logger: d.logger,
	})
}

func (d guardianInvitationDelivery) EnqueueInvitationEmail(ctx context.Context, invitation identityaccess.GuardianInvitation, profile identityPorts.GuardianProfile, schoolName string) (int64, bool) {
	return d.mailer().EnqueueInvitation(ctx, invitation.ID, invitation.Token, invitation.ExpiresAt, guardianMailRecipient(profile), schoolName)
}

func (d guardianInvitationDelivery) EnqueueExistingAccountEmail(ctx context.Context, profile identityPorts.GuardianProfile, schoolName string) (int64, bool) {
	return d.mailer().EnqueueExistingAccount(ctx, guardianMailRecipient(profile), schoolName)
}

// EnqueueWelcomeEmail queues the welcome that follows a new access (#3534).
// Settings failures return a complete help URL without the unresolved
// parameters, but their transaction must be rolled back before enqueueing.
// Each operation therefore has its own savepoint; neither may abort the
// invitation transaction.
func (d guardianInvitationDelivery) EnqueueWelcomeEmail(ctx context.Context, profile identityPorts.GuardianProfile, tenantID int64, schoolName string, precedingOutboxID int64) {
	var helpURL string
	d.inSavepoint(ctx, func(savepointCtx context.Context) error {
		var err error
		helpURL, err = welcomeHelpURL(savepointCtx, d.parentsURL, welcomeHelpRoleParent, tenantID, d.settings, d.logger)
		return err
	})
	d.inSavepoint(ctx, func(savepointCtx context.Context) error {
		return d.mailer().EnqueueWelcome(savepointCtx, profile.ID, guardianMailRecipient(profile), schoolName, helpURL, precedingOutboxID)
	})
}

// inSavepoint runs a best-effort step of the welcome. The step logs its own
// failure; only a savepoint that could not be controlled is reported here.
// Without a transaction there is nothing to protect.
func (d guardianInvitationDelivery) inSavepoint(ctx context.Context, step func(context.Context) error) {
	if err := identityaccessCompose.RunMailStep(ctx, step); err != nil {
		d.logger.Error("guardian welcome email: savepoint failed",
			slog.String("error", err.Error()))
	}
}

func guardianMailRecipient(profile identityPorts.GuardianProfile) GuardianMailRecipient {
	return GuardianMailRecipient{FirstName: profile.FirstName, LastName: profile.LastName, Email: profile.Email}
}

// guardianInvitationReads serves the People Directory guardian list and the
// parents portal over Identity & Access's invitation reads. Both read the
// invitations as the retained model, which stays their ports' value type.
// The reads need only the database, so the guardian service is composed
// before the Identity & Access module.
type guardianInvitationReads struct {
	reads identityaccess.GuardianInvitationReads
}

func NewGuardianInvitationReads(reads identityaccess.GuardianInvitationReads) guardianInvitationReads {
	return guardianInvitationReads{reads: reads}
}

func (r guardianInvitationReads) ListOpen(ctx context.Context, guardianProfileIDs []int64) ([]peoplePorts.GuardianInvitationRecord, error) {
	invitations, err := r.reads.ListOpenGuardianInvitations(ctx, guardianProfileIDs)
	if err != nil {
		return nil, err
	}
	return guardianListRecords(invitations), nil
}

func (r guardianInvitationReads) ListRedeemable(ctx context.Context) ([]peoplePorts.GuardianInvitationRecord, error) {
	invitations, err := r.reads.ListRedeemableGuardianInvitations(ctx)
	if err != nil {
		return nil, err
	}
	return guardianListRecords(invitations), nil
}

// ListByProfile serves the parents portal, which reads the same rows with a
// narrower record.
func (r guardianInvitationReads) ListByProfile(ctx context.Context, guardianProfileID int64) ([]parentportal.GuardianInvitationRecord, error) {
	invitations, err := r.reads.ListGuardianInvitations(ctx, guardianProfileID)
	if err != nil {
		return nil, err
	}
	result := make([]parentportal.GuardianInvitationRecord, 0, len(invitations))
	for _, invitation := range invitations {
		result = append(result, parentportal.GuardianInvitationRecord{
			ID: invitation.ID, GuardianProfileID: invitation.GuardianProfileID, StudentID: invitation.StudentID,
			ExpiresAt: invitation.ExpiresAt, AcceptedAt: invitation.AcceptedAt, Rejected: invitation.IsRejected(),
		})
	}
	return result, nil
}

func guardianListRecords(invitations []identityaccess.GuardianInvitation) []peoplePorts.GuardianInvitationRecord {
	result := make([]peoplePorts.GuardianInvitationRecord, 0, len(invitations))
	for _, invitation := range invitations {
		result = append(result, peoplePorts.GuardianInvitationRecord{
			ID: invitation.ID, TenantID: invitation.TenantID, Token: invitation.Token,
			GuardianProfileID: invitation.GuardianProfileID, CreatedBy: invitation.CreatedBy,
			ExpiresAt: invitation.ExpiresAt, AcceptedAt: invitation.AcceptedAt, EmailSentAt: invitation.EmailSentAt,
			EmailError: invitation.EmailError, StudentID: invitation.StudentID,
			ApprovalStatus: invitation.ApprovalStatus, CreatedAt: invitation.CreatedAt,
		})
	}
	return result
}

// schoolLogoURLFromSettings reads the branding image out of a school's
// settings JSON by Organisation & Tenancy's rule.
func schoolLogoURLFromSettings(settingsJSON string) string {
	return organizationtenancy.SchoolLogoURL(settingsJSON)
}

// The parents portal may not name the owner's contract, so the root serves
// its consumer-owned GuardianAccess port over the relative access capability
// and translates the refusals into the portal's own sentinels (#3332).
type parentGuardianAccess struct {
	access identityaccess.GuardianRelativeAccess
}

// NewParentGuardianAccess binds the parents portal's relative access port. A
// nil capability yields nil, which the portal reports as unavailable.
func NewParentGuardianAccess(access identityaccess.GuardianRelativeAccess) parentportal.GuardianAccess {
	if access == nil {
		return nil
	}
	return parentGuardianAccess{access: access}
}

func (a parentGuardianAccess) InviteToStudent(ctx context.Context, request parentportal.GuardianInviteRequest) (parentportal.GuardianInviteOutcome, error) {
	requestedBy := request.RequestedByAccountID
	result, err := a.access.InviteToStudent(ctx, identityaccess.InviteToStudentRequest{
		StudentID: request.StudentID, Email: request.Email,
		FirstName: request.FirstName, LastName: request.LastName,
		CreatedBy: request.CreatedBy, RequestedByParentAccountID: &requestedBy,
		RequireApproval: request.RequireApproval, ConfirmRoleUpgrade: request.ConfirmRoleUpgrade,
	})
	if err != nil {
		return parentportal.GuardianInviteOutcome{}, parentGuardianAccessError(err)
	}
	return parentportal.GuardianInviteOutcome{
		Outcome: string(result.Outcome), GuardianProfileID: result.GuardianProfileID,
		ExistingRole: result.ExistingRole,
	}, nil
}

// RevokeAccess runs as Identity & Access's parents-portal removal.
func (a parentGuardianAccess) RevokeAccess(ctx context.Context, revocation parentportal.GuardianAccessRevocation) error {
	return parentGuardianAccessError(a.access.RevokeAccess(ctx, identityaccess.ParentRevokeAccessRequest(
		revocation.StudentID, revocation.GuardianProfileID, revocation.ActorAccountID,
	)))
}

// parentGuardianAccessError maps the owner's refusals onto the portal's
// sentinels, keeping each message the portal renders.
func parentGuardianAccessError(err error) error {
	if err == nil {
		return nil
	}
	var operation *identityaccess.AuthenticationError
	if errors.As(err, &operation) && operation == err {
		return parentGuardianAccessError(operation.Err)
	}
	for _, sentinel := range parentGuardianSentinels {
		if !errors.Is(err, sentinel.public) {
			continue
		}
		if err == sentinel.public {
			return sentinel.retained
		}
		return &retainedError{text: err.Error(), sentinel: sentinel.retained, cause: err}
	}
	return err
}

var parentGuardianSentinels = []retainedSentinel{
	{identityaccess.ErrCannotRemovePrimaryGuardian, parentportal.ErrCannotRemovePrimaryGuardian},
	{identityaccess.ErrCannotRemoveStaffManagedGuardian, parentportal.ErrCannotRemoveStaffManagedGuardian},
	{identityaccess.ErrCannotRemoveOwnAccess, parentportal.ErrCannotRemoveOwnAccess},
	{identityaccess.ErrCannotRemovePayerGuardian, parentportal.ErrCannotRemovePayerGuardian},
	{identityaccess.ErrInviteSocialWorkerManaged, parentportal.ErrInviteSocialWorkerManaged},
}
