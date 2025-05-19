package feedback

import (
	"testing"
	"time"
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
				Value:     "Great lunch today!",
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
			name: "Invalid student ID",
			entry: Entry{
				Value:     "Great lunch today!",
				Day:       time.Date(2025, 5, 9, 0, 0, 0, 0, time.UTC),
				Time:      time.Date(0, 0, 0, 12, 30, 0, 0, time.UTC),
				StudentID: 0,
			},
			wantErr: true,
		},
		{
			name: "Missing day",
			entry: Entry{
				Value:     "Great lunch today!",
				Day:       time.Time{},
				Time:      time.Date(0, 0, 0, 12, 30, 0, 0, time.UTC),
				StudentID: 1,
			},
			wantErr: true,
		},
		{
			name: "Missing time",
			entry: Entry{
				Value:     "Great lunch today!",
				Day:       time.Date(2025, 5, 9, 0, 0, 0, 0, time.UTC),
				Time:      time.Time{},
				StudentID: 1,
			},
			wantErr: true,
		},
		{
			name: "Value trimming",
			entry: Entry{
				Value:     "  Great lunch today!  ",
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
			if !tt.wantErr && tt.name == "Value trimming" && tt.entry.Value != "Great lunch today!" {
				t.Errorf("Value was not trimmed properly, got %s", tt.entry.Value)
			}
		})
	}
}

func TestEntry_GetTimestamp(t *testing.T) {
	entry := Entry{
		Value:     "Great lunch today!",
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
		Value:           "Great lunch today!",
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
		Value:     "Great lunch today!",
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
