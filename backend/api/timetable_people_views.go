package api

import (
	"context"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// The adapters below serve the timetable's people ports from the People
// Directory rows (#2729): staff members, persons and the children the
// instance lifecycle checks, in the timetable's own views. Errors pass
// through unchanged, so the timetable keeps classifying a missing row as the
// repository not-found.

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
	FindByAccountID(ctx context.Context, accountID int64) (*peopleValues.DirectoryPersonSnapshot, error)
}

// timetableCorrectionPeople serves the attendance correction's person port.
type timetableCorrectionPeople struct{ rows timetablePersonRows }

func (p timetableCorrectionPeople) FindByAccountID(ctx context.Context, accountID int64) (*timetable.Person, error) {
	return mapRow(timetablePerson)(p.rows.FindByAccountID(ctx, accountID))
}

type timetableOperationPeopleRows interface {
	timetablePersonRows
	GetByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.DirectoryPersonSnapshot, error)
	GetStaffByPersonID(ctx context.Context, personID int64) (*peopleValues.Staff, error)
	GetStaffWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.Staff, error)
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
func NewTimetableLifecycleStudents(rows timetableCompose.RosterStudents) timetablePorts.LifecycleStudents {
	return timetableLifecycleStudents{rows: rows}
}

func (s timetableLifecycleStudents) FindByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.LifecycleStudent, error) {
	return mapRowsByID(timetableRosterLifecycleStudent)(s.rows.FindByIDs(ctx, ids))
}

func (s timetableLifecycleStudents) FindByIDsForUpdate(ctx context.Context, ids []int64) (map[int64]*timetable.LifecycleStudent, error) {
	return mapRowsByID(timetableRosterLifecycleStudent)(s.rows.FindByIDsForUpdate(ctx, ids))
}

func (s timetableLifecycleStudents) FindReadScopeByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.LifecycleStudent, error) {
	return mapRowsByID(timetableRosterLifecycleStudent)(s.rows.FindReadScopeByIDs(ctx, ids))
}

func timetableStaffMember(row *peopleValues.Staff) *timetable.StaffMember {
	if row == nil {
		return nil
	}
	return &timetable.StaffMember{ID: row.ID, PersonID: row.PersonID, IsGuest: row.IsGuest, Person: timetablePerson(row.Person)}
}

func timetablePerson(row *peopleValues.DirectoryPersonSnapshot) *timetable.Person {
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
	return &timetable.OperationStudent{LifecycleStudent: *timetableLifecycleStudent(row), PersonID: row.PersonID, SchoolClass: row.SchoolClass, Person: timetablePerson(row.Person)}
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
	return mapRow(timetableOperationStudent)(s.rows.FindByID(ctx, id))
}
func (s TimetableRosterStudents) FindByIDForUpdate(ctx context.Context, id int64) (*timetable.OperationStudent, error) {
	return mapRow(timetableOperationStudent)(s.rows.FindByIDForUpdate(ctx, id))
}
func (s TimetableRosterStudents) FindByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.OperationStudent, error) {
	return mapRowsByID(timetableOperationStudent)(s.rows.FindByIDs(ctx, ids))
}
func (s TimetableRosterStudents) FindByIDsForUpdate(ctx context.Context, ids []int64) (map[int64]*timetable.OperationStudent, error) {
	return mapRowsByID(timetableOperationStudent)(s.rows.FindByIDsForUpdate(ctx, ids))
}
func (s TimetableRosterStudents) FindReadScopeByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.OperationStudent, error) {
	return mapRowsByID(timetableOperationStudent)(s.rows.FindReadScopeByIDs(ctx, ids))
}

func timetableRosterLifecycleStudent(row *timetable.OperationStudent) *timetable.LifecycleStudent {
	if row == nil {
		return nil
	}
	return &row.LifecycleStudent
}

// TimetableSupervisorDirectory binds the activity transport's native staff reads.
type TimetableSupervisorDirectory struct {
	rows interface {
		GetStaffWithPersonByIDs(context.Context, []int64) (map[int64]*peopleValues.Staff, error)
		ListStaffWithPerson(context.Context) ([]*peopleValues.Staff, error)
	}
	teachers interface {
		GetTeachersBySpecialization(context.Context, string) ([]*peopleValues.Teacher, error)
	}
}

func NewTimetableSupervisorDirectory(rows interface {
	GetStaffWithPersonByIDs(context.Context, []int64) (map[int64]*peopleValues.Staff, error)
	ListStaffWithPerson(context.Context) ([]*peopleValues.Staff, error)
	GetTeachersBySpecialization(context.Context, string) ([]*peopleValues.Teacher, error)
}) TimetableSupervisorDirectory {
	return TimetableSupervisorDirectory{rows: rows, teachers: rows}
}
func (d TimetableSupervisorDirectory) GetTeachersBySpecialization(ctx context.Context, specialization string) ([]*timetable.SupervisorTeacher, error) {
	rows, err := d.teachers.GetTeachersBySpecialization(ctx, specialization)
	if err != nil {
		return nil, err
	}
	result := make([]*timetable.SupervisorTeacher, len(rows))
	for i, row := range rows {
		if row != nil {
			result[i] = &timetable.SupervisorTeacher{StaffID: row.StaffID}
		}
	}
	return result, nil
}
func (d TimetableSupervisorDirectory) GetStaffWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.StaffMember, error) {
	return mapRowsByID(timetableStaffMember)(d.rows.GetStaffWithPersonByIDs(ctx, ids))
}
func (d TimetableSupervisorDirectory) ListStaffWithPerson(ctx context.Context) ([]*timetable.StaffMember, error) {
	rows, err := d.rows.ListStaffWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*timetable.StaffMember, len(rows))
	for i, row := range rows {
		result[i] = timetableStaffMember(row)
	}
	return result, nil
}
