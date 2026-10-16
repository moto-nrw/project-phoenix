package api

import (
	"context"

	users "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

// The adapters below serve the timetable's people ports from the People
// Directory rows (#2729): staff members, persons and the children the
// instance lifecycle checks, in the timetable's own views. Errors pass
// through unchanged, so the timetable keeps classifying a missing row as the
// repository not-found.

type timetableStaffRows interface {
	FindByID(ctx context.Context, id any) (*users.Staff, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*users.Staff, error)
	FindWithPerson(ctx context.Context, id int64) (*users.Staff, error)
	FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*users.Staff, error)
	ListAllWithPerson(ctx context.Context) ([]*users.Staff, error)
}

// TimetableStaff serves the timetable's staff ports.
type TimetableStaff struct{ rows timetableStaffRows }

// NewTimetableStaff binds the staff ports to the People Directory staff rows.
func NewTimetableStaff(rows timetableStaffRows) TimetableStaff { return TimetableStaff{rows: rows} }

func (s TimetableStaff) FindByID(ctx context.Context, id any) (*timetable.StaffMember, error) {
	return mapRow(timetableStaffMember)(s.rows.FindByID(ctx, id))
}

func (s TimetableStaff) FindWithPerson(ctx context.Context, id int64) (*timetable.StaffMember, error) {
	return mapRow(timetableStaffMember)(s.rows.FindWithPerson(ctx, id))
}

func (s TimetableStaff) FindByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.StaffMember, error) {
	return mapRowsByID(timetableStaffMember)(s.rows.FindByIDs(ctx, ids))
}

func (s TimetableStaff) FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.StaffMember, error) {
	return mapRowsByID(timetableStaffMember)(s.rows.FindWithPersonByIDs(ctx, ids))
}

func (s TimetableStaff) ListAllWithPerson(ctx context.Context) ([]*timetable.StaffMember, error) {
	rows, err := s.rows.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*timetable.StaffMember, 0, len(rows))
	for _, row := range rows {
		result = append(result, timetableStaffMember(row))
	}
	return result, nil
}

type timetablePersonRows interface {
	FindByAccountID(ctx context.Context, accountID int64) (*users.Person, error)
}

// timetableCorrectionPeople serves the attendance correction's person port.
type timetableCorrectionPeople struct{ rows timetablePersonRows }

func (p timetableCorrectionPeople) FindByAccountID(ctx context.Context, accountID int64) (*timetable.Person, error) {
	return mapRow(timetablePerson)(p.rows.FindByAccountID(ctx, accountID))
}

type timetableOperationPeopleRows interface {
	timetablePersonRows
	GetByIDs(ctx context.Context, ids []int64) (map[int64]*users.Person, error)
	GetStaffByPersonID(ctx context.Context, personID int64) (*users.Staff, error)
	GetStaffWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*users.Staff, error)
}

// timetableOperationPeople serves the timetable operations' person port.
type timetableOperationPeople struct{ rows timetableOperationPeopleRows }

func newTimetableOperationPeople(rows timetableOperationPeopleRows) timetableOperationPeople {
	return timetableOperationPeople{rows: rows}
}

func (p timetableOperationPeople) FindByAccountID(ctx context.Context, accountID int64) (*timetable.Person, error) {
	return mapRow(timetablePerson)(p.rows.FindByAccountID(ctx, accountID))
}

func (p timetableOperationPeople) GetByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.Person, error) {
	return mapRowsByID(timetablePerson)(p.rows.GetByIDs(ctx, ids))
}

func (p timetableOperationPeople) GetStaffByPersonID(ctx context.Context, personID int64) (*timetable.StaffMember, error) {
	return mapRow(timetableStaffMember)(p.rows.GetStaffByPersonID(ctx, personID))
}

func (p timetableOperationPeople) GetStaffWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.StaffMember, error) {
	return mapRowsByID(timetableStaffMember)(p.rows.GetStaffWithPersonByIDs(ctx, ids))
}

// timetableLifecycleStudents serves the instance lifecycle's student port
// from the roster's People Directory child rows.
type timetableLifecycleStudents struct {
	rows timetableCompose.RosterStudents
}

// NewTimetableLifecycleStudents binds the lifecycle's student port to the
// People Directory child rows.
func NewTimetableLifecycleStudents(rows timetableCompose.RosterStudents) timetableCompose.LifecycleStudents {
	return timetableLifecycleStudents{rows: rows}
}

func (s timetableLifecycleStudents) FindByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.LifecycleStudent, error) {
	return mapRowsByID(timetableLifecycleStudent)(s.rows.FindByIDs(ctx, ids))
}

func (s timetableLifecycleStudents) FindByIDsForUpdate(ctx context.Context, ids []int64) (map[int64]*timetable.LifecycleStudent, error) {
	return mapRowsByID(timetableLifecycleStudent)(s.rows.FindByIDsForUpdate(ctx, ids))
}

func (s timetableLifecycleStudents) FindReadScopeByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.LifecycleStudent, error) {
	return mapRowsByID(timetableLifecycleStudent)(s.rows.FindReadScopeByIDs(ctx, ids))
}

// timetableOperationStudents serves the operational day's student port from
// the roster's People Directory child rows.
type timetableOperationStudents struct {
	rows timetableCompose.RosterStudents
}

// NewTimetableOperationStudents binds the operational day's student port to
// the People Directory child rows.
func NewTimetableOperationStudents(rows timetableCompose.RosterStudents) timetableCompose.OperationStudents {
	return timetableOperationStudents{rows: rows}
}

func (s timetableOperationStudents) FindByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.OperationStudent, error) {
	return mapRowsByID(timetableOperationStudent)(s.rows.FindByIDs(ctx, ids))
}

func timetableStaffMember(row *users.Staff) *timetable.StaffMember {
	if row == nil {
		return nil
	}
	return &timetable.StaffMember{ID: row.ID, PersonID: row.PersonID, IsGuest: row.IsGuest, Person: timetablePerson(row.Person)}
}

func timetablePerson(row *users.Person) *timetable.Person {
	if row == nil {
		return nil
	}
	return &timetable.Person{ID: row.ID, FirstName: row.FirstName, LastName: row.LastName, AccountID: row.AccountID}
}

func timetableLifecycleStudent(row *users.Student) *timetable.LifecycleStudent {
	if row == nil {
		return nil
	}
	return &timetable.LifecycleStudent{ID: row.ID, GroupID: row.GroupID, Alumnus: row.IsAlumnus(), EnrolledUntil: row.EnrolledUntil}
}

func timetableOperationStudent(row *users.Student) *timetable.OperationStudent {
	if row == nil {
		return nil
	}
	return &timetable.OperationStudent{LifecycleStudent: *timetableLifecycleStudent(row), PersonID: row.PersonID, SchoolClass: row.SchoolClass}
}

// mapRow maps the row of a read; an error passes through.
func mapRow[R, V any](convert func(R) *V) func(R, error) (*V, error) {
	return func(row R, err error) (*V, error) {
		if err != nil {
			return nil, err
		}
		return convert(row), nil
	}
}

// mapRowsByID maps the rows of a read keyed by ID; an error passes through.
func mapRowsByID[R, V any](convert func(R) *V) func(map[int64]R, error) (map[int64]*V, error) {
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
