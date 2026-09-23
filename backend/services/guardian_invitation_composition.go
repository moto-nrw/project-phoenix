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
	"github.com/moto-nrw/project-phoenix/services/config"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// Identity & Access owns the guardian invitation lifecycle (#2722). This
// file binds the two seams the flows leave to the root — the mail, which
// knows the parents portal host and the tenant's token lifetime, and the
// enrollment requests an acceptance claims — and the runtimes the consumers
// that may not name the owner's contract reach it through (#3332).

// GuardianInvitationCapability is what the guardian invitation routes, the
// enrollment decisions, the guardian directory and the parents portal
// consume: the invitation lifecycle and the relative access flows of one
// owner. Each consumer takes the narrower half it needs.
type GuardianInvitationCapability interface {
	identityaccess.GuardianInvitations
	identityaccess.GuardianRelativeAccess
}

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
	return w != nil && w.schools != nil && w.outbox != nil && w.enrollments != nil
}

// guardianInvitationDependencies binds the delivery and the enrollment claim
// the lifecycle flows consume; a nil wiring composes the module without
// them, which is what the read-only and fixture roots want.
func guardianInvitationDependencies(wiring *guardianInvitationWiring) (identityaccessCompose.GuardianInvitationDelivery, identityaccessCompose.GuardianEnrollments, error) {
	if wiring == nil {
		return unsentGuardianMail{}, nil, nil
	}
	if !wiring.complete() {
		return nil, nil, errors.New("identity access composition: the guardian invitation seams need the school directory, the e-mail outbox and the enrollment claim")
	}
	return guardianInvitationDelivery{wiring: wiring}, guardianEnrollments{claims: wiring.enrollments}, nil
}

// unsentGuardianMail serves a root composed without the guardian mail — a
// CLI or a fixture that only reads. The flows still run; nothing is sent.
type unsentGuardianMail struct{}

func (unsentGuardianMail) InvitationExpiry(context.Context) time.Duration {
	return GuardianTokenExpiryFallback
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
	return GuardianTokenExpiryFallback
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

func (d guardianInvitationDelivery) mailer() GuardianInvitationMailer {
	return NewGuardianInvitationMailer(GuardianInvitationMailerConfig{
		Outbox: d.wiring.outbox(), FrontendURL: d.wiring.parentsURL, Logger: d.logger(),
	})
}

func (d guardianInvitationDelivery) EnqueueInvitationEmail(ctx context.Context, invitation identityaccess.GuardianInvitation, profile identityaccessCompose.GuardianProfile, schoolName string) {
	d.mailer().EnqueueInvitation(ctx, invitation.ID, invitation.Token, invitation.ExpiresAt, guardianMailRecipient(profile), schoolName)
}

func (d guardianInvitationDelivery) EnqueueExistingAccountEmail(ctx context.Context, profile identityaccessCompose.GuardianProfile, schoolName string) {
	d.mailer().EnqueueExistingAccount(ctx, guardianMailRecipient(profile), schoolName)
}

func guardianMailRecipient(profile identityaccessCompose.GuardianProfile) GuardianMailRecipient {
	return GuardianMailRecipient{FirstName: profile.FirstName, LastName: profile.LastName, Email: profile.Email}
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
		return nil, err
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
		return nil, err
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
		return nil, err
	}
	result := make([]parentportal.GuardianInvitationRecord, 0, len(invitations))
	for _, invitation := range invitations {
		result = append(result, parentportal.GuardianInvitationRecord{
			ID: invitation.ID, GuardianProfileID: invitation.GuardianProfileID, StudentID: invitation.StudentID,
			ExpiresAt: invitation.ExpiresAt, AcceptedAt: invitation.AcceptedAt, Rejected: invitation.ApprovalStatus == identityaccess.GuardianInvitationApprovalRejected,
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

// --- the parents portal's guardian access runtime ---------------------------

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

// RevokeAccess always runs as a parent removal: the parents portal never
// holds the staff authority, and it never carries guardians:financial, so the
// payer guard stays closed.
func (a parentGuardianAccess) RevokeAccess(ctx context.Context, revocation parentportal.GuardianAccessRevocation) error {
	return parentGuardianAccessError(a.access.RevokeAccess(ctx, identityaccess.RevokeAccessRequest{
		StudentID: revocation.StudentID, GuardianProfileID: revocation.GuardianProfileID,
		ActorAccountID: revocation.ActorAccountID, ByParent: true,
	}))
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

// --- the enrollment decisions' guardian invitation runtime ------------------

// GuardianInvitationRuntime is the plain-typed runtime the enrollment
// decision routes consume: approving the first child of a family fires one
// invitation, fire-and-forget, so the runtime needs no outcome beyond the
// error.
type GuardianInvitationRuntime struct {
	Create func(ctx context.Context, guardianProfileID, createdBy int64) error
}

// EnrollmentGuardianInvitationRuntime binds that one call. A nil capability
// yields the zero runtime, which the routes report as not configured.
func EnrollmentGuardianInvitationRuntime(invitations identityaccess.GuardianInvitations) GuardianInvitationRuntime {
	if invitations == nil {
		return GuardianInvitationRuntime{}
	}
	return GuardianInvitationRuntime{
		Create: func(ctx context.Context, guardianProfileID, createdBy int64) error {
			_, err := invitations.CreateGuardianInvitation(ctx, identityaccess.GuardianInvitationRequest{
				GuardianProfileID: guardianProfileID, CreatedBy: createdBy,
			})
			return err
		},
	}
}
