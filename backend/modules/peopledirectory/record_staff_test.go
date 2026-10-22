package peopledirectory

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStaff_JSONOmitsPersonnelNumber(t *testing.T) {
	t.Parallel()

	personnelNumber := "90001"
	staff := &Staff{
		PersonID:        1,
		PersonnelNumber: &personnelNumber,
	}

	payload, err := json.Marshal(staff)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if strings.Contains(string(payload), "personnel_number") {
		t.Fatalf("json.Marshal() exposed personnel_number: %s", payload)
	}
}

func TestStaff_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		staff   *Staff
		wantErr bool
	}{
		{
			name: "valid staff",
			staff: &Staff{
				PersonID: 1,
			},
			wantErr: false,
		},
		{
			name: "valid staff with notes",
			staff: &Staff{
				PersonID:   1,
				StaffNotes: "Some notes about the staff member",
			},
			wantErr: false,
		},
		{
			name: "zero person ID",
			staff: &Staff{
				PersonID: 0,
			},
			wantErr: true,
		},
		{
			name: "negative person ID",
			staff: &Staff{
				PersonID: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.staff.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Staff.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
