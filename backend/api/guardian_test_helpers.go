//go:build testhooks

package api

import (
	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type GuardianTestModule struct {
	PeopleDirectory peopledirectory.Capability
	// PhotoRuntime is the slot the directory resolves its photo runtime from,
	// filled by the graph that also owns the feature gate and the file cleanup.
	PhotoRuntime *peopleCompose.StudentPhotoRuntime
	runtime      GuardianDirectoryRuntime
}

func (m GuardianTestModule) NewGuardianDirectoryRuntime(transactionruntime.Database) GuardianDirectoryRuntime {
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
	g := NewGuardianTestRepositoriesWithoutAudit(db)
	audits := NewTestAuditPersistence(db, command)
	identity, _, err := newStaffIdentityForTests(db)
	if err != nil {
		return GuardianTestModule{}, err
	}
	guardian := peopleCompose.NewGuardianService(peoplePorts.GuardianServiceDependencies{
		GuardianProfileRepo: r.GuardianProfile, GuardianPhoneNumberRepo: g.Phone, StudentGuardianRepo: r.StudentGuardian,
		GuardianInvitations: NewGuardianInvitationReads(auth.AccountAuthentication),
		StudentRepo:         NewUnboundStudentRepository(db), PersonRepo: r.Person,
		GuardianFinancialRepo:  g.Financial,
		GuardianFinancialAudit: NewGuardianFinancialAudit(audits.GuardianFinancialChange), DataAccessLog: NewGuardianDataAccessLog(audits.DataAccessLog),
		RolePresets: guardianRolePresets}, db.Pool())
	people, photoRuntime, err := NewPeopleDirectoryWithPhotosAndObserver(db, func(peopleCompose.Observation) {}, &peopleCompose.GuardianDirectoryProvider{Guardians: guardian, Runtime: peopleCompose.NewLegacyRepositoryRuntime(db.Pool()), PermissionGranted: securityruntime.GuardianPermissionGranted})
	if err != nil {
		return GuardianTestModule{}, err
	}
	return GuardianTestModule{PeopleDirectory: people, PhotoRuntime: photoRuntime, runtime: NewGuardianDirectoryRuntime(guardian, lists.NewRendererService(), auth.GuardianInvitation, identity.HasCurrentStaff, peopleCompose.NewLegacyRepositoryRuntime(db.Pool()))}, nil
}

// ParentGuardianRoleFixture supplies the canonical role preset as stored
// fixture values, without exposing Security's implementation to the workflow.
func ParentGuardianRoleFixture(role string) (string, map[string]interface{}) {
	normalized, granted := securityruntime.StudentGuardianRolePreset(role)
	permissions := make(map[string]interface{}, len(granted))
	for _, permission := range granted {
		permissions[permission] = true
	}
	return normalized, permissions
}
