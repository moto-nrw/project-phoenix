package owners

import (
	"context"

	owner1 "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// CareMembershipCommands is bound by the root to School Membership. Care Plan
// supplies the frozen day and keeps its ledger and membership write in one UOW.
type CareMembershipCommands interface {
	EndCare(context.Context, []int64, string) (int64, error)
	ResumeCare(context.Context, int64, string, string, string) (bool, error)
}

// careStudentPeople is the People Directory owner's child reads the lifecycle
// makes directly: the withdrawal list and the tenant's whole population.
type careStudentPeople interface {
	peopledirectory.StudentQuery
	peopledirectory.StudentDirectoryQuery
}

// CareStudentRows are the People Directory child rows the care lifecycle
// decides on.
type CareStudentRows interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*peopledirectory.StudentState, error)
	FindByIDsForUpdate(ctx context.Context, ids []int64) (map[int64]*peopledirectory.StudentState, error)
	FindActiveDueForDeactivation(ctx context.Context, lastCareDay owner1.Date) ([]*peopledirectory.StudentState, error)
}

// CarePersonRows are the persons of those children.
type CarePersonRows interface {
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*peopledirectory.DirectoryPersonSnapshot, error)
}

// CareStudents serves the care lifecycle's People Directory port: the child
// rows it decides on, their persons, and the reversible membership commands.
type CareStudents struct {
	students   CareStudentRows
	persons    CarePersonRows
	membership CareMembershipCommands
	people     careStudentPeople
}

func (s CareStudents) FindCareStudents(ctx context.Context, studentIDs []int64, lock bool) (map[int64]carePlanCompose.CareStudent, error) {
	var (
		rows map[int64]*peopledirectory.StudentState
		err  error
	)
	if lock {
		rows, err = s.students.FindByIDsForUpdate(ctx, studentIDs)
	} else {
		rows, err = s.students.FindByIDs(ctx, studentIDs)
	}
	if err != nil {
		return nil, err
	}
	result := make(map[int64]carePlanCompose.CareStudent, len(rows))
	for id, row := range rows {
		if row == nil {
			continue
		}
		result[id] = carePlanCompose.CareStudent{
			ID: row.ID, PersonID: row.PersonID, SchoolClass: row.SchoolClass, Status: string(row.Status),
			EnrolledFrom: row.EnrolledFrom, EnrolledUntil: row.EnrolledUntil, UpdatedAt: row.UpdatedAt,
		}
	}
	return result, nil
}

func (s CareStudents) FindCarePersons(ctx context.Context, personIDs []int64) (map[int64]carePlanCompose.CarePerson, error) {
	rows, err := s.persons.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]carePlanCompose.CarePerson, len(rows))
	for id, row := range rows {
		if row == nil {
			continue
		}
		result[id] = carePlanCompose.CarePerson{
			FirstName: row.FirstName, LastName: row.LastName, HasRFIDTag: row.TagID != nil && *row.TagID != "",
		}
	}
	return result, nil
}

func (s CareStudents) ListDueForDeactivation(ctx context.Context, lastCareDay owner1.Date) ([]int64, error) {
	rows, err := s.students.FindActiveDueForDeactivation(ctx, lastCareDay)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids, nil
}

func (s CareStudents) ListStudentIDs(ctx context.Context) ([]int64, error) {
	return s.people.ListAllStudentIDs(ctx)
}

func (s CareStudents) EndCare(ctx context.Context, studentIDs []int64, lastCareDay *owner1.Date) (int64, error) {
	return s.membership.EndCare(ctx, studentIDs, peopledirectory.RenderCalendarDate(lastCareDay))
}

func (s CareStudents) ResumeCare(ctx context.Context, studentID int64, start owner1.Date, status string, on owner1.Date) (bool, error) {
	return s.membership.ResumeCare(ctx, studentID, start.String(), status, on.String())
}
