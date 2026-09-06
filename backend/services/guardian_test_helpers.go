package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/moto-nrw/project-phoenix/services/platform"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type GuardianTestModule struct {
	PeopleDirectory peopledirectory.Capability
	runtime         GuardianDirectoryRuntime
}

func (m GuardianTestModule) NewGuardianDirectoryRuntime(*bun.DB) GuardianDirectoryRuntime {
	return m.runtime
}

func NewGuardianTestModule(db *bun.DB, unit tenant.UnitOfWork) (GuardianTestModule, error) {
	auth, err := NewAuthTestModule(db, unit)
	if err != nil {
		return GuardianTestModule{}, err
	}
	command, err := auditSvc.NewCommand(repositories.NewTestAuditStore(db), func(auditSvc.AppendObservation) {})
	if err != nil {
		return GuardianTestModule{}, err
	}
	r, err := repositories.NewAuthTestRepositories(db, command)
	if err != nil {
		return GuardianTestModule{}, err
	}
	g := repositories.NewGuardianTestRepositories(db, command)
	people, err := repositories.NewPeopleDirectory(db)
	if err != nil {
		return GuardianTestModule{}, err
	}
	identity, _, err := newStaffIdentityForTests(db)
	if err != nil {
		return GuardianTestModule{}, err
	}
	cfg := currentFactoryConfig()
	hours := cfg.InvitationTokenExpiryHours
	if hours <= 0 {
		hours = 48
	} else if hours > 168 {
		hours = 168
	}
	from := email.NewEmail(cfg.EmailFromName, cfg.EmailFromAddress)
	if from.Address == "" {
		from = email.NewEmail("moto", "no-reply@moto.local")
	}
	mailer := email.NewMockMailer()
	mailIdentity := platform.NewTenantMailIdentityService(r.School, func(ctx context.Context, tenantID int64) (string, error) {
		return auth.Settings.ResolveStringForTenant(ctx, tenantID, configModels.KeyEmailReplyToAddress)
	}, slog.Default())
	guardian := users.NewGuardianService(users.GuardianServiceDependencies{
		GuardianProfileRepo: r.GuardianProfile, GuardianPhoneNumberRepo: g.Phone, StudentGuardianRepo: r.StudentGuardian,
		GuardianInvitationRepo: r.GuardianInvitation, AccountRepo: r.Account, AccountParentRepo: r.AccountParent,
		AccountTenantRepo: r.AccountTenant, AccountRoleRepo: r.AccountRole, RoleRepo: r.Role, StudentRepo: r.Student, PersonRepo: r.Person,
		GuardianFinancialRepo: g.Financial, GuardianFinancialAudit: g.FinancialAudit, DataAccessLog: g.AccessLog,
		Mailer: mailer, Dispatcher: email.NewDispatcher(mailer, slog.Default()), FrontendURL: cfg.FrontendURL, DefaultFrom: from,
		InvitationExpiry: time.Duration(hours) * time.Hour, MailIdentity: mailIdentity, DB: db,
	})
	people.(guardianProviderBinder).BindGuardianProvider(&guardianDirectoryProvider{guardians: guardian, db: db})
	// The compatibility carrier only maps the existing four capabilities to
	// the production runtime closures. It constructs no additional services.
	carrier := &Factory{Guardian: guardian, GuardianInvitation: auth.GuardianInvitation, UserContext: identity, ListExport: listexport.NewService()}
	return GuardianTestModule{PeopleDirectory: people, runtime: carrier.NewGuardianDirectoryRuntime(db)}, nil
}
