package facilities

import (
	"testing"
)

// Test helpers - local to avoid external dependencies
func intPtr(i int) *int       { return &i }
func stringPtr(s string) *string { return &s }

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
				Floor:    intPtr(2),
				Capacity: intPtr(20),
				Category: stringPtr("Meeting"),
				Color:    stringPtr("#FF5733"),
			},
			wantErr: false,
		},
		{
			name: "valid room with short hex color",
			room: &Room{
				Name:  "Blue Room",
				Color: stringPtr("#00F"),
			},
			wantErr: false,
		},
		{
			name: "valid room without hash in color",
			room: &Room{
				Name:  "Green Room",
				Color: stringPtr("00FF00"),
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
				Capacity: intPtr(-5),
			},
			wantErr: true,
		},
		{
			name: "zero capacity is valid",
			room: &Room{
				Name:     "Storage Room",
				Capacity: intPtr(0),
			},
			wantErr: false,
		},
		{
			name: "invalid hex color - wrong chars",
			room: &Room{
				Name:  "Bad Color Room",
				Color: stringPtr("#GGHHII"),
			},
			wantErr: true,
		},
		{
			name: "invalid hex color - wrong length",
			room: &Room{
				Name:  "Bad Color Room",
				Color: stringPtr("#12345"),
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
			Color: stringPtr("FF5733"),
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
