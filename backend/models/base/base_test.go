package base

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestModel_BeforeAppend(t *testing.T) {
	t.Run("sets both timestamps when CreatedAt is zero", func(t *testing.T) {
		m := &Model{}

		before := time.Now()
		err := m.BeforeAppend()
		after := time.Now()

		if err != nil {
			t.Fatalf("Model.BeforeAppend() unexpected error = %v", err)
		}

		if m.CreatedAt.IsZero() {
			t.Error("Model.BeforeAppend() should set CreatedAt")
		}

		if m.CreatedAt.Before(before) || m.CreatedAt.After(after) {
			t.Errorf("Model.CreatedAt = %v, want between %v and %v", m.CreatedAt, before, after)
		}

		if m.UpdatedAt.IsZero() {
			t.Error("Model.BeforeAppend() should set UpdatedAt")
		}
	})

	t.Run("preserves CreatedAt, updates UpdatedAt", func(t *testing.T) {
		createdTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		m := &Model{
			CreatedAt: createdTime,
		}

		before := time.Now()
		err := m.BeforeAppend()
		after := time.Now()

		if err != nil {
			t.Fatalf("Model.BeforeAppend() unexpected error = %v", err)
		}

		// CreatedAt should be preserved
		if !m.CreatedAt.Equal(createdTime) {
			t.Errorf("Model.CreatedAt = %v, want %v (should be preserved)", m.CreatedAt, createdTime)
		}

		// UpdatedAt should be updated
		if m.UpdatedAt.Before(before) || m.UpdatedAt.After(after) {
			t.Errorf("Model.UpdatedAt = %v, want between %v and %v", m.UpdatedAt, before, after)
		}
	})
}

func TestModel_Fields(t *testing.T) {
	now := time.Now()
	m := Model{
		ID:        42,
		CreatedAt: now,
		UpdatedAt: now.Add(time.Hour),
	}

	if m.ID != 42 {
		t.Errorf("Model.ID = %v, want 42", m.ID)
	}

	if !m.CreatedAt.Equal(now) {
		t.Errorf("Model.CreatedAt = %v, want %v", m.CreatedAt, now)
	}

	if !m.UpdatedAt.Equal(now.Add(time.Hour)) {
		t.Errorf("Model.UpdatedAt = %v, want %v", m.UpdatedAt, now.Add(time.Hour))
	}
}

func TestStringIDModel_Fields(t *testing.T) {
	now := time.Now()
	m := StringIDModel{
		ID:        "RFID12345678",
		CreatedAt: now,
		UpdatedAt: now.Add(time.Hour),
	}

	if m.ID != "RFID12345678" {
		t.Errorf("StringIDModel.ID = %v, want RFID12345678", m.ID)
	}

	if !m.CreatedAt.Equal(now) {
		t.Errorf("StringIDModel.CreatedAt = %v, want %v", m.CreatedAt, now)
	}

	if !m.UpdatedAt.Equal(now.Add(time.Hour)) {
		t.Errorf("StringIDModel.UpdatedAt = %v, want %v", m.UpdatedAt, now.Add(time.Hour))
	}
}

func TestTimeRange_Fields(t *testing.T) {
	start := time.Now()
	end := start.Add(2 * time.Hour)

	t.Run("with end time", func(t *testing.T) {
		tr := TimeRange{
			StartTime: start,
			EndTime:   &end,
		}

		if !tr.StartTime.Equal(start) {
			t.Errorf("TimeRange.StartTime = %v, want %v", tr.StartTime, start)
		}

		if tr.EndTime == nil {
			t.Fatal("TimeRange.EndTime should not be nil")
		}

		if !tr.EndTime.Equal(end) {
			t.Errorf("TimeRange.EndTime = %v, want %v", tr.EndTime, end)
		}
	})

	t.Run("without end time (open-ended)", func(t *testing.T) {
		tr := TimeRange{
			StartTime: start,
			EndTime:   nil,
		}

		if !tr.StartTime.Equal(start) {
			t.Errorf("TimeRange.StartTime = %v, want %v", tr.StartTime, start)
		}

		if tr.EndTime != nil {
			t.Errorf("TimeRange.EndTime = %v, want nil", tr.EndTime)
		}
	})
}

