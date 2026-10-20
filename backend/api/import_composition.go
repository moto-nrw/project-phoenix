package api

import (
	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/auditlog/consents"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	dataImportCompose "github.com/moto-nrw/project-phoenix/modules/dataimport/compose"
	dataImportRetained "github.com/moto-nrw/project-phoenix/modules/dataimport/compose/imports"
	importPorts "github.com/moto-nrw/project-phoenix/modules/dataimport/ports"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type importIdentity interface {
	identityaccess.RFIDQuery
	identityaccess.SchoolAccountQuery
	identityaccess.InvitedPersonQuery
	identityaccess.SchoolRoleQuery
	identityaccess.RolePermissionQuery
}

// imports is the set of import services the API resource is wired with.
type imports struct {
	Student        importModels.RowImporter[importModels.StudentImportRow]
	Staff          importModels.RowImporter[importModels.StaffImportRow]
	ClassList      importModels.RowImporter[importModels.ClassListEntryImportRow]
	OpeningBalance importModels.OpeningBalanceImportFactory
}

// The Data Import (#2708) commits student, staff and class-list rows through
// the public commands of People Directory, School Membership, Workforce, Care
// Plan, Student Presence and the Audit platform, including Workforce's
// opening-balance commands. The production root and the test composition
// build the same services from the constructors below, so the two cannot
// drift.

// newImportRuntime is the transaction runner, audit trail and observation
// sink every import service of one root shares.
func newImportRuntime(audit auditModels.Command, observe func(importModels.ImportObservation)) importPorts.ImportRuntime {
	return importPorts.ImportRuntime{
		Transactions: dataImportCompose.NewTransactions(), Audit: audit, Observe: observe,
		Fingerprint: securityruntime.Fingerprint,
	}
}

// newStudentImport composes the student import. guardians is the People
// Directory's guardian port; a suite may decorate it to observe an owner
// failure mid-batch.
func newStudentImport(
	runtime importPorts.ImportRuntime, identity importIdentity, groups schoolstructure.GroupListing, rooms facilities.Query,
	persons peopledirectory.Capability, guardians importPorts.GuardianDirectory, carePlan careplan.Capability,
	presence studentpresence.Capability, consentHistory consents.ConsentTransitions,
) importModels.RowImporter[importModels.StudentImportRow] {
	references := owners.NewReferences(groups, rooms)
	resolver := dataImportRetained.NewRelationshipResolver(references.Groups, references.Rooms)
	return dataImportRetained.NewImportService(dataImportRetained.NewStudentImportConfig(
		importPorts.StudentImportDeps{
			Transactions:    runtime.Transactions,
			Persons:         persons,
			Students:        persons,
			Guardians:       guardians,
			Schedules:       carePlan,
			PrivacyConsents: presence,
			FindRFIDCard:    identity.FindRFIDCard,
			Resolver:        resolver,
			ConsentHistory:  consentHistory,
			// The owner's Kontingentzahl rule, so the preview counts what
			// the membership write will count (#3571).
			CountsTowardChildQuota: schoolmembership.CountsTowardChildQuota,
		},
	), runtime)
}

// newStaffImport composes the staff import. It files the Stammdatensatz
// (person, staff, caregiver profile, master data, qualifications)
// immediately and issues an invitation for rows with an e-mail; accepting
// links the account to the imported person (#2600).
func newStaffImport(
	runtime importPorts.ImportRuntime, identity importIdentity, invitations identityaccess.SchoolInvitations,
	organizations organizationtenancy.Capability, persons peopledirectory.Capability,
	membership schoolmembership.Capability, workforce workforceModule.Capability,
) importModels.RowImporter[importModels.StaffImportRow] {
	return dataImportRetained.NewImportService(dataImportRetained.NewStaffImportConfig(
		importPorts.StaffImportDeps{
			Authorization:       NewImportAuthorization(),
			Transactions:        runtime.Transactions,
			Invitations:         NewStaffInviter(invitations),
			FindInvitedPeople:   identity.FindInvitedPersonIDs,
			FindSchoolAccount:   NewImportSchoolAccountLookup(identity),
			Roles:               identity,
			RolePolicy:          dataImportCompose.NewSchoolRolePolicy(),
			FindRolePermissions: identity.FindRolePermissions,
			SchoolName:          NewImportSchoolName(organizations),
			Persons:             persons,
			Membership:          membership,
			Records:             workforce,
		},
	), runtime)
}

// newClassListImport composes the class-list import. The entries (#2382) are
// created through the Membership owner, so the duplicate guards and the
// audit trail apply to imported rows too.
func newClassListImport(
	runtime importPorts.ImportRuntime, membership schoolmembership.Capability, persons peopledirectory.Capability,
) importModels.RowImporter[importModels.ClassListEntryImportRow] {
	return dataImportRetained.NewImportService(dataImportRetained.NewClassListImportConfig(
		importPorts.ClassListImportDeps{
			Transactions: runtime.Transactions,
			Membership:   membership,
			Persons:      persons,
			Students:     persons,
			Audit:        runtime.Audit,
		},
	), runtime)
}

// newOpeningBalanceImport composes the opening-balance import over
// Workforce's preview and booking capabilities in deps. Its config is
// request-scoped (Stichtag, Begründung and acting staff member come from the
// upload form), so the factory closes over the request-independent
// dependencies and builds one service per request.
func newOpeningBalanceImport(
	runtime importPorts.ImportRuntime, deps importPorts.OpeningBalanceImportDeps,
	membership schoolmembership.Capability, persons peopledirectory.Capability, workforce workforceModule.Capability,
) importModels.OpeningBalanceImportFactory {
	deps.Transactions = runtime.Transactions
	deps.References = owners.NewOpeningReferences(membership, persons, workforce)
	return importModels.OpeningBalanceImportFactory(
		func(effectiveDate timezone.Date, note string, decidedByStaffID int64) importModels.RowImporter[importModels.OpeningBalanceImportRow] {
			config := dataImportRetained.NewOpeningBalanceImportConfig(deps, effectiveDate, note, decidedByStaffID)
			return dataImportRetained.NewImportService(config, runtime)
		})
}

// newServeImports builds the import services the server root mounts over
// the owners it composed (#2708, #4013).
func newServeImports(
	audit owners.AuditOwner, identity *identityaccess.Module, groups schoolstructure.GroupListing, rooms facilities.Query,
	persons peopledirectory.Capability, carePlan careplan.Capability, presence owners.PresenceOwner, organizations organizationtenancy.Capability,
	membership schoolmembership.Capability, workTime workforceModule.Capability, workforce owners.WorkforcePlanning,
) imports {
	runtime := newImportRuntime(audit.Command, observeDataImport)
	return imports{
		Student: newStudentImport(
			runtime, identity, groups, rooms, persons, persons, carePlan, presence.Presence,
			auditCompose.NewConsentRecorder(audit.Command),
		),
		Staff:     newStaffImport(runtime, identity, identity, organizations, persons, membership, workTime),
		ClassList: newClassListImport(runtime, membership, persons),
		OpeningBalance: newOpeningBalanceImport(runtime, importPorts.OpeningBalanceImportDeps{
			BalanceAdjustService: workforce.TimeTracking.OpeningBalances,
			StaffAbsenceService:  workforce.TimeTracking.VacationTakeovers,
		}, membership, persons, workTime),
	}
}
