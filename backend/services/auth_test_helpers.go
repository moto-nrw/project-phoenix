package services

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailoutbox"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	authjwt "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type AuthTestModule struct {
	// Auth and AccountAuthentication are the same composed Identity &
	// Access module: the surfaces the behaviour tests drive consume it
	// through the capabilities they were bound to (#3364).
	Auth                  *identityaccess.Module
	AccountAuthentication *identityaccess.Module
	DemoAccess            *identityaccess.DemoAccess
	StaffPINAuth          StaffPINAuthenticator
	Invitation            InvitationCapability
	GuardianInvitation    GuardianInvitationCapability
	Schools               organizationtenancy.Capability
	Settings              config.SettingsService
	// MFA, Passkeys, OperatorMFA and OperatorPasskeys are the Identity &
	// Access second factor and both portals' WebAuthn ceremonies (#3331),
	// composed over the same repositories the root uses.
	MFA              identityaccess.AccountMFA
	Passkeys         identityaccess.AccountPasskeyFlows
	OperatorMFA      identityaccess.OperatorMFAFlows
	OperatorPasskeys identityaccess.OperatorPasskeyFlows
	// Repos and TokenAuth let a behaviour test read the rows the flows
	// wrote and mint the tokens they expect.
	Repos     *repositories.Factory
	TokenAuth *authjwt.TokenAuth
}

type InvitationTestModule struct {
	Persistence *repositories.InvitationPersistence
	Invitation  InvitationCapability
	// Auth is the composed Identity & Access module, for the invitation
	// HTTP tests that mount the whole auth router and therefore need the
	// account-lifecycle capability its unrelated routes bind (#3364).
	Auth *identityaccess.Module
}

func NewInvitationTestModule(db *bun.DB, unit tenant.UnitOfWork) (InvitationTestModule, error) {
	repos, err := repositories.NewInvitationPersistence(db)
	if err != nil {
		return InvitationTestModule{}, err
	}
	auth, err := NewAuthTestModule(db, unit)
	if err != nil {
		return InvitationTestModule{}, err
	}
	return InvitationTestModule{Persistence: repos, Invitation: auth.Invitation, Auth: auth.Auth}, nil
}

// AccountMFARecords is the account MFA record port the composed module
// consumes, named here so a behaviour test can decorate it without reaching
// into the module's composition package.
type AccountMFARecords = identityaccessCompose.AccountMFARecords

// AuthTestOption overrides a default of the composed test module.
type AuthTestOption func(*authTestSettings)

type authTestSettings struct {
	mailer           email.Mailer
	rateLimitEnabled bool
	resetBackoff     []time.Duration
	mfaBackoff       []time.Duration
	mfaRecords       func(AccountMFARecords) AccountMFARecords
	mfaCapability    identityaccess.AccountMFA
	mfaSettings      config.SettingsService
	staffCreateErr   error
	standingDemo     string
}

// WithStandingDemoSchool composes the demo access with the fallback of #3463:
// every access enters the school with this slug instead of its own.
func WithStandingDemoSchool(slug string) AuthTestOption {
	return func(settings *authTestSettings) { settings.standingDemo = slug }
}

// WithAuthTestMailer composes the module on the given mailer, so a test can
// read the mails the flows send.
func WithAuthTestMailer(mailer email.Mailer) AuthTestOption {
	return func(s *authTestSettings) { s.mailer = mailer }
}

// WithAuthTestPasswordResetRateLimit enables the per-address reset rate
// limit, which the test configuration leaves off.
func WithAuthTestPasswordResetRateLimit(enabled bool) AuthTestOption {
	return func(s *authTestSettings) { s.rateLimitEnabled = enabled }
}

// WithAuthTestStaffCreateFailure composes the module with a staff directory
// whose insert fails, so a test can prove that a failure inside the identity
// chain rolls the whole acceptance back.
func WithAuthTestStaffCreateFailure(err error) AuthTestOption {
	return func(s *authTestSettings) { s.staffCreateErr = err }
}

