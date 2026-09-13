package services

import (
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/modules/auditlog/consents"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	dataImportCompose "github.com/moto-nrw/project-phoenix/modules/dataimport/compose"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	authsvc "github.com/moto-nrw/project-phoenix/services/auth"
	importService "github.com/moto-nrw/project-phoenix/services/import"
)

// importWiring is everything the Data Import needs (#2708): the owner
// capabilities its ports are bound to and the platform collaborators. The production root and
// the test composition build the same graph from it, so the two cannot drift.
type importWiring struct {
	Identity      importIdentity
	Organizations organizationtenancy.Capability
	Groups        schoolstructure.GroupListing
	Rooms         facilities.Query
	Persons       peopledirectory.Capability
	Membership    schoolmembership.Capability
	Workforce     workforceModule.Capability
	CarePlan      careplan.Capability
	Presence      studentpresence.Capability

	// InvitationService issues the staff invitation through Identity & Access.
	InvitationService authsvc.InvitationService

	// Opening balances use Workforce's preview and booking capabilities.
	OpeningBalance importService.OpeningBalanceImportDeps

	ConsentHistory consents.ConsentTransitions
	Audit          auditModels.Command
	Observe        func(importService.ImportObservation)

	// GuardianOverride replaces the guardian port of the student import.
	// Only the test composition sets it, to observe an owner failure
	// mid-batch; production leaves it nil and the People Directory serves
	// all three student ports.
	GuardianOverride importService.GuardianPort
}

type importIdentity interface {
	identityaccess.RFIDQuery
	identityaccess.SchoolAccountQuery
	identityaccess.InvitedPersonQuery
	identityaccess.SchoolRoleQuery
	identityaccess.RolePermissionQuery
}

// imports is the set of import services the API resource is wired with.
type imports struct {
	Student        *importService.ImportService[importModels.StudentImportRow]
	Staff          *importService.ImportService[importModels.StaffImportRow]
	ClassList      *importService.ImportService[importModels.ClassListEntryImportRow]
	OpeningBalance importService.OpeningBalanceImportFactory
}

// newImports composes the Data Import over its owner ports. Student, staff
// and class-list rows are committed through the public commands of People
// Directory, School Membership, Workforce, Care Plan, Student Presence and
// the Audit platform, including Workforce's opening-balance commands.
func newImports(wiring importWiring) imports {
	transactions := dataImportCompose.NewTransactions()
	wiring.OpeningBalance.Transactions = transactions
	wiring.OpeningBalance.References = dataImportCompose.NewOpeningReferences(wiring.Membership, wiring.Persons, wiring.Workforce)
	runtime := importService.ImportRuntime{Transactions: transactions, Audit: wiring.Audit, Observe: wiring.Observe, Fingerprint: securityruntime.Fingerprint}
	references := dataImportCompose.NewReferences(wiring.Groups, wiring.Rooms)
	resolver := importService.NewRelationshipResolver(references.Groups, references.Rooms)
	guardians := importService.GuardianPort(wiring.Persons)
	if wiring.GuardianOverride != nil {
		guardians = wiring.GuardianOverride
	}

	student := importService.NewImportService(importService.NewStudentImportConfig(
		importService.StudentImportDeps{
			Transactions:    transactions,
			Persons:         wiring.Persons,
			Students:        wiring.Persons,
			Guardians:       guardians,
			Schedules:       wiring.CarePlan,
			PrivacyConsents: wiring.Presence,
			FindRFIDCard:    wiring.Identity.FindRFIDCard,
			Resolver:        resolver,
			ConsentHistory:  wiring.ConsentHistory,
		},
	), runtime)

	// The staff import files the Stammdatensatz (person, staff, caregiver
	// profile, master data, qualifications) immediately and issues an
	// invitation for rows with an e-mail; accepting links the account to the
	// imported person (#2600).
	staff := importService.NewImportService(importService.NewStaffImportConfig(
		importService.StaffImportDeps{
			Authorization:       dataImportCompose.NewAuthorization(),
			Transactions:        transactions,
			Invitations:         dataImportCompose.NewStaffInviter(wiring.InvitationService),
			FindInvitedPeople:   wiring.Identity.FindInvitedPersonIDs,
			FindSchoolAccount:   dataImportCompose.NewSchoolAccountLookup(wiring.Identity),
			Roles:               wiring.Identity,
			RolePolicy:          dataImportCompose.NewSchoolRolePolicy(),
			FindRolePermissions: wiring.Identity.FindRolePermissions,
			SchoolName:          dataImportCompose.NewSchoolName(wiring.Organizations),
			Persons:             wiring.Persons,
			Membership:          wiring.Membership,
			Records:             wiring.Workforce,
		},
	), runtime)

	// Class-list entries (#2382) are created through the Membership owner, so
	// the duplicate guards and the audit trail apply to imported rows too.
	classList := importService.NewImportService(importService.NewClassListImportConfig(
		importService.ClassListImportDeps{
			Transactions: transactions,
			Membership:   wiring.Membership,
			Persons:      wiring.Persons,
			Students:     wiring.Persons,
			Audit:        wiring.Audit,
		},
	), runtime)

	// The opening-balance config is request-scoped (Stichtag, Begründung and
	// acting staff member come from the upload form), so the factory closes
	// over the request-independent dependencies and builds one service per
	// request.
	openingBalance := importService.OpeningBalanceImportFactory(
		func(effectiveDate timezone.Date, note string, decidedByStaffID int64) importModels.RowImporter[importModels.OpeningBalanceImportRow] {
			config := importService.NewOpeningBalanceImportConfig(wiring.OpeningBalance, effectiveDate, note, decidedByStaffID)
			return importService.NewImportService(config, runtime)
		})

	return imports{Student: student, Staff: staff, ClassList: classList, OpeningBalance: openingBalance}
}
