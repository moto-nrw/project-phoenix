package platform

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// EmailOutbox.Validate enforces the column-level CHECK constraints in
// app code so the worker doesn't have to round-trip a row that's
// guaranteed to fail. Every branch matters because the outbox is the
// single chokepoint for all enrollment + guardian invitation emails.

func validOutbox() *EmailOutbox {
	return &EmailOutbox{
		Kind:    EmailKindEnrollmentSubmitted,
		Payload: map[string]any{"recipient_email": "x@example.test"},
		Status:  EmailOutboxStatusPending,
	}
}

func TestEmailOutbox_Validate_HappyPath(t *testing.T) {
	t.Parallel()

	o := validOutbox()
	o.NextRetryAt = time.Now()
	assert.NoError(t, o.Validate())
}

func TestEmailOutbox_Validate_RequiresKind(t *testing.T) {
	t.Parallel()

	o := validOutbox()
	o.Kind = ""
	err := o.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kind")
}

func TestEmailOutbox_Validate_TrimsKind(t *testing.T) {
	t.Parallel()

	o := validOutbox()
	o.Kind = "   enrollment_submitted   "
	require.NoError(t, o.Validate())
	assert.Equal(t, "enrollment_submitted", o.Kind)
}

func TestEmailOutbox_Validate_KindWhitespaceOnlyRejected(t *testing.T) {
	t.Parallel()

	o := validOutbox()
	o.Kind = "   "
	err := o.Validate()
	require.Error(t, err)
}

func TestEmailOutbox_Validate_DefaultsStatusToPending(t *testing.T) {
	t.Parallel()

	o := validOutbox()
	o.Status = ""
	require.NoError(t, o.Validate())
	assert.Equal(t, EmailOutboxStatusPending, o.Status,
		"empty status backfilled to pending so the worker picks the row up on the next tick")
}

func TestEmailOutbox_Validate_AcceptsAllValidStatuses(t *testing.T) {
	t.Parallel()

	for _, status := range []string{
		EmailOutboxStatusPending,
		EmailOutboxStatusSending,
		EmailOutboxStatusSent,
		EmailOutboxStatusFailed,
	} {
		o := validOutbox()
		o.Status = status
		assert.NoError(t, o.Validate(), "status %q must validate", status)
	}
}

func TestEmailOutbox_Validate_RejectsUnknownStatus(t *testing.T) {
	t.Parallel()

	o := validOutbox()
	o.Status = "weird"
	err := o.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status")
}

func TestEmailOutbox_Validate_DefaultsNextRetryAt(t *testing.T) {
	t.Parallel()

	o := validOutbox()
	o.NextRetryAt = time.Time{}
	before := time.Now()
	require.NoError(t, o.Validate())
	assert.False(t, o.NextRetryAt.IsZero(),
		"empty next_retry_at backfilled to now so the worker picks the row up immediately")
	assert.WithinDuration(t, before, o.NextRetryAt, 5*time.Second,
		"next_retry_at backfill is close to validation time")
}

func TestEmailOutbox_Validate_PreservesExplicitNextRetryAt(t *testing.T) {
	t.Parallel()

	future := time.Now().Add(2 * time.Hour)
	o := validOutbox()
	o.NextRetryAt = future
	require.NoError(t, o.Validate())
	assert.Equal(t, future, o.NextRetryAt,
		"explicit next_retry_at MUST be preserved — that's how backoff works")
}

// TableName is a one-liner but the worker depends on it for the
// schema-qualified table reference; broken renames would break every
// outbox write.

// Kind constants are stable contract: the worker registers per-kind
// renderers using these strings; a typo in the consumer code would
// silently send no email at all. The constants exist precisely to
// catch that at compile time, but a smoke test pins their values
// against the wire shape used in jsonb payloads + audit rows.

func TestEmailKind_StableValues(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "guardian_invitation", EmailKindGuardianInvitation)
	assert.Equal(t, "enrollment_submitted", EmailKindEnrollmentSubmitted)
	assert.Equal(t, "enrollment_admin_notification", EmailKindEnrollmentAdminNotify)
	assert.Equal(t, "enrollment_approved", EmailKindEnrollmentApproved)
	assert.Equal(t, "enrollment_waitlisted", EmailKindEnrollmentWaitlisted)
	assert.Equal(t, "enrollment_rejected", EmailKindEnrollmentRejected)
	assert.Equal(t, "enrollment_rollover_opt_in", EmailKindEnrollmentRolloverOptIn)
	assert.Equal(t, "enrollment_rollover_opt_out", EmailKindEnrollmentRolloverOptOut)
}

func TestEmailOutboxStatus_StableValues(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "pending", EmailOutboxStatusPending)
	assert.Equal(t, "sending", EmailOutboxStatusSending)
	assert.Equal(t, "sent", EmailOutboxStatusSent)
	assert.Equal(t, "failed", EmailOutboxStatusFailed)
}
