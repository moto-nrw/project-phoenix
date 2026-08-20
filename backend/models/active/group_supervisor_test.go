package active

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

func TestGroupSupervisorValidate(t *testing.T) {
	t.Parallel()

	nowDate := timezone.TodayDate()
	futureDate := nowDate.AddDays(30) // 30 days in the future
	pastDate := nowDate.AddDays(-30)  // 30 days in the past

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

func TestGroupSupervisorSetEndDate(t *testing.T) {
	t.Parallel()

	nowDate := timezone.TodayDate()
	futureDate := nowDate.AddDays(30) // 30 days in the future
	pastDate := nowDate.AddDays(-30)  // 30 days in the past

	tests := []struct {
		name            string
		groupSupervisor *GroupSupervisor
		endDate         timezone.Date
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

			if !tt.wantErr && (tt.groupSupervisor.EndDate == nil || tt.endDate != *tt.groupSupervisor.EndDate) {
				t.Errorf("GroupSupervisor.SetEndDate() did not correctly set the end date")
			}
		})
	}
}
