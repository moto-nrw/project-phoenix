package active

import (
	"testing"
	"time"
)

func TestVisitValidate(t *testing.T) {
	nowTime := time.Now()
	futureTime := nowTime.Add(2 * time.Hour)
	pastTime := nowTime.Add(-2 * time.Hour)

	tests := []struct {
		name    string
		visit   *Visit
		wantErr bool
	}{
		{
			name: "Valid visit",
			visit: &Visit{
				StudentID:     1,
				ActiveGroupID: 1,
				EntryTime:     nowTime,
			},
			wantErr: false,
		},
		{
			name: "Valid visit with exit time",
			visit: &Visit{
				StudentID:     1,
				ActiveGroupID: 1,
				EntryTime:     nowTime,
				ExitTime:      &futureTime,
			},
			wantErr: false,
		},
		{
			name: "Missing student ID",
			visit: &Visit{
				ActiveGroupID: 1,
				EntryTime:     nowTime,
			},
			wantErr: true,
		},
		{
			name: "Missing active group ID",
			visit: &Visit{
				StudentID: 1,
				EntryTime: nowTime,
			},
			wantErr: true,
		},
		{
			name: "Missing entry time",
			visit: &Visit{
				StudentID:     1,
				ActiveGroupID: 1,
			},
			wantErr: true,
		},
		{
			name: "Exit time before entry time",
			visit: &Visit{
				StudentID:     1,
				ActiveGroupID: 1,
				EntryTime:     nowTime,
				ExitTime:      &pastTime,
			},
			wantErr: true,
		},
		{
			name: "Invalid student ID",
			visit: &Visit{
				StudentID:     -1,
				ActiveGroupID: 1,
				EntryTime:     nowTime,
			},
			wantErr: true,
		},
		{
			name: "Invalid active group ID",
			visit: &Visit{
				StudentID:     1,
				ActiveGroupID: 0,
				EntryTime:     nowTime,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.visit.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Visit.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVisitIsActive(t *testing.T) {
	nowTime := time.Now()
	futureTime := nowTime.Add(2 * time.Hour)

	tests := []struct {
		name  string
		visit *Visit
		want  bool
	}{
		{
			name: "Active visit (no exit time)",
			visit: &Visit{
				EntryTime: nowTime,
				ExitTime:  nil,
			},
			want: true,
		},
		{
			name: "Inactive visit (has exit time)",
			visit: &Visit{
				EntryTime: nowTime,
				ExitTime:  &futureTime,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.visit.IsActive(); got != tt.want {
				t.Errorf("Visit.IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVisitEndVisit(t *testing.T) {
	nowTime := time.Now()

	visit := &Visit{
		EntryTime: nowTime,
		ExitTime:  nil,
	}

	// Test that EndVisit sets the exit time
	visit.EndVisit()

	if visit.ExitTime == nil {
		t.Errorf("Visit.EndVisit() did not set the exit time")
	}
}

func TestVisitSetExitTime(t *testing.T) {
	nowTime := time.Now()
	futureTime := nowTime.Add(2 * time.Hour)
	pastTime := nowTime.Add(-2 * time.Hour)

	tests := []struct {
		name     string
		visit    *Visit
		exitTime time.Time
		wantErr  bool
	}{
		{
			name: "Valid exit time",
			visit: &Visit{
				EntryTime: nowTime,
			},
			exitTime: futureTime,
			wantErr:  false,
		},
		{
			name: "Exit time before entry time",
			visit: &Visit{
				EntryTime: nowTime,
			},
			exitTime: pastTime,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.visit.SetExitTime(tt.exitTime)
			if (err != nil) != tt.wantErr {
				t.Errorf("Visit.SetExitTime() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && (tt.visit.ExitTime == nil || !tt.exitTime.Equal(*tt.visit.ExitTime)) {
				t.Errorf("Visit.SetExitTime() did not correctly set the exit time")
			}
		})
	}
}

func TestVisitGetDuration(t *testing.T) {
	nowTime := time.Now()
	futureTime := nowTime.Add(2 * time.Hour)

	tests := []struct {
		name         string
		visit        *Visit
		wantDuration time.Duration
	}{
		{
			name: "Active visit (calculates duration from now)",
			visit: &Visit{
				EntryTime: nowTime.Add(-1 * time.Hour), // Started 1 hour ago
				ExitTime:  nil,
			},
			wantDuration: time.Hour, // Approximately 1 hour, may be slightly more
		},
		{
			name: "Inactive visit (fixed duration)",
			visit: &Visit{
				EntryTime: nowTime,
				ExitTime:  &futureTime, // 2 hours in the future
			},
			wantDuration: 2 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.visit.GetDuration()

			if tt.name == "Active visit (calculates duration from now)" {
				// For active visits, we can only check that the duration is approximately correct
				if got < tt.wantDuration || got > tt.wantDuration+10*time.Second {
					t.Errorf("Visit.GetDuration() = %v, want approximately %v", got, tt.wantDuration)
				}
			} else {
				// For inactive visits with fixed end times, we can check exact equality
				if got != tt.wantDuration {
					t.Errorf("Visit.GetDuration() = %v, want %v", got, tt.wantDuration)
				}
			}
		})
	}
}

func TestVisitTableName(t *testing.T) {
	visit := &Visit{}
	want := "active.visits"

	if got := visit.TableName(); got != want {
		t.Errorf("Visit.TableName() = %v, want %v", got, want)
	}
}
