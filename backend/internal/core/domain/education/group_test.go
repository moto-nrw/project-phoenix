package education

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/facilities"
	"github.com/stretchr/testify/assert"
)

// Test helpers - local to avoid external dependencies
func int64Ptr(i int64) *int64 { return &i }

func TestGroup_Validate(t *testing.T) {
	tests := []struct {
		name    string
		group   *Group
		wantErr bool
	}{
		{
			name: "valid group",
			group: &Group{
				Name: "Class 1A",
			},
			wantErr: false,
		},
		{
			name: "valid group with room",
			group: &Group{
				Name:   "Class 2B",
				RoomID: int64Ptr(1),
			},
			wantErr: false,
		},
		{
			name: "empty name",
			group: &Group{
				Name: "",
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

func TestGroup_Validate_Normalization(t *testing.T) {
	group := &Group{Name: "  Class 1A  "}
	err := group.Validate()
	if err != nil {
		t.Fatalf("Group.Validate() unexpected error = %v", err)
	}
	if group.Name != "Class 1A" {
		t.Errorf("Group.Name = %q, want Class 1A", group.Name)
	}
}

func TestGroup_SetRoom(t *testing.T) {
	t.Run("set room", func(t *testing.T) {
		group := &Group{Name: "Test Group"}
		room := &facilities.Room{
			Model: base.Model{ID: 42},
			Name:  "Room 101",
		}

		group.SetRoom(room)

		if group.Room != room {
			t.Error("Group.SetRoom() did not set Room reference")
		}

		if group.RoomID == nil || *group.RoomID != 42 {
			t.Errorf("Group.RoomID = %v, want 42", group.RoomID)
		}
	})

	t.Run("set nil room", func(t *testing.T) {
		roomID := int64(42)
		group := &Group{
			Name:   "Test Group",
			RoomID: &roomID,
		}

		group.SetRoom(nil)

		if group.Room != nil {
			t.Error("Group.SetRoom(nil) did not clear Room reference")
		}

		if group.RoomID != nil {
			t.Error("Group.SetRoom(nil) did not clear RoomID")
		}
	})
}

func TestGroup_HasRoom(t *testing.T) {
	tests := []struct {
		name     string
		group    *Group
		expected bool
	}{
		{
			name: "has room",
			group: &Group{
				Name:   "Test",
				RoomID: int64Ptr(1),
			},
			expected: true,
		},
		{
			name: "nil room ID",
			group: &Group{
				Name:   "Test",
				RoomID: nil,
			},
			expected: false,
		},
		{
			name: "zero room ID",
			group: &Group{
				Name:   "Test",
				RoomID: int64Ptr(0),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.group.HasRoom()
			if got != tt.expected {
				t.Errorf("Group.HasRoom() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGroup_BeforeAppendModel(t *testing.T) {
	t.Run("handles nil query", func(t *testing.T) {
		group := &Group{Name: "Test Group"}
		err := group.BeforeAppendModel(nil)
		assert.NoError(t, err)
	})

	t.Run("returns no error for unknown query type", func(t *testing.T) {
		group := &Group{Name: "Test Group"}
		err := group.BeforeAppendModel("some string")
		assert.NoError(t, err)
	})
}

func TestGroup_TableName(t *testing.T) {
	group := &Group{}
	if got := group.TableName(); got != "education.groups" {
		t.Errorf("TableName() = %v, want education.groups", got)
	}
}

func TestGroup_GetID(t *testing.T) {
	group := &Group{
		Model: base.Model{ID: 42},
		Name:  "Test",
	}

	if got, ok := group.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", group.GetID())
	}
}

func TestGroup_GetCreatedAt(t *testing.T) {
	now := time.Now()
	group := &Group{
		Model: base.Model{CreatedAt: now},
		Name:  "Test",
	}

	if got := group.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestGroup_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	group := &Group{
		Model: base.Model{UpdatedAt: now},
		Name:  "Test",
	}

	if got := group.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}
