package enrollment

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- toCareOfferingResponse ----------------------------------------------

func TestToCareOfferingResponse_StringifiesIDs(t *testing.T) {
	t.Parallel()

	desc := "Spätbetreuung mit Hausaufgabenhilfe"
	cap := 30
	price := 12000
	activityGroupID := int64(8888)
	o := &enrollmentModels.CareOffering{
		ID: 1234, CreatedAt: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
		PhaseID:             5678,
		ActivityGroupID:     &activityGroupID,
		Name:                "OGS-Nachmittag",
		Description:         &desc,
		DaysOfWeekMode:      enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:       []string{"mon", "tue", "wed", "thu", "fri"},
		IncludesHolidayCare: true,
		IncludesLunch:       true,
		Capacity:            &cap,
		PriceCents:          &price,
		IsActive:            true,
		SortOrder:           10,
	}
	out := toCareOfferingResponse(o)
	assert.Equal(t, "1234", out.ID, "int64 ID stringified per CLAUDE rule 4")
	assert.Equal(t, "5678", out.PhaseID)
	require.NotNil(t, out.ActivityGroupID)
	assert.Equal(t, "8888", *out.ActivityGroupID)
	assert.Equal(t, "OGS-Nachmittag", out.Name)
	require.NotNil(t, out.Description)
	assert.Equal(t, desc, *out.Description)
	assert.Equal(t, []string{"mon", "tue", "wed", "thu", "fri"}, out.AvailableDays)
	assert.True(t, out.IncludesHolidayCare)
	assert.True(t, out.IncludesLunch)
	require.NotNil(t, out.Capacity)
	assert.Equal(t, 30, *out.Capacity)
	require.NotNil(t, out.PriceCents)
	assert.Equal(t, 12000, *out.PriceCents)
}

func TestToCareOfferingResponse_PreservesNilPointers(t *testing.T) {
	t.Parallel()

	// Optional pointer fields → nil in / nil out so omitempty drops them
	// from JSON. The form needs to distinguish "no capacity" (unlimited)
	// from "capacity: 0" (full).
	o := &enrollmentModels.CareOffering{
		ID:             1234,
		PhaseID:        5678,
		Name:           "Basic",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
	}
	out := toCareOfferingResponse(o)
	assert.Nil(t, out.ActivityGroupID)
	assert.Nil(t, out.Description)
	assert.Nil(t, out.Capacity)
	assert.Nil(t, out.PriceCents)
}

func TestToCareOfferingResponseIncludesAvailabilityRule(t *testing.T) {
	t.Parallel()

	o := &enrollmentModels.CareOffering{AvailabilityRule: &enrollmentModels.CareOfferingAvailabilityRule{
		Match: enrollmentModels.AvailabilityMatchAll,
		Conditions: []enrollmentModels.CareOfferingAvailabilityCondition{{
			Source: enrollmentModels.AvailabilitySourceGradeLevel, Operator: enrollmentModels.AvailabilityOperatorIn, Value: []int{1, 2},
		}},
	}}
	out := toCareOfferingResponse(o)
	require.NotNil(t, out.AvailabilityRule)
	assert.Equal(t, []int{1, 2}, out.AvailabilityRule.Conditions[0].Value)
}

// --- renderPublicEnrollmentError -----------------------------------------

func TestRenderPublicEnrollmentError_EnrollmentDisabled404WithCode(t *testing.T) {
	t.Parallel()

	// The public catalog endpoint returns 404 with a stable code when
	// enrollment is disabled, so the parent form can render "Anmeldung
	// derzeit nicht möglich" instead of a generic error.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	renderPublicEnrollmentError(w, r, enrollmentService.ErrEnrollmentDisabled)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), ErrCodeEnrollmentDisabled)
}

func TestRenderPublicEnrollmentError_WindowClosed404WithCode(t *testing.T) {
	t.Parallel()

	// A stale parent link to a phase whose enrollment window is closed
	// returns 404 with a stable code so the form explains the
	// Anmeldefrist instead of "nicht gefunden".
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	renderPublicEnrollmentError(w, r, enrollmentService.ErrEnrollmentWindowClosed)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), ErrCodeEnrollmentWindowClosed)
}

