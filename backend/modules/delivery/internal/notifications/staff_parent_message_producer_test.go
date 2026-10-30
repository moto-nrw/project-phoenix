package notifications_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubStaffRecipients struct {
	scopes  []notifications.StaffRecipientScope
	request notifications.StaffRecipientRequest
	err     error
}

func (s *stubStaffRecipients) Resolve(_ context.Context, request notifications.StaffRecipientRequest) ([]notifications.StaffRecipientScope, error) {
	s.request = request
	return s.scopes, s.err
}

type stubParentMessageThreadClaims struct {
	claimed  bool
	threadID int64
}

func (s *stubParentMessageThreadClaims) ClaimStaffMessageNotification(_ context.Context, threadID int64, _ time.Duration) (bool, error) {
	s.threadID = threadID
	return s.claimed, nil
}

func TestStaffParentMessageNotifierAddressesResponsibleStaff(t *testing.T) {
	t.Parallel()

	notifier := &captureNotifier{}
	recipients := &stubStaffRecipients{scopes: []notifications.StaffRecipientScope{
		{AccountID: absenceAccountA, StudentIDs: []int64{absenceStudentA}},
		{AccountID: absenceAdmin, StudentIDs: []int64{absenceStudentA}},
	}}
	claims := &stubParentMessageThreadClaims{claimed: true}
	producer := notifications.NewStaffParentMessageNotifier(notifier, recipients, claims, nil, nil, nil)

	require.NoError(t, producer.NotifyStaffParentMessage(context.Background(), notifications.StaffParentMessageReport{
		TenantID:       absenceTenant,
		ThreadID:       901,
		MessageID:      902,
		StudentID:      absenceStudentA,
		ActorAccountID: absenceActorAcct,
	}))

	require.Len(t, notifier.events, 1)
	event := notifier.events[0]
	assert.Equal(t, notifications.TypeStaffParentMessage, event.Type)
	assert.Equal(t, notifications.ScopeStaff, event.Audience.Scope)
	assert.Equal(t, absenceTenant, event.Audience.TenantID)
	assert.Equal(t, []int64{absenceAccountA, absenceAdmin}, event.Audience.StaffAccountIDs)
	assert.Equal(t, "Neue Nachricht von Eltern", event.Title)
	assert.Equal(t, "Eltern haben der OGS eine neue Nachricht geschrieben.", event.Body)
	assert.Equal(t, "/messages/901", event.DeepLink)
	assert.Equal(t, notifications.PriorityNormal, event.Priority)
	assert.Empty(t, event.Data)

	assert.Equal(t, notifications.StaffRecipientRequest{
		StudentIDs:         []int64{absenceStudentA},
		NotificationType:   notifications.TypeStaffParentMessage,
		ExcludedAccountIDs: []int64{absenceActorAcct},
	}, recipients.request)
	assert.Equal(t, int64(901), claims.threadID)
}

func TestStaffParentMessageNotifierSuppressesUnclaimedThread(t *testing.T) {
	t.Parallel()

	notifier := &captureNotifier{}
	recipients := &stubStaffRecipients{scopes: []notifications.StaffRecipientScope{
		{AccountID: absenceAccountA, StudentIDs: []int64{absenceStudentA}},
	}}
	claims := &stubParentMessageThreadClaims{claimed: false}
	producer := notifications.NewStaffParentMessageNotifier(notifier, recipients, claims, nil, nil, nil)

	require.NoError(t, producer.NotifyStaffParentMessage(context.Background(), notifications.StaffParentMessageReport{
		TenantID:  absenceTenant,
		ThreadID:  901,
		MessageID: 902,
		StudentID: absenceStudentA,
	}))

	assert.Empty(t, notifier.events)
	assert.Equal(t, int64(901), claims.threadID)
}

type stubLeadershipAccounts []int64

func (s stubLeadershipAccounts) LeadershipAccountIDs(context.Context) ([]int64, error) { return s, nil }

// TestStaffParentMessageNotifierAddressesOnlyLeadership pins the push of a
// leadership conversation (#5200): the resolver is asked for exactly the
// leadership accounts, without the group supervisors.
func TestStaffParentMessageNotifierAddressesOnlyLeadership(t *testing.T) {
	t.Parallel()

	const leader = int64(9201)
	notifier := &captureNotifier{}
	recipients := &stubStaffRecipients{scopes: []notifications.StaffRecipientScope{{AccountID: leader, StudentIDs: []int64{absenceStudentA}}}}
	claims := &stubParentMessageThreadClaims{claimed: true}
	producer := notifications.NewStaffParentMessageNotifier(notifier, recipients, claims, stubLeadershipAccounts{leader}, nil, nil)

	require.NoError(t, producer.NotifyStaffParentMessage(context.Background(), notifications.StaffParentMessageReport{
		TenantID: absenceTenant, ThreadID: 901, MessageID: 902, StudentID: absenceStudentA,
		ActorAccountID: absenceActorAcct, Leadership: true,
	}))
	assert.Equal(t, notifications.StaffRecipientRequest{
		StudentIDs:         []int64{absenceStudentA},
		NotificationType:   notifications.TypeStaffParentMessage,
		ExcludedAccountIDs: []int64{absenceActorAcct},
		OnlyAccounts:       true,
		AccountIDs:         []int64{leader},
	}, recipients.request)
	require.Len(t, notifier.events, 1)
	assert.Equal(t, []int64{leader}, notifier.events[0].Audience.StaffAccountIDs)

	// Without a leadership directory nobody is told, never the team.
	unbound := notifications.NewStaffParentMessageNotifier(notifier, recipients, claims, nil, nil, nil)
	require.NoError(t, unbound.NotifyStaffParentMessage(context.Background(), notifications.StaffParentMessageReport{
		TenantID: absenceTenant, ThreadID: 903, MessageID: 904, StudentID: absenceStudentA, Leadership: true,
	}))
	assert.True(t, recipients.request.OnlyAccounts)
	assert.Empty(t, recipients.request.AccountIDs)
}
