package services

import (
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	auditService "github.com/moto-nrw/project-phoenix/services/audit"
	importService "github.com/moto-nrw/project-phoenix/services/import"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type ImportTestModule struct {
	Import               *importService.ImportService[importModels.StudentImportRow]
	StaffImport          *importService.ImportService[importModels.StaffImportRow]
	ClassListImport      *importService.ImportService[importModels.ClassListEntryImportRow]
	OpeningBalanceImport importService.OpeningBalanceImportFactory
	Users                users.PersonService
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
	identity, err := repositories.NewAuthTestRepositories(db, command)
	if err != nil {
		return ImportTestModule{}, err
	}
	auth, err := NewAuthTestModule(db, unit)
	if err != nil {
		return ImportTestModule{}, err
	}
	people, err := NewRFIDTestModule(db)
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
	workforceReads, err := repositories.NewWorkforceTestRepositories(db, command)
	if err != nil {
		return ImportTestModule{}, err
	}

	observations := &[]importService.ImportObservation{}
	persons := guardians.PeopleDirectory
	wiring := importWiring{
		Persons: persons, Membership: membership, Workforce: workTime,
		CarePlan: repos.CarePlan, Presence: repositories.NewStudentPresenceForTests(db),
		InvitationService: auth.Invitation,
		Reads: importService.LegacyReads{
			RFIDCard: identity.RFIDCard, InvitationToken: identity.InvitationToken, Account: repos.Account,
			AccountTenant: repos.AccountTenant, Role: repos.Role, Permission: identity.Permission,
			School: repos.School, Groups: repos.Group, Rooms: repos.Room,
		},
		ConsentHistory: users.NewStudentConsentService(repos.StudentConsentChange),
		OpeningBalance: importService.OpeningBalanceImportDeps{
			StaffRepo: workforceReads.Staff, AdjustmentRepo: workforceReads.StaffBalanceAdjust,
			VacationOpeningRepo:  workforceReads.StaffVacationOpening,
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
		Import: dataImports.Student, StaffImport: dataImports.Staff, ClassListImport: dataImports.ClassList,
		OpeningBalanceImport: dataImports.OpeningBalance,
		Users:                people.Users, Observations: observations,
	}, nil
}
