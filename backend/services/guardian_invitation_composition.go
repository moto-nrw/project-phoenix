package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/config"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal/legacy"
)

// Identity & Access owns the guardian invitation lifecycle (#2722). This
// file serves the retained guardian invitation service over the public
// capability and binds the two seams the flows leave to the root: the mail,
// which knows the parents portal host and the tenant's token lifetime, and
// the enrollment requests an acceptance claims.

// guardianInvitationWiring is the retained material the guardian invitation
// seams are bound to. outbox is read at call time because the delivery
// module is composed after the identity module.
type guardianInvitationWiring struct {
	settings    config.SettingsService
	schools     organizationtenancy.Query
	outbox      func() platformModels.OutboxEnqueuer
	enrollments guardianEnrollmentClaims
	// parentsURL is the origin of the accept and login links; a guardian
	// signs in on the parents portal, never on the staff frontend.
	parentsURL     string
	fallbackExpiry time.Duration
	logger         *slog.Logger
}

// guardianEnrollmentClaims is the narrow contract the acceptance needs from
// the parent enrollment storage.
type guardianEnrollmentClaims interface {
	BackfillGuardianAccountID(ctx context.Context, accountID int64, email string) (int, error)
}

func (w *guardianInvitationWiring) complete() bool {
	return w != nil && w.schools != nil && w.outbox != nil
}

// guardianInvitationDependencies binds the delivery and the enrollment claim
// the lifecycle flows consume; a nil wiring composes the module without
// them, which is what the read-only and fixture roots want.
func guardianInvitationDependencies(wiring *guardianInvitationWiring) (identityaccessCompose.GuardianInvitationDelivery, identityaccessCompose.GuardianEnrollments, error) {
	if wiring == nil {
		return unsentGuardianMail{}, nil, nil
	}
	if !wiring.complete() {
		return nil, nil, errors.New("identity access composition: the guardian invitation delivery needs the school directory and the e-mail outbox")
	}
	delivery := guardianInvitationDelivery{wiring: wiring}
	if wiring.enrollments == nil {
		return delivery, nil, nil
	}
	return delivery, guardianEnrollments{claims: wiring.enrollments}, nil
}

// unsentGuardianMail serves a root composed without the guardian mail — a
// CLI or a fixture that only reads. The flows still run; nothing is sent.
type unsentGuardianMail struct{}

func (unsentGuardianMail) InvitationExpiry(context.Context) time.Duration {
	return auth.GuardianTokenExpiryFallback
}
func (unsentGuardianMail) SchoolName(context.Context, int64) string { return "" }
func (unsentGuardianMail) EnqueueInvitationEmail(context.Context, identityaccess.GuardianInvitation, identityaccessCompose.GuardianProfile, string) {
}
func (unsentGuardianMail) EnqueueExistingAccountEmail(context.Context, identityaccessCompose.GuardianProfile, string) {
}

// --- delivery ---------------------------------------------------------------

type guardianInvitationDelivery struct{ wiring *guardianInvitationWiring }

func (d guardianInvitationDelivery) logger() *slog.Logger {
	if d.wiring.logger != nil {
		return d.wiring.logger
	}
	return slog.Default()
}

// InvitationExpiry follows the tenant override, then the registry default,
// and falls back to the invitation lifetime the deployment configured.
func (d guardianInvitationDelivery) InvitationExpiry(ctx context.Context) time.Duration {
	hours := config.ResolveIntOrDefault(ctx, d.wiring.settings, configModels.KeyGuardianInvitationTokenExpiryHours, 0, d.logger())
	if hours > 0 {
		return time.Duration(hours) * time.Hour
	}
	if d.wiring.fallbackExpiry > 0 {
		return d.wiring.fallbackExpiry
	}
	return auth.GuardianTokenExpiryFallback
}

// SchoolName resolves the school for the mail subject. Best-effort: an
// unreachable or deleted school leaves it out.
func (d guardianInvitationDelivery) SchoolName(ctx context.Context, tenantID int64) string {
	if tenantID <= 0 {
		return ""
	}
	school, found, err := findSchool(ctx, d.wiring.schools, tenantID)
	if err != nil || !found || school.IsDeleted() {
		return ""
	}
	return school.Name
}

func (d guardianInvitationDelivery) mailer() auth.GuardianInvitationMailer {
	return auth.NewGuardianInvitationMailer(auth.GuardianInvitationMailerConfig{
		Outbox: d.wiring.outbox(), FrontendURL: d.wiring.parentsURL, Logger: d.logger(),
	})
}

