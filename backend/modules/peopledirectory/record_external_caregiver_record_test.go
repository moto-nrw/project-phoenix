package peopledirectory

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func datePtr(d calendar.Date) *calendar.Date { return &d }

func TestExternalCaregiverRecord_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		record  *ExternalCaregiverRecord
		wantErr bool
	}{
		{
			name: "valid record with required fields only",
			record: &ExternalCaregiverRecord{
				StaffID:           1,
				ActivityExpertise: "Soccer",
			},
			wantErr: false,
		},
		{
			name: "valid record with all fields",
			record: &ExternalCaregiverRecord{
				StaffID:           1,
				Organization:      "Sports Club",
				ContactEmail:      "extern@example.com",
				ContactPhone:      "+49 123 456789",
				ActivityExpertise: "Basketball",
				StartDate:         datePtr(calendar.TodayDate()),
				EndDate:           datePtr(calendar.TodayDate().AddDays(30)),
			},
			wantErr: false,
		},
		{
			name: "missing staff ID",
			record: &ExternalCaregiverRecord{
				ActivityExpertise: "Soccer",
			},
			wantErr: true,
		},
		{
			name: "zero staff ID",
			record: &ExternalCaregiverRecord{
				StaffID:           0,
				ActivityExpertise: "Soccer",
			},
			wantErr: true,
		},
		{
			name: "negative staff ID",
			record: &ExternalCaregiverRecord{
				StaffID:           -1,
				ActivityExpertise: "Soccer",
			},
			wantErr: true,
		},
		{
			name: "missing activity expertise",
			record: &ExternalCaregiverRecord{
				StaffID: 1,
			},
			wantErr: true,
		},
		{
			name: "empty activity expertise",
			record: &ExternalCaregiverRecord{
				StaffID:           1,
				ActivityExpertise: "",
			},
			wantErr: true,
		},
		{
			name: "invalid email format",
			record: &ExternalCaregiverRecord{
				StaffID:           1,
				ActivityExpertise: "Soccer",
				ContactEmail:      "not-an-email",
			},
			wantErr: true,
		},
		{
			name: "invalid phone format",
			record: &ExternalCaregiverRecord{
				StaffID:           1,
				ActivityExpertise: "Soccer",
				ContactPhone:      "abc",
			},
			wantErr: true,
		},
		{
			name: "end date before start date",
			record: &ExternalCaregiverRecord{
				StaffID:           1,
				ActivityExpertise: "Soccer",
				StartDate:         datePtr(calendar.TodayDate().AddDays(1)),
				EndDate:           datePtr(calendar.TodayDate()),
			},
			wantErr: true,
		},
		{
			name: "trimmed whitespace",
			record: &ExternalCaregiverRecord{
				StaffID:           1,
				ActivityExpertise: "  Soccer  ",
				Organization:      "  Sports Club  ",
				ContactEmail:      "  extern@example.com  ",
				ContactPhone:      "  +49 123 456789  ",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.record.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ExternalCaregiverRecord.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