// WithAuthTestPasswordResetBackoff shortens the retry spacing of the reset
// mail, so a test that asserts the recorded failure waits milliseconds
// instead of the production twenty seconds.
func WithAuthTestPasswordResetBackoff(backoff ...time.Duration) AuthTestOption {
	return func(s *authTestSettings) { s.resetBackoff = backoff }
}

// WithAuthTestMFABackoff shortens the retry spacing of the MFA mails, so a
// test that asserts a failed synchronous send waits milliseconds instead of
// the production twenty seconds.
func WithAuthTestMFABackoff(backoff ...time.Duration) AuthTestOption {
	return func(s *authTestSettings) { s.mfaBackoff = backoff }
}

// WithAuthTestMFARecords wraps the account MFA record port, so a test can
// make one statement fail and prove the flow refuses instead of degrading.
func WithAuthTestMFARecords(decorate func(AccountMFARecords) AccountMFARecords) AuthTestOption {
	return func(s *authTestSettings) { s.mfaRecords = decorate }
}

// WithAuthTestMFASettings resolves the MFA gate's school settings through
// the given service, so a test can drive security.mfa_mode and the
// trusted-device values without touching the rest of the composition.
func WithAuthTestMFASettings(settings config.SettingsService) AuthTestOption {
	return func(s *authTestSettings) { s.mfaSettings = settings }
}

// WithAuthTestMFACapability composes the module over the given account
// second factor instead of the one over the repositories, so a test can
// drive the login branches the gate decides between.
func WithAuthTestMFACapability(capability identityaccess.AccountMFA) AuthTestOption {
	return func(s *authTestSettings) { s.mfaCapability = capability }
}

// authTestRelyingParty is the passkey relying party the composed test module
// runs its ceremonies under. The localhost value makes the module follow the
// request's own origin, so every school subdomain works.
var authTestRelyingParty = identityaccessCompose.PasskeyDependencies{
	RPID: "localhost", RPName: "moto",
	TenantDomain: "localhost", OperatorFrontendURL: "http://operator.localhost",
}

