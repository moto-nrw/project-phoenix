package careplanhttp

import (
	"reflect"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsAbsenceOnly(t *testing.T) {
	t.Parallel()

	strPtr := func(s string) *string { return &s }
	boolPtr := func(b bool) *bool { return &b }

	cases := []struct {
		name string
		req  *UpdateStudentRequest
		want bool
	}{
		{"sick only", &UpdateStudentRequest{Sick: boolPtr(true)}, true},
		{"clearing sick", &UpdateStudentRequest{Sick: boolPtr(false)}, true},
		{"sick with reason", &UpdateStudentRequest{Sick: boolPtr(true), SickReason: strPtr("Fieber")}, true},
		{"excused only", &UpdateStudentRequest{Excused: boolPtr(true)}, true},
		{"sick and excused", &UpdateStudentRequest{Sick: boolPtr(true), Excused: boolPtr(false)}, true},
		{"empty payload", &UpdateStudentRequest{}, false},
		{"nil request", nil, false},
		{"sick plus school class", &UpdateStudentRequest{Sick: boolPtr(true), SchoolClass: strPtr("2b")}, false},
		{"sick plus extra info", &UpdateStudentRequest{Sick: boolPtr(true), ExtraInfo: strPtr("x")}, false},
		{"sick plus health info", &UpdateStudentRequest{Sick: boolPtr(true), HealthInfo: strPtr("x")}, false},
		{"sick plus group move", &UpdateStudentRequest{Sick: boolPtr(true), GroupID: func() *int64 { id := int64(3); return &id }()}, false},
		{"sick plus photo consent", &UpdateStudentRequest{Sick: boolPtr(true), PhotoConsentGiven: boolPtr(true)}, false},
		{"sick plus companions", &UpdateStudentRequest{Sick: boolPtr(true), Companions: &[]CompanionEntry{}}, false},
		{"sick plus companion extension flag", &UpdateStudentRequest{Sick: boolPtr(true), ExtendCompanionPlans: true}, false},
		{"sick plus departure plan", &UpdateStudentRequest{Sick: boolPtr(true), AllowedDepartureModes: &departure.AllowedDepartureModes{"monday": {departure.DepartureAlone}}}, false},
		{"school class only", &UpdateStudentRequest{SchoolClass: strPtr("2b")}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.req.isAbsenceOnly())
		})
	}
}

// TestAbsenceOnlyUpdateFieldsExist guards the allowlist against a rename: if a
// field in absenceOnlyUpdateFields no longer exists on UpdateStudentRequest,
// the gate silently stops recognizing absence payloads (every absence write
// would 403 under open care) and the map entry would be dead weight.
func TestAbsenceOnlyUpdateFieldsExist(t *testing.T) {
	t.Parallel()

	structType := reflect.TypeOf(UpdateStudentRequest{})
	for name := range absenceOnlyUpdateFields {
		_, ok := structType.FieldByName(name)
		require.Truef(t, ok, "absenceOnlyUpdateFields references %q, which UpdateStudentRequest no longer has", name)
	}
}
