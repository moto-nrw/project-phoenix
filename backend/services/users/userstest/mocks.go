// Package userstest provides a shared func-field mock for the
// users.PersonService interface, replacing the per-package full-interface
// stubs that used to be hand-rolled across api/time-tracking, api/timetable,
// and api/iot/data test files. services/users' own internal tests cannot
// import this package (it would create an import cycle), so this mock is
// for external consumers only.
//
// Each method delegates to its Fn field; a nil field returns zero values.
// Tests configure only the methods they exercise:
//
//	svc := &userstest.PersonServiceMock{
//		GetStudentByIDFn: func(ctx context.Context, id int64) (*userModels.Student, error) {
//			return &userModels.Student{}, nil
//		},
//	}
package userstest

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/users"
)

// PersonServiceMock is a func-field test double for users.PersonService.
type PersonServiceMock struct {
	GetFn                            func(ctx context.Context, id interface{}) (*userModels.Person, error)
	GetByIDsFn                       func(ctx context.Context, ids []int64) (map[int64]*userModels.Person, error)
	CreateFn                         func(ctx context.Context, person *userModels.Person) error
	UpdateFn                         func(ctx context.Context, person *userModels.Person) error
	DeleteFn                         func(ctx context.Context, id interface{}) error
	ListFn                           func(ctx context.Context, options *base.QueryOptions) ([]*userModels.Person, error)
	FindByTagIDFn                    func(ctx context.Context, tagID string) (*userModels.Person, error)
	FindByAccountIDFn                func(ctx context.Context, accountID int64) (*userModels.Person, error)
	FindByNameFn                     func(ctx context.Context, firstName, lastName string) ([]*userModels.Person, error)
	LinkToAccountFn                  func(ctx context.Context, personID int64, accountID int64) error
	UnlinkFromAccountFn              func(ctx context.Context, personID int64) error
	LinkToRFIDCardFn                 func(ctx context.Context, personID int64, tagID string) error
	LinkStudentToRFIDCardFn          func(ctx context.Context, studentID int64, tagID string) error
	UnlinkFromRFIDCardFn             func(ctx context.Context, personID int64) error
	GetStaffByIDFn                   func(ctx context.Context, id int64) (*userModels.Staff, error)
	GetStaffByPersonIDFn             func(ctx context.Context, personID int64) (*userModels.Staff, error)
	ResolveStaffIDByAccountIDFn      func(ctx context.Context, accountID int64) (int64, error)
	GetStaffWithPersonFn             func(ctx context.Context, id int64) (*userModels.Staff, error)
	GetStaffWithPersonByIDsFn        func(ctx context.Context, ids []int64) (map[int64]*userModels.Staff, error)
	ListStaffWithPersonFn            func(ctx context.Context) ([]*userModels.Staff, error)
	ListStaffByRolesFn               func(ctx context.Context, roles []string) ([]*userModels.StaffWithRoleInfo, error)
	GetTeacherByStaffIDFn            func(ctx context.Context, staffID int64) (*userModels.Teacher, error)
	GetTeachersByStaffIDsFn          func(ctx context.Context, staffIDs []int64) (map[int64]*userModels.Teacher, error)
	GetTeachersBySpecializationFn    func(ctx context.Context, specialization string) ([]*userModels.Teacher, error)
	GetTeacherWithStaffAndPersonFn   func(ctx context.Context, id int64) (*userModels.Teacher, error)
	ListTeachersWithStaffAndPersonFn func(ctx context.Context) ([]*userModels.Teacher, error)
	GetStudentByIDFn                 func(ctx context.Context, id int64) (*userModels.Student, error)
	GetStudentByIDForUpdateFn        func(ctx context.Context, id int64) (*userModels.Student, error)
	GetStudentByPersonIDFn           func(ctx context.Context, personID int64) (*userModels.Student, error)
	GetStudentsByIDsFn               func(ctx context.Context, ids []int64) (map[int64]*userModels.Student, error)
	GetStudentsByGroupIDFn           func(ctx context.Context, groupID int64) ([]*userModels.Student, error)
	GetStudentsByGroupIDsFn          func(ctx context.Context, groupIDs []int64) ([]*userModels.Student, error)
	GetEligibleGroupStudentsFn       func(ctx context.Context, groupIDs []int64, date, today timezone.Date) ([]*userModels.Student, error)
	CountStudentsByGroupIDsFn        func(ctx context.Context, groupIDs []int64) (map[int64]int, error)
	CreateStaffWithTeacherFn         func(ctx context.Context, input users.CreateStaffInput) (staff *userModels.Staff, teacher *userModels.Teacher, teacherCreationFailed bool, err error)
	UpdateStaffWithTeacherFn         func(ctx context.Context, staff *userModels.Staff, isTeacher bool, specialization, role, qualifications string) (*userModels.Teacher, users.TeacherAction, error)
	UpdatePersonnelNumberFn          func(ctx context.Context, staffID int64, value *string, changedByStaffID int64, note string) (*userModels.Staff, error)
	GetStudentsWithGroupsByTeacherFn func(ctx context.Context, teacherID int64) ([]users.StudentWithGroup, error)
	GetAllStudentsWithGroupsFn       func(ctx context.Context) ([]users.StudentWithGroup, error)

	// Staff Stammdaten (#1423)
	GetStaffStammdatenFn                  func(ctx context.Context, staffID int64) (*users.StaffStammdaten, error)
	UpdateStaffStammdatenPersonFn         func(ctx context.Context, staffID int64, input users.StammdatenPersonInput, changedByStaffID int64, note string) error
	UpdateStaffStammdatenKontaktFn        func(ctx context.Context, staffID int64, input users.StammdatenKontaktInput, changedByStaffID int64, note string) error
	UpdateStaffStammdatenArbeitsvertragFn func(ctx context.Context, staffID int64, input users.StammdatenArbeitsvertragInput, changedByStaffID int64, note string) error
	ReplaceStaffQualificationsFn          func(ctx context.Context, staffID int64, inputs []users.StammdatenQualificationInput, changedByStaffID int64, note string) error
	GetStaffFinancialMaskedFn             func(ctx context.Context, staffID int64, actorAccountID int64, actorRole string) (*users.StaffFinancialMasked, error)
	RevealStaffFinancialFn                func(ctx context.Context, staffID int64, actorAccountID int64, actorRole string) (*users.StaffFinancialPlain, error)
	UpdateStaffFinancialFn                func(ctx context.Context, staffID int64, input users.StammdatenFinancialInput, changedByAccountID int64, note string) error
}