func NewAuthTestModule(db *bun.DB, unit tenant.UnitOfWork, options ...AuthTestOption) (AuthTestModule, error) {
	settings, err := NewSettingsTestModule(db, unit)
	if err != nil {
		return AuthTestModule{}, err
	}
	logger := slog.Default()
	command, err := auditSvc.NewCommand(repositories.NewTestAuditStore(db), func(auditSvc.AppendObservation) {})
	if err != nil {
		return AuthTestModule{}, err
	}
	r, err := repositories.NewAuthTestRepositories(db, command)
	if err != nil {
		return AuthTestModule{}, err
	}
	cfg := currentFactoryConfig()
	settingsOverrides := authTestSettings{mailer: email.NewMockMailer(), rateLimitEnabled: cfg.RateLimitEnabled}
	for _, option := range options {
		option(&settingsOverrides)
	}
	mailer := settingsOverrides.mailer
	dispatcher := email.NewDispatcher(mailer, logger)
	defaultFrom := email.NewEmail(cfg.EmailFromName, cfg.EmailFromAddress)
	if defaultFrom.Address == "" {
		defaultFrom = email.NewEmail("moto", "no-reply@moto.local")
	}
	frontendURL := strings.TrimRight(cfg.FrontendURL, "/")
	parentsURL := strings.TrimRight(cfg.ParentsURL, "/")
	schoolURL := strings.TrimRight(cfg.SchoolURL, "/")
	resetMinutes := cfg.PasswordResetExpiryMinutes
	if resetMinutes <= 0 {
		resetMinutes = 30
	} else if resetMinutes > 1440 {
		resetMinutes = 1440
	}
	inviteHours := cfg.InvitationTokenExpiryHours
	if inviteHours <= 0 {
		inviteHours = 48
	} else if inviteHours > 168 {
		inviteHours = 168
	}
	tokenAuth, err := authjwt.NewTokenAuthWithDurations(cfg.JWTSecret, cfg.JWTExpiry, cfg.JWTRefreshExpiry)
	if err != nil {
		return AuthTestModule{}, err
	}
	codec, err := signedIdentityTokensOf(tokenAuth)
	if err != nil {
		return AuthTestModule{}, err
	}
	// The delivery module is composed after the identity module, so the
	// guardian mail reads its outbox at call time, as the factory does.
	var deliveryModule DeliveryTestModule
	identity := emailoutbox.NewTenantMailIdentity(schoolContactDirectory{schools: r.School}, func(ctx context.Context, tenantID int64) (string, error) {
		return settings.Settings.ResolveStringForTenant(ctx, tenantID, configModels.KeyEmailReplyToAddress)
	}, logger)
	sessionRepos := sessionRepositoriesOf(r, r.School)
	if settingsOverrides.staffCreateErr != nil {
		sessionRepos.lifecycle.staff = failingStaffDirectory{StaffRepository: r.Staff, err: settingsOverrides.staffCreateErr}
	}
	// The operator flows are composed here too: the operator second factor
	// and both portals' passkey ceremonies (#3331) mint their sessions
	// through them, so a test module without them would compose a module
	// production never builds.
	owners, err := newOwnerCapabilitiesForTests(db)
	if err != nil {
		return AuthTestModule{}, err
	}
	operators, err := newOperatorDependencies(operatorAuthenticationWiring{
		repos:         operatorRepositoriesOf(r),
		organizations: owners.organizations,
		persons:       owners.persons,
		membership:    owners.membership,
		logger:        logger,
	})
	if err != nil {
		return AuthTestModule{}, err
	}
	identityAccess, err := newIdentityAccessWithSessions(db, accountAuthenticationWiring{
		repos: sessionRepos, codec: codec, settings: settings.Settings, audit: command, logger: logger,
		operators:  operators,
		demoAccess: true, demoStandingSchool: settingsOverrides.standingDemo,
		mfa: &mfaWiring{
			repos: r, settings: mfaSettingsService(settings.Settings, settingsOverrides),
			dispatcher: dispatcher, defaultFrom: defaultFrom, frontendURL: frontendURL,
			jwtSecret: mfaTestSecret(), logger: logger, backoff: settingsOverrides.mfaBackoff,
			decorate: settingsOverrides.mfaRecords, capability: settingsOverrides.mfaCapability,
			passkeys: &authTestRelyingParty,
		},
		lifecycle: &lifecycleWiring{
			settings: settings.Settings, audit: command,
			guardianMail: &guardianInvitationWiring{
				settings: settings.Settings, schools: r.School,
				outbox:      func() platformModels.OutboxEnqueuer { return outboxEnqueuer{outbox: deliveryModule.EmailOutbox} },
				enrollments: r.ParentEnrollmentRequest, parentsURL: parentsURL,
				fallbackExpiry: time.Duration(inviteHours) * time.Hour, logger: logger,
			},
		},
		resets: &passwordResetWiring{
			dispatcher: dispatcher, defaultFrom: defaultFrom,
			staffURL: frontendURL, parentsURL: parentsURL, schoolURL: schoolURL,
			expiry: time.Duration(resetMinutes) * time.Minute, rateLimitEnabled: settingsOverrides.rateLimitEnabled,
			backoff: settingsOverrides.resetBackoff,
		},
		invitations: &invitationWiring{
			dispatcher: dispatcher, defaultFrom: defaultFrom, staffURL: frontendURL, schoolURL: schoolURL,
			mailIdentity: identity, expiry: time.Duration(inviteHours) * time.Hour,
			backoff: settingsOverrides.resetBackoff,
		},
	})
	if err != nil {
		return AuthTestModule{}, err
	}
	if err := identityAccess.SetTenantRuntime(unit); err != nil {
		return AuthTestModule{}, err
	}
	invitation := InvitationCapability(identityAccess)
	deliveryModule, err = NewDeliveryTestModule(db, unit)
	if err != nil {
		return AuthTestModule{}, err
	}
	guardian := GuardianInvitationCapability(identityAccess)
	return AuthTestModule{
		Auth: identityAccess, AccountAuthentication: identityAccess,
		DemoAccess:   identityAccess.DemoAccess(),
		StaffPINAuth: NewStaffPINAuthenticator(identityAccess),
		MFA:          identityAccess, Passkeys: identityAccess,
		OperatorMFA: identityAccess, OperatorPasskeys: identityAccess,
		Repos: r, TokenAuth: tokenAuth,
		Invitation: invitation, GuardianInvitation: guardian,
		Schools: r.School, Settings: settings.Settings,
	}, nil
}

