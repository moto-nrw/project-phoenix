package presenceprojectionhttp

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/ptrtest"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	people "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type studentReadStaffCaller struct{ CallerContext }

func (studentReadStaffCaller) HasCurrentStaff(context.Context) (bool, error) { return true, nil }
func (studentReadStaffCaller) StudentAccessFacts(context.Context) (bool, bool) {
	return false, true
}

func TestStudentSensitiveReadRequiresUsersRead(t *testing.T) {
	t.Parallel()
	student := &people.StudentState{ID: 42, PhotoPath: ptrtest.Ptr("/uploads/student-photos/test.jpg")}
	rs := NewResource(ResourceConfig{UserContextService: studentReadStaffCaller{}})
	for _, tc := range []struct {
		name    string
		granted []string
		full    bool
	}{
		{name: "guest staff", granted: []string{permissions.StudentsRead}},
		{name: "staff", granted: []string{permissions.UsersRead}, full: true},
		{name: "resource wildcard", granted: []string{"users:*"}, full: true},
		{name: "admin", granted: []string{"admin:*"}, full: true},
		{name: "all permissions", granted: []string{"*:*"}, full: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testpkg.WithPermissionsContext(context.Background(), tc.granted)
			req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
			require.Equal(t, tc.full, rs.checkStudentReadAccess(req, student), "detail read scope")
			snapshot := &studentDataSnapshot{Persons: map[int64]*people.Person{student.PersonID: {FirstName: "Mila"}}}
			response := rs.buildSingleStudentResponse(ctx, student, &studentListParams{}, rs.determineStudentAccess(req), snapshot, nil, true)
			require.NotNil(t, response)
			require.Equal(t, tc.full, response.HasFullAccess, "list read scope")
			require.NotEmpty(t, response.PhotoURL, "verified staff can still read photos")
		})
	}
}

func TestStudentResponseRedactsSensitiveFields(t *testing.T) {
	t.Parallel()
	student := &people.StudentState{
		ID: 42, PersonID: 43, SchoolClass: "2a",
		ExtraInfo: ptrtest.Ptr("Elternnotiz"), HealthInfo: ptrtest.Ptr("Gesundheitsnotiz"),
		DepartureCompanionNote: ptrtest.Ptr("Abholhinweis"),
		PickupStatus:           ptrtest.Ptr("Nur nach Rücksprache mit den Eltern"),
		PhotoPath:              ptrtest.Ptr("/uploads/student-photos/test.jpg"),
		AddressStreet:          ptrtest.Ptr("Teststraße 10"), AddressCity: ptrtest.Ptr("Bonn"), AddressPostalCode: ptrtest.Ptr("53111"),
	}
	person := &people.Person{ID: student.PersonID, FirstName: "Mila", LastName: "Kind"}
	location := "Abwesend"
	for _, full := range []bool{false, true} {
		granted := []string{permissions.StudentsRead}
		if full {
			granted = []string{permissions.UsersRead}
		}
		ctx := testpkg.WithPermissionsContext(context.Background(), granted)
		for _, snapshot := range []bool{false, true} {
			var response StudentResponse
			if snapshot {
				response = newStudentResponseFromSnapshot(ctx, student, person, nil, true, nil, true)
			} else {
				var err error
				response, err = newStudentResponseWithOpts(ctx, StudentResponseOpts{
					Student: student, Person: person, HasFullAccess: true, LocationOverride: &location, PhotosEnabled: true, HasPhotoAccess: true,
				}, StudentResponseServices{})
				require.NoError(t, err)
			}
			responses := []StudentResponse{response}
			applyPickupTimesFromMap(responses, map[int64]*careplan.EffectivePickupTime{student.ID: {Notes: "Abholplan", DayNotes: []careplan.NoteData{{Content: "Tagesnotiz"}}}})
			applyArrivalTimesFromMap(responses, map[int64]*careplan.EffectiveArrivalTime{student.ID: {Notes: "Ankunftsplan", DayNotes: []careplan.ArrivalNoteData{{Content: "Tagesnotiz"}}}})
			encoded, err := json.Marshal(responses[0])
			require.NoError(t, err)
			var body map[string]any
			require.NoError(t, json.Unmarshal(encoded, &body))
			require.Equal(t, "Mila", body["first_name"])
			require.NotEmpty(t, body["photo_url"])
			if full {
				require.Equal(t, *student.PickupStatus, body["pickup_status"])
			} else {
				require.NotEqual(t, *student.PickupStatus, body["pickup_status"])
			}
			require.Equal(t, "2a", body["school_class"])
			require.Equal(t, full, body["has_full_access"])
			for _, field := range []string{"extra_info", "health_info", "departure_companion_note", "address_street", "address_city", "address_postal_code", "pickup_notes", "arrival_notes"} {
				if full {
					require.Contains(t, body, field, "full=%v snapshot=%v", full, snapshot)
				} else {
					require.NotContains(t, body, field, "full=%v snapshot=%v", full, snapshot)
				}
			}
		}
	}
}