func (m *PersonServiceMock) GetStaffStammdaten(ctx context.Context, staffID int64) (*users.StaffStammdaten, error) {
	if m.GetStaffStammdatenFn != nil {
		return m.GetStaffStammdatenFn(ctx, staffID)
	}
	return nil, nil
}

func (m *PersonServiceMock) UpdateStaffStammdatenPerson(ctx context.Context, staffID int64, input users.StammdatenPersonInput, changedByStaffID int64, note string) error {
	if m.UpdateStaffStammdatenPersonFn != nil {
		return m.UpdateStaffStammdatenPersonFn(ctx, staffID, input, changedByStaffID, note)
	}
	return nil
}

func (m *PersonServiceMock) UpdateStaffStammdatenKontakt(ctx context.Context, staffID int64, input users.StammdatenKontaktInput, changedByStaffID int64, note string) error {
	if m.UpdateStaffStammdatenKontaktFn != nil {
		return m.UpdateStaffStammdatenKontaktFn(ctx, staffID, input, changedByStaffID, note)
	}
	return nil
}

func (m *PersonServiceMock) UpdateStaffStammdatenArbeitsvertrag(ctx context.Context, staffID int64, input users.StammdatenArbeitsvertragInput, changedByStaffID int64, note string) error {
	if m.UpdateStaffStammdatenArbeitsvertragFn != nil {
		return m.UpdateStaffStammdatenArbeitsvertragFn(ctx, staffID, input, changedByStaffID, note)
	}
	return nil
}