// IdentityAccessForTests composes the Identity & Access module over the
// given repositories the way the factory does it, for behaviour tests that
// exercise sessions, the account lifecycle or the role administration
// without the whole factory (#3364).
func IdentityAccessForTests(repos *repositories.Factory, cfg IdentityAccessTestConfig, db *bun.DB, logger *slog.Logger) (*identityaccess.Module, error) {
	if logger == nil {
		logger = slog.Default()
	}
	signer := cfg.TokenAuth
	if signer == nil {
		created, err := configuredTokenAuth()
		if err != nil {
			return nil, err
		}
		signer = created
	}
	codec, err := signedIdentityTokensOf(signer)
	if err != nil {
		return nil, err
	}
	module, err := newIdentityAccessWithSessions(db, accountAuthenticationWiring{
		repos: sessionRepositoriesOf(repos, repos.School), codec: codec, settings: cfg.Settings,
		audit: cfg.Audit, logger: logger,
		mfa: &mfaWiring{
			repos: repos, settings: cfg.Settings,
			dispatcher: cfg.Dispatcher, defaultFrom: cfg.DefaultFrom, frontendURL: cfg.FrontendURL,
			jwtSecret: mfaTestSecret(), logger: logger,
		},
		lifecycle: &lifecycleWiring{settings: cfg.Settings, audit: cfg.Audit},
		resets: &passwordResetWiring{
			dispatcher: cfg.Dispatcher, defaultFrom: cfg.DefaultFrom,
			staffURL: cfg.FrontendURL, parentsURL: cfg.ParentsURL, schoolURL: cfg.SchoolURL,
			expiry: cfg.PasswordResetExpiry, rateLimitEnabled: cfg.RateLimitEnabled,
		},
	})
	if err != nil {
		return nil, err
	}
	if err := module.SetTenantRuntime(cfg.TenantRuntime); err != nil {
		return nil, err
	}
	return module, nil
}

// IdentityAccessTestConfig is what a behaviour test supplies to compose the
// module: the signer, the mail transport, the portal hosts and the audit
// ledger. Every zero value composes the module without that seam.
type IdentityAccessTestConfig struct {
	TokenAuth           *authjwt.TokenAuth
	TenantRuntime       tenant.UnitOfWork
	Settings            config.SettingsService
	Audit               auditModels.Command
	Dispatcher          *email.Dispatcher
	DefaultFrom         email.Email
	FrontendURL         string
	ParentsURL          string
	SchoolURL           string
	PasswordResetExpiry time.Duration
	RateLimitEnabled    bool
}

// failingStaffDirectory fails every staff insert; every other operation is
// the real repository.
type failingStaffDirectory struct {
	userModels.StaffRepository
	err error
}

func (d failingStaffDirectory) Create(context.Context, *userModels.Staff) error { return d.err }

// mfaTestSecret is the signing secret the composed test module derives its
// trusted-device HMAC key from: the process configuration, exactly as the
// production root reads it, so no key is stored in source.
func mfaTestSecret() string { return currentFactoryConfig().JWTSecret }

// mfaSettingsService is the settings service the MFA gate resolves through:
// the composed one, or the one a test supplied.
func mfaSettingsService(composed config.SettingsService, overrides authTestSettings) config.SettingsService {
	if overrides.mfaSettings != nil {
		return overrides.mfaSettings
	}
	return composed
}
