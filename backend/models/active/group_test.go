package active

import (
	"testing"
	"time"
)

func TestGroupValidate(t *testing.T) {
	t.Parallel()

	nowTime := time.Now()
	futureTime := nowTime.Add(2 * time.Hour)
	pastTime := nowTime.Add(-2 * time.Hour)
	deviceID := int64(1)
	// WP-B6: GroupID became *int64. Non-nil positive values validate as before,
	// nil is now legal (spontaneous session), non-nil non-positive values are
	// rejected. We keep concrete *int64 helpers below rather than allocating
	// anonymous &int64 literals scattered through the table.
	templateID := int64(1)
	negativeTemplateID := int64(-1)

	tests := []struct {
		name    string
		group   *Group
		wantErr bool
	}{
		{
			name: "Valid group with device",
			group: &Group{
				StartTime: nowTime,
				GroupID:   &templateID,
				DeviceID:  &deviceID,
				RoomID:    1,
			},
			wantErr: false,
		},
		{
			name: "Valid group without device (RFID system)",
			group: &Group{
				StartTime: nowTime,
				GroupID:   &templateID,
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
				GroupID:   &templateID,
				DeviceID:  &deviceID,
				RoomID:    1,
			},
			wantErr: false,
		},
		{
			name: "Missing start time",
			group: &Group{
				GroupID:  &templateID,
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
				GroupID:   &templateID,
				DeviceID:  &deviceID,
				RoomID:    1,
			},
			wantErr: true,
		},
		{
			// Business rule change (WP-B6, RFC E2): a nil GroupID is now the
			// canonical marker for a spontaneous session. The prior test
			// expected this to error because the column was NOT NULL; that
			// constraint has been dropped at the schema level.
			name: "Nil group ID (spontaneous session) — valid",
			group: &Group{
				StartTime: nowTime,
				GroupID:   nil,
				DeviceID:  &deviceID,
				RoomID:    1,
			},
			wantErr: false,
		},
		{
			name: "Missing room ID",
			group: &Group{
				StartTime: nowTime,
				GroupID:   &templateID,
				DeviceID:  &deviceID,
			},
			wantErr: true,
		},
		{
			name: "Invalid group ID (non-nil, non-positive)",
			group: &Group{
				StartTime: nowTime,
				GroupID:   &negativeTemplateID,
				DeviceID:  &deviceID,
				RoomID:    1,
			},
			wantErr: true,
		},
		{
			name: "Invalid room ID",
			group: &Group{
				StartTime: nowTime,
				GroupID:   &templateID,
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

// TestGroupTemplateHelpers covers the WP-B6 semantic helpers — the single
// sanctioned entry point for reading GroupID across the codebase. Every
// call site in services / repositories / handlers now goes through these,
// so a regression here would silently corrupt consumer behaviour.
func TestGroupTemplateHelpers(t *testing.T) {
	t.Parallel()

	templateID := int64(42)

	t.Run("template-backed group", func(t *testing.T) {
		g := &Group{GroupID: &templateID}

		if g.IsSpontaneous() {
			t.Errorf("IsSpontaneous() = true, want false for template-backed group")
		}
		if !g.HasTemplate() {
			t.Errorf("HasTemplate() = false, want true for template-backed group")
		}
		id, ok := g.TemplateID()
		if !ok {
			t.Errorf("TemplateID() ok = false, want true")
		}
		if id != templateID {
			t.Errorf("TemplateID() id = %d, want %d", id, templateID)
		}
	})

	t.Run("spontaneous group", func(t *testing.T) {
		g := &Group{GroupID: nil}

		if !g.IsSpontaneous() {
			t.Errorf("IsSpontaneous() = false, want true for nil GroupID")
		}
		if g.HasTemplate() {
			t.Errorf("HasTemplate() = true, want false for nil GroupID")
		}
		id, ok := g.TemplateID()
		if ok {
			t.Errorf("TemplateID() ok = true, want false for spontaneous group")
		}
		if id != 0 {
			t.Errorf("TemplateID() id = %d, want 0 (sentinel) when ok is false", id)
		}
	})
}

func TestGroupIsActive(t *testing.T) {
	t.Parallel()

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

func TestGroupSetEndTime(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