func (m *PersonServiceMock) ReplaceStaffQualifications(ctx context.Context, staffID int64, inputs []users.StammdatenQualificationInput, changedByStaffID int64, note string) error {
	if m.ReplaceStaffQualificationsFn != nil {
		return m.ReplaceStaffQualificationsFn(ctx, staffID, inputs, changedByStaffID, note)
	}
	return nil
}

func (m *PersonServiceMock) GetStaffFinancialMasked(ctx context.Context, staffID int64, actorAccountID int64, actorRole string) (*users.StaffFinancialMasked, error) {
	if m.GetStaffFinancialMaskedFn != nil {
		return m.GetStaffFinancialMaskedFn(ctx, staffID, actorAccountID, actorRole)
	}
	return nil, nil
}

func (m *PersonServiceMock) RevealStaffFinancial(ctx context.Context, staffID int64, actorAccountID int64, actorRole string) (*users.StaffFinancialPlain, error) {
	if m.RevealStaffFinancialFn != nil {
		return m.RevealStaffFinancialFn(ctx, staffID, actorAccountID, actorRole)
	}
	return nil, nil
}

func (m *PersonServiceMock) UpdateStaffFinancial(ctx context.Context, staffID int64, input users.StammdatenFinancialInput, changedByAccountID int64, note string) error {
	if m.UpdateStaffFinancialFn != nil {
		return m.UpdateStaffFinancialFn(ctx, staffID, input, changedByAccountID, note)
	}
	return nil
}

var _ users.PersonService = (*PersonServiceMock)(nil)

func (m *PersonServiceMock) Get(ctx context.Context, id interface{}) (*userModels.Person, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, id)
	}
	return nil, nil
}

func (m *PersonServiceMock) GetByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Person, error) {
	if m.GetByIDsFn != nil {
		return m.GetByIDsFn(ctx, ids)
	}
	return nil, nil
}

func (m *PersonServiceMock) Create(ctx context.Context, person *userModels.Person) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, person)
	}
	return nil
}

func (m *PersonServiceMock) Update(ctx context.Context, person *userModels.Person) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, person)
	}
	return nil
}

func (m *PersonServiceMock) Delete(ctx context.Context, id interface{}) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, id)
	}
	return nil
}

func (m *PersonServiceMock) List(ctx context.Context, options *base.QueryOptions) ([]*userModels.Person, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, options)
	}
	return nil, nil
}

func (m *PersonServiceMock) FindByTagID(ctx context.Context, tagID string) (*userModels.Person, error) {
	if m.FindByTagIDFn != nil {
		return m.FindByTagIDFn(ctx, tagID)
	}
	return nil, nil
}

func (m *PersonServiceMock) FindByAccountID(ctx context.Context, accountID int64) (*userModels.Person, error) {
	if m.FindByAccountIDFn != nil {
		return m.FindByAccountIDFn(ctx, accountID)
	}
	return nil, nil
}

func (m *PersonServiceMock) FindByName(ctx context.Context, firstName, lastName string) ([]*userModels.Person, error) {
	if m.FindByNameFn != nil {
		return m.FindByNameFn(ctx, firstName, lastName)
	}
	return nil, nil
}

func (m *PersonServiceMock) LinkToAccount(ctx context.Context, personID int64, accountID int64) error {
	if m.LinkToAccountFn != nil {
		return m.LinkToAccountFn(ctx, personID, accountID)
	}
	return nil
}

func (m *PersonServiceMock) UnlinkFromAccount(ctx context.Context, personID int64) error {
	if m.UnlinkFromAccountFn != nil {
		return m.UnlinkFromAccountFn(ctx, personID)
	}
	return nil
}

func (m *PersonServiceMock) LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error {
	if m.LinkToRFIDCardFn != nil {
		return m.LinkToRFIDCardFn(ctx, personID, tagID)
	}
	return nil
}

func (m *PersonServiceMock) LinkStudentToRFIDCard(ctx context.Context, studentID int64, tagID string) error {
	if m.LinkStudentToRFIDCardFn != nil {
		return m.LinkStudentToRFIDCardFn(ctx, studentID, tagID)
	}
	return nil
}

