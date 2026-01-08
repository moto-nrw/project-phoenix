package activities

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestSupervisorPlannedValidate(t *testing.T) {
	tests := []struct {
		name              string
		supervisorPlanned *SupervisorPlanned
		wantErr           bool
	}{
		{
			name: "Valid supervisor - non-primary",
			supervisorPlanned: &SupervisorPlanned{
				StaffID:   1,
				GroupID:   1,
				IsPrimary: false,
			},
			wantErr: false,
		},
		{
			name: "Valid supervisor - primary",
			supervisorPlanned: &SupervisorPlanned{
				StaffID:   1,
				GroupID:   1,
				IsPrimary: true,
			},
			wantErr: false,
		},
		{
			name: "Missing staff ID",
			supervisorPlanned: &SupervisorPlanned{
				GroupID:   1,
				IsPrimary: false,
			},
			wantErr: true,
		},
		{
			name: "Invalid staff ID",
			supervisorPlanned: &SupervisorPlanned{
				StaffID:   -1,
				GroupID:   1,
				IsPrimary: false,
			},
			wantErr: true,
		},
		{
			name: "Missing group ID",
			supervisorPlanned: &SupervisorPlanned{
				StaffID:   1,
				IsPrimary: false,
			},
			wantErr: true,
		},
		{
			name: "Invalid group ID",
			supervisorPlanned: &SupervisorPlanned{
				StaffID:   1,
				GroupID:   -1,
				IsPrimary: false,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.supervisorPlanned.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("SupervisorPlanned.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSupervisorPlannedSetPrimary(t *testing.T) {
	supervisorPlanned := &SupervisorPlanned{
		StaffID:   1,
		GroupID:   1,
		IsPrimary: false,
	}

	// Test setting to primary
	supervisorPlanned.SetPrimary()
	if !supervisorPlanned.IsPrimary {
		t.Errorf("SupervisorPlanned.SetPrimary() failed to set IsPrimary to true")
	}
}

func TestSupervisorPlannedSetNotPrimary(t *testing.T) {
	supervisorPlanned := &SupervisorPlanned{
		StaffID:   1,
		GroupID:   1,
		IsPrimary: true,
	}

	// Test setting to not primary
	supervisorPlanned.SetNotPrimary()
	if supervisorPlanned.IsPrimary {
		t.Errorf("SupervisorPlanned.SetNotPrimary() failed to set IsPrimary to false")
	}
}

func TestSupervisorPlannedTableName(t *testing.T) {
	supervisorPlanned := &SupervisorPlanned{}
	expected := "activities.supervisors"

	if got := supervisorPlanned.TableName(); got != expected {
		t.Errorf("SupervisorPlanned.TableName() = %v, want %v", got, expected)
	}
}

func TestSupervisorPlanned_EntityInterface(t *testing.T) {
	now := time.Now()
	supervisor := &SupervisorPlanned{
		Model: base.Model{
			ID:        123,
			CreatedAt: now,
			UpdatedAt: now.Add(time.Hour),
		},
		StaffID: 1,
		GroupID: 1,
	}

	t.Run("GetID", func(t *testing.T) {
		got := supervisor.GetID()
		if got != int64(123) {
			t.Errorf("SupervisorPlanned.GetID() = %v, want %v", got, int64(123))
		}
	})

	t.Run("GetCreatedAt", func(t *testing.T) {
		got := supervisor.GetCreatedAt()
		if !got.Equal(now) {
			t.Errorf("SupervisorPlanned.GetCreatedAt() = %v, want %v", got, now)
		}
	})

	t.Run("GetUpdatedAt", func(t *testing.T) {
		expected := now.Add(time.Hour)
		got := supervisor.GetUpdatedAt()
		if !got.Equal(expected) {
			t.Errorf("SupervisorPlanned.GetUpdatedAt() = %v, want %v", got, expected)
		}
	})
}
