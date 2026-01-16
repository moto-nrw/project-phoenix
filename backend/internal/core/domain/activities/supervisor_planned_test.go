package activities

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
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

func TestSupervisorPlanned_TableName(t *testing.T) {
	sp := &SupervisorPlanned{}
	if got := sp.TableName(); got != "activities.supervisors" {
		t.Errorf("TableName() = %v, want activities.supervisors", got)
	}
}

func TestSupervisorPlanned_GetID(t *testing.T) {
	sp := &SupervisorPlanned{
		Model:   base.Model{ID: 42},
		StaffID: 1,
		GroupID: 1,
	}

	if got, ok := sp.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", sp.GetID())
	}
}

func TestSupervisorPlanned_GetCreatedAt(t *testing.T) {
	now := time.Now()
	sp := &SupervisorPlanned{
		Model:   base.Model{CreatedAt: now},
		StaffID: 1,
		GroupID: 1,
	}

	if got := sp.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestSupervisorPlanned_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	sp := &SupervisorPlanned{
		Model:   base.Model{UpdatedAt: now},
		StaffID: 1,
		GroupID: 1,
	}

	if got := sp.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}
