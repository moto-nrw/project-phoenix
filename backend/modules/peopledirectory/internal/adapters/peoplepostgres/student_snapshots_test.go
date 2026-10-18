package users

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	models "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Check every flattened field, including metadata and the independently copied
// departure baseline. JSON objects must preserve nil, empty and nil entries.
func TestNativeStudentSnapshotRetainsCompleteHydratedValue(t *testing.T) {
	t.Parallel()
	text, truth := "retained", true
	stamp := time.Date(2026, time.October, 17, 11, 22, 33, 44, time.UTC)
	day := calendar.NewDate(2026, 10, 17)
	row := &models.Student{
		PersonID: 31, SchoolClass: "2a", AddressStreet: &text, AddressCity: &text, AddressPostalCode: &text,
		ExtraInfo: &text, SupervisorNotes: &text, HealthInfo: &text, PickupStatus: &text,
		DepartureDays:          models.DepartureDays{"mon": models.DeparturePickup},
		AllowedDepartureModes:  models.AllowedDepartureModes{"mon": {models.DeparturePickup}, "tue": nil, "wed": {}},
		DepartureCompanionNote: &text, DepartureCompanionDays: map[string]bool{"mon": true},
		PickupDays: models.PickupDays{"mon": true}, BusDays: models.BusDays{},
		Sick: &truth, SickSince: &stamp, Excused: &truth, ExcusedSince: &stamp, Status: models.StudentStatusActive,
		EnrolledFrom: &day, EnrolledUntil: &day, PhotoPath: &text, PhotoConsentGivenAt: &stamp,
		AGBAcceptedAt: &stamp, DataProcessingAcceptedAt: &stamp, EmailContactAcceptedAt: &stamp,
		Person: &models.Person{FirstName: "Mia", LastName: "Meier"},
	}
	row.ID, row.TenantID, row.CreatedAt, row.UpdatedAt = 23, 29, stamp, stamp
	row.GroupID, row.PhotoConsentGivenBy = &row.ID, &row.PersonID
	row.DepartureBaseline = &models.DeparturePlanSnapshot{DepartureDays: models.DepartureDays{}, AllowedDepartureModes: models.AllowedDepartureModes{"thu": nil}, BusDays: nil, PickupDays: models.PickupDays{"fri": true}}
	for _, value := range []*models.Student{row, {}, nil} {
		got := NativeStudentSnapshot(value)
		if value == nil {
			if got != nil {
				t.Fatal("nil student changed")
			}
			continue
		}
		fields := reflect.ValueOf(value).Elem()
		native := reflect.ValueOf(got).Elem()
		for i := 0; i < native.NumField(); i++ {
			name := native.Type().Field(i).Name
			want := fields.FieldByName(name).Interface()
			if name == "Person" {
				want = NativeDirectoryPerson(value.Person)
			}
			wantJSON, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			actual := native.Field(i).Interface()
			// RawMessage nil marshals as null, matching a nil source map.
			gotJSON, err := json.Marshal(actual)
			if err != nil {
				t.Fatal(err)
			}
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("%s changed: got %s, want %s", name, gotJSON, wantJSON)
			}
		}
	}
}
