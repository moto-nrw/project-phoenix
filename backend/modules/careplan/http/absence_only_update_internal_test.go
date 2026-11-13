package careplanhttp_test

import (
	"reflect"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/peopledirectory/departure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	careplanhttp "github.com/moto-ogs/project-phoenix/modules/careplan/http"
)

func TestIsAbsenceOnly(t *testing.T) {
	t.Parallel()

	strPtr := func(s string) *string { return &s }
	boolPtr := func(b bool) *bool { return &b }

	cases := []struct {
		name string
		req  *careplanhttp.UpdateStudentRequest
		want bool
	}{
		{"sick only", &careplanhttp.UpdateStudentRequest{Sick: boolPtr(true)}, true},
		{"clearing sick", &careplanhttp.UpdateStudentRequest{Sick: boolPtr(false)}, true},
		{"sick with reason", &careplanhttp.UpdateStudentRequest{Sick: boolPtr(true), SickReason: strPtr("Fieber")}, true},
		{"excused only", &careplanhttp.UpdateStudentRequest{Excused: boolPtr(true)}, true},
		{"sick and excused", &careplanhttp.UpdateStudentRequest{Sick: boolPtr(true), Excused: boolPtr(false)}, true},
		{"empty payload", &careplanhttp.UpdateStudentRequest{}, false},
		{"nil request", nil, false},
		{"sick plus school class", &careplanhttp.UpdateStudentRequest{Sick: boolPtr(true), SchoolClass: strPtr("2b")}, false},
		{"sick plus extra info", &careplanhttp.UpdateStudentRequest{Sick: boolPtr(true), ExtraInfo: strPtr("x")}, false},
		{"sick plus health info", &careplanhttp.UpdateStudentRequest{Sick: boolPtr(true), HealthInfo: strPtr("x")}, false},
		{"sick plus group move", &careplanhttp.UpdateStudentRequest{Sick: boolPtr(true), GroupID: func() *int64 { id := int64(3); return &id }()}, false},
		{"sick plus photo consent", &careplanhttp.UpdateStudentRequest{Sick: boolPtr(true), PhotoConsentGiven: boolPtr(true)}, false},
		{"sick plus companions", &careplanhttp.UpdateStudentRequest{Sick: boolPtr(true), Companions: &[]careplanhttp.CompanionEntry{}}, false},
		{"sick plus companion extension flag", &careplanhttp.UpdateStudentRequest{Sick: boolPtr(true), ExtendCompanionPlans: true}, false},
		{"sick plus departure plan", &careplanhttp.UpdateStudentRequest{Sick: boolPtr(true), AllowedDepartureModes: &departure.AllowedDepartureModes{"monday": {departure.DepartureAlone}}}, false},
		{"school class only", &careplanhttp.UpdateStudentRequest{SchoolClass: strPtr("2b")}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.req.IsAbsenceOnlyForTest())
		})
	}
}

// TestAbsenceOnlyUpdateFieldsExist guards the allowlist against a rename: if a
// field in absenceOnlyUpdateFields no longer exists on UpdateStudentRequest,
// the gate silently stops recognizing absence payloads (every absence write
// would 403 under open care) and the map entry would be dead weight.
func TestAbsenceOnlyUpdateFieldsExist(t *testing.T) {
	t.Parallel()

	structType := reflect.TypeOf(careplanhttp.UpdateStudentRequest{})
	for name := range careplanhttp.AbsenceOnlyUpdateFieldsForTest {
		_, ok := structType.FieldByName(name)
		require.Truef(t, ok, "absenceOnlyUpdateFields references %q, which UpdateStudentRequest no longer has", name)
	}
}
