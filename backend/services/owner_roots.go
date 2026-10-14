package services

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"

	"fmt"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	staffStore "github.com/moto-nrw/project-phoenix/modules/communication/staffstore"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	platform "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose/vocabulary"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
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
	return NewOperatorAuditLogFromEntries(auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db}).OperatorAuditLog)
}

// NewParentPortalRepositories builds the retained guardian-portal and
// messaging repositories with the People Directory, School Membership,
// School Structure, Identity & Access and Enrollment reads bound. The school
// names are bound by WithSchools.
func NewParentPortalRepositories(db *bun.DB, owners ParentPortalOwners) ParentPortalRepositories {
	identity := owners.Identity
	members := ParentStaffMembers{Membership: owners.Membership}.List
	// The staff accounts are loaded from the person rows.
	persons := NewPersonRepository(db)
	return ParentPortalRepositories{
		ParentPortalRepositories: carePlanCompose.NewParentPortalRepositories(carePlanCompose.ParentPortalDependencies{
			DB: db, Directory: ParentPortalDirectory{People: owners.People},
			AccountSchools: identity.ListActiveAccountSchoolIDs,
			Phases:         owners.Enrollment, Applications: owners.Enrollment,
			Accounts: IdentityAccountDirectory{Accounts: identity},
		}),
		MessagingRepositories: parentStore.MessagingRepositories{
			ParentAnnouncement:  parentStore.NewParentAnnouncementRepository(db, ParentAnnouncementApplicants(owners.Enrollment), owners.Now),
			ParentMessageThread: parentStore.NewParentMessageThreadRepository(db, peopleCompose.NewMessageableGuardians(db, identity.FindActiveSchoolMemberships)),
			ParentMessageRead: parentStore.WithInboxGroupNames(
				parentStore.NewStaffParentMessageReadRepository(db, members, persons), parentGroupNames{groups: owners.Groups}.list),
			StaffMessageRead: staffStore.NewStaffMessageReadRepository(db, peopleCompose.NewMessageableStaff(db,
				parentStore.CurrentTenantStaffAccounts(members, persons), StaffMessageIdentity(identity))),
		},
	}
}

// NewDeviceFleet composes the device owner behind the legacy composition seam
// for graphs that do not record observations and never serve the info-point
// dashboard. Production roots compose the module themselves (api/base.go) so
// runtime evidence and the dashboard collaborators are kept.
func NewDeviceFleet(db *bun.DB, onlineWindows ...func(context.Context) time.Duration) (devicefleet.Capability, error) {
	rooms, err := facilitiesCompose.NewUnobserved(db)
	if err != nil {
		return nil, err
	}
	var onlineWindow func(context.Context) time.Duration
	if len(onlineWindows) > 0 {
		onlineWindow = onlineWindows[0]
	}
	return devicefleetCompose.New(devicefleetCompose.Dependencies{
		DB:           db,
		Rooms:        rooms,
		OnlineWindow: onlineWindow,
		Observe:      func(devicefleetCompose.Observation) {},
	})
}

// NewPersonRepository is the person repository with the Identity & Access
// account lookup FindWithAccount attaches (#2720).
func NewPersonRepository(db *bun.DB) userModels.PersonRepository {
	identity, err := identityaccessCompose.New(identityaccessCompose.Dependencies{DB: db, Observe: func(identityaccessCompose.Observation) {}})
	if err != nil {
		panic(fmt.Sprintf("compose identity access: %v", err))
	}
	return peopleCompose.NewPersonRepository(db, accountLookup(identity))
}

// accountLookup adapts the owner's by-id read to the People Directory
// lookup: a missing account resolves to nil, as the former LEFT JOIN did.
func accountLookup(accounts identityaccess.AccountProfiles) peopleCompose.AccountLookup {
	return func(ctx context.Context, accountID int64) (*userModels.PersonAccount, error) {
		account, err := accounts.FindAccountMetadata(ctx, accountID)
		if errors.Is(err, identityaccess.ErrAccountNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		result := userModels.PersonAccount(account)
		return &result, nil
	}
}

// Identity & Access owns auth.accounts, platform.operators and the operator
// refresh sessions. The legacy repositories that used to join or select
// those tables declare the narrow fact they need and the factory binds it
// at construction to the owner (#2720): the parent dashboard reads the
// account's address through the public capability, the People Directory
// repositories filter or attach accounts through the owner's queries, and
// the retained operator repository contracts are adapters over the module.

// IdentityAccountDirectory adapts the public account lookup to the parent
// enrollment repository's directory port.
type IdentityAccountDirectory struct {
	Accounts identityaccess.GuardianAccessQuery
}

// AccountEmail resolves the address behind an account; an unknown account is
// an empty address, matching the applications by id only.
func (d IdentityAccountDirectory) AccountEmail(ctx context.Context, accountID int64) (string, error) {
	account, err := d.Accounts.FindAccount(ctx, accountID)
	if errors.Is(err, identityaccess.ErrAccountNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return account.Email, nil
}

// StaffMessageIdentity returns the Identity & Access owner queries the staff
// messaging reads filter through, including the role classification
// (#2721).
func StaffMessageIdentity(accounts identityaccess.StaffAccountQueries) peopleCompose.StaffMessageIdentity {
	return peopleCompose.StaffMessageIdentity{
		ActiveSchoolAccounts: accounts.ListActiveAccountIDsForTenant,
		RoleClasses: func(ctx context.Context, tenantID int64, accountIDs []int64) ([]peopleCompose.StaffMessageRole, error) {
			rows, err := accounts.ClassifySchoolRoles(ctx, tenantID, accountIDs)
			if err != nil {
				return nil, err
			}
			result := make([]peopleCompose.StaffMessageRole, 0, len(rows))
			for _, row := range rows {
				result = append(result, peopleCompose.StaffMessageRole{AccountID: row.AccountID, IsAdmin: row.IsAdmin, IsLehrkraft: row.IsLehrkraft})
			}
			return result, nil
		},
	}
}
