package services

import (
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	auditSvc "github.com/moto-nrw/project-phoenix/services/audit"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type GuardianTestModule struct {
	PeopleDirectory peopledirectory.Capability
	// PhotoRuntime is the slot the directory resolves its photo runtime from,
	// filled by the graph that also owns the feature gate and the file cleanup.
	PhotoRuntime *peopleCompose.StudentPhotoRuntime
	runtime      GuardianDirectoryRuntime
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
	people, photoRuntime, err := repositories.NewPeopleDirectoryWithPhotos(db)
	if err != nil {
		return GuardianTestModule{}, err
	}
	identity, _, err := newStaffIdentityForTests(db)
	if err != nil {
		return GuardianTestModule{}, err
	}
	guardian := peopleCompose.NewGuardianService(peopleCompose.GuardianServiceDependencies{
		GuardianProfileRepo: r.GuardianProfile, GuardianPhoneNumberRepo: g.Phone, StudentGuardianRepo: r.StudentGuardian,
		GuardianInvitations: newGuardianInvitationReads(func() identityaccess.GuardianInvitations { return auth.AccountAuthentication }),
		StudentRepo:         r.Student, PersonRepo: r.Person,
		GuardianFinancialRepo:  g.Financial,
		GuardianFinancialAudit: newGuardianFinancialAudit(g.FinancialAudit), DataAccessLog: newGuardianDataAccessLog(g.AccessLog),
		RolePresets: guardianRolePresets{}, DB: db,
	})
	people.(guardianProviderBinder).BindGuardianProvider(&guardianDirectoryProvider{guardians: guardian, db: db})
	// The compatibility carrier only maps the existing four capabilities to
	// the production runtime closures. It constructs no additional services.
	carrier := &Factory{Guardian: guardian, GuardianInvitation: auth.GuardianInvitation, UserContext: identity, ListExport: listexport.NewService()}
	return GuardianTestModule{PeopleDirectory: people, PhotoRuntime: photoRuntime, runtime: carrier.NewGuardianDirectoryRuntime(db)}, nil
}
