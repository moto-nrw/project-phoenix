package api

import (
	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type GuardianTestModule struct {
	PeopleDirectory peopledirectory.Capability
	// PhotoRuntime is the slot the directory resolves its photo runtime from,
	// filled by the graph that also owns the feature gate and the file cleanup.
	PhotoRuntime *peopleCompose.StudentPhotoRuntime
	runtime      services.GuardianDirectoryRuntime
}

func (m GuardianTestModule) NewGuardianDirectoryRuntime(transactionruntime.Database) services.GuardianDirectoryRuntime {
	return m.runtime
}

func NewGuardianTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (GuardianTestModule, error) {
	auth, err := NewAuthTestModule(db, unit)
	if err != nil {
		return GuardianTestModule{}, err
	}
	command, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return GuardianTestModule{}, err
	}
	r, err := NewMembershipTestRepositories(db)
	if err != nil {
		return GuardianTestModule{}, err
	}
	g := NewGuardianTestRepositories(db, command)
	people, photoRuntime, err := services.NewPeopleDirectoryWithPhotos(db.Pool())
	if err != nil {
		return GuardianTestModule{}, err
	}
	identity, _, err := newStaffIdentityForTests(db)
	if err != nil {
		return GuardianTestModule{}, err
	}
	guardian := peopleCompose.NewGuardianService(peopleCompose.GuardianServiceDependencies{
		GuardianProfileRepo: r.GuardianProfile, GuardianPhoneNumberRepo: g.Phone, StudentGuardianRepo: r.StudentGuardian,
		GuardianInvitations: services.NewGuardianInvitationReads(func() identityaccess.GuardianInvitations { return auth.AccountAuthentication }),
		StudentRepo:         NewUnboundStudentRepository(db), PersonRepo: r.Person,
		GuardianFinancialRepo:  g.Financial,
		GuardianFinancialAudit: services.NewGuardianFinancialAudit(g.FinancialAudit), DataAccessLog: services.NewGuardianDataAccessLog(g.AccessLog),
		RolePresets: services.GuardianRolePresets{}, DB: db.Pool(),
	})
	people.(services.GuardianProviderBinder).BindGuardianProvider(&services.GuardianDirectoryProvider{Guardians: guardian, Runtime: peopleCompose.NewLegacyRepositoryRuntime(db.Pool())})
	// The compatibility carrier only maps the existing four capabilities to
	// the production runtime closures. It constructs no additional services.
	carrier := &services.Factory{Guardian: guardian, GuardianInvitation: auth.GuardianInvitation, UserContext: identity, ListExport: lists.NewRendererService()}
	return GuardianTestModule{PeopleDirectory: people, PhotoRuntime: photoRuntime, runtime: carrier.NewGuardianDirectoryRuntime(peopleCompose.NewLegacyRepositoryRuntime(db.Pool()))}, nil
}
