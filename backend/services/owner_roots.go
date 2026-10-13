package services

import (
	"context"
	"time"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	staffStore "github.com/moto-nrw/project-phoenix/modules/communication/staffstore"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	platform "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose/vocabulary"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	reminderCompose "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery/compose"
	"github.com/uptrace/bun"
)

// The roots below build owners from the database. Most forward to one owner
// composition for callers outside this package that may not import it: the
// serving root, the demo process, the CLI roots and other owners' test
// graphs. NewParentPortalRepositories builds the stores of several owners
// over the database, so no single owner composition can host it. They leave
// with the composition root (#2747).

// NewOrganizationTenancy composes the school owner behind the legacy
// composition seam. Consumers should depend on a narrow projection instead
// of importing the module's compose package themselves.
func NewOrganizationTenancy(db *bun.DB) (organizationtenancy.Capability, error) {
	return organizationCompose.New(organizationCompose.Dependencies{
		DB:      db,
		Observe: func(organizationCompose.Observation) {},
	})
}

// NewOperatorAuditLog composes the operator ledger over the Audit handoff on
// the context, with the root database as the fallback, for the roots that
// record operator actions.
func NewOperatorAuditLog(db *bun.DB) platform.OperatorAuditLogRepository {
	return newOperatorAuditLog(auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db}).OperatorAuditLog)
}

// NewCalendarReminderCommand binds the existing notification adapters to the
// reminder workflow without constructing a repository or service factory.
func NewCalendarReminderCommand(db *bun.DB, source CalendarReminderSource) reminder.Command {
	return reminderCompose.NewCommand(db, calendarReminderDelivery(source))
}

// DemoAccessExpiry is the Identity & Access capability the demo process runs
// (#3470): it deletes demo accesses 14 days after their last use and names
// the demo schools that no access enters any more.
type DemoAccessExpiry interface {
	ExpireDemoAccesses(ctx context.Context) (deleted int, orphanedSchoolSlugs []string, err error)
}

// NewDemoAccessExpiry composes the expiry on the demo process's own
// connection; now is the clock it reads.
func NewDemoAccessExpiry(db bun.IDB, now func() time.Time) (DemoAccessExpiry, error) {
	return identityaccessCompose.NewDemoAccessExpiry(db, now)
}

// NewGuardianSeedAccess binds the dev CLI's guardian access command without
// composing sessions, token signers, or the serving service factory. The
// caller supplies its transaction and tenant through the context.
func NewGuardianSeedAccess(db *bun.DB) (func(context.Context, string, string) (int64, bool, error), error) {
	access, err := identityaccessCompose.New(identityaccessCompose.Dependencies{
		DB: db, Observe: func(identityaccessCompose.Observation) {},
	})
	if err != nil {
		return nil, err
	}
	return access.SeedGuardianAccount, nil
}

// NewParentPortalRepositories builds the retained guardian-portal and
// messaging repositories with the People Directory, School Membership,
// School Structure, Identity & Access and Enrollment reads bound. The school
// names are bound by WithSchools.
func NewParentPortalRepositories(db *bun.DB, owners ParentPortalOwners) ParentPortalRepositories {
	identity := owners.Identity
	members := parentStaffMembers{membership: owners.Membership}.list
	// The staff accounts are loaded from the person rows.
	persons := NewPersonRepository(db)
	return ParentPortalRepositories{
		ParentPortalRepositories: carePlanCompose.NewParentPortalRepositories(carePlanCompose.ParentPortalDependencies{
			DB: db, Directory: parentPortalDirectory{people: owners.People},
			AccountSchools: identity.ListActiveAccountSchoolIDs,
			Phases:         owners.Enrollment, Applications: owners.Enrollment,
			Accounts: identityAccountDirectory{accounts: identity},
		}),
		MessagingRepositories: parentStore.MessagingRepositories{
			ParentAnnouncement:  parentStore.NewParentAnnouncementRepository(db, ParentAnnouncementApplicants(owners.Enrollment), owners.Now),
			ParentMessageThread: parentStore.NewParentMessageThreadRepository(db, peopleCompose.NewMessageableGuardians(db, identity.FindActiveSchoolMemberships)),
			ParentMessageRead: parentStore.WithInboxGroupNames(
				parentStore.NewStaffParentMessageReadRepository(db, members, persons), parentGroupNames{groups: owners.Groups}.list),
			StaffMessageRead: staffStore.NewStaffMessageReadRepository(db, peopleCompose.NewMessageableStaff(db,
				parentStore.CurrentTenantStaffAccounts(members, persons), staffMessageIdentity(identity))),
		},
	}
}
