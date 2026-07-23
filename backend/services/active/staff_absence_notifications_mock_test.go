package active

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingAbsenceEmailSettings struct{}

func (failingAbsenceEmailSettings) ResolveBool(context.Context, string) (bool, error) {
	return false, errors.New("settings unavailable")
}

func newAbsenceNotificationTestService(
	t *testing.T,
	settings absenceEmailSettings,
	staffRepo *testpkg.StaffRepoMock,
) (*staffAbsenceService, *testpkg.CapturingMailer) {
	t.Helper()
	mailer := testpkg.NewCapturingMailer()
	svc := &staffAbsenceService{}
	svc.SetAbsenceEmailDeps(AbsenceEmailDeps{
		Settings:    settings,
		Dispatcher:  email.NewDispatcher(mailer, slog.Default()),
		StaffRepo:   staffRepo,
		DefaultFrom: email.NewEmail("moto", "no-reply@moto.test"),
		FrontendURL: "http://tenant.localhost:3000",
	})
	return svc, mailer
}

func notificationTestAbsence(status string) *activeModels.StaffAbsence {
	return &activeModels.StaffAbsence{
		StaffID:     int64(42),
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   timezone.NewDate(2027, 7, 5),
		DateEnd:     timezone.NewDate(2027, 7, 5),
		Status:      status,
	}
}

func TestAbsenceEmailHelpers_CoverLabelsRangesAndLoggers(t *testing.T) {
	assert.Equal(t, "Krankmeldung", absenceTypeLabelGerman(activeModels.AbsenceTypeSick))
	assert.Equal(t, "Urlaub", absenceTypeLabelGerman(activeModels.AbsenceTypeVacation))
	assert.Equal(t, "Fortbildung", absenceTypeLabelGerman(activeModels.AbsenceTypeTraining))
	assert.Equal(t, "Sonstige Abwesenheit", absenceTypeLabelGerman(activeModels.AbsenceTypeOther))

	absence := notificationTestAbsence(activeModels.AbsenceStatusRequested)
	assert.Equal(t, "05.07.2027", formatAbsenceDateRange(absence))
	absence.DateEnd = timezone.NewDate(2027, 7, 6)
	assert.Equal(t, "05.07.2027 bis 06.07.2027", formatAbsenceDateRange(absence))

	svc := &staffAbsenceService{}
	assert.NotNil(t, svc.emailLogger())
	customLogger := slog.New(slog.DiscardHandler)
	svc.emailDeps = &AbsenceEmailDeps{Logger: customLogger}
	assert.Same(t, customLogger, svc.emailLogger())
}

func TestAbsenceEmailsEnabled_RequiresDependenciesAndHandlesSettingFailure(t *testing.T) {
	ctx := context.Background()
	svc := &staffAbsenceService{}
	assert.False(t, svc.absenceEmailsEnabled(ctx))

	svc.emailDeps = &AbsenceEmailDeps{Settings: absSettingsMock{enabled: true}}
	assert.False(t, svc.absenceEmailsEnabled(ctx))

	svc.emailDeps = &AbsenceEmailDeps{
		Dispatcher: email.NewDispatcher(testpkg.NewCapturingMailer(), slog.Default()),
	}
	assert.False(t, svc.absenceEmailsEnabled(ctx))

	svc.emailDeps.Settings = failingAbsenceEmailSettings{}
	assert.False(t, svc.absenceEmailsEnabled(ctx))

	svc.emailDeps.Settings = absSettingsMock{enabled: true}
	assert.True(t, svc.absenceEmailsEnabled(ctx))
}

