package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubOutboxReader / stubDeliveryEventRepo are local doubles: they inject
// errors and serve fixed rows, which the shared doubles do not do.
type stubOutboxReader struct {
	rows []*platformModels.EmailOutbox
	err  error
}

func (s *stubOutboxReader) FindByRelatedEntity(_ context.Context, _ string, _ int64) ([]*platformModels.EmailOutbox, error) {
	return s.rows, s.err
}

type stubDeliveryEventRepo struct {
	events []*platformModels.EmailDeliveryEvent
	err    error
}

func (s *stubDeliveryEventRepo) CreateIfNew(_ context.Context, _ *platformModels.EmailDeliveryEvent) (bool, error) {
	return false, errors.New("not implemented in stub")
}

func (s *stubDeliveryEventRepo) ListByOutboxIDs(_ context.Context, _ []int64) ([]*platformModels.EmailDeliveryEvent, error) {
	return s.events, s.err
}

func outboxRow(id int64, kind, status, deliveryStatus string) *platformModels.EmailOutbox {
	row := &platformModels.EmailOutbox{
		Kind:           kind,
		Status:         status,
		DeliveryStatus: deliveryStatus,
		NextRetryAt:    time.Now().Add(5 * time.Minute),
	}
	row.ID = id
	return row
}

func deliveryService(reader *stubOutboxReader, events *stubDeliveryEventRepo) *guardianInvitationService {
	cfg := GuardianInvitationServiceConfig{OutboxReader: reader}
	if events != nil {
		cfg.DeliveryEvents = events
	}
	return &guardianInvitationService{GuardianInvitationServiceConfig: cfg}
}

func TestDeliveryStatus_ReturnsAttemptsWithEvents(t *testing.T) {
	reasonCode := "5.1.1"
	newest := outboxRow(2, platformModels.EmailKindGuardianInvitation, platformModels.EmailOutboxStatusSent, platformModels.EmailDeliveryStatusBounced)
	older := outboxRow(1, platformModels.EmailKindGuardianInvitation, platformModels.EmailOutboxStatusSent, platformModels.EmailDeliveryStatusDelivered)

	svc := deliveryService(
		&stubOutboxReader{rows: []*platformModels.EmailOutbox{newest, older}},
		&stubDeliveryEventRepo{events: []*platformModels.EmailDeliveryEvent{
			{OutboxID: newest.ID, EventType: platformModels.DeliveryEventBounced, OccurredAt: time.Now(), ReasonCode: &reasonCode},
		}},
	)

	result, err := svc.DeliveryStatus(context.Background(), 42)
	require.NoError(t, err)

	assert.Equal(t, int64(42), result.InvitationID)
	require.Len(t, result.Attempts, 2, "a resend adds another attempt")
	assert.Equal(t, newest.ID, result.Attempts[0].OutboxID, "newest first")
	assert.Equal(t, platformModels.EmailDeliveryStatusBounced, result.Attempts[0].DeliveryStatus)
	require.Len(t, result.Attempts[0].Events, 1)
	require.NotNil(t, result.Attempts[0].Events[0].ReasonCode)
	assert.Equal(t, "5.1.1", *result.Attempts[0].Events[0].ReasonCode)
	// Events belong to their own attempt; the older one has none.
	assert.Empty(t, result.Attempts[1].Events)
}

// The ops bounce notice hangs off the same related entity so support can find
// it. Counting it as a dispatch attempt would let a freshly queued notice mask
// the very bounce it reports.
func TestDeliveryStatus_IgnoresNonInvitationKinds(t *testing.T) {
	notice := outboxRow(9, platformModels.EmailKindDeliveryFailureNotice, platformModels.EmailOutboxStatusPending, platformModels.EmailDeliveryStatusUnknown)
	invitation := outboxRow(5, platformModels.EmailKindGuardianInvitation, platformModels.EmailOutboxStatusSent, platformModels.EmailDeliveryStatusBounced)

	svc := deliveryService(&stubOutboxReader{rows: []*platformModels.EmailOutbox{notice, invitation}}, nil)

	result, err := svc.DeliveryStatus(context.Background(), 42)
	require.NoError(t, err)

	require.Len(t, result.Attempts, 1)
	assert.Equal(t, invitation.ID, result.Attempts[0].OutboxID)
	assert.Equal(t, platformModels.EmailDeliveryStatusBounced, result.Attempts[0].DeliveryStatus)
}

// next_retry_at is only meaningful while a retry is actually pending; on a
// sent or failed row it is a stale artefact of the last attempt.
func TestDeliveryStatus_NextRetryOnlyWhilePending(t *testing.T) {
	svc := deliveryService(&stubOutboxReader{rows: []*platformModels.EmailOutbox{
		outboxRow(2, platformModels.EmailKindGuardianInvitation, platformModels.EmailOutboxStatusPending, platformModels.EmailDeliveryStatusUnknown),
		outboxRow(1, platformModels.EmailKindGuardianInvitation, platformModels.EmailOutboxStatusSent, platformModels.EmailDeliveryStatusDelivered),
	}}, nil)

	result, err := svc.DeliveryStatus(context.Background(), 42)
	require.NoError(t, err)

	assert.NotNil(t, result.Attempts[0].NextRetryAt)
	assert.Nil(t, result.Attempts[1].NextRetryAt)
}

func TestDeliveryStatus_NoAttemptsYet(t *testing.T) {
	svc := deliveryService(&stubOutboxReader{}, nil)

	result, err := svc.DeliveryStatus(context.Background(), 42)
	require.NoError(t, err)
	assert.Empty(t, result.Attempts)
}

func TestDeliveryStatus_PropagatesErrors(t *testing.T) {
	svc := deliveryService(&stubOutboxReader{err: errors.New("db down")}, nil)
	_, err := svc.DeliveryStatus(context.Background(), 42)
	require.Error(t, err)

	svc = deliveryService(
		&stubOutboxReader{rows: []*platformModels.EmailOutbox{
			outboxRow(1, platformModels.EmailKindGuardianInvitation, platformModels.EmailOutboxStatusSent, platformModels.EmailDeliveryStatusDelivered),
		}},
		&stubDeliveryEventRepo{err: errors.New("events unavailable")},
	)
	_, err = svc.DeliveryStatus(context.Background(), 42)
	require.Error(t, err)
}

func TestDeliveryStatus_UnwiredServiceErrors(t *testing.T) {
	svc := &guardianInvitationService{}
	_, err := svc.DeliveryStatus(context.Background(), 42)
	require.Error(t, err)
}