func (d guardianInvitationDelivery) EnqueueInvitationEmail(ctx context.Context, invitation identityaccess.GuardianInvitation, profile identityaccessCompose.GuardianProfile, schoolName string) {
	d.mailer().EnqueueInvitation(ctx, invitation.ID, invitation.Token, invitation.ExpiresAt, guardianMailRecipient(profile), schoolName)
}

func (d guardianInvitationDelivery) EnqueueExistingAccountEmail(ctx context.Context, profile identityaccessCompose.GuardianProfile, schoolName string) {
	d.mailer().EnqueueExistingAccount(ctx, guardianMailRecipient(profile), schoolName)
}

func guardianMailRecipient(profile identityaccessCompose.GuardianProfile) auth.GuardianMailRecipient {
	return auth.GuardianMailRecipient{FirstName: profile.FirstName, LastName: profile.LastName, Email: profile.Email}
}

// --- enrollment claim -------------------------------------------------------

type guardianEnrollments struct{ claims guardianEnrollmentClaims }

// ClaimGuardianEnrollments runs inside a savepoint: the claim is
// best-effort, and a failed statement must not leave the acceptance's
// transaction aborted. A savepoint that could not be controlled is the
// exception — the owner aborts the acceptance rather than commit on top of
// a transaction whose state nobody knows.
func (e guardianEnrollments) ClaimGuardianEnrollments(ctx context.Context, accountID int64, email string) (int, error) {
	claimed := 0
	err := tenant.WithSavepoint(ctx, func(savepointCtx context.Context) error {
		var claimErr error
		claimed, claimErr = e.claims.BackfillGuardianAccountID(savepointCtx, accountID, email)
		return claimErr
	})
	if errors.Is(err, tenant.ErrSavepointControl) {
		return 0, fmt.Errorf("%w: %w", identityaccess.ErrTransactionUnusable, err)
	}
	if err != nil {
		return 0, err
	}
	return claimed, nil
}

// --- the retained service's port --------------------------------------------

// guardianInvitations serves the retained guardian invitation service over
// the owner's capability.
type guardianInvitations struct {
	module identityaccess.GuardianInvitations
}

func newGuardianInvitations(module identityaccess.GuardianInvitations) auth.GuardianInvitations {
	if module == nil {
		return nil
	}
	return guardianInvitations{module: module}
}

func (g guardianInvitations) CreateGuardianInvitation(ctx context.Context, guardianProfileID, createdBy int64) (auth.GuardianInvitationRecord, error) {
	invitation, err := g.module.CreateGuardianInvitation(ctx, identityaccess.GuardianInvitationRequest{
		GuardianProfileID: guardianProfileID, CreatedBy: createdBy,
	})
	if err != nil {
		return auth.GuardianInvitationRecord{}, invitationServiceError(err)
	}
	return auth.GuardianInvitationRecord{
		ID: invitation.ID, TenantID: invitation.TenantID, Token: invitation.Token,
		GuardianProfileID: invitation.GuardianProfileID, CreatedBy: invitation.CreatedBy,
		ExpiresAt: invitation.ExpiresAt, AcceptedAt: invitation.AcceptedAt, StudentID: invitation.StudentID,
		ApprovalStatus: invitation.ApprovalStatus, CreatedAt: invitation.CreatedAt,
	}, nil
}

func (g guardianInvitations) ValidateGuardianInvitation(ctx context.Context, token string) (auth.GuardianInvitationPreview, error) {
	preview, err := g.module.ValidateGuardianInvitation(ctx, token)
	if err != nil {
		return auth.GuardianInvitationPreview{}, invitationServiceError(err)
	}
	return auth.GuardianInvitationPreview(preview), nil
}

func (g guardianInvitations) AcceptGuardianInvitation(ctx context.Context, token, password, confirmPassword string) (auth.GuardianInvitationAccount, error) {
	account, err := g.module.AcceptGuardianInvitation(ctx, token, identityaccess.GuardianRegistration{
		Password: password, ConfirmPassword: confirmPassword,
	})
	if err != nil {
		return auth.GuardianInvitationAccount{}, invitationServiceError(err)
	}
	return auth.GuardianInvitationAccount{ID: account.ID, Email: account.Email}, nil
}

func (g guardianInvitations) ResendGuardianInvitation(ctx context.Context, invitationID, actorAccountID int64) error {
	return invitationServiceError(g.module.ResendGuardianInvitation(ctx, invitationID, actorAccountID))
}

