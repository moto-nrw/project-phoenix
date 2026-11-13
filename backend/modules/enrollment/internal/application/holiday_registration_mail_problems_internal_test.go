package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

type mailStatesDouble struct {
	problems map[string]string
	err      error
	calls    int
	keys     []string
	org      int64
}

func (d *mailStatesDouble) HolidayMailProblems(_ context.Context, organizationID int64, keys []string) (map[string]string, error) {
	d.calls++
	d.org, d.keys = organizationID, keys
	return d.problems, d.err
}

// #6737: the Träger sees which receipt or acceptance mail did not reach the
// parents, for all registrations of the list in one read.
func TestAddMailProblemsNamesTheUndeliveredMailsInOneRead(t *testing.T) {
	t.Parallel()
	states := &mailStatesDouble{problems: map[string]string{
		"holiday-registration-received:1": enrollment.HolidayMailFailed,
		"holiday-registration-accepted:1": enrollment.HolidayMailBlocked,
		"holiday-registration-accepted:3": enrollment.HolidayMailUnknown,
	}}
	review := &HolidayRegistrationReview{mailStates: states}
	registrations := []enrollment.HolidayRegistrationSummary{{ID: 1}, {ID: 2}, {ID: 3}}

	err := review.addMailProblems(context.Background(), 7, []*enrollment.HolidayRegistrationSummary{&registrations[0], &registrations[1], &registrations[2]})

	require.NoError(t, err)
	assert.Equal(t, 1, states.calls, "one read for the whole list")
	assert.Equal(t, int64(7), states.org)
	assert.ElementsMatch(t, []string{
		"holiday-registration-received:1", "holiday-registration-accepted:1",
		"holiday-registration-received:2", "holiday-registration-accepted:2",
		"holiday-registration-received:3", "holiday-registration-accepted:3",
	}, states.keys)
	assert.Equal(t, []enrollment.HolidayRegistrationMailProblem{
		{Kind: enrollment.MailKindHolidayRegistrationReceived, State: enrollment.HolidayMailFailed},
		{Kind: enrollment.MailKindHolidayRegistrationAccepted, State: enrollment.HolidayMailBlocked},
	}, registrations[0].MailProblems)
	assert.Empty(t, registrations[1].MailProblems)
	assert.Equal(t, []enrollment.HolidayRegistrationMailProblem{
		{Kind: enrollment.MailKindHolidayRegistrationAccepted, State: enrollment.HolidayMailUnknown},
	}, registrations[2].MailProblems)
}

func TestAddMailProblemsReportsAFailedRead(t *testing.T) {
	t.Parallel()
	review := &HolidayRegistrationReview{mailStates: &mailStatesDouble{err: errors.New("database gone")}}

	err := review.addMailProblems(context.Background(), 7, []*enrollment.HolidayRegistrationSummary{{ID: 1}})

	require.ErrorContains(t, err, "database gone")
}

func TestAddMailProblemsWithoutMailStatesReadsNothing(t *testing.T) {
	t.Parallel()
	registration := enrollment.HolidayRegistrationSummary{ID: 1}

	require.NoError(t, (&HolidayRegistrationReview{}).addMailProblems(context.Background(), 7, []*enrollment.HolidayRegistrationSummary{&registration}))
	assert.Empty(t, registration.MailProblems)
}
