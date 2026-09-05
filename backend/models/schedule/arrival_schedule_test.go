package schedule

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// StudentArrivalSchedule Validation Tests
// =============================================================================

func TestStudentArrivalSchedule_Validate(t *testing.T) {
	t.Parallel()

	validTime := time.Date(2024, 1, 1, 7, 50, 0, 0, time.UTC)

	tests := []struct {
		name    string
		setup   func() *StudentArrivalSchedule
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid schedule",
			setup: func() *StudentArrivalSchedule {
				return &StudentArrivalSchedule{
					StudentID:       1,
					Weekday:         WeekdayMonday,
					ExpectedArrival: validTime,
					CreatedBy:       1,
					Notes:           nil,
				}
			},
			wantErr: false,
		},
		{
			name: "valid schedule with notes",
			setup: func() *StudentArrivalSchedule {
				notes := "Arrives with older sibling"
				return &StudentArrivalSchedule{
					StudentID:       1,
					Weekday:         WeekdayFriday,
					ExpectedArrival: validTime,
					CreatedBy:       1,
					Notes:           &notes,
				}
			},
			wantErr: false,
		},
		{
			name: "missing student_id",
			setup: func() *StudentArrivalSchedule {
				return &StudentArrivalSchedule{
					StudentID:       0,
					Weekday:         WeekdayMonday,
					ExpectedArrival: validTime,
					CreatedBy:       1,
				}
			},
			wantErr: true,
			errMsg:  "student_id is required",
		},
		{
			name: "negative student_id",
			setup: func() *StudentArrivalSchedule {
				return &StudentArrivalSchedule{
					StudentID:       -1,
					Weekday:         WeekdayMonday,
					ExpectedArrival: validTime,
					CreatedBy:       1,
				}
			},
			wantErr: true,
			errMsg:  "student_id is required",
		},
		{
			name: "weekday too low",
			setup: func() *StudentArrivalSchedule {
				return &StudentArrivalSchedule{
					StudentID:       1,
					Weekday:         0,
					ExpectedArrival: validTime,
					CreatedBy:       1,
				}
			},
			wantErr: true,
			errMsg:  "weekday must be between 1 (Monday) and 5 (Friday)",
		},
		{
			name: "weekday too high (weekend)",
			setup: func() *StudentArrivalSchedule {
				return &StudentArrivalSchedule{
					StudentID:       1,
					Weekday:         6,
					ExpectedArrival: validTime,
					CreatedBy:       1,
				}
			},
			wantErr: true,
			errMsg:  "weekday must be between 1 (Monday) and 5 (Friday)",
		},
		{
			name: "negative weekday",
			setup: func() *StudentArrivalSchedule {
				return &StudentArrivalSchedule{
					StudentID:       1,
					Weekday:         -1,
					ExpectedArrival: validTime,
					CreatedBy:       1,
				}
			},
			wantErr: true,
			errMsg:  "weekday must be between 1 (Monday) and 5 (Friday)",
		},
		{
			// Business rule changed with #2414 / ADR 0005: a row without a
			// time is not incomplete, it is a care day whose time comes from
			// the child's class timetable. A time without a care day is what
			// must not exist, and that is expressed by the absence of a row.
			name: "missing expected_arrival means the class timetable supplies it",
			setup: func() *StudentArrivalSchedule {
				return &StudentArrivalSchedule{
					StudentID: 1,
					Weekday:   WeekdayMonday,
					CreatedBy: 1,
				}
			},
			wantErr: false,
		},
		{
			name: "missing created_by",
			setup: func() *StudentArrivalSchedule {
				return &StudentArrivalSchedule{
					StudentID:       1,
					Weekday:         WeekdayMonday,
					ExpectedArrival: validTime,
					CreatedBy:       0,
				}
			},
			wantErr: true,
			errMsg:  "created_by is required",
		},
		{
			name: "notes too long",
			setup: func() *StudentArrivalSchedule {
				longNotes := string(make([]byte, 501))
				return &StudentArrivalSchedule{
					StudentID:       1,
					Weekday:         WeekdayMonday,
					ExpectedArrival: validTime,
					CreatedBy:       1,
					Notes:           &longNotes,
				}
			},
			wantErr: true,
			errMsg:  "notes cannot exceed 500 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.setup()
			err := s.Validate()

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStudentArrivalSchedule_GetWeekdayName(t *testing.T) {
	t.Parallel()

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
			s := &StudentArrivalSchedule{Weekday: tt.weekday}
			result := s.GetWeekdayName()
			assert.Equal(t, tt.expectedName, result)
		})
	}
}

