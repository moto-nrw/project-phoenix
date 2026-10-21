package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// PHX_TEST_LEFTOVERS=test must check a test once, after every teardown it
// registered. SetupTestDB runs again inside fixtures and subtests; a check
// registered there would run before the fixture's own teardown and blame the
// next test for the rows that teardown removes.
func TestPerTestLeftoverCheckIsClaimedOncePerTopLevelTest(t *testing.T) {
	t.Parallel()
	run := &iterationTB{TB: t, name: t.Name() + "Probe"}
	assert.True(t, claimPerTestLeftoverCheck(run), "the first SetupTestDB of a test registers the check")
	assert.False(t, claimPerTestLeftoverCheck(run), "a later SetupTestDB of the same test must not register another")
	assert.False(t, claimPerTestLeftoverCheck(&iterationTB{TB: t, name: run.name + "/sub"}),
		"a subtest of a checked test is checked with its parent")
	run.finish()

	orphan := &iterationTB{TB: t, name: t.Name() + "Unchecked/sub"}
	assert.True(t, claimPerTestLeftoverCheck(orphan), "a subtest whose parent never set up the database is checked itself")
	assert.False(t, claimPerTestLeftoverCheck(orphan), "but only once")
	orphan.finish()

	rerun := &iterationTB{TB: t, name: run.name}
	assert.True(t, claimPerTestLeftoverCheck(rerun), "a -count rerun is checked again")
	rerun.finish()
}
