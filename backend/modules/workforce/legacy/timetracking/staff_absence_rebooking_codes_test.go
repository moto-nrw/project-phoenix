package timetracking

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// businessRejection is the shape api/common renders as code and details.
type businessRejection interface {
	error
	ErrorCode() string
	ErrorDetails() any
}

// A blocked rebooking names its own code and values (#2514), so the client
// words the refusal from the catalog instead of showing the German reason.
func TestValidateRebookingRequest_NamesEachRefusalByCode(t *testing.T) {
	t.Parallel()

	tooMany := make([]int64, maxRebookedAbsences+1)
	for index := range tooMany {
		tooMany[index] = int64(index + 1)
	}
	cases := []struct {
		name    string
		req     RebookAbsencesRequest
		reason  string
		code    string
		details any
	}{
		{
			name: "nothing selected",
			req:  RebookAbsencesRequest{StaffID: 1, ActorAccountID: 2, DryRun: true},
			code: workforce.RebookingNothingSelectedCode,
		},
		{
			name:    "too many",
			req:     RebookAbsencesRequest{StaffID: 1, ActorAccountID: 2, AbsenceIDs: tooMany, DryRun: true},
			code:    workforce.RebookingTooManyCode,
			details: workforce.RebookingValues{Limit: maxRebookedAbsences},
		},
		{
			name: "reason missing on write",
			req:  RebookAbsencesRequest{StaffID: 1, ActorAccountID: 2, AbsenceIDs: []int64{7}},
			code: workforce.RebookingReasonRequiredCode,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := validateRebookingRequest(tc.req, tc.reason)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrAbsenceRebookingBlocked)
			var rejection businessRejection
			require.True(t, errors.As(err, &rejection))
			assert.Equal(t, tc.code, rejection.ErrorCode())
			assert.Equal(t, tc.details, rejection.ErrorDetails())
		})
	}
}

// The public adapter wraps the retained error as TimeTrackingError; the code
// and the values must still be reachable through the chain.
func TestRebookingBlocked_SurvivesThePublicWrapper(t *testing.T) {
	t.Parallel()

	cause := rebookingBlocked(workforce.RebookingSickReportCode, workforce.RebookingValues{Day: "03.08.2026"},
		"Die Krankmeldung vom %s lässt sich nicht umbuchen.", "03.08.2026")
	wrapped := &workforce.TimeTrackingError{Kind: workforce.ErrAbsenceRebookingBlocked, Cause: cause}

	var rejection businessRejection
	require.True(t, errors.As(wrapped, &rejection))
	assert.Equal(t, workforce.RebookingSickReportCode, rejection.ErrorCode())
	assert.Equal(t, workforce.RebookingValues{Day: "03.08.2026"}, rejection.ErrorDetails())
	assert.Equal(t, "Die Krankmeldung vom 03.08.2026 lässt sich nicht umbuchen.", cause.Error())
}
