package peopledirectory

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func datePtr(d calendar.Date) *calendar.Date { return &d }

func TestGuest_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		guest   *Guest
		wantErr bool
	}{
		{
			name: "valid guest with required fields only",
			guest: &Guest{
				StaffID:           1,
				ActivityExpertise: "Soccer",
			},
			wantErr: false,
		},
		{
			name: "valid guest with all fields",
			guest: &Guest{
				StaffID:           1,
				Organization:      "Sports Club",
				ContactEmail:      "guest@example.com",
				ContactPhone:      "+49 123 456789",
				ActivityExpertise: "Basketball",
				StartDate:         datePtr(calendar.TodayDate()),
				EndDate:           datePtr(calendar.TodayDate().AddDays(30)),
			},
			wantErr: false,
		},
		{
			name: "missing staff ID",
			guest: &Guest{
				ActivityExpertise: "Soccer",
			},
			wantErr: true,
		},
		{
			name: "zero staff ID",
			guest: &Guest{
				StaffID:           0,
				ActivityExpertise: "Soccer",
			},
			wantErr: true,
		},
		{
			name: "negative staff ID",
			guest: &Guest{
				StaffID:           -1,
				ActivityExpertise: "Soccer",
			},
			wantErr: true,
		},
		{
			name: "missing activity expertise",
			guest: &Guest{
				StaffID: 1,
			},
			wantErr: true,
		},
		{
			name: "empty activity expertise",
			guest: &Guest{
				StaffID:           1,
				ActivityExpertise: "",
			},
			wantErr: true,
		},
		{
			name: "invalid email format",
			guest: &Guest{
				StaffID:           1,
				ActivityExpertise: "Soccer",
				ContactEmail:      "not-an-email",
			},
			wantErr: true,
		},
		{
			name: "invalid phone format",
			guest: &Guest{
				StaffID:           1,
				ActivityExpertise: "Soccer",
				ContactPhone:      "abc",
			},
			wantErr: true,
		},
		{
			name: "end date before start date",
			guest: &Guest{
				StaffID:           1,
				ActivityExpertise: "Soccer",
				StartDate:         datePtr(calendar.TodayDate().AddDays(1)),
				EndDate:           datePtr(calendar.TodayDate()),
			},
			wantErr: true,
		},
		{
			name: "trimmed whitespace",
			guest: &Guest{
				StaffID:           1,
				ActivityExpertise: "  Soccer  ",
				Organization:      "  Sports Club  ",
				ContactEmail:      "  guest@example.com  ",
				ContactPhone:      "  +49 123 456789  ",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.guest.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Guest.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
