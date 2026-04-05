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
)

type caregiverDirectoryStub struct {
	listFn func(context.Context) ([]*userModels.ActiveCaregiver, error)
}

func (s caregiverDirectoryStub) Get(context.Context, interface{}) (*userModels.Person, error) {
	return nil, nil
}
func (s caregiverDirectoryStub) GetByIDs(context.Context, []int64) (map[int64]*userModels.Person, error) {
	return nil, nil
}
func (s caregiverDirectoryStub) Create(context.Context, *userModels.Person) error { return nil }
func (s caregiverDirectoryStub) Update(context.Context, *userModels.Person) error { return nil }
func (s caregiverDirectoryStub) Delete(context.Context, interface{}) error        { return nil }
func (s caregiverDirectoryStub) DeleteStaff(context.Context, int64) error         { return nil }
func (s caregiverDirectoryStub) List(context.Context, *modelBase.QueryOptions) ([]*userModels.Person, error) {
	return nil, nil
}
func (s caregiverDirectoryStub) FindByTagID(context.Context, string) (*userModels.Person, error) {
	return nil, nil
}
func (s caregiverDirectoryStub) FindByAccountID(context.Context, int64) (*userModels.Person, error) {
	return nil, nil
}
func (s caregiverDirectoryStub) FindByName(context.Context, string, string) ([]*userModels.Person, error) {
	return nil, nil
}
func (s caregiverDirectoryStub) LinkToAccount(context.Context, int64, int64) error { return nil }
func (s caregiverDirectoryStub) UnlinkFromAccount(context.Context, int64) error    { return nil }
func (s caregiverDirectoryStub) LinkToRFIDCard(context.Context, int64, string) error {
	return nil
}
func (s caregiverDirectoryStub) UnlinkFromRFIDCard(context.Context, int64) error { return nil }
func (s caregiverDirectoryStub) GetFullProfile(context.Context, int64) (*userModels.Person, error) {
	return nil, nil
}
func (s caregiverDirectoryStub) FindByGuardianID(context.Context, int64) ([]*userModels.Person, error) {
	return nil, nil
}
func (s caregiverDirectoryStub) StudentRepository() userModels.StudentRepository { return nil }
func (s caregiverDirectoryStub) StaffRepository() userModels.StaffRepository     { return nil }
func (s caregiverDirectoryStub) TeacherRepository() userModels.TeacherRepository { return nil }
func (s caregiverDirectoryStub) ListAvailableRFIDCards(context.Context) ([]*userModels.RFIDCard, error) {
	return nil, nil
}
func (s caregiverDirectoryStub) ValidateStaffPIN(context.Context, string) (*userModels.Staff, error) {
	return nil, nil
}
func (s caregiverDirectoryStub) ValidateStaffPINForSpecificStaff(context.Context, int64, string) (*userModels.Staff, error) {
	return nil, nil
}
func (s caregiverDirectoryStub) GetStudentsByTeacher(context.Context, int64) ([]*userModels.Student, error) {
	return nil, nil
}
func (s caregiverDirectoryStub) GetStudentsWithGroupsByTeacher(context.Context, int64) ([]usersSvc.StudentWithGroup, error) {
	return nil, nil
}
func (s caregiverDirectoryStub) GetAllStudentsWithGroups(context.Context) ([]usersSvc.StudentWithGroup, error) {
	return nil, nil
}
func (s caregiverDirectoryStub) ListActiveCaregivers(ctx context.Context) ([]*userModels.ActiveCaregiver, error) {
	if s.listFn != nil {
		return s.listFn(ctx)
	}
	return nil, nil
}
func (s caregiverDirectoryStub) FindActiveCaregiverByAccountID(context.Context, int64) (*userModels.ActiveCaregiver, error) {
	return nil, nil
}

type personServiceWithoutDirectory struct{}

