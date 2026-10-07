package compose

import (
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBulkDayErrorNamesTheDay pins that a refused Sammel-Vertretung day keeps
// its code and field and names the day in details as YYYY-MM-DD (#2516), so
// the client words the refusal without reading the sentence.
func TestBulkDayErrorNamesTheDay(t *testing.T) {
	t.Parallel()

	day := timezone.NewDate(2030, 8, 27)
	inner := timetable.DeviationBadRequest("die Terminauswahl ist ungültig").
		WithCode(timetable.CodeInstanceSelectionInvalid).OnField("instance_ids")

	var de *timetable.DeviationError
	require.ErrorAs(t, bulkDayError(day, inner), &de)
	assert.Equal(t, http.StatusBadRequest, de.Status)
	assert.Equal(t, timetable.CodeInstanceSelectionInvalid, de.Code)
	assert.Equal(t, "instance_ids", de.Field)
	assert.Equal(t, timetable.RefusalValues{Date: "27.08.2030"}, de.Details)
	assert.Equal(t, "27.08.2030: die Terminauswahl ist ungültig", de.ClientMsg)
}

// TestBulkDatesRefusalCodes pins the codes of the date selection.
func TestBulkDatesRefusalCodes(t *testing.T) {
	t.Parallel()

	today := timezone.NewDate(2030, 8, 26)
	clock := func() timezone.Date { return today }
	tooMany := make([]timezone.Date, 0, timetable.MaxBulkSubstitutionDates+1)
	for i := 0; i <= timetable.MaxBulkSubstitutionDates; i++ {
		tooMany = append(tooMany, today.AddDays(i))
	}
	cases := []struct {
		name       string
		dates      []timezone.Date
		wantCode   string
		wantDetail timetable.RefusalValues
	}{
		{"empty", nil, timetable.CodeDatesRequired, timetable.RefusalValues{}},
		{"past", []timezone.Date{today.AddDays(-1)}, timetable.CodeDatesInPast, timetable.RefusalValues{Date: "25.08.2030"}},
		{"too many", tooMany, timetable.CodeTooManyDates, timetable.RefusalValues{Max: timetable.MaxBulkSubstitutionDates}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := normalizeBulkDates(tc.dates, clock)
			var de *timetable.DeviationError
			require.ErrorAs(t, err, &de)
			assert.Equal(t, http.StatusBadRequest, de.Status)
			assert.Equal(t, tc.wantCode, de.Code)
			assert.Equal(t, "dates", de.Field)
			assert.Equal(t, tc.wantDetail, de.Details)
		})
	}
}

// TestSubstituteRefusalCodes pins the codes of the substitute rules.
func TestSubstituteRefusalCodes(t *testing.T) {
	t.Parallel()

	const staffID int64 = 4242
	sub := timetable.DeviationSubstitutionInput{AbsentStaffID: staffID, SubstituteStaffID: staffID}
	err := validateSubstituteAvailable(timetable.ApplyDeviationsInput{}, sub, &deviationReadSet{})

	var de *timetable.DeviationError
	require.ErrorAs(t, err, &de)
	assert.Equal(t, timetable.CodeSubstituteSelf, de.Code)
	assert.Equal(t, "substitute_staff_id", de.Field)
	assert.Equal(t, "die abwesende Person kann sich nicht selbst vertreten", de.ClientMsg)

	err = validateExplicitScopeInstances(timezone.NewDate(2030, 1, 1), &[]int64{}, &deviationReadSet{})
	require.ErrorAs(t, err, &de)
	assert.Equal(t, timetable.CodeInstancesRequired, de.Code)
}

// TestTemplateGradeLimitNamesTheCap pins that a Jahrgang above the school's
// maximum carries its code and the requested grade and the cap (#2516).
func TestTemplateGradeLimitNamesTheCap(t *testing.T) {
	t.Parallel()

	gradeSix := int16(6)
	err := ValidateTemplateTargetGradeLimit(4, nil, activitiesModel.TargetGroupTypeJahrgang, &gradeSix)
	require.ErrorIs(t, err, timetable.ErrTemplateTargetGradeExceedsLimit)
	coded, ok := timetable.AsCoded(err)
	require.True(t, ok)
	assert.Equal(t, timetable.CodeTemplateGradeAboveMax, coded.Code)
	assert.Equal(t, timetable.RefusalValues{Grade: 6, Max: 4}, coded.Values)
}
