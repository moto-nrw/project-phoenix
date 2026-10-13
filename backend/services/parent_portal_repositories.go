package services

import (
	"context"
	"time"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	enrollmentCapability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
)

// ParentPortalOwners are the owners the retained guardian-portal and
// messaging repositories read through. Every one is bound at construction
// (#2743).
type ParentPortalOwners struct {
	// People answers the student, guardian and person reads that used to
	// join users.students, users.guardian_profiles and users.persons.
	People peopledirectory.Capability
	// Groups names the groups of the inbox threads.
	Groups schoolstructure.Query
	// Membership resolves the live staff accounts behind the read cursors.
	Membership schoolmembership.Capability
	// Identity owns the accounts, their school memberships and role classes.
	Identity *identityaccess.Module
	// Enrollment owns the phases, applications and announcement applicants.
	Enrollment *enrollmentCapability.Module
	// Now is the announcement clock; nil keeps the store default.
	Now func() time.Time
}

// ParentPortalRepositories are the retained guardian-portal repositories
// (built by the Care Plan composition) and the parent-messaging and
// announcement repositories (built by Communication) over their owners.
type ParentPortalRepositories struct {
	carePlanCompose.ParentPortalRepositories
	parentStore.MessagingRepositories
}

// WithSchools returns the repositories with the school names and the school
// eligibility resolved through the Organisation & Tenancy owner.
func (r ParentPortalRepositories) WithSchools(schools organizationtenancy.Query) ParentPortalRepositories {
	if schools == nil {
		panic("parent portal repositories: organization tenancy query is required")
	}
	return ParentPortalRepositories{
		ParentPortalRepositories: r.ParentPortalRepositories.WithSchools(parentPortalSchools{schools: schools}),
		MessagingRepositories:    r.WithSchoolNames(parentPortalSchools{schools: schools}),
	}
}

// parentPortalDirectory serves the guardian portal's People Directory port.
type parentPortalDirectory struct{ people peopledirectory.Capability }

func (d parentPortalDirectory) ListParentPortalStudents(ctx context.Context, ids []int64) ([]carePlanCompose.ParentPortalStudent, error) {
	students, err := d.people.ListStudentsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]carePlanCompose.ParentPortalStudent, 0, len(students))
	for _, student := range students {
		result = append(result, carePlanCompose.ParentPortalStudent{
			ID: student.ID, TenantID: student.TenantID, PersonID: student.PersonID,
			SchoolClass: student.SchoolClass, Status: student.Status,
			EnrolledFrom: student.EnrolledFrom, EnrolledUntil: student.EnrolledUntil,
		})
	}
	return result, nil
}

func (d parentPortalDirectory) ListParentPortalGuardianLinks(ctx context.Context, accountID int64) ([]carePlanCompose.ParentPortalGuardianLink, error) {
	links, err := d.people.ListGuardianLinksByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	result := make([]carePlanCompose.ParentPortalGuardianLink, 0, len(links))
	for _, link := range links {
		result = append(result, carePlanCompose.ParentPortalGuardianLink{
			TenantID: link.TenantID, StudentID: link.StudentID, GuardianProfileID: link.GuardianProfileID, Permissions: link.Permissions,
		})
	}
	return result, nil
}

func (d parentPortalDirectory) ListParentPortalPersons(ctx context.Context, ids []int64) ([]carePlanCompose.ParentPortalPerson, error) {
	persons, err := d.people.ListPersonsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]carePlanCompose.ParentPortalPerson, 0, len(persons))
	for _, person := range persons {
		result = append(result, carePlanCompose.ParentPortalPerson{ID: person.ID, FirstName: person.FirstName, LastName: person.LastName})
	}
	return result, nil
}

// parentPortalSchools serves the guardian portal's and the parent inbox's
// school ports from the Organisation & Tenancy owner.
type parentPortalSchools struct{ schools organizationtenancy.Query }

func (d parentPortalSchools) ListParentPortalSchools(ctx context.Context, ids []int64) ([]carePlanCompose.ParentPortalSchool, error) {
	schools, err := d.schools.ListSchoolsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]carePlanCompose.ParentPortalSchool, 0, len(schools))
	for _, school := range schools {
		result = append(result, carePlanCompose.ParentPortalSchool{
			ID: school.ID, Name: school.Name, Slug: school.Slug, Subdomain: school.Subdomain,
			Active: school.Active, Hidden: school.Hidden, Deleted: school.IsDeleted(),
		})
	}
	return result, nil
}

func (d parentPortalSchools) SchoolName(ctx context.Context, tenantID int64) (string, error) {
	school, err := d.schools.FindSchool(ctx, tenantID)
	if err != nil {
		return "", err
	}
	return school.Name, nil
}

func (d parentPortalSchools) SchoolNames(ctx context.Context, ids []int64) (map[int64]string, error) {
	schools, err := schoolsByID(ctx, d.schools, ids)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(schools))
	for id, school := range schools {
		names[id] = school.Name
	}
	return names, nil
}

// parentStaffMembers serves the parent inbox's staff port from School
// Membership.
type parentStaffMembers struct{ membership schoolmembership.Query }

func (d parentStaffMembers) list(ctx context.Context, tenantIDs []int64) ([]parentStore.StaffMember, error) {
	members, err := d.membership.ListStaff(ctx, schoolmembership.StaffFilter{TenantIDs: tenantIDs})
	if err != nil {
		return nil, err
	}
	result := make([]parentStore.StaffMember, 0, len(members))
	for _, member := range members {
		result = append(result, parentStore.StaffMember{TenantID: member.TenantID, PersonID: member.PersonID})
	}
	return result, nil
}

// parentGroupNames serves the parent inbox's group port from School
// Structure.
type parentGroupNames struct{ groups schoolstructure.Query }

func (d parentGroupNames) list(ctx context.Context, ids []int64) (map[int64]string, error) {
	return groupNamesByID(ctx, d.groups, ids)
}
