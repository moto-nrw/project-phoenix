package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
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
		Outbox: outbox, Settings: mailSettings{},
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

	err := intake.enqueueSubmissionEmails(context.Background(), 1, &enrollmentModels.Request{GuardianEmail: "parent@example.org"}, nil, "")
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
	intake := NewIntake(IntakeDependencies{
		Outbox:           &decisionOutboxStub{},
		AdminSubscribers: failingAdminSubscribers{err: errors.New("lookup failed")},
		Runtime: Runtime{
			Savepoint:          func(context.Context, func(context.Context) error) error { return controlErr },
			IsSavepointControl: func(err error) bool { return errors.Is(err, controlErr) },
		},
	})

	err := intake.enqueueSubmissionEmails(context.Background(), 1, &enrollmentModels.Request{}, nil, "")
	require.ErrorIs(t, err, controlErr)
}
