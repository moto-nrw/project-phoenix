package schedule

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// strPtr returns a pointer to the given string
func strPtr(s string) *string {
	return &s
}

// =============================================================================
// StudentPickupSchedule Validation Tests
// =============================================================================

func TestStudentPickupSchedule_Validate(t *testing.T) {
	validTime := time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		setup   func() *StudentPickupSchedule
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid schedule",
			setup: func() *StudentPickupSchedule {
				return &StudentPickupSchedule{
					StudentID:  1,
					Weekday:    WeekdayMonday,
					PickupTime: validTime,
					CreatedBy:  1,
					Notes:      nil,
				}
			},
			wantErr: false,
		},
		{
			name: "valid schedule with notes",
			setup: func() *StudentPickupSchedule {
				notes := "Parent pickup only"
				return &StudentPickupSchedule{
					StudentID:  1,
					Weekday:    WeekdayFriday,
					PickupTime: validTime,
					CreatedBy:  1,
					Notes:      &notes,
				}
			},
			wantErr: false,
		},
		{
			name: "missing student_id",
			setup: func() *StudentPickupSchedule {
				return &StudentPickupSchedule{
					StudentID:  0,
					Weekday:    WeekdayMonday,
					PickupTime: validTime,
					CreatedBy:  1,
				}
			},
			wantErr: true,
			errMsg:  "student_id is required",
		},
		{
			name: "negative student_id",
			setup: func() *StudentPickupSchedule {
				return &StudentPickupSchedule{
					StudentID:  -1,
					Weekday:    WeekdayMonday,
					PickupTime: validTime,
					CreatedBy:  1,
				}
			},
			wantErr: true,
			errMsg:  "student_id is required",
		},
		{
			name: "weekday too low",
			setup: func() *StudentPickupSchedule {
				return &StudentPickupSchedule{
					StudentID:  1,
					Weekday:    0,
					PickupTime: validTime,
					CreatedBy:  1,
				}
			},
			wantErr: true,
			errMsg:  "weekday must be between 1 (Monday) and 5 (Friday)",
		},
		{
			name: "weekday too high (weekend)",
			setup: func() *StudentPickupSchedule {
				return &StudentPickupSchedule{
					StudentID:  1,
					Weekday:    6,
					PickupTime: validTime,
					CreatedBy:  1,
				}
			},
			wantErr: true,
			errMsg:  "weekday must be between 1 (Monday) and 5 (Friday)",
		},
		{
			name: "missing pickup_time",
			setup: func() *StudentPickupSchedule {
				return &StudentPickupSchedule{
					StudentID: 1,
					Weekday:   WeekdayMonday,
					CreatedBy: 1,
				}
			},
			wantErr: true,
			errMsg:  "pickup_time is required",
		},
		{
			name: "missing created_by",
			setup: func() *StudentPickupSchedule {
				return &StudentPickupSchedule{
					StudentID:  1,
					Weekday:    WeekdayMonday,
					PickupTime: validTime,
					CreatedBy:  0,
				}
			},
			wantErr: true,
			errMsg:  "created_by is required",
		},
		{
			name: "notes too long",
			setup: func() *StudentPickupSchedule {
				longNotes := string(make([]byte, 501))
				return &StudentPickupSchedule{
					StudentID:  1,
					Weekday:    WeekdayMonday,
					PickupTime: validTime,
					CreatedBy:  1,
					Notes:      &longNotes,
				}
			},
			wantErr: true,
			errMsg:  "notes cannot exceed 500 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedule := tt.setup()
			err := schedule.Validate()

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStudentPickupSchedule_GetWeekdayName(t *testing.T) {
	tests := []struct {
		name         string
		weekday      int
		expectedName string
	}{
		{"Monday", WeekdayMonday, "Montag"},
		{"Tuesday", WeekdayTuesday, "Dienstag"},
		{"Wednesday", WeekdayWednesday, "Mittwoch"},
		{"Thursday", WeekdayThursday, "Donnerstag"},
		{"Friday", WeekdayFriday, "Freitag"},
		{"Saturday", WeekdaySaturday, "Samstag"},
		{"Sunday", WeekdaySunday, "Sonntag"},
		{"Invalid weekday", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schedule := &StudentPickupSchedule{Weekday: tt.weekday}
			result := schedule.GetWeekdayName()
			assert.Equal(t, tt.expectedName, result)
		})
	}
}

func TestStudentPickupSchedule_TableName(t *testing.T) {
	schedule := &StudentPickupSchedule{}
	assert.Equal(t, "schedule.student_pickup_schedules", schedule.TableName())
}

func TestStudentPickupSchedule_GetID(t *testing.T) {
	schedule := &StudentPickupSchedule{}
	schedule.ID = 42
	assert.Equal(t, int64(42), schedule.GetID())
}

func TestStudentPickupSchedule_GetCreatedAt(t *testing.T) {
	now := time.Now()
	schedule := &StudentPickupSchedule{}
	schedule.CreatedAt = now
	assert.Equal(t, now, schedule.GetCreatedAt())
}

func TestStudentPickupSchedule_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	schedule := &StudentPickupSchedule{}
	schedule.UpdatedAt = now
	assert.Equal(t, now, schedule.GetUpdatedAt())
}

// =============================================================================
// StudentPickupException Validation Tests
// =============================================================================

