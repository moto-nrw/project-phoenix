package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStudentValidation(t *testing.T) {
	tests := []struct {
		name        string
		student     *Student
		expectError bool
	}{
		{
			name: "Valid student",
			student: &Student{
				SchoolClass:  "1A",
				NameLG:       "Parent Name",
				ContactLG:    "123-456-7890",
				CustomUserID: 1,
				GroupID:      1,
			},
			expectError: false,
		},
		{
			name: "Missing school class",
			student: &Student{
				SchoolClass:  "",
				NameLG:       "Parent Name",
				ContactLG:    "123-456-7890",
				CustomUserID: 1,
				GroupID:      1,
			},
			expectError: true,
		},
		{
			name: "Missing LG name",
			student: &Student{
				SchoolClass:  "1A",
				NameLG:       "",
				ContactLG:    "123-456-7890",
				CustomUserID: 1,
				GroupID:      1,
			},
			expectError: true,
		},
		{
			name: "Missing LG contact",
			student: &Student{
				SchoolClass:  "1A",
				NameLG:       "Parent Name",
				ContactLG:    "",
				CustomUserID: 1,
				GroupID:      1,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.student.Validate()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestVisitCreation(t *testing.T) {
	now := time.Now()
	visit := &Visit{
		Day:        now,
		StudentID:  1,
		RoomID:     1,
		TimespanID: 1,
	}

	assert.Equal(t, now, visit.Day)
	assert.Equal(t, int64(1), visit.StudentID)
	assert.Equal(t, int64(1), visit.RoomID)
}

func TestFeedbackCreation(t *testing.T) {
	now := time.Now()
	feedback := &Feedback{
		FeedbackValue: "5",
		Day:           now,
		Time:          now,
		StudentID:     1,
		MensaFeedback: true,
	}

	assert.Equal(t, "5", feedback.FeedbackValue)
	assert.Equal(t, now, feedback.Day)
	assert.Equal(t, int64(1), feedback.StudentID)
	assert.True(t, feedback.MensaFeedback)
}
