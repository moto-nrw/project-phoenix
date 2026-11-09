package users

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](v T) *T { return &v }

func TestStaffMasterData_Validate(t *testing.T) {
	t.Parallel()

	entry := calendar.NewDate(2024, 8, 1)
	before := calendar.NewDate(2024, 7, 31)
	after := calendar.NewDate(2025, 1, 31)

	tests := []struct {
		name    string
		data    StaffMasterData
		wantErr string
	}{
		{
			name: "full row",
			data: StaffMasterData{
				StaffID:          9001,
				Gender:           ptr(GenderDiverse),
				WeeklyHours:      ptr(29.5),
				EntryDate:        &entry,
				ContractEndDate:  &after,
				ProbationEndDate: &after,
			},
		},
		{name: "empty optional fields", data: StaffMasterData{StaffID: 9001}},
		{name: "missing staff id", data: StaffMasterData{}, wantErr: "staff_id is required"},
		{
			name:    "unknown gender",
			data:    StaffMasterData{StaffID: 9001, Gender: ptr("unspecified")},
			wantErr: "gender must be 'female', 'male', or 'diverse'",
		},
		{
			name:    "negative weekly hours",
			data:    StaffMasterData{StaffID: 9001, WeeklyHours: ptr(-1.0)},
			wantErr: "weekly_hours must be between 0 and 80",
		},
		{
			name:    "weekly hours beyond the cap",
			data:    StaffMasterData{StaffID: 9001, WeeklyHours: ptr(80.5)},
			wantErr: "weekly_hours must be between 0 and 80",
		},
		{
			name:    "contract ends before it starts",
			data:    StaffMasterData{StaffID: 9001, EntryDate: &entry, ContractEndDate: &before},
			wantErr: "contract_end_date must not be before entry_date",
		},
		{
			name:    "probation ends before entry",
			data:    StaffMasterData{StaffID: 9001, EntryDate: &entry, ProbationEndDate: &before},
			wantErr: "probation_end_date must not be before entry_date",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.data.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Equal(t, tt.wantErr, err.Error())
		})
	}

	// Every accepted gender value stays accepted.
	for _, gender := range []string{GenderFemale, GenderMale, GenderDiverse} {
		data := StaffMasterData{StaffID: 9001, Gender: ptr(gender)}
		require.NoError(t, data.Validate(), gender)
	}
}

func TestStaffQualification_Validate(t *testing.T) {
	t.Parallel()

	acquired := calendar.NewDate(2023, 3, 10)
	expires := calendar.NewDate(2026, 3, 10)
	expired := calendar.NewDate(2022, 3, 10)

	require.NoError(t, (&StaffQualification{
		StaffID:    9001,
		Name:       "Erste-Hilfe-Kurs",
		AcquiredOn: &acquired,
		ExpiresOn:  &expires,
	}).Validate())

	// Undated nachweise are normal — only the order of two set dates is a rule.
	require.NoError(t, (&StaffQualification{StaffID: 9001, Name: "Schwimmschein"}).Validate())

	err := (&StaffQualification{Name: "Erste-Hilfe-Kurs"}).Validate()
	require.Error(t, err)
	assert.Equal(t, "staff_id is required", err.Error())

	err = (&StaffQualification{StaffID: 9001}).Validate()
	require.Error(t, err)
	assert.Equal(t, "name is required", err.Error())

	err = (&StaffQualification{
		StaffID:    9001,
		Name:       "Erste-Hilfe-Kurs",
		AcquiredOn: &acquired,
		ExpiresOn:  &expired,
	}).Validate()
	require.Error(t, err)
	assert.Equal(t, "expires_on must not be before acquired_on", err.Error())
}
