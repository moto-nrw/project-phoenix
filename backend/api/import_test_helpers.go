//go:build testhooks

package api

import (
	"context"
	"time"

	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

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
	// BatchStatementLimit scales the bound of a batch attempt down (E13).
	BatchStatementLimit time.Duration
	// RowPathOnly composes without the statement bound: every chunk takes
	// the row path, as before the chunk executor.
	RowPathOnly bool
	// UnboundedBatches runs batch attempts without a statement timeout, the
	// counterprobe of the bound.
	UnboundedBatches bool
	// WrapClassListBatches decorates School Membership's class-list batch
	// slice, so a suite can fail or delay a batch command.
	WrapClassListBatches func(importPorts.ClassListEntryBatches) importPorts.ClassListEntryBatches
	// WrapPersonBatches decorates People Directory's person batch slice, so
	// a suite can fail or delay the student batch command.
	WrapPersonBatches func(importPorts.PersonBatches) importPorts.PersonBatches
	// Today is the calendar day a new child's status is taken against
	// (#5552 step 7); nil is the real day.
	Today func() timezone.Date
}

// NewImportTestRuntime is the import runtime NewImportTestModule composes,
// for suites that drive their own config through the chunk executor.
func NewImportTestRuntime(db transactionruntime.Database, options ImportTestOptions) (importPorts.ImportRuntime, *[]importModels.ImportObservation, error) {
	command, err := auditCompose.NewCommand(owners.NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return importPorts.ImportRuntime{}, nil, err
	}
	observations := &[]importModels.ImportObservation{}
	return importTestRuntime(command, observations, options), observations, nil
}

func importTestRuntime(command auditModels.Command, observations *[]importModels.ImportObservation, options ImportTestOptions) importPorts.ImportRuntime {
	runtime := newImportRuntime(command, func(observation importModels.ImportObservation) {
		*observations = append(*observations, observation)
	})
	runtime.BatchStatementLimit = options.BatchStatementLimit
	switch {
	case options.RowPathOnly:
		runtime.BoundStatements = nil
	case options.UnboundedBatches:
		runtime.BoundStatements = func(ctx context.Context, _ time.Duration, fn func(context.Context) error) error { return fn(ctx) }
	}
	return runtime
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

// NewImportTestModuleWithAuth is NewImportTestModuleWithOptions whose auth
// module takes the given options.
func NewImportTestModuleWithAuth(db transactionruntime.Database, unit tenant.UnitOfWork, options ImportTestOptions, auth ...AuthTestOption) (ImportTestModule, error) {
	module, _, _, err := NewImportTestModuleWithOwners(db, unit, options, auth...)
	return module, err
}

// NewImportTestModuleWithOwners is NewImportTestModuleWithOptions together
// with the People Directory, guardian provider bound, and the School
// Membership the import writes through, for suites that mount its routes.
// The auth module, which sends the staff invitations of an import, takes
// authOptions (for example WithAuthTestMailer to accept and count mail).
func NewImportTestModuleWithOwners(db transactionruntime.Database, unit tenant.UnitOfWork, options ImportTestOptions, authOptions ...AuthTestOption) (ImportTestModule, peopledirectory.Capability, schoolmembership.Capability, error) {
	command, err := auditCompose.NewCommand(owners.NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return ImportTestModule{}, nil, nil, err
	}
	students, _, err := NewStudentTestRepositories(db, command)
	if err != nil {
		return ImportTestModule{}, nil, nil, err
	}
	auth, err := NewAuthTestModule(db, unit, authOptions...)
	if err != nil {
		return ImportTestModule{}, nil, nil, err
	}
	guardians, err := NewGuardianTestModule(db, unit)
	if err != nil {
		return ImportTestModule{}, nil, nil, err
	}
	membership, err := owners.NewSchoolMembership(db)
	if err != nil {
		return ImportTestModule{}, nil, nil, err
	}
	workTime, err := owners.NewWorkforce(db, membership)
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
	groups, err := owners.NewSchoolStructure(db)
	if err != nil {
		return ImportTestModule{}, nil, nil, err
	}
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return ImportTestModule{}, nil, nil, err
	}
	organizations, err := owners.NewOrganizationTenancy(db)
	if err != nil {
		return ImportTestModule{}, nil, nil, err
	}
	rfid, err := NewIdentityAccessForTests(db)
	if err != nil {
		return ImportTestModule{}, nil, nil, err
	}
	runtime := importTestRuntime(command, observations, options)
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
			runtime, rfid, groups, rooms, persons, wrapPersonBatches(personBatches(persons), options.WrapPersonBatches),
			studentGuardians, students.CarePlan, NewStudentPresenceForTests(db), auditCompose.NewConsentRecorder(command), options.Today,
		),
		Staff:     newStaffImport(runtime, rfid, auth.Invitation, organizations, persons, membership, workTime),
		ClassList: newClassListImport(runtime, membership, persons, wrapClassListBatches(classListBatches(membership), options.WrapClassListBatches)),
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

func wrapClassListBatches(batches importPorts.ClassListEntryBatches, wrap func(importPorts.ClassListEntryBatches) importPorts.ClassListEntryBatches) importPorts.ClassListEntryBatches {
	if wrap == nil || batches == nil {
		return batches
	}
	return wrap(batches)
}

func wrapPersonBatches(batches importPorts.PersonBatches, wrap func(importPorts.PersonBatches) importPorts.PersonBatches) importPorts.PersonBatches {
	if wrap == nil || batches == nil {
		return batches
	}
	return wrap(batches)
}
