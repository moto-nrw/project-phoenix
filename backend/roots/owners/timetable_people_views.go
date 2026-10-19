package owners

import (
	"context"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type timetableStaffRows interface {
	FindByID(ctx context.Context, id any) (*peopleValues.Staff, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.Staff, error)
	FindWithPerson(ctx context.Context, id int64) (*peopleValues.Staff, error)
	FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.Staff, error)
	ListAllWithPerson(ctx context.Context) ([]*peopleValues.Staff, error)
}

// TimetableStaff serves the timetable's staff ports.
type TimetableStaff struct{ rows timetableStaffRows }

// NewTimetableStaff binds the staff ports to the People Directory staff rows.
func NewTimetableStaff(rows timetableStaffRows) TimetableStaff {
	// TimetableStaff maps only membership and person fields, never employment.
	if members, ok := rows.(StaffMembershipRepository); ok {
		members.membershipOnly = true
		rows = members
	}
	return TimetableStaff{rows: rows}
}

func (s TimetableStaff) FindByID(ctx context.Context, id any) (*timetable.StaffMember, error) {
	return MapRow(TimetableStaffMember)(s.rows.FindByID(ctx, id))
}

func (s TimetableStaff) FindWithPerson(ctx context.Context, id int64) (*timetable.StaffMember, error) {
	return MapRow(TimetableStaffMember)(s.rows.FindWithPerson(ctx, id))
}

func (s TimetableStaff) FindByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.StaffMember, error) {
	return MapRowsByID(TimetableStaffMember)(s.rows.FindByIDs(ctx, ids))
}

func (s TimetableStaff) FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.StaffMember, error) {
	return MapRowsByID(TimetableStaffMember)(s.rows.FindWithPersonByIDs(ctx, ids))
}

func (s TimetableStaff) ListAllWithPerson(ctx context.Context) ([]*timetable.StaffMember, error) {
	rows, err := s.rows.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*timetable.StaffMember, 0, len(rows))
	for _, row := range rows {
		result = append(result, TimetableStaffMember(row))
	}
	return result, nil
}

type TimetablePersonRows interface {
	FindByAccountID(ctx context.Context, accountID int64) (*peopleValues.DirectoryPersonSnapshot, error)
}

type timetableOperationPeopleRows interface {
	TimetablePersonRows
	GetByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.DirectoryPersonSnapshot, error)
	GetStaffByPersonID(ctx context.Context, personID int64) (*peopleValues.Staff, error)
	GetStaffWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.Staff, error)
}

// timetableOperationPeople serves the timetable operations' person port.
type timetableOperationPeople struct{ rows timetableOperationPeopleRows }

func NewTimetableOperationPeople(rows timetableOperationPeopleRows) timetableOperationPeople {
	return timetableOperationPeople{rows: rows}
}

func (p timetableOperationPeople) FindByAccountID(ctx context.Context, accountID int64) (*timetable.Person, error) {
	return MapRow(TimetablePerson)(p.rows.FindByAccountID(ctx, accountID))
}

func (p timetableOperationPeople) GetByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.Person, error) {
	return MapRowsByID(TimetablePerson)(p.rows.GetByIDs(ctx, ids))
}

func (p timetableOperationPeople) GetStaffByPersonID(ctx context.Context, personID int64) (*timetable.StaffMember, error) {
	return MapRow(TimetableStaffMember)(p.rows.GetStaffByPersonID(ctx, personID))
}

func (p timetableOperationPeople) GetStaffWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.StaffMember, error) {
	return MapRowsByID(TimetableStaffMember)(p.rows.GetStaffWithPersonByIDs(ctx, ids))
}

// timetableLifecycleStudents serves the instance lifecycle's student port
// from the roster's People Directory child rows.
type timetableLifecycleStudents struct {
	rows timetableCompose.RosterStudents
}

// NewTimetableLifecycleStudents binds the lifecycle's student port to the
// People Directory child rows.
func NewTimetableLifecycleStudents(rows timetableCompose.RosterStudents) timetablePorts.LifecycleStudents {
	return timetableLifecycleStudents{rows: rows}
}

func (s timetableLifecycleStudents) FindByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.LifecycleStudent, error) {
	return MapRowsByID(timetableRosterLifecycleStudent)(s.rows.FindByIDs(ctx, ids))
}

func (s timetableLifecycleStudents) FindByIDsForUpdate(ctx context.Context, ids []int64) (map[int64]*timetable.LifecycleStudent, error) {
	return MapRowsByID(timetableRosterLifecycleStudent)(s.rows.FindByIDsForUpdate(ctx, ids))
}

