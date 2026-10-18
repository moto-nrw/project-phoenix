package api

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
)

// visitorProjection composes open visits and home-school identity through
// their public owners. It never reads another owner's tables.
type visitorProjection struct {
	visits interface {
		ListOpenVisitStudentIDs(context.Context, int64) ([]int64, error)
	}
	students peopledirectory.StudentQuery
	persons  peopledirectory.Query
}

func (r *visitorProjection) FindCrossTenantStudents(ctx context.Context, hostingTenantID int64) ([]presenceCompose.CrossTenantStudent, error) {
	if r.students == nil {
		return nil, errors.New("visitor projection: people directory is required")
	}
	visitingIDs, err := r.visits.ListOpenVisitStudentIDs(ctx, hostingTenantID)
	if err != nil {
		return nil, err
	}
	students := []presenceCompose.CrossTenantStudent{}
	if len(visitingIDs) == 0 {
		return students, nil
	}
	visitors, err := r.students.ListStudentsAcrossTenantsByID(ctx, visitingIDs)
	if err != nil {
		return nil, err
	}
	for _, visitor := range visitors {
		if !studentpresence.VisitsFromAnotherSchool(visitor.TenantID, hostingTenantID) {
			continue
		}
		students = append(students, presenceCompose.CrossTenantStudent{StudentID: visitor.ID, PersonID: visitor.PersonID, HomeTenantID: visitor.TenantID, GroupID: visitor.GroupID})
	}
	if len(students) == 0 || r.persons == nil {
		return students, nil
	}
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		ids = append(ids, student.PersonID)
	}
	// Visitors belong to their home school: resolve their names through the
	// cross-tenant query instead of the hosting tenant's scope.
	values, err := r.persons.ListPersonsAcrossTenantsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	persons := make(map[int64]peopledirectory.Person, len(values))
	for _, value := range values {
		persons[value.ID] = value
	}
	// A person row always exists behind a student; one the directory no
	// longer shows (soft-deleted) keeps the visit with blank names.
	for index := range students {
		if person, found := persons[students[index].PersonID]; found {
			students[index].FirstName = person.FirstName
			students[index].LastName = person.LastName
		}
	}
	return students, nil
}
