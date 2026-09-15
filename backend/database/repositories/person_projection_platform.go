package repositories

import (
	"context"
	"fmt"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/uptrace/bun"
)

// personOperatorSummariesRepository resolves operator person summaries and
// listings through the People Directory.
type personOperatorSummariesRepository struct {
	platformModels.OperatorSummariesRepository
	persons    peopledirectory.Query
	schools    func() platformModels.SchoolRepository
	accounts   func() authModels.AccountRepository
	membership func() schoolmembership.Capability
	db         *bun.DB
}

func (r personOperatorSummariesRepository) OrganizationSummaries(ctx context.Context) ([]*platformModels.OrganizationSummary, error) {
	rows, err := r.OperatorSummariesRepository.OrganizationSummaries(ctx)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	counts, err := r.persons.CountPersonsByTenant(ctx)
	if err != nil {
		return nil, fmt.Errorf("count persons for organization summaries: %w", err)
	}
	schools, err := r.schools().ListNonDeleted(ctx)
	if err != nil {
		return nil, fmt.Errorf("load schools for organization summaries: %w", err)
	}
	byOrganization := make(map[int64]int, len(rows))
	for _, school := range schools {
		byOrganization[school.OrganizationID] += counts[school.ID]
	}
	for _, row := range rows {
		row.PersonenCount = byOrganization[row.ID]
	}
	return rows, nil
}

func (r personOperatorSummariesRepository) SchoolSummaries(ctx context.Context) ([]*platformModels.SchoolSummary, error) {
	rows, err := r.OperatorSummariesRepository.SchoolSummaries(ctx)
	if err != nil {
		return nil, err
	}
	return rows, r.attachSchoolPersonCounts(ctx, rows)
}

func (r personOperatorSummariesRepository) SchoolSummariesByOrganization(ctx context.Context, organizationID int64) ([]*platformModels.SchoolSummary, error) {
	rows, err := r.OperatorSummariesRepository.SchoolSummariesByOrganization(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	return rows, r.attachSchoolPersonCounts(ctx, rows)
}

func (r personOperatorSummariesRepository) attachSchoolPersonCounts(ctx context.Context, rows []*platformModels.SchoolSummary) error {
	if len(rows) == 0 {
		return nil
	}
	counts, err := r.persons.CountPersonsByTenant(ctx)
	if err != nil {
		return fmt.Errorf("count persons for school summaries: %w", err)
	}
	for _, row := range rows {
		row.PersonenCount = counts[row.ID]
	}
	return nil
}

func (r personOperatorSummariesRepository) PersonsBySchool(ctx context.Context, schoolID int64) ([]platformModels.OperatorPersonInfo, error) {
	schools, err := r.OperatorSummariesRepository.SchoolSummaries(ctx)
	if err != nil {
		return nil, err
	}
	for _, school := range schools {
		if school.ID == schoolID && school.DeletedAt == nil {
			return r.listOperatorPersons(ctx, []*platformModels.SchoolSummary{school})
		}
	}
	return []platformModels.OperatorPersonInfo{}, nil
}

func (r personOperatorSummariesRepository) PersonsByOrganization(ctx context.Context, organizationID int64) ([]platformModels.OperatorPersonInfo, error) {
	schools, err := r.OperatorSummariesRepository.SchoolSummariesByOrganization(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	active := make([]*platformModels.SchoolSummary, 0, len(schools))
	for _, school := range schools {
		if school.DeletedAt == nil {
			active = append(active, school)
		}
	}
	return r.listOperatorPersons(ctx, active)
}

func (r personOperatorSummariesRepository) listOperatorPersons(ctx context.Context, schools []*platformModels.SchoolSummary) ([]platformModels.OperatorPersonInfo, error) {
	schoolByID := make(map[int64]*platformModels.SchoolSummary, len(schools))
	tenantIDs := make([]int64, 0, len(schools))
	for _, school := range schools {
		schoolByID[school.ID] = school
		tenantIDs = append(tenantIDs, school.ID)
	}
	persons, err := r.persons.ListPersonsByTenantIDs(ctx, tenantIDs)
	if err != nil || len(persons) == 0 {
		return personsToOperatorInfos(persons, schoolByID, nil, nil, nil), err
	}
	staff, students, emails, err := r.operatorPersonFacts(ctx, persons)
	if err != nil {
		return nil, err
	}
	return personsToOperatorInfos(persons, schoolByID, staff, students, emails), nil
}

func (r personOperatorSummariesRepository) operatorPersonFacts(ctx context.Context, persons []peopledirectory.Person) (map[int64]bool, map[int64]bool, map[int64]string, error) {
	personIDs, accountIDs := operatorPersonIDs(persons)
	students, err := usersRepo.FindOperatorPersonStudentMembership(ctx, r.db, personIDs)
	if err != nil {
		return nil, nil, nil, err
	}
	// Staff membership belongs to School Membership; the operator directory
	// asks the owner instead of joining users.staff.
	staff := make(map[int64]bool, len(personIDs))
	if len(personIDs) > 0 {
		members, listErr := r.membership().ListStaff(ctx, schoolmembership.StaffFilter{PersonIDs: personIDs})
		if listErr != nil {
			return nil, nil, nil, fmt.Errorf("load operator staff membership: %w", listErr)
		}
		for _, member := range members {
			staff[member.PersonID] = true
		}
	}
	emails, err := r.accounts().FindEmailsByAccountIDs(ctx, accountIDs)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("load operator account emails: %w", err)
	}
	return staff, students, emails, nil
}

func operatorPersonIDs(persons []peopledirectory.Person) ([]int64, []int64) {
	personIDs := make([]int64, 0, len(persons))
	accountIDs := make([]int64, 0, len(persons))
	for _, person := range persons {
		personIDs = append(personIDs, person.ID)
		if person.AccountID != nil {
			accountIDs = append(accountIDs, *person.AccountID)
		}
	}
	return personIDs, accountIDs
}

func personsToOperatorInfos(persons []peopledirectory.Person, schools map[int64]*platformModels.SchoolSummary, staff, students map[int64]bool, emails map[int64]string) []platformModels.OperatorPersonInfo {
	result := make([]platformModels.OperatorPersonInfo, 0, len(persons))
	for _, person := range persons {
		school := schools[person.TenantID]
		if school == nil {
			continue
		}
		info := platformModels.OperatorPersonInfo{
			ID: person.ID, FirstName: person.FirstName, LastName: person.LastName,
			HasAccount: person.AccountID != nil, HasRFIDCard: person.TagID != nil,
			IsStaff: staff[person.ID], IsStudent: students[person.ID],
			SchoolID: school.ID, SchoolName: school.Name,
			OrganizationID: school.OrganizationID, OrganizationName: school.OrganizationName,
			CreatedAt: person.CreatedAt,
		}
		if person.AccountID != nil {
			if email, found := emails[*person.AccountID]; found {
				info.AccountEmail = &email
			}
		}
		result = append(result, info)
	}
	return result
}
