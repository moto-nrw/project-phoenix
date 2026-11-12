package application

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

var accountProofNow = time.Date(2027, 3, 1, 9, 0, 0, 0, time.UTC)

func accountProofRegistrations(now time.Time) *HolidayRegistrations {
	return &HolidayRegistrations{deps: HolidayRegistrationDependencies{Health: boundCipher{}, Now: func() time.Time { return now }}}
}

// The proof of the prefill names the account for the link of one period of
// one Träger and counts for a day (ADR 0216 point 2). Anything else is no
// proof, and the registration stays without an account.
func TestHolidayParentAccountProofOpensOnlyForItsLinkAndDay(t *testing.T) {
	t.Parallel()
	link := enrollment.HolidayRegistrationLink{OrganizationID: 7, PeriodID: 70}
	proof, err := NewHolidayParentAccountProofs(boundCipher{}, func() time.Time { return accountProofNow }).HolidayParentAccountProof(7, 70, 501)
	require.NoError(t, err)
	require.NotEmpty(t, proof)

	resolve := func(link enrollment.HolidayRegistrationLink, now time.Time, input enrollment.HolidayRegistrationInput) int64 {
		resolved := accountProofRegistrations(now).resolveHolidayParentAccount(link, input)
		assert.Empty(t, resolved.ParentAccountProof, "the proof leaves the input")
		return resolved.ParentAccountID
	}
	assert.Equal(t, int64(501), resolve(link, accountProofNow, enrollment.HolidayRegistrationInput{ParentAccountProof: proof}))
	assert.Equal(t, int64(501), resolve(link, accountProofNow.Add(enrollment.HolidayParentAccountProofLifetime), enrollment.HolidayRegistrationInput{ParentAccountProof: " " + proof + " "}),
		"the proof counts to the end of its day")

	for name, tc := range map[string]struct {
		link  enrollment.HolidayRegistrationLink
		now   time.Time
		proof string
	}{
		"expired":        {link, accountProofNow.Add(enrollment.HolidayParentAccountProofLifetime + time.Second), proof},
		"other period":   {enrollment.HolidayRegistrationLink{OrganizationID: 7, PeriodID: 71}, accountProofNow, proof},
		"other Träger":   {enrollment.HolidayRegistrationLink{OrganizationID: 8, PeriodID: 70}, accountProofNow, proof},
		"garbage":        {link, accountProofNow, "not-a-proof"},
		"empty":          {link, accountProofNow, ""},
		"truncated":      {link, accountProofNow, proof[:len(proof)/2]},
		"no base64 text": {link, accountProofNow, "%%%"},
	} {
		assert.Zero(t, resolve(tc.link, tc.now, enrollment.HolidayRegistrationInput{ParentAccountProof: tc.proof}), name)
	}
	assert.Zero(t, resolve(link, accountProofNow, enrollment.HolidayRegistrationInput{ParentAccountID: 501}),
		"an account the caller puts in is dropped")

	_, err = NewHolidayParentAccountProofs(boundCipher{}, nil).HolidayParentAccountProof(7, 70, 0)
	assert.Error(t, err, "a proof names an account")
}
