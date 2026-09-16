package repositories

import (
	"context"
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
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

func (r *visitorProjection) FindCrossTenantStudents(ctx context.Context, hostingTenantID int64) ([]activeModels.CrossTenantStudent, error) {
	if r.students == nil {
		return nil, errors.New("visitor projection: people directory is required")
	}
	visitingIDs, err := r.visits.ListOpenVisitStudentIDs(ctx, hostingTenantID)
	if err != nil {
		return nil, err
	}
	students := []activeModels.CrossTenantStudent{}
	if len(visitingIDs) == 0 {
		return students, nil
	}
	visitors, err := r.students.ListStudentsAcrossTenantsByID(ctx, visitingIDs)
	if err != nil {
		return nil, err
	}
	for _, visitor := range visitors {
		if visitor.TenantID == hostingTenantID {
			continue
		}
		students = append(students, activeModels.CrossTenantStudent{StudentID: visitor.ID, PersonID: visitor.PersonID, HomeTenantID: visitor.TenantID, GroupID: visitor.GroupID})
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

// personGroupSupervisorRepository attaches Staff.Person to active-group
// supervisions.
type personGroupSupervisorRepository struct {
	activeModels.GroupSupervisorRepository
	persons peopledirectory.Query
}

func (r personGroupSupervisorRepository) FindByActiveGroupID(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*activeModels.GroupSupervisor, error) {
	rows, err := r.GroupSupervisorRepository.FindByActiveGroupID(ctx, activeGroupID, activeOnly)
	if err != nil {
		return nil, err
	}
	return rows, attachSupervisionPersons(ctx, r.persons, rows)
}

func (r personGroupSupervisorRepository) FindByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, activeOnly bool) ([]*activeModels.GroupSupervisor, error) {
	rows, err := r.GroupSupervisorRepository.FindByActiveGroupIDs(ctx, activeGroupIDs, activeOnly)
	if err != nil {
		return nil, err
	}
	return rows, attachSupervisionPersons(ctx, r.persons, rows)
}

func attachSupervisionPersons(ctx context.Context, query peopledirectory.Query, rows []*activeModels.GroupSupervisor) error {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row != nil && row.Staff != nil {
			ids = append(ids, row.Staff.PersonID)
		}
	}
	persons, err := personsByID(ctx, query, ids)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row == nil || row.Staff == nil {
			continue
		}
		if person, found := persons[row.Staff.PersonID]; found {
			value := toLegacyPerson(person)
			row.Staff.Person = &activeModels.SessionStaffPerson{
				ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
				FirstName: value.FirstName, LastName: value.LastName, Birthday: value.Birthday,
				TagID: value.TagID, AccountID: value.AccountID,
			}
		}
	}
	return nil
}

// personStaffAbsenceRepository turns the free-text subject search into a
// person filter and attaches the subject and decider names.
type personStaffAbsenceRepository struct {
	activeModels.StaffAbsenceRepository
	persons peopledirectory.Query
}

func (r personStaffAbsenceRepository) ListRequests(ctx context.Context, filter activeModels.AbsenceRequestFilter) ([]*activeModels.AbsenceRequestRow, error) {
	if search := strings.TrimSpace(filter.Search); search != "" && filter.SubjectPersonIDs == nil {
		matches, err := r.persons.SearchPersons(ctx, peopledirectory.PersonFilter{FullNameContains: search})
		if err != nil {
			return nil, err
		}
		filter.SubjectPersonIDs = make([]int64, 0, len(matches))
		for _, match := range matches {
			filter.SubjectPersonIDs = append(filter.SubjectPersonIDs, match.ID)
		}
	}
	rows, err := r.StaffAbsenceRepository.ListRequests(ctx, filter)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	ids := make([]int64, 0, 2*len(rows))
	for _, row := range rows {
		if row.SubjectPersonID != nil {
			ids = append(ids, *row.SubjectPersonID)
		}
		if row.DeciderPersonID != nil {
			ids = append(ids, *row.DeciderPersonID)
		}
	}
	persons, err := personsByID(ctx, r.persons, ids)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		row.StaffName = fullNameOf(persons, row.SubjectPersonID)
		row.DecidedByName = fullNameOf(persons, row.DeciderPersonID)
	}
	return rows, nil
}

func fullNameOf(persons map[int64]peopledirectory.Person, id *int64) string {
	if id == nil {
		return ""
	}
	person, found := persons[*id]
	if !found {
		return ""
	}
	return person.FullName()
}
