package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The shared projection both portals render (#3349): staff must not see a
// different consent state than parents, so the rules live here once.

func TestCurrentStudentConsentsReportsRecordedTimestamps(t *testing.T) {
	t.Parallel()

	grantedAt := time.Date(2026, time.August, 20, 9, 0, 0, 0, time.UTC)
	states := CurrentStudentConsents(StudentConsentSnapshot{
		StudentID:                91,
		AGBAcceptedAt:            &grantedAt,
		DataProcessingAcceptedAt: &grantedAt,
		PhotoConsentGivenAt:      &grantedAt,
	}, false, nil)

	require.Len(t, states, 4)
	assert.Equal(t, StudentConsentAGB, states[0].Key)
	assert.Equal(t, StudentConsentStateGranted, states[0].State)
	assert.Equal(t, grantedAt, *states[0].ChangedAt)
	assert.Equal(t, StudentConsentStateGranted, states[1].State)
	assert.Equal(t, StudentConsentStateNotRecorded, states[2].State)
	assert.Nil(t, states[2].ChangedAt)
	assert.Equal(t, StudentConsentStateGranted, states[3].State)
}

// The three enrollment acknowledgements are preconditions of the booking, not
// choices a portal may take back.
func TestCurrentStudentConsentsOffersOnlyThePhotoConsentAsWithdrawable(t *testing.T) {
	t.Parallel()

	grantedAt := time.Date(2026, time.August, 20, 9, 0, 0, 0, time.UTC)
	states := CurrentStudentConsents(StudentConsentSnapshot{
		StudentID:                91,
		AGBAcceptedAt:            &grantedAt,
		DataProcessingAcceptedAt: &grantedAt,
		EmailContactAcceptedAt:   &grantedAt,
		PhotoConsentGivenAt:      &grantedAt,
	}, true, nil)

	require.Len(t, states, 4)
	for _, state := range states[:3] {
		assert.False(t, state.CanWithdraw, "%s must not be withdrawable", state.Key)
		assert.False(t, state.CanGrant, "%s must not be grantable", state.Key)
	}
	assert.True(t, states[3].CanWithdraw)
	assert.False(t, states[3].CanGrant)
}

func TestCurrentStudentConsentsReportsAWithdrawnPhotoConsent(t *testing.T) {
	t.Parallel()

	withdrawnAt := time.Date(2026, time.August, 31, 15, 0, 0, 0, time.UTC)

	t.Run("withdrawal is offered for a new grant", func(t *testing.T) {
		states := CurrentStudentConsents(StudentConsentSnapshot{StudentID: 91}, true, &withdrawnAt)
		require.Len(t, states, 4)
		assert.Equal(t, StudentConsentStateWithdrawn, states[3].State)
		assert.Equal(t, withdrawnAt, *states[3].ChangedAt)
		assert.False(t, states[3].CanWithdraw)
		assert.True(t, states[3].CanGrant)
	})

	t.Run("a caller without the permission cannot grant", func(t *testing.T) {
		states := CurrentStudentConsents(StudentConsentSnapshot{StudentID: 91}, false, &withdrawnAt)
		assert.False(t, states[3].CanGrant)
	})

	// A live consent is the current state by definition: a withdrawal older
	// than the grant must not override it.
	t.Run("a live consent wins over an earlier withdrawal", func(t *testing.T) {
		grantedAt := withdrawnAt.Add(time.Hour)
		states := CurrentStudentConsents(
			StudentConsentSnapshot{StudentID: 91, PhotoConsentGivenAt: &grantedAt}, true, &withdrawnAt)
		assert.Equal(t, StudentConsentStateGranted, states[3].State)
		assert.Equal(t, grantedAt, *states[3].ChangedAt)
	})

	t.Run("no recorded withdrawal reads as never recorded", func(t *testing.T) {
		states := CurrentStudentConsents(StudentConsentSnapshot{StudentID: 91}, true, nil)
		assert.Equal(t, StudentConsentStateNotRecorded, states[3].State)
		assert.Nil(t, states[3].ChangedAt)
	})
}