func (m *PersonServiceMock) UnlinkFromRFIDCard(ctx context.Context, personID int64) error {
	if m.UnlinkFromRFIDCardFn != nil {
		return m.UnlinkFromRFIDCardFn(ctx, personID)
	}
	return nil
}

func (m *PersonServiceMock) GetStaffByID(ctx context.Context, id int64) (*userModels.Staff, error) {
	if m.GetStaffByIDFn != nil {
		return m.GetStaffByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *PersonServiceMock) GetStaffByPersonID(ctx context.Context, personID int64) (*userModels.Staff, error) {
	if m.GetStaffByPersonIDFn != nil {
		return m.GetStaffByPersonIDFn(ctx, personID)
	}
	return nil, nil
}

func (m *PersonServiceMock) ResolveStaffIDByAccountID(ctx context.Context, accountID int64) (int64, error) {
	if m.ResolveStaffIDByAccountIDFn != nil {
		return m.ResolveStaffIDByAccountIDFn(ctx, accountID)
	}
	return 0, nil
}

func (m *PersonServiceMock) GetStaffWithPerson(ctx context.Context, id int64) (*userModels.Staff, error) {
	if m.GetStaffWithPersonFn != nil {
		return m.GetStaffWithPersonFn(ctx, id)
	}
	return nil, nil
}

func (m *PersonServiceMock) GetStaffWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Staff, error) {
	if m.GetStaffWithPersonByIDsFn != nil {
		return m.GetStaffWithPersonByIDsFn(ctx, ids)
	}
	return nil, nil
}

func (m *PersonServiceMock) ListStaffWithPerson(ctx context.Context) ([]*userModels.Staff, error) {
	if m.ListStaffWithPersonFn != nil {
		return m.ListStaffWithPersonFn(ctx)
	}
	return nil, nil
}

func (m *PersonServiceMock) ListStaffByRoles(ctx context.Context, roles []string) ([]*userModels.StaffWithRoleInfo, error) {
	if m.ListStaffByRolesFn != nil {
		return m.ListStaffByRolesFn(ctx, roles)
	}
	return nil, nil
}

func (m *PersonServiceMock) GetTeacherByStaffID(ctx context.Context, staffID int64) (*userModels.Teacher, error) {
	if m.GetTeacherByStaffIDFn != nil {
		return m.GetTeacherByStaffIDFn(ctx, staffID)
	}
	return nil, nil
}

func (m *PersonServiceMock) GetTeachersByStaffIDs(ctx context.Context, staffIDs []int64) (map[int64]*userModels.Teacher, error) {
	if m.GetTeachersByStaffIDsFn != nil {
		return m.GetTeachersByStaffIDsFn(ctx, staffIDs)
	}
	return nil, nil
}

func (m *PersonServiceMock) GetTeachersBySpecialization(ctx context.Context, specialization string) ([]*userModels.Teacher, error) {
	if m.GetTeachersBySpecializationFn != nil {
		return m.GetTeachersBySpecializationFn(ctx, specialization)
	}
	return nil, nil
}

func (m *PersonServiceMock) GetTeacherWithStaffAndPerson(ctx context.Context, id int64) (*userModels.Teacher, error) {
	if m.GetTeacherWithStaffAndPersonFn != nil {
		return m.GetTeacherWithStaffAndPersonFn(ctx, id)
	}
	return nil, nil
}

func (m *PersonServiceMock) ListTeachersWithStaffAndPerson(ctx context.Context) ([]*userModels.Teacher, error) {
	if m.ListTeachersWithStaffAndPersonFn != nil {
		return m.ListTeachersWithStaffAndPersonFn(ctx)
	}
	return nil, nil
}

func (m *PersonServiceMock) GetStudentByID(ctx context.Context, id int64) (*userModels.Student, error) {
	if m.GetStudentByIDFn != nil {
		return m.GetStudentByIDFn(ctx, id)
	}
	return nil, nil
}

