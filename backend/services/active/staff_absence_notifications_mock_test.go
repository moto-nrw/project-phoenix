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
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingAbsenceEmailSettings struct{}

func (failingAbsenceEmailSettings) ResolveBool(context.Context, string) (bool, error) {
	return false, errors.New("settings unavailable")
}

type absenceEmailSchoolFinderStub struct {
	school *platformModels.School
	err    error
}

func (s absenceEmailSchoolFinderStub) FindByID(context.Context, int64) (*platformModels.School, error) {
	return s.school, s.err
}

func newAbsenceNotificationTestService(
	t *testing.T,
	settings absenceEmailSettingResolver,
	staffRepo *testpkg.StaffRepoMock,
) (*staffAbsenceService, *testpkg.CapturingMailer) {
	t.Helper()
	mailer := testpkg.NewCapturingMailer()
	svc := &staffAbsenceService{}
	svc.SetAbsenceEmailDeps(AbsenceEmailDeps{
		Settings:    settings,
		Dispatcher:  email.NewDispatcher(mailer, slog.Default()),
		StaffRepo:   staffRepo,
		SchoolRepo:  absenceEmailSchoolFinderStub{school: &platformModels.School{Subdomain: "tenant"}},
		DefaultFrom: email.NewEmail("moto", "no-reply@moto.test"),
		FrontendURL: "http://localhost:3000",
	})
	return svc, mailer
}

func notificationTestAbsence(status string) *activeModels.StaffAbsence {
	absence := &activeModels.StaffAbsence{
		StaffID:     int64(42),
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   timezone.NewDate(2027, 7, 5),
		DateEnd:     timezone.NewDate(2027, 7, 5),
		Status:      status,
	}
	absence.SetTenantID(int64(7001))
	return absence
}

func TestAbsenceEmailHelpers_CoverLabelsRangesAndLoggers(t *testing.T) {
	t.Parallel()

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

func TestAbsenceEmailHelpers_CompTimeLabel(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t,
		"Freizeitausgleich",
		absenceTypeLabelGerman(activeModels.AbsenceTypeCompTime),
	)
}

func TestAbsenceEmailsEnabled_RequiresDependenciesAndHandlesSettingFailure(t *testing.T) {
	t.Parallel()

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
	svc.emailDeps.StaffRepo = &testpkg.StaffRepoMock{}
	svc.emailDeps.SchoolRepo = absenceEmailSchoolFinderStub{
		school: &platformModels.School{Subdomain: "tenant"},
	}
	assert.True(t, svc.absenceEmailsEnabled(ctx))
}

func TestBuildTenantFrontendURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		frontendURL string
		subdomain   string
		targetPath  string
		want        string
		wantErr     string
	}{
		{
			name:        "localhost with port",
			frontendURL: "http://localhost:3000",
			subdomain:   "school-a",
			targetPath:  "/staff",
			want:        "http://school-a.localhost:3000/staff",
		},
		{
			name:        "staging host",
			frontendURL: "https://staging.moto-app.de/base?ignored=true",
			subdomain:   "school-b",
			targetPath:  "/time-tracking",
			want:        "https://school-b.staging.moto-app.de/time-tracking",
		},
		{
			name:        "missing subdomain",
			frontendURL: "https://moto-app.de",
			targetPath:  "/staff",
			wantErr:     "school subdomain is required",
		},
		{
			name:        "relative frontend URL",
			frontendURL: "moto-app.de",
			subdomain:   "school-a",
			targetPath:  "/staff",
			wantErr:     "frontend URL must include scheme and host",
		},
		{
			name:        "relative target path",
			frontendURL: "https://moto-app.de",
			subdomain:   "school-a",
			targetPath:  "staff",
			wantErr:     "target path must start with '/'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildTenantFrontendURL(tt.frontendURL, tt.subdomain, tt.targetPath)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNotifyAbsenceRequested_StopsOnLookupFailuresOrMissingApprovers(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	assert.Equal(t, "", content["PreviousQuestion"])
	assert.Equal(t, "http://tenant.localhost:3000/staff", content["LinkURL"])
}

func TestNotifyAbsenceRequested_IncludesResubmissionContext(t *testing.T) {
	t.Parallel()

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
				{StaffID: int64(44), FirstName: "Lena", LastName: "Leitung", Email: "lena@example.test"},
			}, nil
		},
	}
	svc, mailer := newAbsenceNotificationTestService(
		t,
		absSettingsMock{enabled: true},
		staffRepo,
	)
	absence := notificationTestAbsence(activeModels.AbsenceStatusRequested)
	absence.Note = "Vertretung ist geklärt"
	absence.DecisionNote = "Wer übernimmt die Frühschicht?"

	svc.notifyAbsenceRequested(context.Background(), absence)

	require.True(t, mailer.WaitForMessages(1, 2*time.Second))
	messages := mailer.Messages()
	require.Len(t, messages, 1)
	assert.Equal(t, "Abwesenheitsantrag erneut eingereicht von Mila Muster", messages[0].Subject)
	content, ok := messages[0].Content.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Vertretung ist geklärt", content["Note"])
	assert.Equal(t, "Wer übernimmt die Frühschicht?", content["PreviousQuestion"])
}

func TestNotifyAbsenceRequested_DispatchesOnlyAfterCommit(t *testing.T) {
	t.Parallel()

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
				{StaffID: int64(44), FirstName: "Lena", LastName: "Leitung", Email: "lena@example.test"},
			}, nil
		},
	}
	svc, mailer := newAbsenceNotificationTestService(t, absSettingsMock{enabled: true}, staffRepo)
	ctx, commit := tenant.WithAfterCommitHooksForTest(context.Background())

	svc.notifyAbsenceRequested(ctx, notificationTestAbsence(activeModels.AbsenceStatusRequested))

	assert.False(t, mailer.WaitForMessages(1, 100*time.Millisecond), "email must remain queued before commit")
	commit()
	require.True(t, mailer.WaitForMessages(1, 2*time.Second), "email must dispatch after commit")
}

func TestNotifyAbsenceRequested_DropsDispatchOnRollback(t *testing.T) {
	t.Parallel()

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
				{StaffID: int64(44), FirstName: "Lena", LastName: "Leitung", Email: "lena@example.test"},
			}, nil
		},
	}
	svc, mailer := newAbsenceNotificationTestService(t, absSettingsMock{enabled: true}, staffRepo)
	ctx, _ := tenant.WithAfterCommitHooksForTest(context.Background())

	svc.notifyAbsenceRequested(ctx, notificationTestAbsence(activeModels.AbsenceStatusRequested))

	assert.False(t, mailer.WaitForMessages(1, 150*time.Millisecond), "rollback must drop the queued email")
	assert.Empty(t, mailer.Messages())
}

func TestNotifyAbsenceDecision_CoversStatusesAndRecipientFailures(t *testing.T) {
	t.Parallel()

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