func TestRenderPublicEnrollmentError_OtherErrorPlain404(t *testing.T) {
	t.Parallel()

	// Any other error → bare 404 (no stable code). The catalog endpoint
	// intentionally hides the difference between "phase not found" and
	// "phase not yet open" from unauthenticated callers.
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/x", nil)
	renderPublicEnrollmentError(w, r, errors.New("phase not found"))
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.NotContains(t, w.Body.String(), ErrCodeEnrollmentDisabled)
}

// --- toPhaseResponse -----------------------------------------------------

func TestToPhaseResponse_StringifiesIDsAndFormatsDates(t *testing.T) {
	t.Parallel()

	openAt := time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC)
	closeAt := time.Date(2026, 8, 31, 23, 59, 0, 0, time.UTC)
	deadline := time.Date(2027, 6, 30, 18, 0, 0, 0, time.UTC)
	schemaID := int64(7777)
	srcPhase := int64(4321)
	mode := capability.PhaseRolloverModeOptOut

	p := &capability.Phase{

		ID:        1234,
		CreatedAt: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 2, 12, 0, 0, 0, time.UTC),

		Name:                      "Schuljahr 2026/27",
		Kind:                      capability.PhaseKindSchoolYear,
		ServiceStartDate:          "2026-09-01",
		ServiceEndDate:            "2027-07-31",
		EnrollmentOpenAt:          &openAt,
		EnrollmentCloseAt:         &closeAt,
		FormSchemaID:              &schemaID,
		ShowStatusReasonToParent:  true,
		CareOverflowMode:          capability.PhaseCareOverflowWaitlist,
		CareOfferingSelectionMode: capability.PhaseCareOfferingSelectionAtLeastOne,
		IsActive:                  true,
		RolloverSourcePhaseID:     &srcPhase,
		RolloverMode:              &mode,
		RolloverAutoApprove:       true,
		RolloverDeadline:          &deadline,
		RolloverBumpsGrade:        true,
	}
	out := toPhaseResponse(p)
	assert.Equal(t, "1234", out.ID, "int64 ID stringified per CLAUDE rule 4")
	assert.Equal(t, "Schuljahr 2026/27", out.Name)
	assert.Equal(t, "2026-09-01", out.ServiceStartDate, "date fields are YYYY-MM-DD")
	assert.Equal(t, "2027-07-31", out.ServiceEndDate)
	require.NotNil(t, out.EnrollmentOpenAt)
	assert.Equal(t, "2026-08-01T08:00:00Z", *out.EnrollmentOpenAt, "timestamps are RFC3339")
	require.NotNil(t, out.EnrollmentCloseAt)
	assert.Equal(t, "2026-08-31T23:59:00Z", *out.EnrollmentCloseAt)
	require.NotNil(t, out.FormSchemaID)
	assert.Equal(t, "7777", *out.FormSchemaID)
	require.NotNil(t, out.RolloverSourcePhaseID)
	assert.Equal(t, "4321", *out.RolloverSourcePhaseID)
	require.NotNil(t, out.RolloverMode)
	assert.Equal(t, capability.PhaseRolloverModeOptOut, *out.RolloverMode)
	assert.True(t, out.RolloverAutoApprove)
	require.NotNil(t, out.RolloverDeadline)
	assert.Equal(t, "2027-06-30T18:00:00Z", *out.RolloverDeadline)
	assert.True(t, out.RolloverBumpsGrade)
	assert.True(t, out.ShowStatusReasonToParent)
	assert.Equal(t, capability.PhaseCareOverflowWaitlist, out.CareOverflowMode)
	assert.Equal(t, capability.PhaseCareOfferingSelectionAtLeastOne, out.CareOfferingSelectionMode)
	assert.Equal(t, "2026-04-01T12:00:00Z", out.CreatedAt)
	assert.Equal(t, "2026-04-02T12:00:00Z", out.UpdatedAt)
}