func (personServiceWithoutDirectory) Get(context.Context, interface{}) (*userModels.Person, error) {
	return nil, nil
}
func (personServiceWithoutDirectory) GetByIDs(context.Context, []int64) (map[int64]*userModels.Person, error) {
	return nil, nil
}
func (personServiceWithoutDirectory) Create(context.Context, *userModels.Person) error { return nil }
func (personServiceWithoutDirectory) Update(context.Context, *userModels.Person) error { return nil }
func (personServiceWithoutDirectory) Delete(context.Context, interface{}) error        { return nil }
func (personServiceWithoutDirectory) DeleteStaff(context.Context, int64) error         { return nil }
func (personServiceWithoutDirectory) List(context.Context, *modelBase.QueryOptions) ([]*userModels.Person, error) {
	return nil, nil
}
func (personServiceWithoutDirectory) FindByTagID(context.Context, string) (*userModels.Person, error) {
	return nil, nil
}
func (personServiceWithoutDirectory) FindByAccountID(context.Context, int64) (*userModels.Person, error) {
	return nil, nil
}
func (personServiceWithoutDirectory) FindByName(context.Context, string, string) ([]*userModels.Person, error) {
	return nil, nil
}
func (personServiceWithoutDirectory) LinkToAccount(context.Context, int64, int64) error { return nil }
func (personServiceWithoutDirectory) UnlinkFromAccount(context.Context, int64) error    { return nil }
func (personServiceWithoutDirectory) LinkToRFIDCard(context.Context, int64, string) error {
	return nil
}
func (personServiceWithoutDirectory) UnlinkFromRFIDCard(context.Context, int64) error { return nil }
func (personServiceWithoutDirectory) GetFullProfile(context.Context, int64) (*userModels.Person, error) {
	return nil, nil
}
func (personServiceWithoutDirectory) FindByGuardianID(context.Context, int64) ([]*userModels.Person, error) {
	return nil, nil
}
func (personServiceWithoutDirectory) StudentRepository() userModels.StudentRepository { return nil }
func (personServiceWithoutDirectory) StaffRepository() userModels.StaffRepository     { return nil }
func (personServiceWithoutDirectory) TeacherRepository() userModels.TeacherRepository { return nil }
func (personServiceWithoutDirectory) ListAvailableRFIDCards(context.Context) ([]*userModels.RFIDCard, error) {
	return nil, nil
}
func (personServiceWithoutDirectory) ValidateStaffPIN(context.Context, string) (*userModels.Staff, error) {
	return nil, nil
}
func (personServiceWithoutDirectory) ValidateStaffPINForSpecificStaff(context.Context, int64, string) (*userModels.Staff, error) {
	return nil, nil
}
func (personServiceWithoutDirectory) GetStudentsByTeacher(context.Context, int64) ([]*userModels.Student, error) {
	return nil, nil
}
func (personServiceWithoutDirectory) GetStudentsWithGroupsByTeacher(context.Context, int64) ([]usersSvc.StudentWithGroup, error) {
	return nil, nil
}
func (personServiceWithoutDirectory) GetAllStudentsWithGroups(context.Context) ([]usersSvc.StudentWithGroup, error) {
	return nil, nil
}

func requestWithDeviceContext() *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/teachers", nil)
	deviceModel := &iotModels.Device{DeviceID: "dev-1"}
	ctx := context.WithValue(req.Context(), device.CtxDevice, deviceModel)
	return req.WithContext(ctx)
}

func TestGetAvailableTeachers_RequiresCaregiverDirectory(t *testing.T) {
	resource := &Resource{
		UsersService: personServiceWithoutDirectory{},
	}
	router := chi.NewRouter()
	router.Get("/teachers", resource.GetAvailableTeachersHandler())

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, requestWithDeviceContext())

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestGetAvailableTeachers_RendersDirectoryErrors(t *testing.T) {
	resource := &Resource{
		UsersService: caregiverDirectoryStub{
			listFn: func(context.Context) ([]*userModels.ActiveCaregiver, error) {
				return nil, errors.New("directory unavailable")
			},
		},
	}
	router := chi.NewRouter()
	router.Get("/teachers", resource.GetAvailableTeachersHandler())

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, requestWithDeviceContext())

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

var _ usersSvc.PersonService = caregiverDirectoryStub{}
var _ usersSvc.CaregiverDirectory = caregiverDirectoryStub{}
var _ usersSvc.PersonService = personServiceWithoutDirectory{}
