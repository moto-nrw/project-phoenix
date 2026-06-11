package data

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/auth/device"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type teacherRepositoryStub struct {
	listAllWithStaffAndPersonFn func(context.Context) ([]*userModels.Teacher, error)
}

func (s teacherRepositoryStub) Create(context.Context, *userModels.Teacher) error { return nil }
func (s teacherRepositoryStub) FindByID(context.Context, interface{}) (*userModels.Teacher, error) {
	return nil, nil
}
func (s teacherRepositoryStub) FindByStaffID(context.Context, int64) (*userModels.Teacher, error) {
	return nil, nil
}
func (s teacherRepositoryStub) FindByStaffIDs(context.Context, []int64) (map[int64]*userModels.Teacher, error) {
	return nil, nil
}
func (s teacherRepositoryStub) FindBySpecialization(context.Context, string) ([]*userModels.Teacher, error) {
	return nil, nil
}
func (s teacherRepositoryStub) Update(context.Context, *userModels.Teacher) error { return nil }
func (s teacherRepositoryStub) Delete(context.Context, interface{}) error         { return nil }
func (s teacherRepositoryStub) List(context.Context, map[string]interface{}) ([]*userModels.Teacher, error) {
	return nil, nil
}
func (s teacherRepositoryStub) ListWithOptions(context.Context, *modelBase.QueryOptions) ([]*userModels.Teacher, error) {
	return nil, nil
}
func (s teacherRepositoryStub) FindByGroupID(context.Context, int64) ([]*userModels.Teacher, error) {
	return nil, nil
}
func (s teacherRepositoryStub) UpdateQualifications(context.Context, int64, string) error { return nil }
func (s teacherRepositoryStub) FindWithStaffAndPerson(context.Context, int64) (*userModels.Teacher, error) {
	return nil, nil
}
func (s teacherRepositoryStub) ListAllWithStaffAndPerson(ctx context.Context) ([]*userModels.Teacher, error) {
	if s.listAllWithStaffAndPersonFn != nil {
		return s.listAllWithStaffAndPersonFn(ctx)
	}
	return nil, nil
}
func (s teacherRepositoryStub) FindWithStaffAndPersonByIDs(context.Context, []int64) ([]*userModels.Teacher, error) {
	return nil, nil
}

type personServiceWithTeacherRepo struct {
	teacherRepo userModels.TeacherRepository
}

