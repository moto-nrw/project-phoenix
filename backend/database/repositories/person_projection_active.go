package repositories

import (
	"context"
	"strings"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// personCrossTenantRepository attaches the visiting students' names.
type personCrossTenantRepository struct {
	activeModels.CrossTenantRepository
	persons peopledirectory.Query
}

func (r personCrossTenantRepository) FindCrossTenantStudents(ctx context.Context, hostingTenantID int64) ([]activeModels.CrossTenantStudent, error) {
	students, err := r.CrossTenantRepository.FindCrossTenantStudents(ctx, hostingTenantID)
	if err != nil || len(students) == 0 {
		return students, err
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
	staff := make([]*usersModels.Staff, 0, len(rows))
	for _, row := range rows {
		if row != nil && row.Staff != nil {
			staff = append(staff, row.Staff)
		}
	}
	return attachStaffPersons(ctx, query, staff)
}

// attachStaffPersons resolves Staff.Person for every staff row through the
// owner query. Staff without a resolvable person keep a nil Person, which
// is what the previous LEFT JOIN produced.
func attachStaffPersons(ctx context.Context, query peopledirectory.Query, staff []*usersModels.Staff) error {
	ids := make([]int64, 0, len(staff))
	for _, member := range staff {
		ids = append(ids, member.PersonID)
	}
	persons, err := personsByID(ctx, query, ids)
	if err != nil {
		return err
	}
	for _, member := range staff {
		if person, found := persons[member.PersonID]; found {
			member.Person = toLegacyPerson(person)
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

// personVisitRepository attaches the student names to the open-visit
// display rows.
type personVisitRepository struct {
	activeModels.VisitRepository
	persons peopledirectory.Query
}

func (r personVisitRepository) FindActiveWithStudentDisplayByGroup(ctx context.Context, activeGroupID int64) ([]*activeModels.VisitWithStudentDisplay, error) {
	rows, err := r.VisitRepository.FindActiveWithStudentDisplayByGroup(ctx, activeGroupID)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.PersonID)
	}
	persons, err := personsByID(ctx, r.persons, ids)
	if err != nil {
		return nil, err
	}
	// A person row always exists behind a student; one the directory no
	// longer shows (soft-deleted) keeps the visit with blank names.
	for _, row := range rows {
		if person, found := persons[row.PersonID]; found {
			row.FirstName = person.FirstName
			row.LastName = person.LastName
		}
	}
	return rows, nil
}
