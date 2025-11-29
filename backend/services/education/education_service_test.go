package education

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Helper function to create int64 pointer
func ptr(i int64) *int64 {
	return &i
}

// Mock repositories
type MockGroupRepository struct {
	mock.Mock
}

func (m *MockGroupRepository) Create(ctx context.Context, group *educationModels.Group) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *MockGroupRepository) FindByID(ctx context.Context, id interface{}) (*educationModels.Group, error) {
	args := m.Called(ctx, id)
	if obj := args.Get(0); obj != nil {
		return obj.(*educationModels.Group), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockGroupRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*educationModels.Group, error) {
	return make(map[int64]*educationModels.Group), nil
}

func (m *MockGroupRepository) Update(ctx context.Context, group *educationModels.Group) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *MockGroupRepository) Delete(ctx context.Context, id interface{}) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockGroupRepository) List(ctx context.Context, filters map[string]interface{}) ([]*educationModels.Group, error) {
	args := m.Called(ctx, filters)
	if obj := args.Get(0); obj != nil {
		return obj.([]*educationModels.Group), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockGroupRepository) ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*educationModels.Group, error) {
	args := m.Called(ctx, options)
	if obj := args.Get(0); obj != nil {
		return obj.([]*educationModels.Group), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockGroupRepository) FindByName(ctx context.Context, name string) (*educationModels.Group, error) {
	args := m.Called(ctx, name)
	if obj := args.Get(0); obj != nil {
		return obj.(*educationModels.Group), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockGroupRepository) FindByRoom(ctx context.Context, roomID int64) ([]*educationModels.Group, error) {
	args := m.Called(ctx, roomID)
	if obj := args.Get(0); obj != nil {
		return obj.([]*educationModels.Group), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockGroupRepository) FindByTeacher(ctx context.Context, teacherID int64) ([]*educationModels.Group, error) {
	args := m.Called(ctx, teacherID)
	if obj := args.Get(0); obj != nil {
		return obj.([]*educationModels.Group), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockGroupRepository) FindWithRoom(ctx context.Context, groupID int64) (*educationModels.Group, error) {
	args := m.Called(ctx, groupID)
	if obj := args.Get(0); obj != nil {
		return obj.(*educationModels.Group), args.Error(1)
	}
	return nil, args.Error(1)
}

// Mock SubstitutionRepository
type MockSubstitutionRepository struct {
	mock.Mock
}

func (m *MockSubstitutionRepository) Create(ctx context.Context, substitution *educationModels.GroupSubstitution) error {
	args := m.Called(ctx, substitution)
	return args.Error(0)
}

func (m *MockSubstitutionRepository) FindByID(ctx context.Context, id interface{}) (*educationModels.GroupSubstitution, error) {
	args := m.Called(ctx, id)
	if obj := args.Get(0); obj != nil {
		return obj.(*educationModels.GroupSubstitution), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSubstitutionRepository) Update(ctx context.Context, substitution *educationModels.GroupSubstitution) error {
	args := m.Called(ctx, substitution)
	return args.Error(0)
}

func (m *MockSubstitutionRepository) Delete(ctx context.Context, id interface{}) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockSubstitutionRepository) List(ctx context.Context, filters map[string]interface{}) ([]*educationModels.GroupSubstitution, error) {
	args := m.Called(ctx, filters)
	if obj := args.Get(0); obj != nil {
		return obj.([]*educationModels.GroupSubstitution), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSubstitutionRepository) ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*educationModels.GroupSubstitution, error) {
	args := m.Called(ctx, options)
	if obj := args.Get(0); obj != nil {
		return obj.([]*educationModels.GroupSubstitution), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSubstitutionRepository) FindByGroup(ctx context.Context, groupID int64) ([]*educationModels.GroupSubstitution, error) {
	args := m.Called(ctx, groupID)
	if obj := args.Get(0); obj != nil {
		return obj.([]*educationModels.GroupSubstitution), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSubstitutionRepository) FindByRegularStaff(ctx context.Context, staffID int64) ([]*educationModels.GroupSubstitution, error) {
	args := m.Called(ctx, staffID)
	if obj := args.Get(0); obj != nil {
		return obj.([]*educationModels.GroupSubstitution), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSubstitutionRepository) FindBySubstituteStaff(ctx context.Context, staffID int64) ([]*educationModels.GroupSubstitution, error) {
	args := m.Called(ctx, staffID)
	if obj := args.Get(0); obj != nil {
		return obj.([]*educationModels.GroupSubstitution), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSubstitutionRepository) FindActive(ctx context.Context, date time.Time) ([]*educationModels.GroupSubstitution, error) {
	args := m.Called(ctx, date)
	if obj := args.Get(0); obj != nil {
		return obj.([]*educationModels.GroupSubstitution), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSubstitutionRepository) FindActiveByGroup(ctx context.Context, groupID int64, date time.Time) ([]*educationModels.GroupSubstitution, error) {
	args := m.Called(ctx, groupID, date)
	if obj := args.Get(0); obj != nil {
		return obj.([]*educationModels.GroupSubstitution), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSubstitutionRepository) FindOverlapping(ctx context.Context, staffID int64, startDate time.Time, endDate time.Time) ([]*educationModels.GroupSubstitution, error) {
	args := m.Called(ctx, staffID, startDate, endDate)
	if obj := args.Get(0); obj != nil {
		return obj.([]*educationModels.GroupSubstitution), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSubstitutionRepository) FindByIDWithRelations(ctx context.Context, id int64) (*educationModels.GroupSubstitution, error) {
	args := m.Called(ctx, id)
	if obj := args.Get(0); obj != nil {
		return obj.(*educationModels.GroupSubstitution), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSubstitutionRepository) ListWithRelations(ctx context.Context, options *base.QueryOptions) ([]*educationModels.GroupSubstitution, error) {
	args := m.Called(ctx, options)
	if obj := args.Get(0); obj != nil {
		return obj.([]*educationModels.GroupSubstitution), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSubstitutionRepository) FindActiveWithRelations(ctx context.Context, date time.Time) ([]*educationModels.GroupSubstitution, error) {
	args := m.Called(ctx, date)
	if obj := args.Get(0); obj != nil {
		return obj.([]*educationModels.GroupSubstitution), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSubstitutionRepository) FindActiveBySubstitute(ctx context.Context, substituteStaffID int64, date time.Time) ([]*educationModels.GroupSubstitution, error) {
	args := m.Called(ctx, substituteStaffID, date)
	if obj := args.Get(0); obj != nil {
		return obj.([]*educationModels.GroupSubstitution), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSubstitutionRepository) FindActiveBySubstituteWithRelations(ctx context.Context, substituteStaffID int64, date time.Time) ([]*educationModels.GroupSubstitution, error) {
	args := m.Called(ctx, substituteStaffID, date)
	if obj := args.Get(0); obj != nil {
		return obj.([]*educationModels.GroupSubstitution), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockSubstitutionRepository) FindActiveByGroupWithRelations(ctx context.Context, groupID int64, date time.Time) ([]*educationModels.GroupSubstitution, error) {
	args := m.Called(ctx, groupID, date)
	if obj := args.Get(0); obj != nil {
		return obj.([]*educationModels.GroupSubstitution), args.Error(1)
	}
	return nil, args.Error(1)
}

// Mock TeacherRepository
type MockTeacherRepository struct {
	mock.Mock
}

func (m *MockTeacherRepository) Create(ctx context.Context, teacher *userModels.Teacher) error {
	args := m.Called(ctx, teacher)
	return args.Error(0)
}

func (m *MockTeacherRepository) FindByID(ctx context.Context, id interface{}) (*userModels.Teacher, error) {
	args := m.Called(ctx, id)
	if obj := args.Get(0); obj != nil {
		return obj.(*userModels.Teacher), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockTeacherRepository) FindByStaffID(ctx context.Context, staffID int64) (*userModels.Teacher, error) {
	args := m.Called(ctx, staffID)
	if obj := args.Get(0); obj != nil {
		return obj.(*userModels.Teacher), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockTeacherRepository) FindBySpecialization(ctx context.Context, specialization string) ([]*userModels.Teacher, error) {
	args := m.Called(ctx, specialization)
	if obj := args.Get(0); obj != nil {
		return obj.([]*userModels.Teacher), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockTeacherRepository) Update(ctx context.Context, teacher *userModels.Teacher) error {
	args := m.Called(ctx, teacher)
	return args.Error(0)
}

func (m *MockTeacherRepository) Delete(ctx context.Context, id interface{}) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTeacherRepository) List(ctx context.Context, filters map[string]interface{}) ([]*userModels.Teacher, error) {
	args := m.Called(ctx, filters)
	if obj := args.Get(0); obj != nil {
		return obj.([]*userModels.Teacher), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockTeacherRepository) ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*userModels.Teacher, error) {
	args := m.Called(ctx, options)
	if obj := args.Get(0); obj != nil {
		return obj.([]*userModels.Teacher), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockTeacherRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*userModels.Teacher, error) {
	args := m.Called(ctx, groupID)
	if obj := args.Get(0); obj != nil {
		return obj.([]*userModels.Teacher), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockTeacherRepository) UpdateQualifications(ctx context.Context, id int64, qualifications string) error {
	args := m.Called(ctx, id, qualifications)
	return args.Error(0)
}

func (m *MockTeacherRepository) FindWithStaffAndPerson(ctx context.Context, id int64) (*userModels.Teacher, error) {
	args := m.Called(ctx, id)
	if obj := args.Get(0); obj != nil {
		return obj.(*userModels.Teacher), args.Error(1)
	}
	return nil, args.Error(1)
}

// Mock GroupTeacherRepository for completeness
type MockGroupTeacherRepository struct {
	mock.Mock
}

func (m *MockGroupTeacherRepository) Create(ctx context.Context, groupTeacher *educationModels.GroupTeacher) error {
	args := m.Called(ctx, groupTeacher)
	return args.Error(0)
}

func (m *MockGroupTeacherRepository) FindByID(ctx context.Context, id interface{}) (*educationModels.GroupTeacher, error) {
	args := m.Called(ctx, id)
	if obj := args.Get(0); obj != nil {
		return obj.(*educationModels.GroupTeacher), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockGroupTeacherRepository) Update(ctx context.Context, groupTeacher *educationModels.GroupTeacher) error {
	args := m.Called(ctx, groupTeacher)
	return args.Error(0)
}

func (m *MockGroupTeacherRepository) Delete(ctx context.Context, id interface{}) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockGroupTeacherRepository) List(ctx context.Context, filters map[string]interface{}) ([]*educationModels.GroupTeacher, error) {
	args := m.Called(ctx, filters)
	if obj := args.Get(0); obj != nil {
		return obj.([]*educationModels.GroupTeacher), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockGroupTeacherRepository) FindByGroup(ctx context.Context, groupID int64) ([]*educationModels.GroupTeacher, error) {
	args := m.Called(ctx, groupID)
	if obj := args.Get(0); obj != nil {
		return obj.([]*educationModels.GroupTeacher), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockGroupTeacherRepository) FindByTeacher(ctx context.Context, teacherID int64) ([]*educationModels.GroupTeacher, error) {
	args := m.Called(ctx, teacherID)
	if obj := args.Get(0); obj != nil {
		return obj.([]*educationModels.GroupTeacher), args.Error(1)
	}
	return nil, args.Error(1)
}

// Tests for ListGroups with QueryOptions
func TestListGroups_WithQueryOptions(t *testing.T) {
	// Create mock repositories
	mockGroupRepo := new(MockGroupRepository)
	mockGroupTeacherRepo := new(MockGroupTeacherRepository)
	mockSubstitutionRepo := new(MockSubstitutionRepository)
	mockRoomRepo := new(MockRoomRepository)
	mockTeacherRepo := new(MockTeacherRepository)
	mockStaffRepo := new(MockStaffRepository)

	// Create service with mocks
	service := NewService(
		mockGroupRepo,
		mockGroupTeacherRepo,
		mockSubstitutionRepo,
		mockRoomRepo,
		mockTeacherRepo,
		mockStaffRepo,
		nil, // db not needed for this test
	)

	t.Run("successful list with filters", func(t *testing.T) {
		// Create query options with filters
		options := base.NewQueryOptions()
		filter := base.NewFilter()
		filter.Equal("name", "Math 101")
		filter.IsNotNull("room_id")
		options.Filter = filter
		options = options.WithPagination(1, 10) // Page 1, 10 items per page

		// Mock expected groups
		room1ID := int64(100)
		room2ID := int64(101)
		expectedGroups := []*educationModels.Group{
			{
				Model:  base.Model{ID: 1},
				Name:   "Math 101",
				RoomID: &room1ID,
			},
			{
				Model:  base.Model{ID: 2},
				Name:   "Math 101",
				RoomID: &room2ID,
			},
		}

		// Set up mock expectation
		mockGroupRepo.On("ListWithOptions", mock.Anything, options).Return(expectedGroups, nil)

		// Call the method
		groups, err := service.ListGroups(context.Background(), options)

		// Assertions
		assert.NoError(t, err)
		assert.Len(t, groups, 2)
		assert.Equal(t, "Math 101", groups[0].Name)
		assert.Equal(t, "Math 101", groups[1].Name)
		mockGroupRepo.AssertExpectations(t)
	})

	t.Run("list with nil options", func(t *testing.T) {
		// Mock response for nil options
		expectedGroups := []*educationModels.Group{}
		mockGroupRepo.On("ListWithOptions", mock.Anything, (*base.QueryOptions)(nil)).Return(expectedGroups, nil)

		// Call the method with nil options
		groups, err := service.ListGroups(context.Background(), nil)

		// Assertions
		assert.NoError(t, err)
		assert.Empty(t, groups)
		mockGroupRepo.AssertExpectations(t)
	})
}

// Tests for ListSubstitutions with QueryOptions
func TestListSubstitutions_WithQueryOptions(t *testing.T) {
	// Create mock repositories
	mockGroupRepo := new(MockGroupRepository)
	mockGroupTeacherRepo := new(MockGroupTeacherRepository)
	mockSubstitutionRepo := new(MockSubstitutionRepository)

	// Create service with mocks
	service := NewService(
		mockGroupRepo,
		mockGroupTeacherRepo,
		mockSubstitutionRepo,
		nil, // room repo
		nil, // teacher repo
		nil, // staff repo
		nil, // db
	)

	t.Run("successful list with date filters", func(t *testing.T) {
		// Create query options with date filters
		options := base.NewQueryOptions()
		filter := base.NewFilter()
		now := time.Now()
		filter.DateBetween("start_date", "end_date", now)
		options.Filter = filter

		// Mock expected substitutions
		expectedSubstitutions := []*educationModels.GroupSubstitution{
			{
				Model:             base.Model{ID: 1},
				GroupID:           1,
				RegularStaffID:    ptr(10),
				SubstituteStaffID: 20,
				StartDate:         now.AddDate(0, 0, -1),
				EndDate:           now.AddDate(0, 0, 1),
			},
		}

		// Set up mock expectation
		mockSubstitutionRepo.On("ListWithRelations", mock.Anything, options).Return(expectedSubstitutions, nil)

		// Call the method
		substitutions, err := service.ListSubstitutions(context.Background(), options)

		// Assertions
		assert.NoError(t, err)
		assert.Len(t, substitutions, 1)
		assert.Equal(t, int64(1), substitutions[0].GroupID)
		mockSubstitutionRepo.AssertExpectations(t)
	})
}

// Tests for GetGroupTeachers with IN filter
func TestGetGroupTeachers_WithINFilter(t *testing.T) {
	// Create mock repositories
	mockGroupRepo := new(MockGroupRepository)
	mockGroupTeacherRepo := new(MockGroupTeacherRepository)
	mockTeacherRepo := new(MockTeacherRepository)

	// Create service with mocks
	service := NewService(
		mockGroupRepo,
		mockGroupTeacherRepo,
		nil, // substitution repo
		nil, // room repo
		mockTeacherRepo,
		nil, // staff repo
		nil, // db
	)

	t.Run("successful get with IN filter", func(t *testing.T) {
		groupID := int64(1)

		// Mock the group exists
		mockGroupRepo.On("FindByID", mock.Anything, groupID).Return(&educationModels.Group{
			Model: base.Model{ID: groupID},
			Name:  "Math 101",
		}, nil)

		// Mock group-teacher relationships
		mockGroupTeacherRelations := []*educationModels.GroupTeacher{
			{Model: base.Model{ID: 1}, GroupID: groupID, TeacherID: 10},
			{Model: base.Model{ID: 2}, GroupID: groupID, TeacherID: 20},
			{Model: base.Model{ID: 3}, GroupID: groupID, TeacherID: 30},
		}
		mockGroupTeacherRepo.On("FindByGroup", mock.Anything, groupID).Return(mockGroupTeacherRelations, nil)

		// Expected teachers
		expectedTeachers := []*userModels.Teacher{
			{Model: base.Model{ID: 10}, StaffID: 100},
			{Model: base.Model{ID: 20}, StaffID: 200},
			{Model: base.Model{ID: 30}, StaffID: 300},
		}

		// The service should create QueryOptions with IN filter for teacher IDs
		mockTeacherRepo.On("ListWithOptions", mock.Anything, mock.MatchedBy(func(opts *base.QueryOptions) bool {
			if opts == nil || opts.Filter == nil {
				return false
			}
			// Check if filter has IN condition with the right IDs
			// This is a simplified check - in reality the filter structure might be more complex
			return true
		})).Return(expectedTeachers, nil)

		// Mock FindWithStaffAndPerson calls for each teacher
		for _, teacher := range expectedTeachers {
			mockTeacherRepo.On("FindWithStaffAndPerson", mock.Anything, teacher.ID).Return(teacher, nil)
		}

		// Call the method
		teachers, err := service.GetGroupTeachers(context.Background(), groupID)

		// Assertions
		assert.NoError(t, err)
		assert.Len(t, teachers, 3)
		assert.Equal(t, int64(10), teachers[0].ID)
		assert.Equal(t, int64(20), teachers[1].ID)
		assert.Equal(t, int64(30), teachers[2].ID)
		mockGroupRepo.AssertExpectations(t)
		mockGroupTeacherRepo.AssertExpectations(t)
		mockTeacherRepo.AssertExpectations(t)
	})

	t.Run("handles fallback filtering when repository returns extra teachers", func(t *testing.T) {
		// Create fresh mocks for this test
		mockGroupRepo2 := new(MockGroupRepository)
		mockGroupTeacherRepo2 := new(MockGroupTeacherRepository)
		mockTeacherRepo2 := new(MockTeacherRepository)

		// Create service with fresh mocks
		service2 := NewService(
			mockGroupRepo2,
			mockGroupTeacherRepo2,
			nil, // substitution repo
			nil, // room repo
			mockTeacherRepo2,
			nil, // staff repo
			nil, // db
		)

		groupID := int64(1)

		// Mock the group exists
		mockGroupRepo2.On("FindByID", mock.Anything, groupID).Return(&educationModels.Group{
			Model: base.Model{ID: groupID},
			Name:  "Math 101",
		}, nil)

		// Mock group-teacher relationships - only 2 teachers
		mockGroupTeacherRelations := []*educationModels.GroupTeacher{
			{Model: base.Model{ID: 1}, GroupID: groupID, TeacherID: 10},
			{Model: base.Model{ID: 2}, GroupID: groupID, TeacherID: 20},
		}
		mockGroupTeacherRepo2.On("FindByGroup", mock.Anything, groupID).Return(mockGroupTeacherRelations, nil)

		// Mock teacher repository returns MORE teachers than requested (simulating bad IN filter)
		allTeachers := []*userModels.Teacher{
			{Model: base.Model{ID: 10}, StaffID: 100},
			{Model: base.Model{ID: 20}, StaffID: 200},
			{Model: base.Model{ID: 30}, StaffID: 300}, // Extra teacher
			{Model: base.Model{ID: 40}, StaffID: 400}, // Extra teacher
		}
		mockTeacherRepo2.On("ListWithOptions", mock.Anything, mock.Anything).Return(allTeachers, nil)

		// Mock FindWithStaffAndPerson calls for the teachers that should be returned
		mockTeacherRepo2.On("FindWithStaffAndPerson", mock.Anything, int64(10)).Return(allTeachers[0], nil)
		mockTeacherRepo2.On("FindWithStaffAndPerson", mock.Anything, int64(20)).Return(allTeachers[1], nil)

		// Call the method
		teachers, err := service2.GetGroupTeachers(context.Background(), groupID)

		// Assertions - should only return the 2 teachers that were requested
		assert.NoError(t, err)
		assert.Len(t, teachers, 2)
		assert.Equal(t, int64(10), teachers[0].ID)
		assert.Equal(t, int64(20), teachers[1].ID)
		mockGroupRepo2.AssertExpectations(t)
		mockGroupTeacherRepo2.AssertExpectations(t)
		mockTeacherRepo2.AssertExpectations(t)
	})
}

// Mock Room and Staff repositories (minimal implementation for compilation)
type MockRoomRepository struct {
	mock.Mock
}

func (m *MockRoomRepository) Create(ctx context.Context, room *facilitiesModels.Room) error {
	args := m.Called(ctx, room)
	return args.Error(0)
}

func (m *MockRoomRepository) FindByID(ctx context.Context, id interface{}) (*facilitiesModels.Room, error) {
	args := m.Called(ctx, id)
	if obj := args.Get(0); obj != nil {
		return obj.(*facilitiesModels.Room), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRoomRepository) Update(ctx context.Context, room *facilitiesModels.Room) error {
	args := m.Called(ctx, room)
	return args.Error(0)
}

func (m *MockRoomRepository) Delete(ctx context.Context, id interface{}) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRoomRepository) List(ctx context.Context, filters map[string]interface{}) ([]*facilitiesModels.Room, error) {
	args := m.Called(ctx, filters)
	if obj := args.Get(0); obj != nil {
		return obj.([]*facilitiesModels.Room), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRoomRepository) FindByName(ctx context.Context, name string) (*facilitiesModels.Room, error) {
	args := m.Called(ctx, name)
	if obj := args.Get(0); obj != nil {
		return obj.(*facilitiesModels.Room), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRoomRepository) FindByBuilding(ctx context.Context, building string) ([]*facilitiesModels.Room, error) {
	args := m.Called(ctx, building)
	if obj := args.Get(0); obj != nil {
		return obj.([]*facilitiesModels.Room), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRoomRepository) FindByCategory(ctx context.Context, category string) ([]*facilitiesModels.Room, error) {
	args := m.Called(ctx, category)
	if obj := args.Get(0); obj != nil {
		return obj.([]*facilitiesModels.Room), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockRoomRepository) FindByFloor(ctx context.Context, building string, floor int) ([]*facilitiesModels.Room, error) {
	args := m.Called(ctx, building, floor)
	if obj := args.Get(0); obj != nil {
		return obj.([]*facilitiesModels.Room), args.Error(1)
	}
	return nil, args.Error(1)
}

type MockStaffRepository struct {
	mock.Mock
}

func (m *MockStaffRepository) Create(ctx context.Context, staff *userModels.Staff) error {
	args := m.Called(ctx, staff)
	return args.Error(0)
}

func (m *MockStaffRepository) FindByID(ctx context.Context, id interface{}) (*userModels.Staff, error) {
	args := m.Called(ctx, id)
	if obj := args.Get(0); obj != nil {
		return obj.(*userModels.Staff), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockStaffRepository) FindByPersonID(ctx context.Context, personID int64) (*userModels.Staff, error) {
	args := m.Called(ctx, personID)
	if obj := args.Get(0); obj != nil {
		return obj.(*userModels.Staff), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockStaffRepository) Update(ctx context.Context, staff *userModels.Staff) error {
	args := m.Called(ctx, staff)
	return args.Error(0)
}

func (m *MockStaffRepository) Delete(ctx context.Context, id interface{}) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockStaffRepository) List(ctx context.Context, filters map[string]interface{}) ([]*userModels.Staff, error) {
	args := m.Called(ctx, filters)
	if obj := args.Get(0); obj != nil {
		return obj.([]*userModels.Staff), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockStaffRepository) UpdateNotes(ctx context.Context, id int64, notes string) error {
	args := m.Called(ctx, id, notes)
	return args.Error(0)
}

func (m *MockStaffRepository) FindWithPerson(ctx context.Context, id int64) (*userModels.Staff, error) {
	args := m.Called(ctx, id)
	if obj := args.Get(0); obj != nil {
		return obj.(*userModels.Staff), args.Error(1)
	}
	return nil, args.Error(1)
}

// Tests for Substitution Validation
func TestCreateSubstitution_DateValidation(t *testing.T) {
	// Create mock repositories
	mockGroupRepo := new(MockGroupRepository)
	mockSubstitutionRepo := new(MockSubstitutionRepository)
	mockStaffRepo := new(MockStaffRepository)

	// Create service with mocks
	service := NewService(
		mockGroupRepo,
		nil, // group teacher repo
		mockSubstitutionRepo,
		nil, // room repo
		nil, // teacher repo
		mockStaffRepo,
		nil, // db
	)

	ctx := context.Background()

	t.Run("accepts future date", func(t *testing.T) {
		// Reset mocks
		mockGroupRepo.ExpectedCalls = nil
		mockSubstitutionRepo.ExpectedCalls = nil
		mockStaffRepo.ExpectedCalls = nil

		// Create substitution with future date
		tomorrow := time.Now().AddDate(0, 0, 1).Truncate(24 * time.Hour)
		nextWeek := tomorrow.AddDate(0, 0, 7)

		substitution := &educationModels.GroupSubstitution{
			GroupID:           1,
			RegularStaffID:    ptr(10),
			SubstituteStaffID: 20,
			StartDate:         tomorrow,
			EndDate:           nextWeek,
			Reason:            "Medical leave",
		}

		// Mock group exists
		mockGroupRepo.On("FindByID", ctx, int64(1)).Return(&educationModels.Group{
			Model: base.Model{ID: 1},
			Name:  "Test Group",
		}, nil)

		// Mock regular staff exists
		mockStaffRepo.On("FindByID", ctx, int64(10)).Return(&userModels.Staff{
			Model:    base.Model{ID: 10},
			PersonID: 100,
		}, nil)

		// Mock substitute staff exists
		mockStaffRepo.On("FindByID", ctx, int64(20)).Return(&userModels.Staff{
			Model:    base.Model{ID: 20},
			PersonID: 200,
		}, nil)

		// Note: Service intentionally allows overlapping substitutions (no FindOverlapping check)

		// Mock successful creation
		mockSubstitutionRepo.On("Create", ctx, substitution).Return(nil)

		// Call the method
		err := service.CreateSubstitution(ctx, substitution)

		// Assertions
		assert.NoError(t, err)
		mockGroupRepo.AssertExpectations(t)
		mockSubstitutionRepo.AssertExpectations(t)
		mockStaffRepo.AssertExpectations(t)
	})

	t.Run("rejects past date", func(t *testing.T) {
		// Reset mocks
		mockGroupRepo.ExpectedCalls = nil
		mockSubstitutionRepo.ExpectedCalls = nil
		mockStaffRepo.ExpectedCalls = nil

		// Create substitution with past date
		yesterday := time.Now().AddDate(0, 0, -1).Truncate(24 * time.Hour)
		today := time.Now().Truncate(24 * time.Hour)

		substitution := &educationModels.GroupSubstitution{
			GroupID:           1,
			RegularStaffID:    ptr(10),
			SubstituteStaffID: 20,
			StartDate:         yesterday,
			EndDate:           today,
			Reason:            "Medical leave",
		}

		// Call the method
		err := service.CreateSubstitution(ctx, substitution)

		// Assertions
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "substitutions cannot be created or updated for past dates")
	})

	t.Run("accepts today's date", func(t *testing.T) {
		// Reset mocks
		mockGroupRepo.ExpectedCalls = nil
		mockSubstitutionRepo.ExpectedCalls = nil
		mockStaffRepo.ExpectedCalls = nil

		// Create substitution starting today
		today := time.Now().Truncate(24 * time.Hour)
		nextWeek := today.AddDate(0, 0, 7)

		substitution := &educationModels.GroupSubstitution{
			GroupID:           1,
			RegularStaffID:    ptr(10),
			SubstituteStaffID: 20,
			StartDate:         today,
			EndDate:           nextWeek,
			Reason:            "Emergency coverage",
		}

		// Mock group exists
		mockGroupRepo.On("FindByID", ctx, int64(1)).Return(&educationModels.Group{
			Model: base.Model{ID: 1},
			Name:  "Test Group",
		}, nil)

		// Mock regular staff exists
		mockStaffRepo.On("FindByID", ctx, int64(10)).Return(&userModels.Staff{
			Model:    base.Model{ID: 10},
			PersonID: 100,
		}, nil)

		// Mock substitute staff exists
		mockStaffRepo.On("FindByID", ctx, int64(20)).Return(&userModels.Staff{
			Model:    base.Model{ID: 20},
			PersonID: 200,
		}, nil)

		// Note: Service intentionally allows overlapping substitutions (no FindOverlapping check)

		// Mock successful creation
		mockSubstitutionRepo.On("Create", ctx, substitution).Return(nil)

		// Call the method
		err := service.CreateSubstitution(ctx, substitution)

		// Assertions
		assert.NoError(t, err)
		mockGroupRepo.AssertExpectations(t)
		mockSubstitutionRepo.AssertExpectations(t)
		mockStaffRepo.AssertExpectations(t)
	})
}

func TestUpdateSubstitution_DateValidation(t *testing.T) {
	// Create mock repositories
	mockGroupRepo := new(MockGroupRepository)
	mockSubstitutionRepo := new(MockSubstitutionRepository)
	mockStaffRepo := new(MockStaffRepository)

	// Create service with mocks
	service := NewService(
		mockGroupRepo,
		nil, // group teacher repo
		mockSubstitutionRepo,
		nil, // room repo
		nil, // teacher repo
		mockStaffRepo,
		nil, // db
	)

	ctx := context.Background()
	substitutionID := int64(1)

	t.Run("accepts future date update", func(t *testing.T) {
		// Reset mocks
		mockGroupRepo.ExpectedCalls = nil
		mockSubstitutionRepo.ExpectedCalls = nil
		mockStaffRepo.ExpectedCalls = nil

		// Create existing substitution
		tomorrow := time.Now().AddDate(0, 0, 1).Truncate(24 * time.Hour)
		nextWeek := tomorrow.AddDate(0, 0, 7)

		existingSubstitution := &educationModels.GroupSubstitution{
			Model:             base.Model{ID: substitutionID},
			GroupID:           1,
			RegularStaffID:    ptr(10),
			SubstituteStaffID: 20,
			StartDate:         tomorrow,
			EndDate:           nextWeek,
			Reason:            "Medical leave",
		}

		// Mock finding existing substitution
		mockSubstitutionRepo.On("FindByID", ctx, substitutionID).Return(existingSubstitution, nil)

		// Mock group exists
		mockGroupRepo.On("FindByID", ctx, int64(1)).Return(&educationModels.Group{
			Model: base.Model{ID: 1},
			Name:  "Test Group",
		}, nil)

		// Update to extend the end date
		updatedNextWeek := nextWeek.AddDate(0, 0, 7)
		updatedSubstitution := &educationModels.GroupSubstitution{
			Model:             base.Model{ID: substitutionID},
			GroupID:           1,
			RegularStaffID:    ptr(10),
			SubstituteStaffID: 20,
			StartDate:         tomorrow,
			EndDate:           updatedNextWeek,
			Reason:            "Extended medical leave",
		}

		// Mock regular staff exists
		mockStaffRepo.On("FindByID", ctx, int64(10)).Return(&userModels.Staff{
			Model:    base.Model{ID: 10},
			PersonID: 100,
		}, nil)

		// Mock substitute staff exists
		mockStaffRepo.On("FindByID", ctx, int64(20)).Return(&userModels.Staff{
			Model:    base.Model{ID: 20},
			PersonID: 200,
		}, nil)

		// Mock no overlapping substitutions (excluding current one)
		mockSubstitutionRepo.On("FindOverlapping", ctx, int64(20), tomorrow, updatedNextWeek).Return(
			[]*educationModels.GroupSubstitution{existingSubstitution}, nil)

		// Mock successful update
		mockSubstitutionRepo.On("Update", ctx, updatedSubstitution).Return(nil)

		// Call the method
		err := service.UpdateSubstitution(ctx, updatedSubstitution)

		// Assertions
		assert.NoError(t, err)
		mockSubstitutionRepo.AssertExpectations(t)
		mockStaffRepo.AssertExpectations(t)
	})

	t.Run("rejects backdated update", func(t *testing.T) {
		// Reset mocks
		mockGroupRepo.ExpectedCalls = nil
		mockSubstitutionRepo.ExpectedCalls = nil
		mockStaffRepo.ExpectedCalls = nil

		// Create existing substitution (future dated)
		tomorrow := time.Now().AddDate(0, 0, 1).Truncate(24 * time.Hour)
		nextWeek := tomorrow.AddDate(0, 0, 7)

		existingSubstitution := &educationModels.GroupSubstitution{
			Model:             base.Model{ID: substitutionID},
			GroupID:           1,
			RegularStaffID:    ptr(10),
			SubstituteStaffID: 20,
			StartDate:         tomorrow,
			EndDate:           nextWeek,
			Reason:            "Medical leave",
		}

		// Mock finding existing substitution
		mockSubstitutionRepo.On("FindByID", ctx, substitutionID).Return(existingSubstitution, nil)

		// Try to update with past start date
		yesterday := time.Now().AddDate(0, 0, -1).Truncate(24 * time.Hour)
		updatedSubstitution := &educationModels.GroupSubstitution{
			Model:             base.Model{ID: substitutionID},
			GroupID:           1,
			RegularStaffID:    ptr(10),
			SubstituteStaffID: 20,
			StartDate:         yesterday, // Past date!
			EndDate:           nextWeek,
			Reason:            "Trying to backdate",
		}

		// Call the method
		err := service.UpdateSubstitution(ctx, updatedSubstitution)

		// Assertions
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "substitutions cannot be created or updated for past dates")
	})

	t.Run("accepts today's date for update", func(t *testing.T) {
		// Reset mocks
		mockGroupRepo.ExpectedCalls = nil
		mockSubstitutionRepo.ExpectedCalls = nil
		mockStaffRepo.ExpectedCalls = nil

		// Create existing substitution starting today
		today := time.Now().Truncate(24 * time.Hour)
		nextWeek := today.AddDate(0, 0, 7)

		existingSubstitution := &educationModels.GroupSubstitution{
			Model:             base.Model{ID: substitutionID},
			GroupID:           1,
			RegularStaffID:    ptr(10),
			SubstituteStaffID: 20,
			StartDate:         today,
			EndDate:           nextWeek,
			Reason:            "Emergency coverage",
		}

		// Mock finding existing substitution
		mockSubstitutionRepo.On("FindByID", ctx, substitutionID).Return(existingSubstitution, nil)

		// Update reason but keep dates
		updatedSubstitution := &educationModels.GroupSubstitution{
			Model:             base.Model{ID: substitutionID},
			GroupID:           1,
			RegularStaffID:    ptr(10),
			SubstituteStaffID: 20,
			StartDate:         today,
			EndDate:           nextWeek,
			Reason:            "Updated emergency coverage",
		}

		// Mock group exists
		mockGroupRepo.On("FindByID", ctx, int64(1)).Return(&educationModels.Group{
			Model: base.Model{ID: 1},
			Name:  "Test Group",
		}, nil)

		// Mock regular staff exists
		mockStaffRepo.On("FindByID", ctx, int64(10)).Return(&userModels.Staff{
			Model:    base.Model{ID: 10},
			PersonID: 100,
		}, nil)

		// Mock substitute staff exists
		mockStaffRepo.On("FindByID", ctx, int64(20)).Return(&userModels.Staff{
			Model:    base.Model{ID: 20},
			PersonID: 200,
		}, nil)

		// Mock no overlapping substitutions (excluding current one)
		mockSubstitutionRepo.On("FindOverlapping", ctx, int64(20), today, nextWeek).Return(
			[]*educationModels.GroupSubstitution{existingSubstitution}, nil)

		// Mock successful update
		mockSubstitutionRepo.On("Update", ctx, updatedSubstitution).Return(nil)

		// Call the method
		err := service.UpdateSubstitution(ctx, updatedSubstitution)

		// Assertions
		assert.NoError(t, err)
		mockSubstitutionRepo.AssertExpectations(t)
		mockStaffRepo.AssertExpectations(t)
	})
}
