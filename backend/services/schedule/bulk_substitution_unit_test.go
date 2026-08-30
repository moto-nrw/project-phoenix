package schedule

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeBulkDates_DedupesAndSortsAscending(t *testing.T) {
	t.Parallel()

	today := timezone.NewDate(2030, 8, 26)
	later := today.AddDays(3)
	sooner := today.AddDays(1)

	out, err := normalizeBulkDates([]timezone.Date{later, sooner, later, sooner})

	require.NoError(t, err)
	// Ascending order is the day-lock ordering requirement, not cosmetics.
	assert.Equal(t, []timezone.Date{sooner, later}, out)
}

func TestNormalizeBulkDates_RejectsEmptyInput(t *testing.T) {
	t.Parallel()

	_, err := normalizeBulkDates(nil)

	var de *DeviationError
	require.ErrorAs(t, err, &de)
	assert.Equal(t, http.StatusBadRequest, de.Status)
	assert.Contains(t, de.ClientMsg, "must not be empty")
}

func TestNormalizeBulkDates_RejectsPastDates(t *testing.T) {
	t.Parallel()

	_, err := normalizeBulkDates([]timezone.Date{timezone.TodayDate().AddDays(-1)})

	var de *DeviationError
	require.ErrorAs(t, err, &de)
	assert.Equal(t, http.StatusBadRequest, de.Status)
	assert.Contains(t, de.ClientMsg, "past")
}

func TestNormalizeBulkDates_CapsAtMaxDatesAfterDedupe(t *testing.T) {
	t.Parallel()

	today := timezone.TodayDate()
	dates := make([]timezone.Date, 0, MaxBulkSubstitutionDates+1)
	for i := 0; i <= MaxBulkSubstitutionDates; i++ {
		dates = append(dates, today.AddDays(i))
	}

	_, err := normalizeBulkDates(dates)

	var de *DeviationError
	require.ErrorAs(t, err, &de)
	assert.Equal(t, http.StatusBadRequest, de.Status)
	assert.Contains(t, de.ClientMsg, "at most 31 dates")

	// Duplicates do not count against the cap: 32 raw entries collapsing to
	// 31 distinct dates pass.
	withDup := append(dates[:MaxBulkSubstitutionDates], dates[0])
	out, err := normalizeBulkDates(withDup)
	require.NoError(t, err)
	assert.Len(t, out, MaxBulkSubstitutionDates)
}

func TestBulkDayError_PrefixesDeviationErrorWithFailingDate(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, time.August, 18)
	cause := errors.New("root cause")
	inner := &DeviationError{
		Status:    http.StatusConflict,
		Code:      "staff_conflict",
		ClientMsg: "die Ersatzperson ist bereits verplant",
		Cause:     cause,
	}

	err := bulkDayError(date, inner)

	var de *DeviationError
	require.ErrorAs(t, err, &de)
	assert.Equal(t, http.StatusConflict, de.Status)
	assert.Equal(t, "staff_conflict", de.Code)
	assert.Equal(t, "18.08.2026: die Ersatzperson ist bereits verplant", de.ClientMsg)
	assert.Same(t, cause, de.Cause)
}

func TestBulkDayError_PassesNonDeviationErrorsThrough(t *testing.T) {
	t.Parallel()

	plain := errors.New("boom")
	assert.Same(t, plain, bulkDayError(timezone.TodayDate(), plain))
}
