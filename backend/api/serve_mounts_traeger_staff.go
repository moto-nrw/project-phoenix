package api

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/moto-ogs/project-phoenix/api/common"
	auditModels "github.com/moto-ogs/project-phoenix/modules/auditlog/records"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/schoolmembership"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/modules/workforce/http/traegerstaff"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
)

// mountTraegerStaffRoutes serves the Personalstamm of ADR 0089 (#4705) at
// /traeger/staff-records behind the Träger right „Personal“, over the
// personnel record staffAdmin the root built for the schools.
func mountTraegerStaffRoutes(root chi.Router, staffAdmin *workforce.StaffAdmin, organizations HolidayOrganizations,
	persons TraegerPersons, membership TraegerStaff, accessLog owners.DataAccessAppender,
) error {
	if staffAdmin == nil {
		return errors.New("traeger staff records: the personnel record is required")
	}
	records := staffAdmin.TraegerStaffRecords(workforce.TraegerStaffPorts{
		Schools:     traegerStaffSchools{organizations: organizations},
		Employments: traegerSchoolEmployments{staff: membership},
		Persons:     traegerSchoolPersons{persons: persons},
		Access:      traegerStaffAccessLog{log: accessLog},
		Runtime:     traegerDocumentRuntime{},
	})
	resource := traegerstaff.NewRecordsResource(records, common.RequiresPermission(identityaccess.TraegerRightStaffManage))
	root.Mount("/traeger/staff-records", resource.TraegerRouter(ProtectedTraegerRoutes))
	return nil
}

// traegerStaffSchools lists the undeleted schools of the Träger by name.
type traegerStaffSchools struct {
	organizations HolidayOrganizations
}

func (s traegerStaffSchools) TraegerSchools(ctx context.Context, organizationID int64) ([]workforce.TraegerStaffSchool, error) {
	schools, err := s.organizations.ListSchoolsByOrganization(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	result := make([]workforce.TraegerStaffSchool, 0, len(schools))
	for _, school := range schools {
		if !school.IsDeleted() {
			result = append(result, workforce.TraegerStaffSchool{ID: school.ID, Name: school.Name})
		}
	}
	slices.SortFunc(result, func(a, b workforce.TraegerStaffSchool) int {
		if order := strings.Compare(a.Name, b.Name); order != 0 {
			return order
		}
		return int(a.ID - b.ID)
	})
	return result, nil
}

// traegerSchoolEmployments reads the live employments of the school of the
// tenant transaction from School Membership. External caregivers are no
// staff of the Träger (ADR 0089 section 1) and stay out.
type traegerSchoolEmployments struct {
	staff TraegerStaff
}

func (s traegerSchoolEmployments) SchoolEmployments(ctx context.Context, staffIDs []int64) ([]workforce.TraegerSchoolEmployment, error) {
	members, err := s.staff.ListStaff(ctx, schoolmembership.StaffFilter{IDs: staffIDs})
	if err != nil {
		return nil, err
	}
	result := make([]workforce.TraegerSchoolEmployment, 0, len(members))
	for _, member := range members {
		if member.IsDeleted() || member.HasExternalCaregiverRecord {
			continue
		}
		result = append(result, workforce.TraegerSchoolEmployment{StaffID: member.ID, PersonID: member.PersonID, PersonnelNumber: member.PersonnelNumber})
	}
	return result, nil
}

// traegerSchoolPersons reads the persons behind the employments from the
// People Directory.
type traegerSchoolPersons struct {
	persons TraegerPersons
}

func (p traegerSchoolPersons) SchoolPersons(ctx context.Context, personIDs []int64) ([]workforce.TraegerSchoolPerson, error) {
	persons, err := p.persons.ListPersonsByID(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	result := make([]workforce.TraegerSchoolPerson, 0, len(persons))
	for _, person := range persons {
		result = append(result, workforce.TraegerSchoolPerson{
			PersonID: person.ID, AccountID: person.AccountID, FirstName: person.FirstName, LastName: person.LastName, Birthday: person.Birthday,
		})
	}
	return result, nil
}

// traegerStaffAccessLog writes a read of the Personalverwaltung into the
// access log of the school of the tenant transaction (ADR 0084 section 7).
type traegerStaffAccessLog struct {
	log owners.DataAccessAppender
}

func (l traegerStaffAccessLog) RecordTraegerStaffAccess(ctx context.Context, access workforce.TraegerStaffAccess) error {
	resource := auditModels.ResourceTypeTraegerStaffRecords
	if access.View == workforce.TraegerStaffViewFinancial || access.View == workforce.TraegerStaffViewFinancialReveal {
		resource = auditModels.ResourceTypeTraegerStaffFinancial
	}
	now := time.Now()
	return l.log.Create(ctx, &auditModels.DataAccessLog{
		ActorAccountID: access.AccountID, ActorRole: traegerAccessActorRole, ResourceType: resource,
		RangeStart: now, RangeEnd: now,
		Metadata: map[string]any{
			"channel": "traeger_portal", "organization_id": access.OrganizationID, "view": access.View, "rows": access.Rows,
			"staff_ids": access.StaffIDs, "staff_record_id": access.RecordID,
		},
	})
}
