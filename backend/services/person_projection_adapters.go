package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

// educationPersonQuery adapts the People Directory to the person port the
// substitution module declares for staff names.
type educationPersonQuery struct {
	persons peopledirectory.Query
}

func (q educationPersonQuery) ListPersonsByID(ctx context.Context, ids []int64) ([]educationService.Person, error) {
	persons, err := q.persons.ListPersonsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]educationService.Person, 0, len(persons))
	for _, person := range persons {
		result = append(result, educationService.Person{ID: person.ID, FirstName: person.FirstName, LastName: person.LastName})
	}
	return result, nil
}

func newEducationPersonQuery(persons peopledirectory.Query) educationService.PersonQuery {
	return educationPersonQuery{persons: persons}
}

// staffNoticeNameLookup adapts the People Directory to the name port the
// Tagesinformationen declare for their acknowledgement list (#2208): account
// ids in, display names of the tenant's active persons out.
type staffNoticeNameLookup struct {
	persons peopledirectory.Query
}

func (q staffNoticeNameLookup) ListPersonNamesByAccount(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	persons, err := q.persons.ListPersonsByAccount(ctx, accountIDs)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(persons))
	for _, person := range persons {
		if person.AccountID == nil {
			continue
		}
		names[*person.AccountID] = person.FullName()
	}
	return names, nil
}

func newStaffNoticeNameLookup(persons peopledirectory.Query) scheduleService.StaffNoticeNameLookup {
	return staffNoticeNameLookup{persons: persons}
}