func TestStudentArrivalSchedule_GetID(t *testing.T) {
	t.Parallel()

	s := &StudentArrivalSchedule{}
	s.ID = 42
	assert.Equal(t, int64(42), s.ID)
}

func TestStudentArrivalSchedule_GetCreatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	s := &StudentArrivalSchedule{}
	s.CreatedAt = now
	assert.Equal(t, now, s.CreatedAt)
}

func TestStudentArrivalSchedule_GetUpdatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	s := &StudentArrivalSchedule{}
	s.UpdatedAt = now
	assert.Equal(t, now, s.UpdatedAt)
}

// =============================================================================
// StudentArrivalException Validation Tests
// =============================================================================

func TestStudentArrivalException_Validate(t *testing.T) {
	t.Parallel()

	validDate := timezone.NewDate(2024, 1, 15)
	validTime := time.Date(2024, 1, 15, 8, 30, 0, 0, time.UTC)

	tests := []struct {
		name    string
		setup   func() *StudentArrivalException
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid exception with arrival time",
			setup: func() *StudentArrivalException {
				return &StudentArrivalException{
					StudentID:       1,
					ExceptionDate:   validDate,
					ExpectedArrival: &validTime,
					Reason:          strPtr("Doctor appointment"),
					CreatedBy:       1,
				}
			},
			wantErr: false,
		},
		{
			name: "valid exception without arrival time (absent)",
			setup: func() *StudentArrivalException {
				return &StudentArrivalException{
					StudentID:       1,
					ExceptionDate:   validDate,
					ExpectedArrival: nil,
					Reason:          strPtr("Student is sick"),
					CreatedBy:       1,
				}
			},
			wantErr: false,
		},
		{
			name: "missing student_id",
			setup: func() *StudentArrivalException {
				return &StudentArrivalException{
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
			setup: func() *StudentArrivalException {
				return &StudentArrivalException{
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
			setup: func() *StudentArrivalException {
				return &StudentArrivalException{
					StudentID: 1,
					Reason:    strPtr("Test reason"),
					CreatedBy: 1,
				}
			},
			wantErr: true,
			errMsg:  "exception_date is required",
		},
		{
			name: "missing reason (nil is valid)",
			setup: func() *StudentArrivalException {
				return &StudentArrivalException{
					StudentID:     1,
					ExceptionDate: validDate,
					Reason:        nil,
					CreatedBy:     1,
				}
			},
			wantErr: false,
		},
		{
			name: "reason too long",
			setup: func() *StudentArrivalException {
				return &StudentArrivalException{
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
			setup: func() *StudentArrivalException {
				return &StudentArrivalException{
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

func TestStudentArrivalException_IsAbsent(t *testing.T) {
	t.Parallel()

	validDate := timezone.NewDate(2024, 1, 15)
	validTime := time.Date(2024, 1, 15, 8, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		setup    func() *StudentArrivalException
		expected bool
	}{
		{
			name: "with arrival time - not absent",
			setup: func() *StudentArrivalException {
				return &StudentArrivalException{
					StudentID:       1,
					ExceptionDate:   validDate,
					ExpectedArrival: &validTime,
					Reason:          strPtr("Late arrival"),
					CreatedBy:       1,
				}
			},
			expected: false,
		},
		{
			name: "without arrival time - absent",
			setup: func() *StudentArrivalException {
				return &StudentArrivalException{
					StudentID:       1,
					ExceptionDate:   validDate,
					ExpectedArrival: nil,
					Reason:          strPtr("Student is sick"),
					CreatedBy:       1,
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

func TestStudentArrivalException_GetID(t *testing.T) {
	t.Parallel()

	exception := &StudentArrivalException{}
	exception.ID = 42
	assert.Equal(t, int64(42), exception.ID)
}

func TestStudentArrivalException_GetCreatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	exception := &StudentArrivalException{}
	exception.CreatedAt = now
	assert.Equal(t, now, exception.CreatedAt)
}

func TestStudentArrivalException_GetUpdatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	exception := &StudentArrivalException{}
	exception.UpdatedAt = now
	assert.Equal(t, now, exception.UpdatedAt)
}

// =============================================================================
// StudentArrivalNote Validation Tests
// =============================================================================

func TestStudentArrivalNote_Validate(t *testing.T) {
	t.Parallel()

	validDate := timezone.NewDate(2024, 1, 15)

	tests := []struct {
		name    string
		setup   func() *StudentArrivalNote
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid note",
			setup: func() *StudentArrivalNote {
				return &StudentArrivalNote{
					StudentID: 1,
					NoteDate:  validDate,
					Content:   "Arrives by bus today",
					CreatedBy: 1,
				}
			},
			wantErr: false,
		},
		{
			name: "missing student_id",
			setup: func() *StudentArrivalNote {
				return &StudentArrivalNote{
					StudentID: 0,
					NoteDate:  validDate,
					Content:   "Test note",
					CreatedBy: 1,
				}
			},
			wantErr: true,
			errMsg:  "student_id is required",
		},
		{
			name: "negative student_id",
			setup: func() *StudentArrivalNote {
				return &StudentArrivalNote{
					StudentID: -1,
					NoteDate:  validDate,
					Content:   "Test note",
					CreatedBy: 1,
				}
			},
			wantErr: true,
			errMsg:  "student_id is required",
		},
		{
			name: "missing note_date",
			setup: func() *StudentArrivalNote {
				return &StudentArrivalNote{
					StudentID: 1,
					Content:   "Test note",
					CreatedBy: 1,
				}
			},
			wantErr: true,
			errMsg:  "note_date is required",
		},
		{
			name: "missing content",
			setup: func() *StudentArrivalNote {
				return &StudentArrivalNote{
					StudentID: 1,
					NoteDate:  validDate,
					Content:   "",
					CreatedBy: 1,
				}
			},
			wantErr: true,
			errMsg:  "content is required",
		},
		{
			name: "content too long",
			setup: func() *StudentArrivalNote {
				return &StudentArrivalNote{
					StudentID: 1,
					NoteDate:  validDate,
					Content:   string(make([]byte, 501)),
					CreatedBy: 1,
				}
			},
			wantErr: true,
			errMsg:  "content cannot exceed 500 characters",
		},
		{
			name: "content exactly 500 characters",
			setup: func() *StudentArrivalNote {
				return &StudentArrivalNote{
					StudentID: 1,
					NoteDate:  validDate,
					Content:   string(make([]byte, 500)),
					CreatedBy: 1,
				}
			},
			wantErr: false,
		},
		{
			name: "missing created_by",
			setup: func() *StudentArrivalNote {
				return &StudentArrivalNote{
					StudentID: 1,
					NoteDate:  validDate,
					Content:   "Test note",
					CreatedBy: 0,
				}
			},
			wantErr: true,
			errMsg:  "created_by is required",
		},
		{
			name: "negative created_by",
			setup: func() *StudentArrivalNote {
				return &StudentArrivalNote{
					StudentID: 1,
					NoteDate:  validDate,
					Content:   "Test note",
					CreatedBy: -1,
				}
			},
			wantErr: true,
			errMsg:  "created_by is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			note := tt.setup()
			err := note.Validate()

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStudentArrivalNote_GetID(t *testing.T) {
	t.Parallel()

	note := &StudentArrivalNote{}
	note.ID = 42
	assert.Equal(t, int64(42), note.ID)
}

func TestStudentArrivalNote_GetCreatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	note := &StudentArrivalNote{}
	note.CreatedAt = now
	assert.Equal(t, now, note.CreatedAt)
}

func TestStudentArrivalNote_GetUpdatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	note := &StudentArrivalNote{}
	note.UpdatedAt = now
	assert.Equal(t, now, note.UpdatedAt)
}

// TestStudentArrivalScheduleInheritsClassTime pins the meaning the #2414 split
// gave the row: it is the care-day marker, the time on it is optional.
func TestStudentArrivalScheduleInheritsClassTime(t *testing.T) {
	t.Parallel()

	careDayOnly := &StudentArrivalSchedule{StudentID: 1, Weekday: WeekdayMonday, CreatedBy: 1}
	require.NoError(t, careDayOnly.Validate())
	assert.True(t, careDayOnly.InheritsClassTime())

	deviating := &StudentArrivalSchedule{
		StudentID:       1,
		Weekday:         WeekdayMonday,
		ExpectedArrival: time.Date(2000, 1, 1, 12, 15, 0, 0, time.UTC),
		CreatedBy:       1,
	}
	require.NoError(t, deviating.Validate())
	assert.False(t, deviating.InheritsClassTime())
}