// GetStudentByIDForUpdate falls back to GetStudentByIDFn when no dedicated stub
// is set: the locked read differs from the plain one only in the row lock, which
// a mock has nothing to emulate, so a test that stubs the lookup once covers
// both call sites.
func (m *PersonServiceMock) GetStudentByIDForUpdate(ctx context.Context, id int64) (*userModels.Student, error) {
	if m.GetStudentByIDForUpdateFn != nil {
		return m.GetStudentByIDForUpdateFn(ctx, id)
	}
	if m.GetStudentByIDFn != nil {
		return m.GetStudentByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *PersonServiceMock) GetStudentByPersonID(ctx context.Context, personID int64) (*userModels.Student, error) {
	if m.GetStudentByPersonIDFn != nil {
		return m.GetStudentByPersonIDFn(ctx, personID)
	}
	return nil, nil
}

func (m *PersonServiceMock) GetStudentsByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Student, error) {
	if m.GetStudentsByIDsFn != nil {
		return m.GetStudentsByIDsFn(ctx, ids)
	}
	return nil, nil
}

func (m *PersonServiceMock) GetStudentsByGroupID(ctx context.Context, groupID int64) ([]*userModels.Student, error) {
	if m.GetStudentsByGroupIDFn != nil {
		return m.GetStudentsByGroupIDFn(ctx, groupID)
	}
	return nil, nil
}

func (m *PersonServiceMock) GetStudentsByGroupIDs(ctx context.Context, groupIDs []int64) ([]*userModels.Student, error) {
	if m.GetStudentsByGroupIDsFn != nil {
		return m.GetStudentsByGroupIDsFn(ctx, groupIDs)
	}
	return nil, nil
}

func (m *PersonServiceMock) GetEligibleStudentsByGroupIDsOnDate(ctx context.Context, groupIDs []int64, date, today timezone.Date) ([]*userModels.Student, error) {
	if m.GetEligibleGroupStudentsFn != nil {
		return m.GetEligibleGroupStudentsFn(ctx, groupIDs, date, today)
	}
	return nil, nil
}

func (m *PersonServiceMock) CountStudentsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64]int, error) {
	if m.CountStudentsByGroupIDsFn != nil {
		return m.CountStudentsByGroupIDsFn(ctx, groupIDs)
	}
	return nil, nil
}

func (m *PersonServiceMock) CreateStaffWithTeacher(ctx context.Context, input users.CreateStaffInput) (*userModels.Staff, *userModels.Teacher, bool, error) {
	if m.CreateStaffWithTeacherFn != nil {
		return m.CreateStaffWithTeacherFn(ctx, input)
	}
	return nil, nil, false, nil
}

func (m *PersonServiceMock) UpdateStaffWithTeacher(ctx context.Context, staff *userModels.Staff, isTeacher bool, specialization, role, qualifications string) (*userModels.Teacher, users.TeacherAction, error) {
	if m.UpdateStaffWithTeacherFn != nil {
		return m.UpdateStaffWithTeacherFn(ctx, staff, isTeacher, specialization, role, qualifications)
	}
	return nil, users.TeacherActionNone, nil
}

func (m *PersonServiceMock) UpdatePersonnelNumber(ctx context.Context, staffID int64, value *string, changedByStaffID int64, note string) (*userModels.Staff, error) {
	if m.UpdatePersonnelNumberFn != nil {
		return m.UpdatePersonnelNumberFn(ctx, staffID, value, changedByStaffID, note)
	}
	return nil, nil
}

func (m *PersonServiceMock) GetStudentsWithGroupsByTeacher(ctx context.Context, teacherID int64) ([]users.StudentWithGroup, error) {
	if m.GetStudentsWithGroupsByTeacherFn != nil {
		return m.GetStudentsWithGroupsByTeacherFn(ctx, teacherID)
	}
	return nil, nil
}

func (m *PersonServiceMock) GetAllStudentsWithGroups(ctx context.Context) ([]users.StudentWithGroup, error) {
	if m.GetAllStudentsWithGroupsFn != nil {
		return m.GetAllStudentsWithGroupsFn(ctx)
	}
	return nil, nil
}
