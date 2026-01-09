package active

import (
	"testing"
	"time"
)

func TestGroupSupervisorValidate(t *testing.T) {
	nowDate := time.Now()
	futureDate := nowDate.Add(30 * 24 * time.Hour) // 30 days in the future
	pastDate := nowDate.Add(-30 * 24 * time.Hour)  // 30 days in the past

	tests := []struct {
		name            string
		groupSupervisor *GroupSupervisor
		wantErr         bool
	}{
		{
			name: "Valid group supervisor",
			groupSupervisor: &GroupSupervisor{
				StaffID:   1,
				GroupID:   1,
				Role:      "supervisor",
				StartDate: nowDate,
			},
			wantErr: false,
		},
		{
			name: "Valid group supervisor with end date",
			groupSupervisor: &GroupSupervisor{
				StaffID:   1,
				GroupID:   1,
				Role:      "supervisor",
				StartDate: nowDate,
				EndDate:   &futureDate,
			},
			wantErr: false,
		},
		{
			name: "Missing staff ID",
			groupSupervisor: &GroupSupervisor{
				GroupID:   1,
				Role:      "supervisor",
				StartDate: nowDate,
			},
			wantErr: true,
		},
		{
			name: "Missing group ID",
			groupSupervisor: &GroupSupervisor{
				StaffID:   1,
				Role:      "supervisor",
				StartDate: nowDate,
			},
			wantErr: true,
		},
		{
			name: "Missing role",
			groupSupervisor: &GroupSupervisor{
				StaffID:   1,
				GroupID:   1,
				StartDate: nowDate,
			},
			wantErr: true,
		},
		{
			name: "Missing start date",
			groupSupervisor: &GroupSupervisor{
				StaffID: 1,
				GroupID: 1,
				Role:    "supervisor",
			},
			wantErr: true,
		},
		{
			name: "End date before start date",
			groupSupervisor: &GroupSupervisor{
				StaffID:   1,
				GroupID:   1,
				Role:      "supervisor",
				StartDate: nowDate,
				EndDate:   &pastDate,
			},
			wantErr: true,
		},
		{
			name: "Invalid staff ID",
			groupSupervisor: &GroupSupervisor{
				StaffID:   -1,
				GroupID:   1,
				Role:      "supervisor",
				StartDate: nowDate,
			},
			wantErr: true,
		},
		{
			name: "Invalid group ID",
			groupSupervisor: &GroupSupervisor{
				StaffID:   1,
				GroupID:   0,
				Role:      "supervisor",
				StartDate: nowDate,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.groupSupervisor.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("GroupSupervisor.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGroupSupervisorIsActive(t *testing.T) {
	nowDate := time.Now()
	futureDate := nowDate.Add(30 * 24 * time.Hour) // 30 days in the future
	pastDate := nowDate.Add(-30 * 24 * time.Hour)  // 30 days in the past

	tests := []struct {
		name            string
		groupSupervisor *GroupSupervisor
		want            bool
	}{
		{
			name: "Active supervision (no end date)",
			groupSupervisor: &GroupSupervisor{
				StartDate: nowDate,
				EndDate:   nil,
			},
			want: true,
		},
		{
			name: "Active supervision (future end date)",
			groupSupervisor: &GroupSupervisor{
				StartDate: nowDate,
				EndDate:   &futureDate,
			},
			want: true,
		},
		{
			name: "Inactive supervision (past end date)",
			groupSupervisor: &GroupSupervisor{
				StartDate: pastDate,
				EndDate:   &pastDate,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.groupSupervisor.IsActive(); got != tt.want {
				t.Errorf("GroupSupervisor.IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGroupSupervisorEndSupervision(t *testing.T) {
	nowDate := time.Now()

	groupSupervisor := &GroupSupervisor{
		StartDate: nowDate,
		EndDate:   nil,
	}

	// Test that EndSupervision sets the end date
	groupSupervisor.EndSupervision()

	if groupSupervisor.EndDate == nil {
		t.Errorf("GroupSupervisor.EndSupervision() did not set the end date")
	}
}

func TestGroupSupervisorSetEndDate(t *testing.T) {
	nowDate := time.Now()
	futureDate := nowDate.Add(30 * 24 * time.Hour) // 30 days in the future
	pastDate := nowDate.Add(-30 * 24 * time.Hour)  // 30 days in the past

	tests := []struct {
		name            string
		groupSupervisor *GroupSupervisor
		endDate         time.Time
		wantErr         bool
	}{
		{
			name: "Valid end date",
			groupSupervisor: &GroupSupervisor{
				StartDate: nowDate,
			},
			endDate: futureDate,
			wantErr: false,
		},
		{
			name: "End date before start date",
			groupSupervisor: &GroupSupervisor{
				StartDate: nowDate,
			},
			endDate: pastDate,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.groupSupervisor.SetEndDate(tt.endDate)
			if (err != nil) != tt.wantErr {
				t.Errorf("GroupSupervisor.SetEndDate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && (tt.groupSupervisor.EndDate == nil || !tt.endDate.Equal(*tt.groupSupervisor.EndDate)) {
				t.Errorf("GroupSupervisor.SetEndDate() did not correctly set the end date")
			}
		})
	}
}

func TestGroupSupervisorTableName(t *testing.T) {
	groupSupervisor := &GroupSupervisor{}
	want := "active.group_supervisors"

	if got := groupSupervisor.TableName(); got != want {
		t.Errorf("GroupSupervisor.TableName() = %v, want %v", got, want)
	}
}
