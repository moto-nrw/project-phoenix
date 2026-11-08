package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// RequestChild has three small predicates the rollover worker + the
// decision service depend on. Pure logic, no DB.

// --- IsTerminal -----------------------------------------------------------

// IsTerminal short-circuits the deadline worker and the admin decision
// pipeline. If a row is terminal, no further state transition is
// possible (waitlisted is intentionally NOT terminal — it can still be
// promoted to approved).

func TestRequestChild_IsTerminal_ApprovedIsTerminal(t *testing.T) {
	t.Parallel()

	c := &RequestChild{Status: ChildStatusApproved}
	assert.True(t, c.IsTerminal())
}

func TestRequestChild_IsTerminal_RejectedIsTerminal(t *testing.T) {
	t.Parallel()

	c := &RequestChild{Status: ChildStatusRejected}
	assert.True(t, c.IsTerminal())
}

func TestRequestChild_IsTerminal_WithdrawnIsTerminal(t *testing.T) {
	t.Parallel()

	c := &RequestChild{Status: ChildStatusWithdrawn}
	assert.True(t, c.IsTerminal())
}

func TestRequestChild_IsTerminal_WaitlistedNotTerminal(t *testing.T) {
	t.Parallel()

	// Waitlisted can be promoted to approved when capacity opens up,
	// so it's intentionally non-terminal.
	c := &RequestChild{Status: ChildStatusWaitlisted}
	assert.False(t, c.IsTerminal())
}

func TestRequestChild_IsTerminal_SubmittedNotTerminal(t *testing.T) {
	t.Parallel()

	c := &RequestChild{Status: ChildStatusSubmitted}
	assert.False(t, c.IsTerminal())
}

func TestRequestChild_IsTerminal_UnderReviewNotTerminal(t *testing.T) {
	t.Parallel()

	c := &RequestChild{Status: ChildStatusUnderReview}
	assert.False(t, c.IsTerminal())
}

func TestRequestChild_IsTerminal_RolloverStatesNotTerminal(t *testing.T) {
	t.Parallel()

	// pending_renewal + auto_renewed + pending_admin_review are all
	// rollover-intermediate states; the deadline worker / admin queue
	// promotes them onward.
	for _, status := range []string{
		ChildStatusPendingRenewal,
		ChildStatusAutoRenewed,
		ChildStatusPendingAdminReview,
	} {
		c := &RequestChild{Status: status}
		assert.False(t, c.IsTerminal(), "status %q must NOT be terminal", status)
	}
}

func TestRequestChild_IsTerminal_EmptyStatusNotTerminal(t *testing.T) {
	t.Parallel()

	// Defensive: an empty status shouldn't be treated as terminal. The
	// DB has a CHECK constraint on this column but the predicate runs
	// in-memory before any DB write happens during rollover.
	c := &RequestChild{Status: ""}
	assert.False(t, c.IsTerminal())
}
