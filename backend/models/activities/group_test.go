package activities

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

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
				PlannedRoomID:   func() *int64 { id := int64(3); return &id }(),
			},
			wantErr: false,
		},
		{
			name: "Missing name",
			group: &Group{
				MaxParticipants: 10,
				IsOpen:          true,
				CategoryID:      1,
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
			},
			wantErr: true,
		},
		{
			name: "Missing category ID",
			group: &Group{
				Name:            "Test Group",
				MaxParticipants: 10,
				IsOpen:          true,
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

func TestGroupCanJoin(t *testing.T) {
	tests := []struct {
		name                   string
		group                  *Group
		currentEnrollmentCount int
		want                   bool
	}{
		{
			name: "Can join (open and has spots)",
			group: &Group{
				IsOpen:          true,
				MaxParticipants: 10,
			},
			currentEnrollmentCount: 5,
			want:                   true,
		},
		{
			name: "Cannot join (closed)",
			group: &Group{
				IsOpen:          false,
				MaxParticipants: 10,
			},
			currentEnrollmentCount: 5,
			want:                   false,
		},
		{
			name: "Cannot join (open but full)",
			group: &Group{
				IsOpen:          true,
				MaxParticipants: 10,
			},
			currentEnrollmentCount: 10,
			want:                   false,
		},
		{
			name: "Cannot join (closed and full)",
			group: &Group{
				IsOpen:          false,
				MaxParticipants: 10,
			},
			currentEnrollmentCount: 10,
			want:                   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.group.CanJoin(tt.currentEnrollmentCount); got != tt.want {
				t.Errorf("Group.CanJoin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGroupTableName(t *testing.T) {
	group := &Group{}
	expected := "activities.groups"

	if got := group.TableName(); got != expected {
		t.Errorf("Group.TableName() = %v, want %v", got, expected)
	}
}

func TestGroup_EntityInterface(t *testing.T) {
	now := time.Now()
	group := &Group{
		Model: base.Model{
			ID:        123,
			CreatedAt: now,
			UpdatedAt: now.Add(time.Hour),
		},
		Name:       "Test Group",
		CategoryID: 1,
	}

	t.Run("GetID", func(t *testing.T) {
		got := group.GetID()
		if got != int64(123) {
			t.Errorf("Group.GetID() = %v, want %v", got, int64(123))
		}
	})

	t.Run("GetCreatedAt", func(t *testing.T) {
		got := group.GetCreatedAt()
		if !got.Equal(now) {
			t.Errorf("Group.GetCreatedAt() = %v, want %v", got, now)
		}
	})

	t.Run("GetUpdatedAt", func(t *testing.T) {
		expected := now.Add(time.Hour)
		got := group.GetUpdatedAt()
		if !got.Equal(expected) {
			t.Errorf("Group.GetUpdatedAt() = %v, want %v", got, expected)
		}
	})
}
