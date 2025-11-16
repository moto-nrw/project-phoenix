package facilities

import "testing"

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

