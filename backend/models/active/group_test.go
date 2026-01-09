package active

import (
	"testing"
	"time"
)

func TestGroupValidate(t *testing.T) {
	nowTime := time.Now()
	futureTime := nowTime.Add(2 * time.Hour)
	pastTime := nowTime.Add(-2 * time.Hour)
	deviceID := int64(1)

	tests := []struct {
		name    string
		group   *Group
		wantErr bool
	}{
		{
			name: "Valid group with device",
			group: &Group{
				StartTime: nowTime,
				GroupID:   1,
				DeviceID:  &deviceID,
				RoomID:    1,
			},
			wantErr: false,
		},
		{
			name: "Valid group without device (RFID system)",
			group: &Group{
				StartTime: nowTime,
				GroupID:   1,
				DeviceID:  nil, // Optional for RFID
				RoomID:    1,
			},
			wantErr: false,
		},
		{
			name: "Valid group with end time",
			group: &Group{
				StartTime: nowTime,
				EndTime:   &futureTime,
				GroupID:   1,
				DeviceID:  &deviceID,
				RoomID:    1,
			},
			wantErr: false,
		},
		{
			name: "Missing start time",
			group: &Group{
				GroupID:  1,
				DeviceID: &deviceID,
				RoomID:   1,
			},
			wantErr: true,
		},
		{
			name: "End time before start time",
			group: &Group{
				StartTime: nowTime,
				EndTime:   &pastTime,
				GroupID:   1,
				DeviceID:  &deviceID,
				RoomID:    1,
			},
			wantErr: true,
		},
		{
			name: "Missing group ID",
			group: &Group{
				StartTime: nowTime,
				DeviceID:  &deviceID,
				RoomID:    1,
			},
			wantErr: true,
		},
		{
			name: "Missing room ID",
			group: &Group{
				StartTime: nowTime,
				GroupID:   1,
				DeviceID:  &deviceID,
			},
			wantErr: true,
		},
		{
			name: "Invalid group ID",
			group: &Group{
				StartTime: nowTime,
				GroupID:   -1,
				DeviceID:  &deviceID,
				RoomID:    1,
			},
			wantErr: true,
		},
		{
			name: "Invalid room ID",
			group: &Group{
				StartTime: nowTime,
				GroupID:   1,
				DeviceID:  &deviceID,
				RoomID:    -5,
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

func TestGroupIsActive(t *testing.T) {
	nowTime := time.Now()
	futureTime := nowTime.Add(2 * time.Hour)

	tests := []struct {
		name  string
		group *Group
		want  bool
	}{
		{
			name: "Active group (no end time)",
			group: &Group{
				StartTime: nowTime,
				EndTime:   nil,
			},
			want: true,
		},
		{
			name: "Inactive group (has end time)",
			group: &Group{
				StartTime: nowTime,
				EndTime:   &futureTime,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.group.IsActive(); got != tt.want {
				t.Errorf("Group.IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGroupEndSession(t *testing.T) {
	nowTime := time.Now()

	group := &Group{
		StartTime: nowTime,
		EndTime:   nil,
	}

	// Test that EndSession sets the end time
	group.EndSession()

	if group.EndTime == nil {
		t.Errorf("Group.EndSession() did not set the end time")
	}
}

func TestGroupSetEndTime(t *testing.T) {
	nowTime := time.Now()
	futureTime := nowTime.Add(2 * time.Hour)
	pastTime := nowTime.Add(-2 * time.Hour)

	tests := []struct {
		name    string
		group   *Group
		endTime time.Time
		wantErr bool
	}{
		{
			name: "Valid end time",
			group: &Group{
				StartTime: nowTime,
			},
			endTime: futureTime,
			wantErr: false,
		},
		{
			name: "End time before start time",
			group: &Group{
				StartTime: nowTime,
			},
			endTime: pastTime,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.group.SetEndTime(tt.endTime)
			if (err != nil) != tt.wantErr {
				t.Errorf("Group.SetEndTime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && (tt.group.EndTime == nil || !tt.endTime.Equal(*tt.group.EndTime)) {
				t.Errorf("Group.SetEndTime() did not correctly set the end time")
			}
		})
	}
}

func TestGroupGetDuration(t *testing.T) {
	nowTime := time.Now()
	futureTime := nowTime.Add(2 * time.Hour)

	tests := []struct {
		name         string
		group        *Group
		wantDuration time.Duration
	}{
		{
			name: "Active group (calculates duration from now)",
			group: &Group{
				StartTime: nowTime.Add(-1 * time.Hour), // Started 1 hour ago
				EndTime:   nil,
			},
			wantDuration: time.Hour, // Approximately 1 hour, may be slightly more
		},
		{
			name: "Inactive group (fixed duration)",
			group: &Group{
				StartTime: nowTime,
				EndTime:   &futureTime, // 2 hours in the future
			},
			wantDuration: 2 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.group.GetDuration()

			if tt.name == "Active group (calculates duration from now)" {
				// For active groups, we can only check that the duration is approximately correct
				if got < tt.wantDuration || got > tt.wantDuration+10*time.Second {
					t.Errorf("Group.GetDuration() = %v, want approximately %v", got, tt.wantDuration)
				}
			} else {
				// For inactive groups with fixed end times, we can check exact equality
				if got != tt.wantDuration {
					t.Errorf("Group.GetDuration() = %v, want %v", got, tt.wantDuration)
				}
			}
		})
	}
}

func TestGroupTableName(t *testing.T) {
	group := &Group{}
	want := "active.groups"

	if got := group.TableName(); got != want {
		t.Errorf("Group.TableName() = %v, want %v", got, want)
	}
}
