package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingAdminSubscribers struct{ err error }

func (s failingAdminSubscribers) AdminNotificationRecipients(context.Context, []string) ([]string, error) {
	return nil, s.err
}

type mailSettings struct{ intakeSettingsFake }

func (mailSettings) AdminNotificationEmails(context.Context) string {
	return "admin@example.org, ADMIN@example.org"
}

func TestSubmissionMailSubscriberLookupFailureKeepsConfirmationAndConfiguredRecipients(t *testing.T) {
	t.Parallel()
	outbox := &decisionOutboxStub{}
	savepointCalled := false
	intake := NewIntake(IntakeDependencies{
		Notifications: newNotificationsForTest(NotificationDependencies{}),
		Outbox:        outbox, Settings: mailSettings{},
		AdminSubscribers: failingAdminSubscribers{err: errors.New("lookup failed")},
		Logger:           slog.New(slog.DiscardHandler),
		Runtime: Runtime{
			Savepoint: func(ctx context.Context, fn func(context.Context) error) error {
				savepointCalled = true
				return fn(ctx)
			},
			IsSavepointControl: func(error) bool { return false },
		},
	})

	err := intake.enqueueSubmissionEmails(context.Background(), 1, &Request{GuardianEmail: "parent@example.org"}, nil, "")
	require.NoError(t, err)
	assert.True(t, savepointCalled)
	require.Len(t, outbox.mails, 2)
	assert.Equal(t, enrollment.MailKindSubmitted, outbox.mails[0].Kind)
	assert.Equal(t, enrollment.MailKindAdminNotification, outbox.mails[1].Kind)
	assert.Equal(t, "admin@example.org", outbox.mails[1].Payload[enrollment.EnrollmentPayloadRecipientEmail])
}

func TestSubmissionMailSavepointControlFailureAborts(t *testing.T) {
	t.Parallel()
	controlErr := errors.New("savepoint control failed")
	intake := NewIntake(IntakeDependencies{Settings: intakeSettingsFake{},
		Notifications:    newNotificationsForTest(NotificationDependencies{}),
		Outbox:           &decisionOutboxStub{},
		AdminSubscribers: failingAdminSubscribers{err: errors.New("lookup failed")},
		Runtime: Runtime{
			Savepoint:          func(context.Context, func(context.Context) error) error { return controlErr },
			IsSavepointControl: func(err error) bool { return errors.Is(err, controlErr) },
		},
	})

	err := intake.enqueueSubmissionEmails(context.Background(), 1, &Request{}, nil, "")
	require.ErrorIs(t, err, controlErr)
}

// The staff manual enrollment suppresses the submission mails, the opt-in
// admin notification included (#3780); an ordinary submission sends them.
func TestNotifySubmissionHonoursSuppression(t *testing.T) {
	t.Parallel()
	for _, suppressed := range []bool{false, true} {
		t.Run(fmt.Sprintf("suppressed=%t", suppressed), func(t *testing.T) {
			t.Parallel()
			outbox := &decisionOutboxStub{}
			intake := NewIntake(IntakeDependencies{
				Notifications: newNotificationsForTest(NotificationDependencies{}),
				Outbox:        outbox, Settings: mailSettings{},
				Logger: slog.New(slog.DiscardHandler),
				Runtime: Runtime{
					Savepoint:          func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
					IsSavepointControl: func(error) bool { return false },
				},
			})
			req := &SubmitRequest{TenantID: 7, SuppressSubmissionEmails: suppressed}
			outcome := &submitOutcome{Request: &Request{GuardianEmail: "parent@example.org"}}

			require.NoError(t, intake.notifySubmission(context.Background(), req, &submissionPlan{}, outcome, false))

			if suppressed {
				assert.Empty(t, outbox.mails, "a suppressed submission enqueues no mail")
				return
			}
			require.NotEmpty(t, outbox.mails)
			assert.Equal(t, enrollment.MailKindSubmitted, outbox.mails[0].Kind)
			assert.Equal(t, enrollment.MailKindAdminNotification, outbox.mails[len(outbox.mails)-1].Kind)
		})
	}
}
