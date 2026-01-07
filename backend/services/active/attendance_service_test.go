package active

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock AttendanceRepository for focused unit testing
type MockAttendanceRepository struct {
	mock.Mock
}

func (m *MockAttendanceRepository) Create(ctx context.Context, attendance *active.Attendance) error {
	args := m.Called(ctx, attendance)
	// Set ID for created record to simulate database behavior
	if args.Error(0) == nil {
		attendance.ID = 1
	}
	return args.Error(0)
}

func (m *MockAttendanceRepository) Update(ctx context.Context, attendance *active.Attendance) error {
	args := m.Called(ctx, attendance)
	return args.Error(0)
}

func (m *MockAttendanceRepository) FindByID(ctx context.Context, id int64) (*active.Attendance, error) {
	args := m.Called(ctx, id)
	if obj := args.Get(0); obj != nil {
		return obj.(*active.Attendance), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAttendanceRepository) FindByStudentAndDate(ctx context.Context, studentID int64, date time.Time) ([]*active.Attendance, error) {
	args := m.Called(ctx, studentID, date)
	if obj := args.Get(0); obj != nil {
		return obj.([]*active.Attendance), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAttendanceRepository) FindLatestByStudent(ctx context.Context, studentID int64) (*active.Attendance, error) {
	args := m.Called(ctx, studentID)
	if obj := args.Get(0); obj != nil {
		return obj.(*active.Attendance), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAttendanceRepository) GetStudentCurrentStatus(ctx context.Context, studentID int64) (*active.Attendance, error) {
	args := m.Called(ctx, studentID)
	if obj := args.Get(0); obj != nil {
		return obj.(*active.Attendance), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAttendanceRepository) GetTodayByStudentID(ctx context.Context, studentID int64) (*active.Attendance, error) {
	args := m.Called(ctx, studentID)
	if obj := args.Get(0); obj != nil {
		return obj.(*active.Attendance), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAttendanceRepository) GetTodayByStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]*active.Attendance, error) {
	args := m.Called(ctx, studentIDs)
	if obj := args.Get(0); obj != nil {
		return obj.(map[int64]*active.Attendance), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAttendanceRepository) FindForDate(ctx context.Context, date time.Time) ([]*active.Attendance, error) {
	args := m.Called(ctx, date)
	if obj := args.Get(0); obj != nil {
		return obj.([]*active.Attendance), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockAttendanceRepository) Delete(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// Test GetStudentAttendanceStatus - Not Checked In
func TestGetStudentAttendanceStatus_NotCheckedIn(t *testing.T) {
	ctx := context.Background()
	mockRepo := &MockAttendanceRepository{}

	// Create service with mock repository
	svc := &service{
		attendanceRepo: mockRepo,
		// Other dependencies are nil, which will cause errors if accessed
		// This test only verifies the "not found" path
	}

	studentID := int64(1)

	// Mock: No attendance record found
	mockRepo.On("GetStudentCurrentStatus", ctx, studentID).Return(nil, fmt.Errorf("not found"))

	// Execute
	result, err := svc.GetStudentAttendanceStatus(ctx, studentID)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, studentID, result.StudentID)
	assert.Equal(t, "not_checked_in", result.Status)
	assert.Equal(t, time.Now().Truncate(24*time.Hour), result.Date.Truncate(24*time.Hour))
	assert.Nil(t, result.CheckInTime)
	assert.Nil(t, result.CheckOutTime)
	assert.Empty(t, result.CheckedInBy)
	assert.Empty(t, result.CheckedOutBy)

	mockRepo.AssertExpectations(t)
}

func TestGetStudentsAttendanceStatuses(t *testing.T) {
	ctx := context.Background()
	mockRepo := &MockAttendanceRepository{}

	svc := &service{
		attendanceRepo: mockRepo,
	}

	studentIDs := []int64{101, 202}
	today := time.Now().UTC().Truncate(24 * time.Hour)

	attendanceRecords := map[int64]*active.Attendance{
		101: {
			StudentID:   101,
			Date:        today,
			CheckInTime: today.Add(-1 * time.Hour),
		},
	}

	mockRepo.On("GetTodayByStudentIDs", ctx, studentIDs).Return(attendanceRecords, nil)

	statuses, err := svc.GetStudentsAttendanceStatuses(ctx, studentIDs)

	require.NoError(t, err)
	require.Len(t, statuses, 2)

	checkedIn := statuses[101]
	if assert.NotNil(t, checkedIn) {
		assert.Equal(t, "checked_in", checkedIn.Status)
		assert.NotNil(t, checkedIn.CheckInTime)
		assert.Nil(t, checkedIn.CheckOutTime)
		assert.Equal(t, today, checkedIn.Date)
	}

	notCheckedIn := statuses[202]
	if assert.NotNil(t, notCheckedIn) {
		assert.Equal(t, "not_checked_in", notCheckedIn.Status)
		assert.Nil(t, notCheckedIn.CheckInTime)
		assert.Nil(t, notCheckedIn.CheckOutTime)
		assert.Equal(t, today, notCheckedIn.Date.Truncate(24*time.Hour))
	}

	mockRepo.AssertExpectations(t)
}

// Test IsCheckedIn helper method on Attendance model
func TestAttendance_IsCheckedIn(t *testing.T) {
	tests := []struct {
		name           string
		checkOutTime   *time.Time
		expectedResult bool
	}{
		{
			name:           "Student is checked in",
			checkOutTime:   nil,
			expectedResult: true,
		},
		{
			name:           "Student is checked out",
			checkOutTime:   func() *time.Time { t := time.Now(); return &t }(),
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attendance := &active.Attendance{
				CheckOutTime: tt.checkOutTime,
			}

			result := attendance.IsCheckedIn()
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

// Test attendance record creation and ID assignment
func TestAttendanceRepository_MockBehavior(t *testing.T) {
	ctx := context.Background()
	mockRepo := &MockAttendanceRepository{}

	attendance := &active.Attendance{
		StudentID:   1,
		Date:        time.Now().Truncate(24 * time.Hour),
		CheckInTime: time.Now(),
		CheckedInBy: 100,
		DeviceID:    300,
	}

	// Mock successful creation
	mockRepo.On("Create", ctx, attendance).Return(nil)

	// Execute
	err := mockRepo.Create(ctx, attendance)

	// Verify
	require.NoError(t, err)
	assert.Equal(t, int64(1), attendance.ID) // Mock sets ID to 1
	mockRepo.AssertExpectations(t)
}

// Test attendance record update
func TestAttendanceRepository_UpdateBehavior(t *testing.T) {
	ctx := context.Background()
	mockRepo := &MockAttendanceRepository{}

	now := time.Now()
	attendance := &active.Attendance{
		StudentID:    1,
		CheckInTime:  now.Add(-2 * time.Hour),
		CheckOutTime: &now,
		CheckedOutBy: func() *int64 { id := int64(101); return &id }(),
	}
	attendance.ID = 1000

	// Mock successful update
	mockRepo.On("Update", ctx, attendance).Return(nil)

	// Execute
	err := mockRepo.Update(ctx, attendance)

	// Verify
	require.NoError(t, err)
	assert.NotNil(t, attendance.CheckOutTime)
	assert.NotNil(t, attendance.CheckedOutBy)
	mockRepo.AssertExpectations(t)
}

// Comprehensive test demonstrating the service layer testing pattern
// This shows how to structure tests for business logic orchestration
func TestAttendanceService_TestingPattern(t *testing.T) {
	t.Run("demonstrates service testing approach", func(t *testing.T) {
		// 1. Create mocks for all dependencies
		mockAttendanceRepo := &MockAttendanceRepository{}
		// In a complete test, you'd create mocks for:
		// - educationService (for permission checks)
		// - usersService (for staff name loading)
		// - staffRepo (for staff lookup)
		// - teacherRepo (for teacher lookup)
		// - studentRepo (for student lookup)

		// 2. Create service with mocked dependencies
		svc := &service{
			attendanceRepo: mockAttendanceRepo,
			// Other dependencies would be mocked here
		}

		// 3. Set up mock expectations for the test scenario
		ctx := context.Background()
		studentID := int64(1)
		mockAttendanceRepo.On("GetStudentCurrentStatus", ctx, studentID).Return(nil, fmt.Errorf("not found"))

		// 4. Execute the business logic
		result, err := svc.GetStudentAttendanceStatus(ctx, studentID)

		// 5. Verify the business logic results
		require.NoError(t, err)
		assert.Equal(t, "not_checked_in", result.Status)

		// 6. Verify all mock expectations were met
		mockAttendanceRepo.AssertExpectations(t)
	})

	t.Log("This test demonstrates the pattern for comprehensive service testing:")
	t.Log("1. Mock all external dependencies (repositories, services)")
	t.Log("2. Set up specific test scenarios with mock expectations")
	t.Log("3. Exercise the business logic methods")
	t.Log("4. Verify both return values and mock interaction patterns")
	t.Log("5. Test permission checks, status determination, and data orchestration")
}
