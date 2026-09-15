package services

import (
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	auditService "github.com/moto-nrw/project-phoenix/services/audit"
	importService "github.com/moto-nrw/project-phoenix/services/import"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type ImportTestModule struct {
	PeopleDirectory      peopledirectory.Capability
	Membership           schoolmembership.Capability
	Import               *importService.ImportService[importModels.StudentImportRow]
	StaffImport          *importService.ImportService[importModels.StaffImportRow]
	ClassListImport      *importService.ImportService[importModels.ClassListEntryImportRow]
	OpeningBalanceImport importService.OpeningBalanceImportFactory
	// Observations collects the per-run counters the production observer
	// receives, so tests can assert the runtime evidence contract.
	Observations *[]importService.ImportObservation
}

// ImportTestOptions decorate the owner ports of the composed import.
// WrapGuardians lets a test inject an owner failure at a chosen row, so the
// per-row savepoint and the batch rollback are observable without a
// contrived data shape.
type ImportTestOptions struct {
	WrapGuardians importService.GuardianPortDecorator
	Clock         func() time.Time
}

// NewImportTestModule composes the Data Import over the real owners
// (People Directory with its guardian provider, School Membership,
// Workforce, Care Plan, Student Presence, Audit) on a test database,
// through the same composer the production root uses.
func NewImportTestModule(db *bun.DB, unit tenant.UnitOfWork) (ImportTestModule, error) {
	return NewImportTestModuleWithOptions(db, unit, ImportTestOptions{})
}

// NewImportTestModuleWithOptions is NewImportTestModule with decorated owner
// ports.
func NewImportTestModuleWithOptions(db *bun.DB, unit tenant.UnitOfWork, options ImportTestOptions) (ImportTestModule, error) {
	command, err := auditService.NewCommand(repositories.NewTestAuditStore(db), func(auditService.AppendObservation) {})
	if err != nil {
		return ImportTestModule{}, err
	}
	repos, err := repositories.NewStudentTestRepositories(db, command)
	if err != nil {
		return ImportTestModule{}, err
	}
	auth, err := NewAuthTestModule(db, unit)
	if err != nil {
		return ImportTestModule{}, err
	}
	guardians, err := NewGuardianTestModule(db, unit)
	if err != nil {
		return ImportTestModule{}, err
	}
	membership, err := repositories.NewSchoolMembership(db)
	if err != nil {
		return ImportTestModule{}, err
	}
	workTime, err := repositories.NewWorkforce(db, membership)
	if err != nil {
		return ImportTestModule{}, err
	}
	var clocks []func() time.Time
	if options.Clock != nil {
		clocks = append(clocks, options.Clock)
	}
	workforce, err := NewWorkforceTestModule(db, unit, clocks...)
	if err != nil {
		return ImportTestModule{}, err
	}

	observations := &[]importService.ImportObservation{}
	persons := guardians.PeopleDirectory
	groups, err := repositories.NewSchoolStructure(db)
	if err != nil {
		return ImportTestModule{}, err
	}
	rooms, err := repositories.NewFacilities(db)
	if err != nil {
		return ImportTestModule{}, err
	}
	organizations, err := repositories.NewOrganizationTenancy(db)
	if err != nil {
		return ImportTestModule{}, err
	}
	rfid, err := repositories.NewIdentityAccessForTests(db)
	if err != nil {
		return ImportTestModule{}, err
	}
	wiring := importWiring{
		Identity:      rfid,
		Organizations: organizations,
		Groups:        groups, Rooms: rooms,
		Persons: persons, Membership: membership, Workforce: workTime,
		CarePlan: repos.CarePlan, Presence: repositories.NewStudentPresenceForTests(db),
		InvitationService: auth.Invitation,
		ConsentHistory:    auditService.NewConsentRecorder(command),
		OpeningBalance: importService.OpeningBalanceImportDeps{
			BalanceAdjustService: OpeningBalanceBookingCapability(workforce.StaffBalanceAdjust),
			StaffAbsenceService:  VacationTakeoverCapability(workforce.StaffAbsence),
		},
		Audit: command,
		Observe: func(observation importService.ImportObservation) {
			*observations = append(*observations, observation)
		},
	}
	if options.WrapGuardians != nil {
		// The composer binds one People Directory to three ports; only the
		// guardian one is decorated, so the person and student paths keep
		// talking to the real owner.
		wiring.GuardianOverride = options.WrapGuardians(persons)
	}

	dataImports := newImports(wiring)
	return ImportTestModule{
		PeopleDirectory: persons, Membership: membership,
		Import: dataImports.Student, StaffImport: dataImports.Staff, ClassListImport: dataImports.ClassList,
		OpeningBalanceImport: dataImports.OpeningBalance,
		Observations:         observations,
	}, nil
}
