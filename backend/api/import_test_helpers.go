//go:build testhooks

package api

import (
	"time"

	importPorts "github.com/moto-nrw/project-phoenix/modules/dataimport/ports"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// ImportTestModule is the Data Import a suite drives; the owners it imports
// into come from NewImportTestModuleWithOwners.
type ImportTestModule struct {
	Import               importModels.RowImporter[importModels.StudentImportRow]
	StaffImport          importModels.RowImporter[importModels.StaffImportRow]
	ClassListImport      importModels.RowImporter[importModels.ClassListEntryImportRow]
	OpeningBalanceImport importModels.OpeningBalanceImportFactory
	// Observations collects the per-run counters the production observer
	// receives, so tests can assert the runtime evidence contract.
	Observations *[]importModels.ImportObservation
}

// ImportTestOptions decorate the owner ports of the composed import.
// WrapGuardians lets a test inject an owner failure at a chosen row, so the
// per-row savepoint and the batch rollback are observable without a
// contrived data shape.
type ImportTestOptions struct {
	WrapGuardians importPorts.GuardianPortDecorator
	Clock         func() time.Time
}

// NewImportTestModule composes the Data Import over the real owners
// (People Directory with its guardian provider, School Membership,
// Workforce, Care Plan, Student Presence, Audit) on a test database,
// through the same composer the production root uses.
func NewImportTestModule(db transactionruntime.Database, unit tenant.UnitOfWork) (ImportTestModule, error) {
	return NewImportTestModuleWithOptions(db, unit, ImportTestOptions{})
}

// NewImportTestModuleWithOptions is NewImportTestModule with decorated owner
// ports.
func NewImportTestModuleWithOptions(db transactionruntime.Database, unit tenant.UnitOfWork, options ImportTestOptions) (ImportTestModule, error) {
	module, _, _, err := NewImportTestModuleWithOwners(db, unit, options)
	return module, err
}

// NewImportTestModuleWithOwners is NewImportTestModuleWithOptions together
// with the People Directory, guardian provider bound, and the School
// Membership the import writes through, for suites that mount its routes.
func NewImportTestModuleWithOwners(db transactionruntime.Database, unit tenant.UnitOfWork, options ImportTestOptions) (ImportTestModule, peopledirectory.Capability, schoolmembership.Capability, error) {
	command, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return ImportTestModule{}, nil, nil, err
	}
	students, _, err := NewStudentTestRepositories(db, command)
	if err != nil {
		return ImportTestModule{}, nil, nil, err
	}
	auth, err := NewAuthTestModule(db, unit)
	if err != nil {
		return ImportTestModule{}, nil, nil, err
	}
	guardians, err := NewGuardianTestModule(db, unit)
	if err != nil {
		return ImportTestModule{}, nil, nil, err
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return ImportTestModule{}, nil, nil, err
	}
	workTime, err := NewWorkforce(db, membership)
	if err != nil {
		return ImportTestModule{}, nil, nil, err
	}
	var clocks []func() time.Time
	if options.Clock != nil {
		clocks = append(clocks, options.Clock)
	}
	workforce, _, err := NewWorkforceTestModule(db, unit, clocks...)
	if err != nil {
		return ImportTestModule{}, nil, nil, err
	}

	observations := &[]importModels.ImportObservation{}
	persons := guardians.PeopleDirectory
	groups, err := NewSchoolStructure(db)
	if err != nil {
		return ImportTestModule{}, nil, nil, err
	}
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return ImportTestModule{}, nil, nil, err
	}
	organizations, err := NewOrganizationTenancy(db)
	if err != nil {
		return ImportTestModule{}, nil, nil, err
	}
	rfid, err := NewIdentityAccessForTests(db)
	if err != nil {
		return ImportTestModule{}, nil, nil, err
	}
	runtime := newImportRuntime(command, func(observation importModels.ImportObservation) {
		*observations = append(*observations, observation)
	})
	// The composer binds one People Directory to three ports; only the
	// guardian one is decorated, so the person and student paths keep
	// talking to the real owner.
	var studentGuardians importPorts.GuardianDirectory = persons
	if options.WrapGuardians != nil {
		if wrapped := options.WrapGuardians(persons); wrapped != nil {
			studentGuardians = wrapped
		}
	}
	dataImports := imports{
		Student: newStudentImport(
			runtime, rfid, groups, rooms, persons, studentGuardians, students.CarePlan, NewStudentPresenceForTests(db),
			auditCompose.NewConsentRecorder(command),
		),
		Staff:     newStaffImport(runtime, rfid, auth.Invitation, organizations, persons, membership, workTime),
		ClassList: newClassListImport(runtime, membership, persons),
		OpeningBalance: newOpeningBalanceImport(runtime, importPorts.OpeningBalanceImportDeps{
			BalanceAdjustService: workforce.TimeTracking.OpeningBalances,
			StaffAbsenceService:  workforce.TimeTracking.VacationTakeovers,
		}, membership, persons, workTime),
	}
	return ImportTestModule{
		Import: dataImports.Student, StaffImport: dataImports.Staff, ClassListImport: dataImports.ClassList,
		OpeningBalanceImport: dataImports.OpeningBalance,
		Observations:         observations,
	}, persons, membership, nil
}
