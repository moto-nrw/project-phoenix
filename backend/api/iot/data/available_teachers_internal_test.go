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
	"github.com/moto-nrw/project-phoenix/services/users/userstest"
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

// personServiceWithTeacherRepo embeds the shared PersonServiceMock and
// overrides only the teacher-lookup methods, delegating them to a
// userModels.TeacherRepository so tests can drive teacher-repository errors
// through the PersonService seam.
type personServiceWithTeacherRepo struct {
	*userstest.PersonServiceMock
	teacherRepo userModels.TeacherRepository
}

func (s personServiceWithTeacherRepo) GetTeacherByStaffID(ctx context.Context, staffID int64) (*userModels.Teacher, error) {
	if s.teacherRepo == nil {
		return nil, nil
	}
	return s.teacherRepo.FindByStaffID(ctx, staffID)
}
func (s personServiceWithTeacherRepo) GetTeachersByStaffIDs(ctx context.Context, staffIDs []int64) (map[int64]*userModels.Teacher, error) {
	if s.teacherRepo == nil {
		return nil, nil
	}
	return s.teacherRepo.FindByStaffIDs(ctx, staffIDs)
}
func (s personServiceWithTeacherRepo) ListTeachersWithStaffAndPerson(ctx context.Context) ([]*userModels.Teacher, error) {
	if s.teacherRepo == nil {
		return nil, nil
	}
	return s.teacherRepo.ListAllWithStaffAndPerson(ctx)
}
func (s personServiceWithTeacherRepo) GetTeachersBySpecialization(ctx context.Context, spec string) ([]*userModels.Teacher, error) {
	if s.teacherRepo == nil {
		return nil, nil
	}
	return s.teacherRepo.FindBySpecialization(ctx, spec)
}
func (s personServiceWithTeacherRepo) GetTeacherWithStaffAndPerson(ctx context.Context, id int64) (*userModels.Teacher, error) {
	if s.teacherRepo == nil {
		return nil, nil
	}
	return s.teacherRepo.FindWithStaffAndPerson(ctx, id)
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
			PersonServiceMock: &userstest.PersonServiceMock{},
			teacherRepo: teacherRepositoryStub{
				listAllWithStaffAndPersonFn: func(context.Context) ([]*userModels.Teacher, error) {
					return nil, errors.New("teacher repository unavailable")
				},
			},
		},
	}
	router := chi.NewRouter()
	router.Get("/teachers", resource.getAvailableTeachers)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, requestWithDeviceContext())

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestGetAvailableTeachers_UsesTeacherRoster(t *testing.T) {
	resource := &Resource{
		UsersService: personServiceWithTeacherRepo{
			PersonServiceMock: &userstest.PersonServiceMock{},
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
	router.Get("/teachers", resource.getAvailableTeachers)

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
