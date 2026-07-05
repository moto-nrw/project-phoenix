package activities

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// int64Ptr returns a pointer to the given int64 value.
func int64Ptr(v int64) *int64 { return &v }

func TestGroupValidate(t *testing.T) {
	tests := []struct {
		name    string
		group   *Group
		wantErr bool
	}{
		{
			name: "Valid group",
			group: &Group{
				Name:            "Test Group",
				MaxParticipants: 10,
				IsOpen:          true,
				CategoryID:      1,
				CreatedBy:       int64Ptr(1),
			},
			wantErr: false,
		},
		{
			name: "Valid group with planned room",
			group: &Group{
				Name:            "Test Group with Room",
				MaxParticipants: 15,
				IsOpen:          false,
				CategoryID:      2,
				CreatedBy:       int64Ptr(1),
				PlannedRoomID:   func() *int64 { id := int64(3); return &id }(),
			},
			wantErr: false,
		},
		{
			name: "Valid group with nil CreatedBy (system-created)",
			group: &Group{
				Name:            "System Group",
				MaxParticipants: 10,
				IsOpen:          true,
				CategoryID:      1,
				CreatedBy:       nil,
			},
			wantErr: false,
		},
		{
			name: "Missing name",
			group: &Group{
				MaxParticipants: 10,
				IsOpen:          true,
				CategoryID:      1,
				CreatedBy:       int64Ptr(1),
			},
			wantErr: true,
		},
		{
			name: "Invalid max participants (zero)",
			group: &Group{
				Name:            "Test Group",
				MaxParticipants: 0,
				IsOpen:          true,
				CategoryID:      1,
				CreatedBy:       int64Ptr(1),
			},
			wantErr: true,
		},
		{
			name: "Invalid max participants (negative)",
			group: &Group{
				Name:            "Test Group",
				MaxParticipants: -5,
				IsOpen:          true,
				CategoryID:      1,
				CreatedBy:       int64Ptr(1),
			},
			wantErr: true,
		},
		{
			name: "Missing category ID",
			group: &Group{
				Name:            "Test Group",
				MaxParticipants: 10,
				IsOpen:          true,
				CreatedBy:       int64Ptr(1),
			},
			wantErr: true,
		},
		{
			name: "Invalid category ID",
			group: &Group{
				Name:            "Test Group",
				MaxParticipants: 10,
				IsOpen:          true,
				CategoryID:      -1,
				CreatedBy:       int64Ptr(1),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.group.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Group.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGroupHasAvailableSpots(t *testing.T) {
	tests := []struct {
		name                   string
		group                  *Group
		currentEnrollmentCount int
		want                   bool
	}{
		{
			name: "Has available spots",
			group: &Group{
				MaxParticipants: 10,
			},
			currentEnrollmentCount: 5,
			want:                   true,
		},
		{
			name: "No available spots (full)",
			group: &Group{
				MaxParticipants: 10,
			},
			currentEnrollmentCount: 10,
			want:                   false,
		},
		{
			name: "No available spots (over capacity)",
			group: &Group{
				MaxParticipants: 10,
			},
			currentEnrollmentCount: 12,
			want:                   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.group.HasAvailableSpots(tt.currentEnrollmentCount); got != tt.want {
				t.Errorf("Group.HasAvailableSpots() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGroup_BeforeAppendModel(t *testing.T) {
	t.Run("handles nil query", func(t *testing.T) {
		group := &Group{Name: "Test", CategoryID: 1, MaxParticipants: 10}
		err := group.BeforeAppendModel(nil)
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})

	t.Run("returns no error for unknown query type", func(t *testing.T) {
		group := &Group{Name: "Test", CategoryID: 1, MaxParticipants: 10}
		err := group.BeforeAppendModel("some string")
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})
}

func TestGroup_GetID(t *testing.T) {
	group := &Group{
		Model:           base.Model{ID: 42},
		Name:            "Test",
		CategoryID:      1,
		MaxParticipants: 10,
	}

	if got, ok := group.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", group.GetID())
	}
}

func TestGroup_GetCreatedAt(t *testing.T) {
	now := time.Now()
	group := &Group{
		Model:           base.Model{CreatedAt: now},
		Name:            "Test",
		CategoryID:      1,
		MaxParticipants: 10,
	}

	if got := group.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestGroup_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	group := &Group{
		Model:           base.Model{UpdatedAt: now},
		Name:            "Test",
		CategoryID:      1,
		MaxParticipants: 10,
	}

	if got := group.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}

// TestGroupValidate_CreatedBy verifies that Validate() does not require CreatedBy.
// CreatedBy is nullable (system-created groups have created_by = NULL).
func TestGroupValidate_CreatedBy(t *testing.T) {
	tests := []struct {
		name    string
		group   *Group
		wantErr bool
	}{
		{
			name: "Valid group with CreatedBy set",
			group: &Group{
				Name:            "Test Group",
				MaxParticipants: 10,
				IsOpen:          true,
				CategoryID:      1,
				CreatedBy:       int64Ptr(42),
			},
			wantErr: false,
		},
		{
			name: "Valid group with nil CreatedBy (system-created)",
			group: &Group{
				Name:            "Test Group",
				MaxParticipants: 10,
				IsOpen:          true,
				CategoryID:      1,
				CreatedBy:       nil,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.group.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Group.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGroup_IsOwnedBy(t *testing.T) {
	tests := []struct {
		name    string
		group   *Group
		staffID int64
		want    bool
	}{
		{
			name: "Staff is owner",
			group: &Group{
				CreatedBy: int64Ptr(42),
			},
			staffID: 42,
			want:    true,
		},
		{
			name: "Staff is not owner",
			group: &Group{
				CreatedBy: int64Ptr(42),
			},
			staffID: 99,
			want:    false,
		},
		{
			name: "Staff ID is zero",
			group: &Group{
				CreatedBy: int64Ptr(42),
			},
			staffID: 0,
			want:    false,
		},
		{
			name: "System-created group (nil CreatedBy) is not owned by any staff",
			group: &Group{
				CreatedBy: nil,
			},
			staffID: 42,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.group.IsOwnedBy(tt.staffID); got != tt.want {
				t.Errorf("Group.IsOwnedBy() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGroup_IsSupervisedBy(t *testing.T) {
	tests := []struct {
		name    string
		group   *Group
		staffID int64
		want    bool
	}{
		{
			name: "Staff is a supervisor",
			group: &Group{
				Supervisors: []*SupervisorPlanned{
					{StaffID: 10},
					{StaffID: 42},
					{StaffID: 30},
				},
			},
			staffID: 42,
			want:    true,
		},
		{
			name: "Staff is not a supervisor",
			group: &Group{
				Supervisors: []*SupervisorPlanned{
					{StaffID: 10},
					{StaffID: 20},
					{StaffID: 30},
				},
			},
			staffID: 42,
			want:    false,
		},
		{
			name: "No supervisors",
			group: &Group{
				Supervisors: []*SupervisorPlanned{},
			},
			staffID: 42,
			want:    false,
		},
		{
			name: "Nil supervisors slice",
			group: &Group{
				Supervisors: nil,
			},
			staffID: 42,
			want:    false,
		},
		{
			name: "Supervisor slice contains nil entry",
			group: &Group{
				Supervisors: []*SupervisorPlanned{
					{StaffID: 10},
					nil,
					{StaffID: 42},
				},
			},
			staffID: 42,
			want:    true,
		},
		{
			name: "Supervisor slice only contains nil entries",
			group: &Group{
				Supervisors: []*SupervisorPlanned{
					nil,
					nil,
				},
			},
			staffID: 42,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.group.IsSupervisedBy(tt.staffID); got != tt.want {
				t.Errorf("Group.IsSupervisedBy() = %v, want %v", got, tt.want)
			}
		})
	}
}
