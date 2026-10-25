package domain

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCatererInvitationExpiresAtTheExpirationInstant(t *testing.T) {
	t.Parallel()
	expires := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	invitation := CatererInvitation{ExpiresAt: expires}

	assert.NoError(t, invitation.Redeemable(expires.Add(-time.Nanosecond)))
	assert.True(t, invitation.Open(expires.Add(-time.Nanosecond)))
	for _, now := range []time.Time{expires, expires.Add(time.Nanosecond)} {
		assert.ErrorIs(t, invitation.Redeemable(now), ErrInvitationExpired)
		assert.False(t, invitation.Open(now))
	}
}