func (s personServiceWithTeacherRepo) Get(context.Context, interface{}) (*userModels.Person, error) {
	return nil, nil
}
func (s personServiceWithTeacherRepo) GetByIDs(context.Context, []int64) (map[int64]*userModels.Person, error) {
	return nil, nil
}
func (s personServiceWithTeacherRepo) Create(context.Context, *userModels.Person) error { return nil }
func (s personServiceWithTeacherRepo) Update(context.Context, *userModels.Person) error { return nil }
func (s personServiceWithTeacherRepo) Delete(context.Context, interface{}) error        { return nil }
func (s personServiceWithTeacherRepo) List(context.Context, *modelBase.QueryOptions) ([]*userModels.Person, error) {
	return nil, nil
}
func (s personServiceWithTeacherRepo) FindByTagID(context.Context, string) (*userModels.Person, error) {
	return nil, nil
}
func (s personServiceWithTeacherRepo) FindByAccountID(context.Context, int64) (*userModels.Person, error) {
	return nil, nil
}
func (s personServiceWithTeacherRepo) FindByName(context.Context, string, string) ([]*userModels.Person, error) {
	return nil, nil
}
func (s personServiceWithTeacherRepo) LinkToAccount(context.Context, int64, int64) error { return nil }
func (s personServiceWithTeacherRepo) UnlinkFromAccount(context.Context, int64) error    { return nil }
func (s personServiceWithTeacherRepo) LinkToRFIDCard(context.Context, int64, string) error {
	return nil
}
func (s personServiceWithTeacherRepo) UnlinkFromRFIDCard(context.Context, int64) error { return nil }
func (s personServiceWithTeacherRepo) GetFullProfile(context.Context, int64) (*userModels.Person, error) {
	return nil, nil
}
func (s personServiceWithTeacherRepo) FindByGuardianID(context.Context, int64) ([]*userModels.Person, error) {
	return nil, nil
}
func (s personServiceWithTeacherRepo) StudentRepository() userModels.StudentRepository { return nil }
func (s personServiceWithTeacherRepo) StaffRepository() userModels.StaffRepository     { return nil }
func (s personServiceWithTeacherRepo) TeacherRepository() userModels.TeacherRepository {
	return s.teacherRepo
}
func (s personServiceWithTeacherRepo) ListAvailableRFIDCards(context.Context) ([]*userModels.RFIDCard, error) {
	return nil, nil
}
func (s personServiceWithTeacherRepo) ValidateStaffPIN(context.Context, string) (*userModels.Staff, error) {
	return nil, nil
}
func (s personServiceWithTeacherRepo) ValidateStaffPINForSpecificStaff(context.Context, int64, string) (*userModels.Staff, error) {
	return nil, nil
}
func (s personServiceWithTeacherRepo) GetStudentsByTeacher(context.Context, int64) ([]*userModels.Student, error) {
	return nil, nil
}
func (s personServiceWithTeacherRepo) GetStudentsWithGroupsByTeacher(context.Context, int64) ([]usersSvc.StudentWithGroup, error) {
	return nil, nil
}
func (s personServiceWithTeacherRepo) GetAllStudentsWithGroups(context.Context) ([]usersSvc.StudentWithGroup, error) {
	return nil, nil
}

func requestWithDeviceContext() *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/teachers", nil)
	deviceModel := &iotModels.Device{DeviceID: "dev-1"}
	ctx := context.WithValue(req.Context(), device.CtxDevice, deviceModel)
	return req.WithContext(ctx)
}

func TestGetAvailableTeachers_RendersTeacherRepositoryErrors(t *testing.T) {
	resource := &Resource{
		UsersService: personServiceWithTeacherRepo{
			teacherRepo: teacherRepositoryStub{
				listAllWithStaffAndPersonFn: func(context.Context) ([]*userModels.Teacher, error) {
					return nil, errors.New("teacher repository unavailable")
				},
			},
		},
	}
	router := chi.NewRouter()
	router.Get("/teachers", resource.GetAvailableTeachersHandler())

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, requestWithDeviceContext())

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestGetAvailableTeachers_UsesTeacherRoster(t *testing.T) {
	resource := &Resource{
		UsersService: personServiceWithTeacherRepo{
			teacherRepo: teacherRepositoryStub{
				listAllWithStaffAndPersonFn: func(context.Context) ([]*userModels.Teacher, error) {
					return []*userModels.Teacher{
						{
							StaffID: 11,
							Staff: &userModels.Staff{
								Person: &userModels.Person{
									Model:     modelBase.Model{ID: 21},
									FirstName: "Legacy",
									LastName:  "Teacher",
								},
							},
						},
						{
							StaffID: 12,
						},
					}, nil
				},
			},
		},
	}
	router := chi.NewRouter()
	router.Get("/teachers", resource.GetAvailableTeachersHandler())

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, requestWithDeviceContext())

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "Legacy Teacher")
	assert.NotContains(t, rr.Body.String(), "\"staff_id\":12")
}

var _ userModels.TeacherRepository = teacherRepositoryStub{}
var _ usersSvc.PersonService = personServiceWithTeacherRepo{}

// Stubs for the issue #585 refactor interface additions — unused here.
func (s teacherRepositoryStub) ListActiveCaregivers(context.Context) ([]*userModels.ActiveCaregiver, error) {
	return nil, nil
}

func (s teacherRepositoryStub) FindActiveCaregiverByAccountID(context.Context, int64) (*userModels.ActiveCaregiver, error) {
	return nil, nil
}
