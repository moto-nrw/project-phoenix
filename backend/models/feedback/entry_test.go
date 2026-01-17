package feedback

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
)

func TestEntry_Validate(t *testing.T) {
	tests := []struct {
		name    string
		entry   Entry
		wantErr bool
	}{
		{
			name: "Valid entry",
			entry: Entry{
				Value:     "positive",
				Day:       time.Date(2025, 5, 9, 0, 0, 0, 0, time.UTC),
				Time:      time.Date(0, 0, 0, 12, 30, 0, 0, time.UTC),
				StudentID: 1,
			},
			wantErr: false,
		},
		{
			name: "Empty value",
			entry: Entry{
				Value:     "",
				Day:       time.Date(2025, 5, 9, 0, 0, 0, 0, time.UTC),
				Time:      time.Date(0, 0, 0, 12, 30, 0, 0, time.UTC),
				StudentID: 1,
			},
			wantErr: true,
		},
		{
			name: "Invalid enum value",
			entry: Entry{
				Value:     "good",
				Day:       time.Date(2025, 5, 9, 0, 0, 0, 0, time.UTC),
				Time:      time.Date(0, 0, 0, 12, 30, 0, 0, time.UTC),
				StudentID: 1,
			},
			wantErr: true,
		},
		{
			name: "Invalid student ID",
			entry: Entry{
				Value:     "positive",
				Day:       time.Date(2025, 5, 9, 0, 0, 0, 0, time.UTC),
				Time:      time.Date(0, 0, 0, 12, 30, 0, 0, time.UTC),
				StudentID: 0,
			},
			wantErr: true,
		},
		{
			name: "Missing day",
			entry: Entry{
				Value:     "neutral",
				Day:       time.Time{},
				Time:      time.Date(0, 0, 0, 12, 30, 0, 0, time.UTC),
				StudentID: 1,
			},
			wantErr: true,
		},
		{
			name: "Missing time",
			entry: Entry{
				Value:     "negative",
				Day:       time.Date(2025, 5, 9, 0, 0, 0, 0, time.UTC),
				Time:      time.Time{},
				StudentID: 1,
			},
			wantErr: true,
		},
		{
			name: "Value trimming",
			entry: Entry{
				Value:     "  positive  ",
				Day:       time.Date(2025, 5, 9, 0, 0, 0, 0, time.UTC),
				Time:      time.Date(0, 0, 0, 12, 30, 0, 0, time.UTC),
				StudentID: 1,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.entry.Validate()

			// Check error condition
			if (err != nil) != tt.wantErr {
				t.Errorf("Entry.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check value trimming
			if !tt.wantErr && tt.name == "Value trimming" && tt.entry.Value != "positive" {
				t.Errorf("Value was not trimmed properly, got %s", tt.entry.Value)
			}
		})
	}
}

func TestEntry_GetTimestamp(t *testing.T) {
	entry := Entry{
		Value:     "positive",
		Day:       time.Date(2025, 5, 9, 0, 0, 0, 0, time.UTC),
		Time:      time.Date(0, 0, 0, 12, 30, 45, 0, time.UTC),
		StudentID: 1,
	}

	expected := time.Date(2025, 5, 9, 12, 30, 45, 0, time.UTC)
	result := entry.GetTimestamp()

	if !result.Equal(expected) {
		t.Errorf("GetTimestamp() = %v, want %v", result, expected)
	}
}

func TestEntry_IsForMensa(t *testing.T) {
	entry := Entry{
		Value:           "positive",
		Day:             time.Date(2025, 5, 9, 0, 0, 0, 0, time.UTC),
		Time:            time.Date(0, 0, 0, 12, 30, 0, 0, time.UTC),
		StudentID:       1,
		IsMensaFeedback: true,
	}

	if !entry.IsForMensa() {
		t.Error("IsForMensa() should return true")
	}

	entry.SetMensaFeedback(false)
	if entry.IsForMensa() {
		t.Error("IsForMensa() should return false after SetMensaFeedback(false)")
	}
}

func TestEntry_FormatMethods(t *testing.T) {
	entry := Entry{
		Value:     "neutral",
		Day:       time.Date(2025, 5, 9, 0, 0, 0, 0, time.UTC),
		Time:      time.Date(0, 0, 0, 12, 30, 45, 0, time.UTC),
		StudentID: 1,
	}

	if entry.GetFormattedDate() != "2025-05-09" {
		t.Errorf("GetFormattedDate() = %s, want 2025-05-09", entry.GetFormattedDate())
	}

	if entry.GetFormattedTime() != "12:30:45" {
		t.Errorf("GetFormattedTime() = %s, want 12:30:45", entry.GetFormattedTime())
	}
}

func TestEntry_SetStudent(t *testing.T) {
	t.Run("set with student", func(t *testing.T) {
		entry := &Entry{
			Value:     "positive",
			Day:       time.Date(2025, 5, 9, 0, 0, 0, 0, time.UTC),
			Time:      time.Date(0, 0, 0, 12, 30, 0, 0, time.UTC),
			StudentID: 0,
		}

		student := &users.Student{
			Model: base.Model{ID: 42},
		}

		entry.SetStudent(student)

		if entry.Student != student {
			t.Error("SetStudent should set the Student field")
		}
		if entry.StudentID != 42 {
			t.Errorf("SetStudent should set StudentID = 42, got %d", entry.StudentID)
		}
	})

	t.Run("set with nil student", func(t *testing.T) {
		entry := &Entry{
			Value:     "positive",
			Day:       time.Date(2025, 5, 9, 0, 0, 0, 0, time.UTC),
			Time:      time.Date(0, 0, 0, 12, 30, 0, 0, time.UTC),
			StudentID: 10,
		}

		entry.SetStudent(nil)

		if entry.Student != nil {
			t.Error("SetStudent(nil) should set Student to nil")
		}
		// StudentID should remain unchanged when setting nil
		if entry.StudentID != 10 {
			t.Errorf("SetStudent(nil) should not change StudentID, got %d", entry.StudentID)
		}
	})
}

func TestEntry_BeforeAppendModel(t *testing.T) {
	t.Run("handles nil query", func(t *testing.T) {
		entry := &Entry{
			Value:     "positive",
			Day:       time.Date(2025, 5, 9, 0, 0, 0, 0, time.UTC),
			Time:      time.Date(0, 0, 0, 12, 30, 0, 0, time.UTC),
			StudentID: 1,
		}
		err := entry.BeforeAppendModel(nil)
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})

	t.Run("returns no error for unknown query type", func(t *testing.T) {
		entry := &Entry{
			Value:     "positive",
			Day:       time.Date(2025, 5, 9, 0, 0, 0, 0, time.UTC),
			Time:      time.Date(0, 0, 0, 12, 30, 0, 0, time.UTC),
			StudentID: 1,
		}
		err := entry.BeforeAppendModel("some string")
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})
}

func TestEntry_TableName(t *testing.T) {
	entry := &Entry{}
	if got := entry.TableName(); got != "feedback.entries" {
		t.Errorf("TableName() = %v, want feedback.entries", got)
	}
}

func TestEntry_GetID(t *testing.T) {
	entry := &Entry{
		Model:     base.Model{ID: 42},
		Value:     "positive",
		Day:       time.Date(2025, 5, 9, 0, 0, 0, 0, time.UTC),
		Time:      time.Date(0, 0, 0, 12, 30, 0, 0, time.UTC),
		StudentID: 1,
	}

	if got, ok := entry.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", entry.GetID())
	}
}

func TestEntry_GetCreatedAt(t *testing.T) {
	now := time.Now()
	entry := &Entry{
		Model:     base.Model{CreatedAt: now},
		Value:     "positive",
		Day:       time.Date(2025, 5, 9, 0, 0, 0, 0, time.UTC),
		Time:      time.Date(0, 0, 0, 12, 30, 0, 0, time.UTC),
		StudentID: 1,
	}

	if got := entry.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestEntry_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	entry := &Entry{
		Model:     base.Model{UpdatedAt: now},
		Value:     "positive",
		Day:       time.Date(2025, 5, 9, 0, 0, 0, 0, time.UTC),
		Time:      time.Date(0, 0, 0, 12, 30, 0, 0, time.UTC),
		StudentID: 1,
	}

	if got := entry.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}
