package users

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

func TestParentRequestIsPast(t *testing.T) {
	t.Parallel()

	today := timezone.NewDate(2026, 8, 29)

	// Today is deliberately NOT past: a request for today can still be applied
	// for the rest of the day. This is the Berlin-midnight boundary the whole
	// feature turns on.
	assert.False(t, ParentRequestIsPast(today, today))
	assert.False(t, ParentRequestIsPast(today.AddDays(1), today))
	assert.True(t, ParentRequestIsPast(today.AddDays(-1), today))

	// No effective scope (weekly care plan, Stammdaten) is never past.
	assert.False(t, ParentRequestIsPast(timezone.Date(""), today))
}
