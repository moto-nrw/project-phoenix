package education

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/ptrtest"
)

func TestGroup_Validate(t *testing.T) {
	t.Parallel()

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
				RoomID: ptrtest.Ptr(int64(1)),
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
	t.Parallel()

	group := &Group{Name: "  Class 1A  "}
	err := group.Validate()
	if err != nil {
		t.Fatalf("Group.Validate() unexpected error = %v", err)
	}
	if group.Name != "Class 1A" {
		t.Errorf("Group.Name = %q, want Class 1A", group.Name)
	}
}

func TestGroup_HasRoom(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		group    *Group
		expected bool
	}{
		{
			name: "has room",
			group: &Group{
				Name:   "Test",
				RoomID: ptrtest.Ptr(int64(1)),
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
				RoomID: ptrtest.Ptr(int64(0)),
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
