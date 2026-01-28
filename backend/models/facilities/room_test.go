package facilities

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestRoom_Validate(t *testing.T) {
	tests := []struct {
		name    string
		room    *Room
		wantErr bool
	}{
		{
			name: "valid room with name only",
			room: &Room{
				Name: "Room 101",
			},
			wantErr: false,
		},
		{
			name: "valid room with all fields",
			room: &Room{
				Name:     "Conference Room A",
				Building: "Main Building",
				Floor:    base.IntPtr(2),
				Capacity: base.IntPtr(20),
				Category: base.StringPtr("Meeting"),
				Color:    base.StringPtr("#FF5733"),
			},
			wantErr: false,
		},
		{
			name: "valid room with short hex color",
			room: &Room{
				Name:  "Blue Room",
				Color: base.StringPtr("#00F"),
			},
			wantErr: false,
		},
		{
			name: "valid room without hash in color",
			room: &Room{
				Name:  "Green Room",
				Color: base.StringPtr("00FF00"),
			},
			wantErr: false,
		},
		{
			name: "empty name",
			room: &Room{
				Name: "",
			},
			wantErr: true,
		},
		{
			name: "negative capacity",
			room: &Room{
				Name:     "Small Room",
				Capacity: base.IntPtr(-5),
			},
			wantErr: true,
		},
		{
			name: "zero capacity is valid",
			room: &Room{
				Name:     "Storage Room",
				Capacity: base.IntPtr(0),
			},
			wantErr: false,
		},
		{
			name: "invalid hex color - wrong chars",
			room: &Room{
				Name:  "Bad Color Room",
				Color: base.StringPtr("#GGHHII"),
			},
			wantErr: true,
		},
		{
			name: "invalid hex color - wrong length",
			room: &Room{
				Name:  "Bad Color Room",
				Color: base.StringPtr("#12345"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.room.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Room.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRoom_Validate_Normalization(t *testing.T) {
	t.Run("trims name whitespace", func(t *testing.T) {
		room := &Room{Name: "  Room 101  "}
		err := room.Validate()
		if err != nil {
			t.Fatalf("Room.Validate() unexpected error = %v", err)
		}
		if room.Name != "Room 101" {
			t.Errorf("Room.Name = %q, want Room 101", room.Name)
		}
	})

	t.Run("adds hash to color", func(t *testing.T) {
		room := &Room{
			Name:  "Test Room",
			Color: base.StringPtr("FF5733"),
		}
		err := room.Validate()
		if err != nil {
			t.Fatalf("Room.Validate() unexpected error = %v", err)
		}
		if *room.Color != "#FF5733" {
			t.Errorf("Room.Color = %q, want #FF5733", *room.Color)
		}
	})
}

func TestRoomIsAvailableWithNilCapacity(t *testing.T) {
	room := &Room{Capacity: nil}

	if !room.IsAvailable(0) {
		t.Fatalf("expected room with nil capacity to be available for 0 requirement")
	}

	if room.IsAvailable(5) {
		t.Fatalf("expected room with nil capacity to be unavailable for capacity > 0")
	}
}

func TestRoomIsAvailableWithCapacityValue(t *testing.T) {
	capacity := 10
	room := &Room{Capacity: &capacity}

	cases := []struct {
		required int
		expected bool
	}{
		{0, true},
		{5, true},
		{10, true},
		{11, false},
	}

	for _, c := range cases {
		if got := room.IsAvailable(c.required); got != c.expected {
			t.Fatalf("IsAvailable(%d) = %v, expected %v", c.required, got, c.expected)
		}
	}
}

func TestRoom_GetFullName(t *testing.T) {
	tests := []struct {
		name     string
		room     *Room
		expected string
	}{
		{
			name: "with building",
			room: &Room{
				Name:     "Room 101",
				Building: "Main Building",
			},
			expected: "Main Building - Room 101",
		},
		{
			name: "without building",
			room: &Room{
				Name:     "Conference Room",
				Building: "",
			},
			expected: "Conference Room",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.room.GetFullName()
			if got != tt.expected {
				t.Errorf("Room.GetFullName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestRoom_TableName(t *testing.T) {
	room := &Room{}
	if got := room.TableName(); got != "facilities.rooms" {
		t.Errorf("TableName() = %v, want facilities.rooms", got)
	}
}

func TestRoom_BeforeAppendModel(t *testing.T) {
	// BeforeAppendModel modifies query table expressions for different query types
	// It doesn't set timestamps - those are handled by the base model or repository

	t.Run("handles nil query", func(t *testing.T) {
		room := &Room{Name: "Test Room"}
		err := room.BeforeAppendModel(nil)
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})

	t.Run("returns no error for unknown query type", func(t *testing.T) {
		room := &Room{Name: "Test Room"}
		err := room.BeforeAppendModel("some string")
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})
}

func TestRoom_GetID(t *testing.T) {
	room := &Room{
		Model: base.Model{ID: 42},
		Name:  "Test Room",
	}

	// GetID returns interface{}, so we compare with int64
	if got, ok := room.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", room.GetID())
	}
}

func TestRoom_GetCreatedAt(t *testing.T) {
	now := time.Now()
	room := &Room{
		Model: base.Model{CreatedAt: now},
		Name:  "Test Room",
	}

	if got := room.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestRoom_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	room := &Room{
		Model: base.Model{UpdatedAt: now},
		Name:  "Test Room",
	}

	if got := room.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}
