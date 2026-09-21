package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

const offeringChangeSearchPageSize = 100

// offeringChangeStudentSearch binds the offering change queue's name search to
// the People Directory. It is a named tenant-safe read, not a join: the
// directory matches the persons, and the enrolled students are filtered
// against that match in the directory's listing order.
type offeringChangeStudentSearch struct{ people peopledirectory.Query }

func (s offeringChangeStudentSearch) SearchEnrolledStudentIDs(ctx context.Context, name string) ([]int64, error) {
	personIDs, err := s.searchPersonIDs(ctx, name)
	if err != nil {
		return nil, err
	}
	students, err := s.people.ListEnrolledStudents(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]int64, 0)
	for _, student := range students {
		if _, matches := personIDs[student.PersonID]; matches {
			result = append(result, student.ID)
		}
	}
	return result, nil
}

func (s offeringChangeStudentSearch) searchPersonIDs(ctx context.Context, name string) (map[int64]struct{}, error) {
	result := map[int64]struct{}{}
	for page := 1; ; page++ {
		people, err := s.people.SearchPersons(ctx, peopledirectory.PersonFilter{FullNameContains: name, Page: page, PageSize: offeringChangeSearchPageSize})
		if err != nil {
			return nil, err
		}
		for _, person := range people {
			result[person.ID] = struct{}{}
		}
		if len(people) < offeringChangeSearchPageSize {
			return result, nil
		}
	}
}