func TestStudentPickupException_Validate(t *testing.T) {
	validDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	validTime := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		setup   func() *StudentPickupException
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid exception with pickup time",
			setup: func() *StudentPickupException {
				return &StudentPickupException{
					StudentID:     1,
					ExceptionDate: validDate,
					PickupTime:    &validTime,
					Reason:        strPtr("Doctor appointment"),
					CreatedBy:     1,
				}
			},
			wantErr: false,
		},
		{
			name: "valid exception without pickup time (absent)",
			setup: func() *StudentPickupException {
				return &StudentPickupException{
					StudentID:     1,
					ExceptionDate: validDate,
					PickupTime:    nil,
					Reason:        strPtr("Student is sick"),
					CreatedBy:     1,
				}
			},
			wantErr: false,
		},
		{
			name: "missing student_id",
			setup: func() *StudentPickupException {
				return &StudentPickupException{
					StudentID:     0,
					ExceptionDate: validDate,
					Reason:        strPtr("Test reason"),
					CreatedBy:     1,
				}
			},
			wantErr: true,
			errMsg:  "student_id is required",
		},
		{
			name: "negative student_id",
			setup: func() *StudentPickupException {
				return &StudentPickupException{
					StudentID:     -1,
					ExceptionDate: validDate,
					Reason:        strPtr("Test reason"),
					CreatedBy:     1,
				}
			},
			wantErr: true,
			errMsg:  "student_id is required",
		},
		{
			name: "missing exception_date",
			setup: func() *StudentPickupException {
				return &StudentPickupException{
					StudentID: 1,
					Reason:    strPtr("Test reason"),
					CreatedBy: 1,
				}
			},
			wantErr: true,
			errMsg:  "exception_date is required",
		},
		{
			name: "missing reason",
			setup: func() *StudentPickupException {
				return &StudentPickupException{
					StudentID:     1,
					ExceptionDate: validDate,
					Reason:        nil,
					CreatedBy:     1,
				}
			},
			wantErr: false, // Reason is optional (nullable field)
		},
		{
			name: "reason too long",
			setup: func() *StudentPickupException {
				return &StudentPickupException{
					StudentID:     1,
					ExceptionDate: validDate,
					Reason:        strPtr(string(make([]byte, 256))),
					CreatedBy:     1,
				}
			},
			wantErr: true,
			errMsg:  "reason cannot exceed 255 characters",
		},
		{
			name: "missing created_by",
			setup: func() *StudentPickupException {
				return &StudentPickupException{
					StudentID:     1,
					ExceptionDate: validDate,
					Reason:        strPtr("Test reason"),
					CreatedBy:     0,
				}
			},
			wantErr: true,
			errMsg:  "created_by is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exception := tt.setup()
			err := exception.Validate()

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStudentPickupException_IsAbsent(t *testing.T) {
	validDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	validTime := time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		setup    func() *StudentPickupException
		expected bool
	}{
		{
			name: "with pickup time - not absent",
			setup: func() *StudentPickupException {
				return &StudentPickupException{
					StudentID:     1,
					ExceptionDate: validDate,
					PickupTime:    &validTime,
					Reason:        strPtr("Early pickup"),
					CreatedBy:     1,
				}
			},
			expected: false,
		},
		{
			name: "without pickup time - absent",
			setup: func() *StudentPickupException {
				return &StudentPickupException{
					StudentID:     1,
					ExceptionDate: validDate,
					PickupTime:    nil,
					Reason:        strPtr("Student is sick"),
					CreatedBy:     1,
				}
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exception := tt.setup()
			result := exception.IsAbsent()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestStudentPickupException_TableName(t *testing.T) {
	exception := &StudentPickupException{}
	assert.Equal(t, "schedule.student_pickup_exceptions", exception.TableName())
}

func TestStudentPickupException_GetID(t *testing.T) {
	exception := &StudentPickupException{}
	exception.ID = 42
	assert.Equal(t, int64(42), exception.GetID())
}

func TestStudentPickupException_GetCreatedAt(t *testing.T) {
	now := time.Now()
	exception := &StudentPickupException{}
	exception.CreatedAt = now
	assert.Equal(t, now, exception.GetCreatedAt())
}

func TestStudentPickupException_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	exception := &StudentPickupException{}
	exception.UpdatedAt = now
	assert.Equal(t, now, exception.GetUpdatedAt())
}

// =============================================================================
// Weekday Constants and Names Tests
// =============================================================================

func TestWeekdayConstants(t *testing.T) {
	assert.Equal(t, 1, WeekdayMonday)
	assert.Equal(t, 2, WeekdayTuesday)
	assert.Equal(t, 3, WeekdayWednesday)
	assert.Equal(t, 4, WeekdayThursday)
	assert.Equal(t, 5, WeekdayFriday)
	assert.Equal(t, 6, WeekdaySaturday)
	assert.Equal(t, 7, WeekdaySunday)
}

func TestWeekdayNames(t *testing.T) {
	assert.Equal(t, "Montag", WeekdayNames[WeekdayMonday])
	assert.Equal(t, "Dienstag", WeekdayNames[WeekdayTuesday])
	assert.Equal(t, "Mittwoch", WeekdayNames[WeekdayWednesday])
	assert.Equal(t, "Donnerstag", WeekdayNames[WeekdayThursday])
	assert.Equal(t, "Freitag", WeekdayNames[WeekdayFriday])
	assert.Equal(t, "Samstag", WeekdayNames[WeekdaySaturday])
	assert.Equal(t, "Sonntag", WeekdayNames[WeekdaySunday])
}