func TestNotifyAbsenceRequested_StopsOnLookupFailuresOrMissingApprovers(t *testing.T) {
	absence := notificationTestAbsence(activeModels.AbsenceStatusRequested)

	tests := []struct {
		name      string
		staffRepo *testpkg.StaffRepoMock
	}{
		{
			name: "requester lookup fails",
			staffRepo: &testpkg.StaffRepoMock{
				GetStaffContactInfoFn: func(context.Context, int64) (*usersModels.StaffWithRoleInfo, error) {
					return nil, errors.New("requester unavailable")
				},
			},
		},
		{
			name: "approver lookup fails",
			staffRepo: &testpkg.StaffRepoMock{
				GetStaffContactInfoFn: func(context.Context, int64) (*usersModels.StaffWithRoleInfo, error) {
					return &usersModels.StaffWithRoleInfo{FirstName: "Mila", LastName: "Muster"}, nil
				},
				ListStaffWithPermissionFn: func(context.Context, string) ([]*usersModels.StaffWithRoleInfo, error) {
					return nil, errors.New("approvers unavailable")
				},
			},
		},
		{
			name: "no approvers found",
			staffRepo: &testpkg.StaffRepoMock{
				GetStaffContactInfoFn: func(context.Context, int64) (*usersModels.StaffWithRoleInfo, error) {
					return &usersModels.StaffWithRoleInfo{FirstName: "Mila", LastName: "Muster"}, nil
				},
				ListStaffWithPermissionFn: func(context.Context, string) ([]*usersModels.StaffWithRoleInfo, error) {
					return nil, nil
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, mailer := newAbsenceNotificationTestService(
				t,
				absSettingsMock{enabled: true},
				tt.staffRepo,
			)

			svc.notifyAbsenceRequested(context.Background(), absence)

			assert.Empty(t, mailer.Messages())
		})
	}
}

func TestNotifyAbsenceRequested_SkipsSelfAndMissingEmail(t *testing.T) {
	staffRepo := &testpkg.StaffRepoMock{
		GetStaffContactInfoFn: func(_ context.Context, staffID int64) (*usersModels.StaffWithRoleInfo, error) {
			return &usersModels.StaffWithRoleInfo{
				StaffID:   staffID,
				FirstName: "Mila",
				LastName:  "Muster",
			}, nil
		},
		ListStaffWithPermissionFn: func(context.Context, string) ([]*usersModels.StaffWithRoleInfo, error) {
			return []*usersModels.StaffWithRoleInfo{
				{StaffID: int64(42), FirstName: "Mila", LastName: "Muster", Email: "mila@example.test"},
				{StaffID: int64(43), FirstName: "Ohne", LastName: "Adresse"},
				{StaffID: int64(44), FirstName: "Lena", LastName: "Leitung", Email: "lena@example.test"},
			}, nil
		},
	}
	svc, mailer := newAbsenceNotificationTestService(
		t,
		absSettingsMock{enabled: true},
		staffRepo,
	)

	svc.notifyAbsenceRequested(
		context.Background(),
		notificationTestAbsence(activeModels.AbsenceStatusRequested),
	)

	require.True(t, mailer.WaitForMessages(1, 2*time.Second))
	messages := mailer.Messages()
	require.Len(t, messages, 1)
	assert.Equal(t, "lena@example.test", messages[0].To.Address)
	content, ok := messages[0].Content.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Krankmeldung", content["AbsenceTypeLabel"])
	assert.Equal(t, "05.07.2027", content["DateRange"])
}

func TestNotifyAbsenceDecision_CoversStatusesAndRecipientFailures(t *testing.T) {
	t.Run("ignores unrelated status", func(t *testing.T) {
		staffRepo := &testpkg.StaffRepoMock{
			GetStaffContactInfoFn: func(context.Context, int64) (*usersModels.StaffWithRoleInfo, error) {
				t.Fatal("unrelated status must not load the requester")
				return nil, nil
			},
		}
		svc, mailer := newAbsenceNotificationTestService(t, absSettingsMock{enabled: true}, staffRepo)

		svc.notifyAbsenceDecision(context.Background(), notificationTestAbsence(activeModels.AbsenceStatusRequested))

		assert.Empty(t, mailer.Messages())
	})

	t.Run("requester lookup fails", func(t *testing.T) {
		staffRepo := &testpkg.StaffRepoMock{
			GetStaffContactInfoFn: func(context.Context, int64) (*usersModels.StaffWithRoleInfo, error) {
				return nil, errors.New("requester unavailable")
			},
		}
		svc, mailer := newAbsenceNotificationTestService(t, absSettingsMock{enabled: true}, staffRepo)

		svc.notifyAbsenceDecision(context.Background(), notificationTestAbsence(activeModels.AbsenceStatusApproved))

		assert.Empty(t, mailer.Messages())
	})

	t.Run("requester has no email", func(t *testing.T) {
		staffRepo := &testpkg.StaffRepoMock{
			GetStaffContactInfoFn: func(context.Context, int64) (*usersModels.StaffWithRoleInfo, error) {
				return &usersModels.StaffWithRoleInfo{FirstName: "Mila", LastName: "Muster"}, nil
			},
		}
		svc, mailer := newAbsenceNotificationTestService(t, absSettingsMock{enabled: true}, staffRepo)

		svc.notifyAbsenceDecision(context.Background(), notificationTestAbsence(activeModels.AbsenceStatusDeclined))

		assert.Empty(t, mailer.Messages())
	})

	for _, tt := range []struct {
		status   string
		template string
		subject  string
	}{
		{
			status:   activeModels.AbsenceStatusApproved,
			template: "absence-request-approved.html",
			subject:  "Dein Abwesenheitsantrag wurde genehmigt",
		},
		{
			status:   activeModels.AbsenceStatusDeclined,
			template: "absence-request-declined.html",
			subject:  "Dein Abwesenheitsantrag wurde abgelehnt",
		},
	} {
		t.Run(tt.status, func(t *testing.T) {
			staffRepo := &testpkg.StaffRepoMock{
				GetStaffContactInfoFn: func(context.Context, int64) (*usersModels.StaffWithRoleInfo, error) {
					return &usersModels.StaffWithRoleInfo{
						FirstName: "Mila",
						LastName:  "Muster",
						Email:     "mila@example.test",
					}, nil
				},
			}
			svc, mailer := newAbsenceNotificationTestService(t, absSettingsMock{enabled: true}, staffRepo)

			svc.notifyAbsenceDecision(context.Background(), notificationTestAbsence(tt.status))

			require.True(t, mailer.WaitForMessages(1, 2*time.Second))
			messages := mailer.Messages()
			require.Len(t, messages, 1)
			assert.Equal(t, tt.template, messages[0].Template)
			assert.Equal(t, tt.subject, messages[0].Subject)
		})
	}
}