func TestToPhaseResponse_NilOptionalPointersStayNil(t *testing.T) {
	t.Parallel()

	// Fresh, non-rollover phase with no enrollment window — every
	// optional pointer field must round-trip as nil so omitempty drops
	// them from JSON. A non-rollover phase shouldn't render the review
	// queue link in the admin UI.
	p := &capability.Phase{
		ID:               1234,
		Name:             "Fresh Phase",
		Kind:             capability.PhaseKindSchoolYear,
		ServiceStartDate: "2026-09-01",
		ServiceEndDate:   "2027-07-31",
		CareOverflowMode: capability.PhaseCareOverflowWaitlist,
	}
	out := toPhaseResponse(p)
	assert.Nil(t, out.EnrollmentOpenAt)
	assert.Nil(t, out.EnrollmentCloseAt)
	assert.Nil(t, out.FormSchemaID)
	assert.Nil(t, out.RolloverSourcePhaseID)
	assert.Nil(t, out.RolloverMode)
	assert.Nil(t, out.RolloverDeadline)
	assert.False(t, out.RolloverAutoApprove)
	assert.False(t, out.RolloverBumpsGrade)
}

// --- toFormSchemaResponse ------------------------------------------------

func TestToFormSchemaResponse_StringifiesIDs(t *testing.T) {
	t.Parallel()

	created := time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC)
	s := &capability.FormSchema{
		ID: 1234, CreatedAt: created,
		Name:      "Schuljahr",
		Version:   2,
		IsActive:  true,
		CreatedBy: 4321,
		Fields: []capability.FormField{
			{Key: "allergies", Label: "Allergien", Type: capability.FormFieldText, SortOrder: 0},
		},
	}
	out := toFormSchemaResponse(s)
	assert.Equal(t, "1234", out.ID, "int64 ID stringified")
	assert.Equal(t, "4321", out.CreatedBy, "created_by also stringified — frontend treats as opaque string")
	assert.Equal(t, "Schuljahr", out.Name)
	assert.Equal(t, 2, out.Version)
	assert.True(t, out.IsActive)
	assert.Equal(t, created, out.CreatedAt, "CreatedAt round-trips as time.Time (not formatted)")
	require.Len(t, out.Fields, 1)
	assert.Equal(t, "allergies", out.Fields[0].Key)
}

func TestToFormSchemaResponse_PreservesFieldOrder(t *testing.T) {
	t.Parallel()

	// Fields ordering matters — the form renders them in sort_order.
	// The shaper must not reorder, even if sort_order is set out of
	// sequence.
	s := &capability.FormSchema{
		ID:        1234,
		Name:      "Test",
		Version:   1,
		CreatedBy: 4321,
		Fields: []capability.FormField{
			{Key: "z", Label: "Z", Type: capability.FormFieldText, SortOrder: 5},
			{Key: "a", Label: "A", Type: capability.FormFieldText, SortOrder: 0},
		},
	}
	out := toFormSchemaResponse(s)
	require.Len(t, out.Fields, 2)
	assert.Equal(t, "z", out.Fields[0].Key, "shaper does not reorder — that's the form's job")
	assert.Equal(t, "a", out.Fields[1].Key)
}

func TestToPublicFormSchemaResponse_OmitsRawLegalBlocks(t *testing.T) {
	t.Parallel()

	s := &capability.FormSchema{
		ID:      1234,
		Version: 2,
		Fields:  []capability.FormField{},
		LegalBlocks: []capability.FormLegalBlock{
			{
				Key:       "photo",
				Kind:      capability.LegalBlockKindConsent,
				Title:     "Fotoeinwilligung",
				Label:     "Fotos erlauben",
				Text:      "disabled draft text must not leak",
				Required:  false,
				Enabled:   false,
				SortOrder: 10,
			},
		},
	}

	out := toPublicFormSchemaResponse(s)
	require.NotNil(t, out)
	raw, err := json.Marshal(out)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "legal_blocks")
	assert.NotContains(t, string(raw), "disabled draft text")
}
