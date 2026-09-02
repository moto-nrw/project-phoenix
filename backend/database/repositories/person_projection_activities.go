package repositories

import (
	"context"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// personActivityGroupRepository attaches Staff.Person to the supervisors a
// group is loaded with.
type personActivityGroupRepository struct {
	activitiesModels.GroupRepository
	persons peopledirectory.Query
}

func (r personActivityGroupRepository) FindWithSupervisors(ctx context.Context, groupID int64) (*activitiesModels.Group, []*activitiesModels.SupervisorPlanned, error) {
	group, supervisors, err := r.GroupRepository.FindWithSupervisors(ctx, groupID)
	if err != nil {
		return nil, nil, err
	}
	if err := attachPlannedSupervisorPersons(ctx, r.persons, supervisors); err != nil {
		return nil, nil, err
	}
	return group, supervisors, nil
}

// personSupervisorPlannedRepository attaches Staff.Person to planned
// supervisor rows.
type personSupervisorPlannedRepository struct {
	activitiesModels.SupervisorPlannedRepository
	persons peopledirectory.Query
}

func (r personSupervisorPlannedRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*activitiesModels.SupervisorPlanned, error) {
	rows, err := r.SupervisorPlannedRepository.FindByGroupID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return rows, attachPlannedSupervisorPersons(ctx, r.persons, rows)
}

func (r personSupervisorPlannedRepository) FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*activitiesModels.SupervisorPlanned, error) {
	rows, err := r.SupervisorPlannedRepository.FindByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	return rows, attachPlannedSupervisorPersons(ctx, r.persons, rows)
}

func attachPlannedSupervisorPersons(ctx context.Context, query peopledirectory.Query, rows []*activitiesModels.SupervisorPlanned) error {
	staff := make([]*usersModels.Staff, 0, len(rows))
	for _, row := range rows {
		if row != nil && row.Staff != nil {
			staff = append(staff, row.Staff)
		}
	}
	return attachStaffPersons(ctx, query, staff)
}

// personStudentEnrollmentRepository attaches Student.Person to the
// enrollments of a group.
type personStudentEnrollmentRepository struct {
	activitiesModels.StudentEnrollmentRepository
	persons peopledirectory.Query
}

func (r personStudentEnrollmentRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*activitiesModels.StudentEnrollment, error) {
	rows, err := r.StudentEnrollmentRepository.FindByGroupID(ctx, groupID)
	if err != nil {
		return nil, err
	}
	students := make([]*usersModels.Student, 0, len(rows))
	for _, row := range rows {
		if row != nil && row.Student != nil {
			students = append(students, row.Student)
		}
	}
	return rows, attachStudentPersons(ctx, r.persons, students)
}

// attachStudentPersons resolves Student.Person for every student row.
// Students without a resolvable person keep a nil Person, matching the
// previous LEFT JOIN.
func attachStudentPersons(ctx context.Context, query peopledirectory.Query, students []*usersModels.Student) error {
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		ids = append(ids, student.PersonID)
	}
	persons, err := personsByID(ctx, query, ids)
	if err != nil {
		return err
	}
	for _, student := range students {
		if person, found := persons[student.PersonID]; found {
			student.Person = toLegacyPerson(person)
		}
	}
	return nil
}
