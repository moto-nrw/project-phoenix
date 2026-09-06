package enrollment

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/stretchr/testify/assert"
)

// These are pure-function regression tests for the leaf value-formatters
// that turn arbitrary jsonb custom-field answers into the German export
// strings. They guard the edge cases the renderers depend on (numbers,
// nested maps, truthy strings, composite-decode fallbacks) without a DB.

func TestStringifyValue(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		raw  any
		want string
	}{
		{"nil", nil, ""},
		{"string", "hallo", "hallo"},
		{"bool true", true, "Ja"},
		{"bool false", false, "Nein"},
		{"integer float", float64(7), "7"},
		{"fractional float", float64(2.5), "2.5"},
		{"slice", []any{"a", float64(2), true}, "a, 2, Ja"},
		{"map sorted keys", map[string]any{"b": "2", "a": "1"}, "a: 1; b: 2"},
		{"unknown type via default", int32(9), "9"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stringifyValue(tc.raw); got != tc.want {
				t.Errorf("stringifyValue(%v) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestIsTruthy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw  any
		want bool
	}{
		{true, true},
		{false, false},
		{"true", true},
		{"Ja", true},
		{"1", true},
		{"yes", true},
		{"nein", false},
		{"", false},
		{float64(0), false},
		{float64(3), true},
		{nil, false},
		{int(1), false}, // unsupported type → default false
	}
	for _, tc := range cases {
		if got := isTruthy(tc.raw); got != tc.want {
			t.Errorf("isTruthy(%#v) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

func TestStatusLabelDE_UnknownPassesThrough(t *testing.T) {
	t.Parallel()

	if got := statusLabelDE(enrollmentModels.ChildStatusApproved); got != "Angenommen" {
		t.Errorf("known status = %q, want Angenommen", got)
	}
	if got := statusLabelDE("not_a_status"); got != "not_a_status" {
		t.Errorf("unknown status should pass through, got %q", got)
	}
}

func TestGradeLabel(t *testing.T) {
	t.Parallel()

	if got := gradeLabel(nil); got != "" {
		t.Errorf("nil grade = %q, want empty", got)
	}
	g := int16(3)
	if got := gradeLabel(&g); got != "3. Klasse" {
		t.Errorf("grade 3 = %q, want '3. Klasse'", got)
	}
}

func TestActivationSummary(t *testing.T) {
	t.Parallel()

	scheduled := &enrollmentModels.RequestChild{ActivationMode: enrollmentModels.ChildActivationScheduled}
	if got := activationSummary(scheduled); got != "Geplant" {
		t.Errorf("scheduled w/o date = %q, want Geplant", got)
	}
	immediate := &enrollmentModels.RequestChild{ActivationMode: enrollmentModels.ChildActivationImmediate}
	if got := activationSummary(immediate); got != "Sofort" {
		t.Errorf("immediate = %q, want Sofort", got)
	}
	on := timezone.Date("2026-08-01")
	dated := &enrollmentModels.RequestChild{ActivationMode: enrollmentModels.ChildActivationScheduled, ActivateOn: &on}
	if got := activationSummary(dated); got != "Geplant (01.08.2026)" {
		t.Errorf("dated = %q, want 'Geplant (01.08.2026)'", got)
	}
}

func TestFormatDayCodes(t *testing.T) {
	t.Parallel()

	if got := formatDayCodes(nil); got != "" {
		t.Errorf("empty = %q, want empty", got)
	}
	// Known codes map to German; an unknown code passes through verbatim.
	if got := formatDayCodes([]string{"mon", "FRI", "xyz"}); got != "Mo, Fr, xyz" {
		t.Errorf("got %q, want 'Mo, Fr, xyz'", got)
	}
}

func TestDecodeComposite_MarshalFailureReturnsFalse(t *testing.T) {
	t.Parallel()

	var dst map[string]string
	// A channel cannot be JSON-marshalled → decodeComposite reports false.
	if decodeComposite(make(chan int), &dst) {
		t.Error("expected decodeComposite to fail on an unmarshalable value")
	}
}

func TestFormatWeekdaySchedule_FallsBackOnGarbage(t *testing.T) {
	t.Parallel()

	// A bare string cannot decode into WeekdaySchedule (a map) → falls back
	// to the generic stringifier rather than dropping the answer.
	if got := formatWeekdaySchedule("not-a-schedule"); got != "not-a-schedule" {
		t.Errorf("garbage = %q, want passthrough", got)
	}
	sched := map[string]any{"mon": "07:30", "wed": "08:00"}
	if got := formatWeekdaySchedule(sched); got != "Mo 07:30, Mi 08:00" {
		t.Errorf("schedule = %q, want 'Mo 07:30, Mi 08:00'", got)
	}
}

func TestFormatCustomValue_WeekdayBoolean_GermanWeekOrder(t *testing.T) {
	t.Parallel()

	field := capability.FormField{Type: capability.FormFieldWeekdayBoolean}
	raw := map[string]any{"fri": true, "mon": true, "wed": false}
	assert.Equal(t, "Mo, Fr", formatCustomValue(field, raw))
}

func TestFormatCustomValue_WeekdayMode_GermanLabels(t *testing.T) {
	t.Parallel()

	field := capability.FormField{Type: capability.FormFieldWeekdayMode}
	raw := map[string]any{"mon": "bus", "wed": "pickup", "thu": "alone"}
	// Week order, alone/unset days skipped.
	assert.Equal(t, "Mo: fährt Bus, Mi: wird abgeholt", formatCustomValue(field, raw))
}

func TestFormatCustomValue_WeekdayMode_AllAloneIsExplicit(t *testing.T) {
	t.Parallel()

	field := capability.FormField{Type: capability.FormFieldWeekdayMode}
	raw := map[string]any{
		"mon": "alone",
		"tue": "alone",
		"wed": "alone",
		"thu": "alone",
		"fri": "alone",
	}
	assert.Equal(t, "Geht immer alleine", formatCustomValue(field, raw))
}

func TestFormatCustomValue_WeekdayMode_EmptyMapIsExplicitAllAlone(t *testing.T) {
	t.Parallel()

	field := capability.FormField{Type: capability.FormFieldWeekdayMode}
	assert.Equal(t, "Geht immer alleine", formatCustomValue(field, map[string]any{}))
}

func TestFormatCustomValue_WeekdayMode_NilStaysMissing(t *testing.T) {
	t.Parallel()

	field := capability.FormField{Type: capability.FormFieldWeekdayMode}
	assert.Empty(t, formatCustomValue(field, nil))
}

func TestFormatPhoneList_FallsBackOnGarbage(t *testing.T) {
	t.Parallel()

	if got := formatPhoneList("0151"); got != "0151" {
		t.Errorf("garbage = %q, want passthrough", got)
	}
	list := []any{map[string]any{"phone_number": "0151", "label": "Handy", "is_primary": true}}
	if got := formatPhoneList(list); got != "Handy: 0151 (primär)" {
		t.Errorf("list = %q, want 'Handy: 0151 (primär)'", got)
	}
}

func TestFormatContactList(t *testing.T) {
	t.Parallel()

	// Decode failure → generic fallback.
	if got := formatContactList("garbage"); got != "garbage" {
		t.Errorf("garbage = %q, want passthrough", got)
	}
	// A contact with no name renders the "Kontakt" placeholder; flags are shown.
	list := []any{
		map[string]any{"can_pickup": true, "is_emergency_contact": true},
		map[string]any{"first_name": "Mara", "last_name": "Lopez", "relationship_type": "Tante"},
	}
	got := formatContactList(list)
	want := "Kontakt (abholberechtigt, Notfallkontakt) | Mara Lopez (Tante)"
	if got != want {
		t.Errorf("contacts = %q, want %q", got, want)
	}
}

// An authorised pickup contact may carry only an email address
// (ContactEntry.Validate accepts email OR phone). The email is that
// contact's only channel and must appear in the export.
func TestFormatContactList_EmailOnlyContactKeepsEmail(t *testing.T) {
	t.Parallel()

	list := []any{
		map[string]any{
			"first_name":           "Jonas",
			"last_name":            "Berg",
			"relationship_type":    "Onkel",
			"email":                "jonas.berg@example.com",
			"can_pickup":           true,
			"is_emergency_contact": true,
		},
	}
	got := formatContactList(list)
	want := "Jonas Berg (Onkel; E-Mail: jonas.berg@example.com; abholberechtigt, Notfallkontakt)"
	if got != want {
		t.Errorf("email-only contact = %q, want %q", got, want)
	}
}

// When a contact has both a phone number and an email, phone is rendered
// first (priority channel in an outage) and the email is still kept.
func TestFormatContactList_PhoneAndEmailBothRendered(t *testing.T) {
	t.Parallel()

	list := []any{
		map[string]any{
			"first_name": "Lea",
			"last_name":  "Frey",
			"email":      "lea@example.com",
			"phone_numbers": []any{
				map[string]any{"phone_number": "0151123", "phone_type": "mobile"},
			},
		},
	}
	got := formatContactList(list)
	want := "Lea Frey (Tel: Mobil: 0151123; E-Mail: lea@example.com)"
	if got != want {
		t.Errorf("phone+email contact = %q, want %q", got, want)
	}
}

func TestTimeOrEmpty(t *testing.T) {
	t.Parallel()

	if got := timeOrEmpty(nil); got != "" {
		t.Errorf("nil = %q, want empty", got)
	}
	ts := time.Date(2026, 1, 2, 15, 4, 0, 0, time.UTC)
	if got := timeOrEmpty(&ts); got != "02.01.2026 15:04" {
		t.Errorf("got %q, want '02.01.2026 15:04'", got)
	}
}

func TestFormatOfferings_FallsBackToAvailableDaysAndOfferingID(t *testing.T) {
	t.Parallel()

	if got := formatOfferings(nil); got != "" {
		t.Errorf("empty = %q, want empty", got)
	}
	// No name + no selected days → "Angebot #<id>" using the offering's
	// available days as the displayed schedule.
	rows := []enrollmentService.ChildOfferingRow{
		{OfferingID: 42, AvailableDays: []string{"mon", "tue"}},
	}
	if got := formatOfferings(rows); got != "Angebot #42 (Mo, Di)" {
		t.Errorf("got %q, want 'Angebot #42 (Mo, Di)'", got)
	}
}

func TestFormatCustomValue_BooleanAndSelect(t *testing.T) {
	t.Parallel()

	boolField := capability.FormField{Type: capability.FormFieldBoolean}
	if got := formatCustomValue(boolField, false); got != "Nein" {
		t.Errorf("boolean false = %q, want Nein", got)
	}
	// Select with no matching option returns the raw value verbatim.
	selField := capability.FormField{
		Type:    capability.FormFieldSelect,
		Options: []capability.FormFieldOption{{Value: "a", Label: "Apfel"}},
	}
	if got := formatCustomValue(selField, "z"); got != "z" {
		t.Errorf("select no-match = %q, want 'z'", got)
	}
	if got := formatCustomValue(selField, "a"); got != "Apfel" {
		t.Errorf("select match = %q, want 'Apfel'", got)
	}
}