func (g guardianInvitations) GuardianInvitationSchoolSlug(ctx context.Context, token string) string {
	return g.module.GuardianInvitationSchoolSlug(ctx, token)
}

// --- the invitation reads other owners' flows need ---------------------------

// guardianInvitationReads serves the People Directory guardian list and the
// parents portal over the owner capability. Both read the invitations as the
// retained model, which stays their ports' value type. The capability is
// read at call time: the guardian service is composed before the module.
type guardianInvitationReads struct {
	current func() identityaccess.GuardianInvitations
}

func newGuardianInvitationReads(current func() identityaccess.GuardianInvitations) guardianInvitationReads {
	return guardianInvitationReads{current: current}
}

func (r guardianInvitationReads) owner() (identityaccess.GuardianInvitations, error) {
	if r.current == nil {
		return nil, errors.New("identity access composition: the guardian invitations are not composed")
	}
	module := r.current()
	if module == nil {
		return nil, errors.New("identity access composition: the guardian invitations are not composed")
	}
	return module, nil
}

func (r guardianInvitationReads) ListOpen(ctx context.Context, guardianProfileIDs []int64) ([]usersSvc.GuardianInvitationRecord, error) {
	module, err := r.owner()
	if err != nil {
		return nil, err
	}
	invitations, err := module.ListOpenGuardianInvitations(ctx, guardianProfileIDs)
	if err != nil {
		return nil, invitationServiceError(err)
	}
	return guardianListRecords(invitations), nil
}

func (r guardianInvitationReads) ListRedeemable(ctx context.Context) ([]usersSvc.GuardianInvitationRecord, error) {
	module, err := r.owner()
	if err != nil {
		return nil, err
	}
	invitations, err := module.ListRedeemableGuardianInvitations(ctx)
	if err != nil {
		return nil, invitationServiceError(err)
	}
	return guardianListRecords(invitations), nil
}

// ListByProfile serves the parents portal, which reads the same rows with a
// narrower record.
func (r guardianInvitationReads) ListByProfile(ctx context.Context, guardianProfileID int64) ([]parentportal.GuardianInvitationRecord, error) {
	module, err := r.owner()
	if err != nil {
		return nil, err
	}
	invitations, err := module.ListGuardianInvitations(ctx, guardianProfileID)
	if err != nil {
		return nil, invitationServiceError(err)
	}
	result := make([]parentportal.GuardianInvitationRecord, 0, len(invitations))
	for _, invitation := range invitations {
		result = append(result, parentportal.GuardianInvitationRecord{
			ID: invitation.ID, GuardianProfileID: invitation.GuardianProfileID, StudentID: invitation.StudentID,
			ExpiresAt: invitation.ExpiresAt, AcceptedAt: invitation.AcceptedAt, ApprovalStatus: invitation.ApprovalStatus,
		})
	}
	return result, nil
}

func guardianListRecords(invitations []identityaccess.GuardianInvitation) []usersSvc.GuardianInvitationRecord {
	result := make([]usersSvc.GuardianInvitationRecord, 0, len(invitations))
	for _, invitation := range invitations {
		result = append(result, usersSvc.GuardianInvitationRecord{
			ID: invitation.ID, TenantID: invitation.TenantID, Token: invitation.Token,
			GuardianProfileID: invitation.GuardianProfileID, CreatedBy: invitation.CreatedBy,
			ExpiresAt: invitation.ExpiresAt, AcceptedAt: invitation.AcceptedAt, EmailSentAt: invitation.EmailSentAt,
			EmailError: invitation.EmailError, StudentID: invitation.StudentID,
			ApprovalStatus: invitation.ApprovalStatus, CreatedAt: invitation.CreatedAt,
		})
	}
	return result
}

// --- school branding --------------------------------------------------------

// The keys a school's settings JSON carries its branding image under. The
// login image serves as the fallback so a school that only set that one
// still brands its invitation pages.
const (
	schoolLogoURLKey       = "logoUrl"
	schoolLoginImageURLKey = "loginImageUrl"
)

// schoolLogoURLFromSettings reads the branding image out of a school's
// settings JSON; malformed settings simply carry no branding.
func schoolLogoURLFromSettings(settingsJSON string) string {
	settingsJSON = strings.TrimSpace(settingsJSON)
	if settingsJSON == "" {
		return ""
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
		return ""
	}
	for _, key := range []string{schoolLogoURLKey, schoolLoginImageURLKey} {
		value, ok := settings[key].(string)
		if ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