func (s timetableLifecycleStudents) FindReadScopeByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.LifecycleStudent, error) {
	return MapRowsByID(timetableRosterLifecycleStudent)(s.rows.FindReadScopeByIDs(ctx, ids))
}

func TimetableStaffMember(row *peopleValues.Staff) *timetable.StaffMember {
	if row == nil {
		return nil
	}
	return &timetable.StaffMember{ID: row.ID, PersonID: row.PersonID, IsGuest: row.IsGuest, Person: TimetablePerson(row.Person)}
}

func TimetablePerson(row *peopleValues.DirectoryPersonSnapshot) *timetable.Person {
	if row == nil {
		return nil
	}
	return &timetable.Person{ID: row.ID, FirstName: row.FirstName, LastName: row.LastName, AccountID: row.AccountID}
}

func timetableLifecycleStudent(row *peopleValues.StudentState) *timetable.LifecycleStudent {
	if row == nil {
		return nil
	}
	return &timetable.LifecycleStudent{ID: row.ID, GroupID: row.GroupID, Alumnus: row.IsAlumnus(), EnrolledUntil: row.EnrolledUntil}
}

func timetableOperationStudent(row *peopleValues.StudentState) *timetable.OperationStudent {
	if row == nil {
		return nil
	}
	return &timetable.OperationStudent{LifecycleStudent: *timetableLifecycleStudent(row), PersonID: row.PersonID, SchoolClass: row.SchoolClass, Person: TimetablePerson(row.Person)}
}

// MapRow maps the row of a read; an error passes through.
func MapRow[R, V any](convert func(R) *V) func(R, error) (*V, error) {
	return func(row R, err error) (*V, error) {
		if err != nil {
			return nil, err
		}
		return convert(row), nil
	}
}

// MapRowsByID maps the rows of a read keyed by ID; an error passes through.
func MapRowsByID[R, V any](convert func(R) *V) func(map[int64]R, error) (map[int64]*V, error) {
	return func(rows map[int64]R, err error) (map[int64]*V, error) {
		if err != nil {
			return nil, err
		}
		result := make(map[int64]*V, len(rows))
		for id, row := range rows {
			result[id] = convert(row)
		}
		return result, nil
	}
}

// timetableStudentRows is the existing Directory read and mutation-lock source.
type timetableStudentRows interface {
	FindByID(context.Context, any) (*peopleValues.StudentState, error)
	FindByIDForUpdate(context.Context, int64) (*peopleValues.StudentState, error)
	FindByIDs(context.Context, []int64) (map[int64]*peopleValues.StudentState, error)
	FindByIDsForUpdate(context.Context, []int64) (map[int64]*peopleValues.StudentState, error)
	FindReadScopeByIDs(context.Context, []int64) (map[int64]*peopleValues.StudentState, error)
}

// TimetableRosterStudents binds the activity roster to the existing Directory rows.
type TimetableRosterStudents struct{ rows timetableStudentRows }

func NewTimetableRosterStudents(rows timetableStudentRows) TimetableRosterStudents {
	return TimetableRosterStudents{rows: rows}
}

func (s TimetableRosterStudents) FindByID(ctx context.Context, id any) (*timetable.OperationStudent, error) {
	return MapRow(timetableOperationStudent)(s.rows.FindByID(ctx, id))
}

func (s TimetableRosterStudents) FindByIDForUpdate(ctx context.Context, id int64) (*timetable.OperationStudent, error) {
	return MapRow(timetableOperationStudent)(s.rows.FindByIDForUpdate(ctx, id))
}

func (s TimetableRosterStudents) FindByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.OperationStudent, error) {
	return MapRowsByID(timetableOperationStudent)(s.rows.FindByIDs(ctx, ids))
}

func (s TimetableRosterStudents) FindByIDsForUpdate(ctx context.Context, ids []int64) (map[int64]*timetable.OperationStudent, error) {
	return MapRowsByID(timetableOperationStudent)(s.rows.FindByIDsForUpdate(ctx, ids))
}

func (s TimetableRosterStudents) FindReadScopeByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.OperationStudent, error) {
	return MapRowsByID(timetableOperationStudent)(s.rows.FindReadScopeByIDs(ctx, ids))
}

func timetableRosterLifecycleStudent(row *timetable.OperationStudent) *timetable.LifecycleStudent {
	if row == nil {
		return nil
	}
	return &row.LifecycleStudent
}