func TestDateRange_Fields(t *testing.T) {
	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)

	dr := DateRange{
		StartDate: start,
		EndDate:   end,
	}

	if !dr.StartDate.Equal(start) {
		t.Errorf("DateRange.StartDate = %v, want %v", dr.StartDate, start)
	}

	if !dr.EndDate.Equal(end) {
		t.Errorf("DateRange.EndDate = %v, want %v", dr.EndDate, end)
	}
}

func TestActivatable_Fields(t *testing.T) {
	t.Run("active", func(t *testing.T) {
		a := Activatable{IsActive: true}
		if !a.IsActive {
			t.Error("Activatable.IsActive should be true")
		}
	})

	t.Run("inactive", func(t *testing.T) {
		a := Activatable{IsActive: false}
		if a.IsActive {
			t.Error("Activatable.IsActive should be false")
		}
	})

	t.Run("default is false", func(t *testing.T) {
		a := Activatable{}
		if a.IsActive {
			t.Error("Activatable.IsActive default should be false")
		}
	})
}

func TestNameable_Fields(t *testing.T) {
	n := Nameable{Name: "Test Name"}

	if n.Name != "Test Name" {
		t.Errorf("Nameable.Name = %q, want Test Name", n.Name)
	}
}

func TestNameableUnique_Fields(t *testing.T) {
	n := NameableUnique{Name: "Unique Name"}

	if n.Name != "Unique Name" {
		t.Errorf("NameableUnique.Name = %q, want Unique Name", n.Name)
	}
}

func TestDescribable_Fields(t *testing.T) {
	d := Describable{Description: "A detailed description"}

	if d.Description != "A detailed description" {
		t.Errorf("Describable.Description = %q, want A detailed description", d.Description)
	}
}

func TestSchemaConstants(t *testing.T) {
	tests := []struct {
		constant string
		expected string
	}{
		{SchemaAuth, "auth"},
		{SchemaUsers, "users"},
		{SchemaEducation, "education"},
		{SchemaSchedule, "schedule"},
		{SchemaActivities, "activities"},
		{SchemaFacilities, "facilities"},
		{SchemaIoT, "iot"},
		{SchemaFeedback, "feedback"},
		{SchemaActive, "active"},
		{SchemaConfig, "config"},
		{SchemaMeta, "meta"},
	}

	for _, tt := range tests {
		if tt.constant != tt.expected {
			t.Errorf("Schema constant = %q, want %q", tt.constant, tt.expected)
		}
	}
}

func TestDatabaseError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *DatabaseError
		expected string
	}{
		{
			name: "with original error",
			err: &DatabaseError{
				Op:  "create",
				Err: errors.New("connection refused"),
			},
			expected: "database error during create: connection refused",
		},
		{
			name: "without original error",
			err: &DatabaseError{
				Op:  "update",
				Err: nil,
			},
			expected: "database error during update",
		},
		{
			name: "with delete operation",
			err: &DatabaseError{
				Op:  "delete",
				Err: errors.New("foreign key constraint"),
			},
			expected: "database error during delete: foreign key constraint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("DatabaseError.Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestDatabaseError_Unwrap(t *testing.T) {
	t.Run("with original error", func(t *testing.T) {
		originalErr := errors.New("original error")
		dbErr := &DatabaseError{
			Op:  "create",
			Err: originalErr,
		}

		if got := dbErr.Unwrap(); got != originalErr {
			t.Errorf("DatabaseError.Unwrap() = %v, want %v", got, originalErr)
		}
	})

	t.Run("without original error", func(t *testing.T) {
		dbErr := &DatabaseError{
			Op:  "create",
			Err: nil,
		}

		if got := dbErr.Unwrap(); got != nil {
			t.Errorf("DatabaseError.Unwrap() = %v, want nil", got)
		}
	})
}

func TestContextWithTx_And_TxFromContext(t *testing.T) {
	t.Run("extract tx from context without tx", func(t *testing.T) {
		ctx := context.Background()

		tx, ok := TxFromContext(ctx)
		if ok {
			t.Error("TxFromContext() should return false when no tx in context")
		}
		if tx != nil {
			t.Errorf("TxFromContext() = %v, want nil", tx)
		}
	})

	// Note: Testing with a real bun.Tx requires a database connection
	// The functions ContextWithTx and TxFromContext are tested together
	// in integration tests that have database access
}
