package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	auditService "github.com/moto-nrw/project-phoenix/services/audit"
	importService "github.com/moto-nrw/project-phoenix/services/import"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type ImportTestModule struct {
	Import          *importService.ImportService[importModels.StudentImportRow]
	StaffImport     *importService.ImportService[importModels.StaffImportRow]
	ClassListImport *importService.ImportService[importModels.ClassListEntryImportRow]
	Users           users.PersonService
	// Observations collects the per-run counters the production observer
	// receives, so router tests can assert the runtime evidence contract.
	Observations *[]importService.ImportObservation
}

// ImportTestOptions decorate the owner ports of the composed import.
// WrapGuardians lets a test inject an owner failure at a chosen row, so the
// per-row savepoint and the batch rollback are observable without a
// contrived data shape.
type ImportTestOptions struct {
	WrapGuardians importService.GuardianPortDecorator
}

// NewImportTestModule composes the Data Import over the real owners
// (People Directory with its guardian provider, School Membership,
// Workforce, Care Plan, Student Presence, Audit) on a test database. It
// mirrors the production wiring in newFactory without the legacy graph.
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
	observations := &[]importService.ImportObservation{}
	runtime := importService.ImportRuntime{Audit: command, Observe: func(observation importService.ImportObservation) {
		*observations = append(*observations, observation)
	}}
	persons := guardians.PeopleDirectory
	var guardianPort importService.GuardianPort = persons
	if options.WrapGuardians != nil {
		guardianPort = options.WrapGuardians(guardianPort)
	}
	studentConsentService := users.NewStudentConsentService(repos.StudentConsentChange)
	relationshipResolver := importService.NewRelationshipResolver(repos.Group, repos.Room)
	studentImportConfig := importService.NewStudentImportConfig(
		importService.StudentImportDeps{
			Persons:         persons,
			Students:        persons,
			Guardians:       guardianPort,
			Schedules:       repos.CarePlan,
			PrivacyConsents: newStudentPresence(db, slog.Default()),
			RFIDCardRepo:    identity.RFIDCard,
			Resolver:        relationshipResolver,
			ConsentHistory:  studentConsentService,
		},
	)
	studentImportService := importService.NewImportServiceWithRuntime(studentImportConfig, runtime)

	// Staff import files the Stammdatensatz (Person/Staff/Teacher/master
	// data) immediately and issues an invitation for rows with an e-mail;
	// accepting links the account to the imported person (#2600).
	staffImportConfig := importService.NewStaffImportConfig(
		importService.StaffImportDeps{
			InvitationService: auth.Invitation,
			InvitationRepo:    identity.InvitationToken,
			AccountRepo:       repos.Account,
			AccountTenantRepo: repos.AccountTenant,
			RoleRepo:          repos.Role,
			PermissionRepo:    identity.Permission,
			SchoolRepo:        repos.School,
			Persons:           persons,
			Membership:        membership,
			Records:           workTime,
		},
	)
	staffImportService := importService.NewImportServiceWithRuntime(staffImportConfig, runtime)

	// Class-list entry import (#2382): creates through the Membership owner
	// so the duplicate guards and the audit trail apply to imported rows too.
	classListImportConfig := importService.NewClassListImportConfig(importService.ClassListImportDeps{
		Membership: membership,
		Persons:    persons,
		Students:   persons,
		Audit:      command,
	})
	classListImportService := importService.NewImportServiceWithRuntime(classListImportConfig, runtime)
	return ImportTestModule{Import: studentImportService, StaffImport: staffImportService, ClassListImport: classListImportService, Users: people.Users, Observations: observations}, nil
}
