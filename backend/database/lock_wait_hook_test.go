package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCanWaitForLock(t *testing.T) {
	t.Parallel()
	for _, query := range []string{
		"SELECT pg_advisory_xact_lock(1)",
		"SELECT id FROM users.students FOR UPDATE",
		"SELECT id FROM auth.accounts FOR SHARE",
		"LOCK TABLE users.students IN SHARE MODE",
	} {
		assert.True(t, canWaitForLock(query), query)
	}
	assert.False(t, canWaitForLock("SELECT pg_try_advisory_xact_lock(1)"))
	assert.False(t, canWaitForLock("SELECT id FROM users.students"))
}
