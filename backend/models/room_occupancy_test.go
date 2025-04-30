package models

import (
	"testing"
	"time"
)

func TestRoomOccupancy_Validate(t *testing.T) {
	tests := []struct {
		name    string
		ro      *RoomOccupancy
		wantErr bool
	}{
		{
			name: "valid with AgID",
			ro: &RoomOccupancy{
				DeviceID:   "test-device-1",
				RoomID:     1,
				AgID:       intPtr(2),
				TimespanID: 3,
			},
			wantErr: false,
		},
		{
			name: "valid with GroupID",
			ro: &RoomOccupancy{
				DeviceID:   "test-device-2",
				RoomID:     1,
				GroupID:    intPtr(2),
				TimespanID: 3,
			},
			wantErr: false,
		},
		{
			name: "valid with both AgID and GroupID",
			ro: &RoomOccupancy{
				DeviceID:   "test-device-3",
				RoomID:     1,
				AgID:       intPtr(2),
				GroupID:    intPtr(3),
				TimespanID: 4,
			},
			wantErr: false,
		},
		{
			name: "invalid - missing both AgID and GroupID",
			ro: &RoomOccupancy{
				DeviceID:   "test-device-4",
				RoomID:     1,
				TimespanID: 2,
			},
			wantErr: true,
		},
		{
			name: "invalid - missing DeviceID",
			ro: &RoomOccupancy{
				RoomID:     1,
				AgID:       intPtr(2),
				TimespanID: 3,
			},
			wantErr: true,
		},
		{
			name: "invalid - missing RoomID",
			ro: &RoomOccupancy{
				DeviceID:   "test-device-5",
				AgID:       intPtr(2),
				TimespanID: 3,
			},
			wantErr: true,
		},
		{
			name: "invalid - missing TimespanID",
			ro: &RoomOccupancy{
				DeviceID: "test-device-6",
				RoomID:   1,
				AgID:     intPtr(2),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ro.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("RoomOccupancy.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRoomOccupancy_IsActive(t *testing.T) {
	now := time.Now()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	tests := []struct {
		name     string
		setup    func() *RoomOccupancy
		expected bool
	}{
		{
			name: "active with no end time",
			setup: func() *RoomOccupancy {
				return &RoomOccupancy{
					Timespan: &Timespan{
						StartTime: past,
						EndTime:   nil,
					},
				}
			},
			expected: true,
		},
		{
			name: "active with future end time",
			setup: func() *RoomOccupancy {
				return &RoomOccupancy{
					Timespan: &Timespan{
						StartTime: past,
						EndTime:   &future,
					},
				}
			},
			expected: true,
		},
		{
			name: "inactive with past end time",
			setup: func() *RoomOccupancy {
				return &RoomOccupancy{
					Timespan: &Timespan{
						StartTime: past,
						EndTime:   &past,
					},
				}
			},
			expected: false,
		},
		{
			name: "inactive with nil timespan",
			setup: func() *RoomOccupancy {
				return &RoomOccupancy{
					Timespan: nil,
				}
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ro := tt.setup()
			if got := ro.IsActive(); got != tt.expected {
				t.Errorf("RoomOccupancy.IsActive() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestRoomOccupancy_String(t *testing.T) {
	tests := []struct {
		name     string
		ro       *RoomOccupancy
		expected string
	}{
		{
			name: "with Ag",
			ro: &RoomOccupancy{
				Room: &Room{RoomName: "Test Room"},
				Ag:   &Ag{Name: "Test AG"},
			},
			expected: "Test Room - Test AG",
		},
		{
			name: "with Group",
			ro: &RoomOccupancy{
				Room:  &Room{RoomName: "Test Room"},
				Group: &Group{Name: "Test Group"},
			},
			expected: "Test Room - Test Group",
		},
		{
			name: "with both Ag and Group (Ag takes precedence)",
			ro: &RoomOccupancy{
				Room:  &Room{RoomName: "Test Room"},
				Ag:    &Ag{Name: "Test AG"},
				Group: &Group{Name: "Test Group"},
			},
			expected: "Test Room - Test AG",
		},
		{
			name: "with no Ag or Group",
			ro: &RoomOccupancy{
				Room: &Room{RoomName: "Test Room"},
			},
			expected: "Test Room - Unspecified activity",
		},
		{
			name: "with no Room",
			ro: &RoomOccupancy{
				Ag: &Ag{Name: "Test AG"},
			},
			expected: "Unknown room - Test AG",
		},
		{
			name:     "with nothing",
			ro:       &RoomOccupancy{},
			expected: "Unknown room - Unspecified activity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ro.String()
			if result != tt.expected {
				t.Errorf("RoomOccupancy.String() = %q, expected %q", result, tt.expected)
			}
		})
	}
}

// Helper function for int pointer
func intPtr(i int64) *int64 {
	return &i
}
