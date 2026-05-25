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
	c := &RequestChild{Status: ChildStatusApproved}
	assert.True(t, c.IsTerminal())
}

func TestRequestChild_IsTerminal_RejectedIsTerminal(t *testing.T) {
	c := &RequestChild{Status: ChildStatusRejected}
	assert.True(t, c.IsTerminal())
}

func TestRequestChild_IsTerminal_WithdrawnIsTerminal(t *testing.T) {
	c := &RequestChild{Status: ChildStatusWithdrawn}
	assert.True(t, c.IsTerminal())
}

func TestRequestChild_IsTerminal_WaitlistedNotTerminal(t *testing.T) {
	// Waitlisted can be promoted to approved when capacity opens up,
	// so it's intentionally non-terminal.
	c := &RequestChild{Status: ChildStatusWaitlisted}
	assert.False(t, c.IsTerminal())
}

func TestRequestChild_IsTerminal_SubmittedNotTerminal(t *testing.T) {
	c := &RequestChild{Status: ChildStatusSubmitted}
	assert.False(t, c.IsTerminal())
}

func TestRequestChild_IsTerminal_UnderReviewNotTerminal(t *testing.T) {
	c := &RequestChild{Status: ChildStatusUnderReview}
	assert.False(t, c.IsTerminal())
}

func TestRequestChild_IsTerminal_RolloverStatesNotTerminal(t *testing.T) {
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
	// Defensive: an empty status shouldn't be treated as terminal. The
	// DB has a CHECK constraint on this column but the predicate runs
	// in-memory before any DB write happens during rollover.
	c := &RequestChild{Status: ""}
	assert.False(t, c.IsTerminal())
}

// --- IsRenewalPending ----------------------------------------------------

// IsRenewalPending scopes the deadline worker's scan. Only rows in
// these two states need a deadline check — everything else has
// already been resolved (or wasn't a rollover row).

func TestRequestChild_IsRenewalPending_PendingRenewalTrue(t *testing.T) {
	c := &RequestChild{Status: ChildStatusPendingRenewal}
	assert.True(t, c.IsRenewalPending())
}

func TestRequestChild_IsRenewalPending_AutoRenewedTrue(t *testing.T) {
	c := &RequestChild{Status: ChildStatusAutoRenewed}
	assert.True(t, c.IsRenewalPending())
}

func TestRequestChild_IsRenewalPending_OtherStatesFalse(t *testing.T) {
	for _, status := range []string{
		ChildStatusSubmitted,
		ChildStatusUnderReview,
		ChildStatusApproved,
		ChildStatusWaitlisted,
		ChildStatusRejected,
		ChildStatusWithdrawn,
		ChildStatusPendingAdminReview,
		"",
	} {
		c := &RequestChild{Status: status}
		assert.False(t, c.IsRenewalPending(), "status %q must NOT be renewal-pending", status)
	}
}

// --- IsRollover ----------------------------------------------------------

// IsRollover is the decision service's signal to update the existing
// student record (the previous-year approval) instead of inserting a
// brand-new student row. Driven by the presence/absence of the back-
// link column rather than the status (rollover rows can transition
// through several statuses).

func TestRequestChild_IsRollover_NilSourceFalse(t *testing.T) {
	c := &RequestChild{}
	assert.False(t, c.IsRollover())
}

func TestRequestChild_IsRollover_SetSourceTrue(t *testing.T) {
	src := int64(7777)
	c := &RequestChild{RolloverSourceChildID: &src}
	assert.True(t, c.IsRollover())
}

func TestRequestChild_IsRollover_TerminalStatusStillRollover(t *testing.T) {
	// A rollover row that's been approved/rejected/withdrawn is still
	// a rollover for decision-service purposes — the back-link drives
	// the predicate, not the status.
	src := int64(7777)
	c := &RequestChild{
		Status:                ChildStatusApproved,
		RolloverSourceChildID: &src,
	}
	assert.True(t, c.IsRollover())
}

// --- TableName -----------------------------------------------------------

func TestRequestChild_TableName(t *testing.T) {
	c := &RequestChild{}
	assert.Equal(t, "enrollment.request_children", c.TableName())
}

// --- Status constants ----------------------------------------------------

// Pin the wire values — the rollover worker, admin UI labels, and
// frontend status badges all string-compare these.

func TestChildStatus_StableValues(t *testing.T) {
	assert.Equal(t, "submitted", ChildStatusSubmitted)
	assert.Equal(t, "under_review", ChildStatusUnderReview)
	assert.Equal(t, "approved", ChildStatusApproved)
	assert.Equal(t, "waitlisted", ChildStatusWaitlisted)
	assert.Equal(t, "rejected", ChildStatusRejected)
	assert.Equal(t, "withdrawn", ChildStatusWithdrawn)
	assert.Equal(t, "pending_renewal", ChildStatusPendingRenewal)
	assert.Equal(t, "auto_renewed", ChildStatusAutoRenewed)
	assert.Equal(t, "pending_admin_review", ChildStatusPendingAdminReview)
}

func TestReviewReason_StableValues(t *testing.T) {
	assert.Equal(t, "grade_above_max", ReviewReasonGradeAboveMax)
	assert.Equal(t, "no_grade_level", ReviewReasonNoGradeLevel)
}

func TestChildActivation_StableValues(t *testing.T) {
	assert.Equal(t, "immediate", ChildActivationImmediate)
	assert.Equal(t, "scheduled", ChildActivationScheduled)
}
