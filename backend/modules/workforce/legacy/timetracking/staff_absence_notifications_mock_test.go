package timetracking

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingAbsenceEmailSettings struct{}

// absenceEmailStaffStub drives the absence email directory directly in the
// service's own contact vocabulary. Unset hooks return nothing, so a test only
// scripts the lookup it exercises and an unexpected call surfaces as a missing
// recipient rather than a panic.
type absenceEmailStaffStub struct {
	ContactFn   func(ctx context.Context, staffID int64) (*AbsenceEmailContact, error)
	ApproversFn func(ctx context.Context) ([]*AbsenceEmailContact, error)
}

func (d *absenceEmailStaffStub) GetStaffContactInfo(ctx context.Context, id int64) (*AbsenceEmailContact, error) {
	if d.ContactFn == nil {
		return nil, nil
	}
	return d.ContactFn(ctx, id)
}

func (d *absenceEmailStaffStub) ListAbsenceApprovers(ctx context.Context) ([]*AbsenceEmailContact, error) {
	if d.ApproversFn == nil {
		return nil, nil
	}
	return d.ApproversFn(ctx)
}

func (failingAbsenceEmailSettings) AbsenceApprovalEmailEnabled(context.Context) (bool, error) {
	return false, errors.New("settings unavailable")
}

type absenceEmailSchoolFinderStub struct {
	subdomain string
	found     bool
	err       error
}

func (s absenceEmailSchoolFinderStub) FindSchoolSubdomain(context.Context, int64) (string, bool, error) {
	return s.subdomain, s.found, s.err
}

func newAbsenceNotificationTestService(
	t *testing.T,
	settings absenceEmailSettingResolver,
	staffRepo *absenceEmailStaffStub,
) (*staffAbsenceService, *capturingAbsenceEmails) {
	t.Helper()
	mailer := newCapturingAbsenceEmails()
	svc := &staffAbsenceService{}
	svc.emailDeps = &AbsenceEmailDeps{
		Settings:    settings,
		Dispatcher:  mailer,
		StaffRepo:   staffRepo,
		SchoolRepo:  absenceEmailSchoolFinderStub{subdomain: "tenant", found: true},
		FrontendURL: "http://localhost:3000",
	}
	return svc, mailer
}

func notificationTestAbsence(status string) *StaffAbsence {
	absence := &StaffAbsence{
		StaffID:     int64(42),
		AbsenceType: AbsenceTypeSick,
		DateStart:   NewDate(2027, 7, 5),
		DateEnd:     NewDate(2027, 7, 5),
		Status:      status,
	}
	absence.SetTenantID(int64(7001))
	return absence
}

func TestAbsenceEmailHelpers_CoverLabelsRangesAndLoggers(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "Krankmeldung", absenceTypeLabelGerman(AbsenceTypeSick))
	assert.Equal(t, "Urlaub", absenceTypeLabelGerman(AbsenceTypeVacation))
	assert.Equal(t, "Fortbildung", absenceTypeLabelGerman(AbsenceTypeTraining))
	assert.Equal(t, "Sonstige Abwesenheit", absenceTypeLabelGerman(AbsenceTypeOther))

	absence := notificationTestAbsence(AbsenceStatusRequested)
	assert.Equal(t, "05.07.2027", formatAbsenceDateRange(absence))
	absence.DateEnd = NewDate(2027, 7, 6)
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
		absenceTypeLabelGerman(AbsenceTypeCompTime),
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
		Dispatcher: newCapturingAbsenceEmails(),
	}
	assert.False(t, svc.absenceEmailsEnabled(ctx))

	svc.emailDeps.Settings = failingAbsenceEmailSettings{}
	assert.False(t, svc.absenceEmailsEnabled(ctx))

	svc.emailDeps.Settings = absSettingsMock{enabled: true}
	svc.emailDeps.StaffRepo = &absenceEmailStaffStub{}
	svc.emailDeps.SchoolRepo = absenceEmailSchoolFinderStub{
		subdomain: "tenant", found: true,
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

	absence := notificationTestAbsence(AbsenceStatusRequested)

	tests := []struct {
		name      string
		staffRepo *absenceEmailStaffStub
	}{
		{
			name: "requester lookup fails",
			staffRepo: &absenceEmailStaffStub{
				ContactFn: func(context.Context, int64) (*AbsenceEmailContact, error) {
					return nil, errors.New("requester unavailable")
				},
			},
		},
		{
			name: "approver lookup fails",
			staffRepo: &absenceEmailStaffStub{
				ContactFn: func(context.Context, int64) (*AbsenceEmailContact, error) {
					return &AbsenceEmailContact{FirstName: "Mila", LastName: "Muster"}, nil
				},
				ApproversFn: func(context.Context) ([]*AbsenceEmailContact, error) {
					return nil, errors.New("approvers unavailable")
				},
			},
		},
		{
			name: "no approvers found",
			staffRepo: &absenceEmailStaffStub{
				ContactFn: func(context.Context, int64) (*AbsenceEmailContact, error) {
					return &AbsenceEmailContact{FirstName: "Mila", LastName: "Muster"}, nil
				},
				ApproversFn: func(context.Context) ([]*AbsenceEmailContact, error) {
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

	staffRepo := &absenceEmailStaffStub{
		ContactFn: func(_ context.Context, staffID int64) (*AbsenceEmailContact, error) {
			return &AbsenceEmailContact{
				StaffID:   staffID,
				FirstName: "Mila",
				LastName:  "Muster",
			}, nil
		},
		ApproversFn: func(context.Context) ([]*AbsenceEmailContact, error) {
			return []*AbsenceEmailContact{
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
		notificationTestAbsence(AbsenceStatusRequested),
	)

	require.True(t, mailer.WaitForMessages(1, 2*time.Second))
	messages := mailer.Messages()
	require.Len(t, messages, 1)
	assert.Equal(t, "lena@example.test", messages[0].To.Address)
	content := messages[0].Content
	assert.Equal(t, "Krankmeldung", content.AbsenceTypeLabel)
	assert.Equal(t, "05.07.2027", content.DateRange)
	assert.Equal(t, "", content.PreviousQuestion)
	assert.Equal(t, "http://tenant.localhost:3000/staff", content.LinkURL)
}

func TestNotifyAbsenceRequested_IncludesResubmissionContext(t *testing.T) {
	t.Parallel()

	staffRepo := &absenceEmailStaffStub{
		ContactFn: func(_ context.Context, staffID int64) (*AbsenceEmailContact, error) {
			return &AbsenceEmailContact{
				StaffID:   staffID,
				FirstName: "Mila",
				LastName:  "Muster",
			}, nil
		},
		ApproversFn: func(context.Context) ([]*AbsenceEmailContact, error) {
			return []*AbsenceEmailContact{
				{StaffID: int64(44), FirstName: "Lena", LastName: "Leitung", Email: "lena@example.test"},
			}, nil
		},
	}
	svc, mailer := newAbsenceNotificationTestService(
		t,
		absSettingsMock{enabled: true},
		staffRepo,
	)
	absence := notificationTestAbsence(AbsenceStatusRequested)
	absence.Note = "Vertretung ist geklärt"
	absence.DecisionNote = "Wer übernimmt die Frühschicht?"

	svc.notifyAbsenceRequested(context.Background(), absence)

	require.True(t, mailer.WaitForMessages(1, 2*time.Second))
	messages := mailer.Messages()
	require.Len(t, messages, 1)
	assert.Equal(t, "Abwesenheitsantrag erneut eingereicht von Mila Muster", messages[0].Subject)
	content := messages[0].Content
	assert.Equal(t, "Vertretung ist geklärt", content.Note)
	assert.Equal(t, "Wer übernimmt die Frühschicht?", content.PreviousQuestion)
}

func TestNotifyAbsenceRequested_DispatchesOnlyAfterCommit(t *testing.T) {
	t.Parallel()

	staffRepo := &absenceEmailStaffStub{
		ContactFn: func(_ context.Context, staffID int64) (*AbsenceEmailContact, error) {
			return &AbsenceEmailContact{
				StaffID:   staffID,
				FirstName: "Mila",
				LastName:  "Muster",
			}, nil
		},
		ApproversFn: func(context.Context) ([]*AbsenceEmailContact, error) {
			return []*AbsenceEmailContact{
				{StaffID: int64(44), FirstName: "Lena", LastName: "Leitung", Email: "lena@example.test"},
			}, nil
		},
	}
	svc, mailer := newAbsenceNotificationTestService(t, absSettingsMock{enabled: true}, staffRepo)
	ctx, commit := tenant.WithAfterCommitHooksForTest(context.Background())

	svc.notifyAbsenceRequested(ctx, notificationTestAbsence(AbsenceStatusRequested))

	assert.False(t, mailer.WaitForMessages(1, 100*time.Millisecond), "email must remain queued before commit")
	commit()
	require.True(t, mailer.WaitForMessages(1, 2*time.Second), "email must dispatch after commit")
}

func TestNotifyAbsenceRequested_DropsDispatchOnRollback(t *testing.T) {
	t.Parallel()

	staffRepo := &absenceEmailStaffStub{
		ContactFn: func(_ context.Context, staffID int64) (*AbsenceEmailContact, error) {
			return &AbsenceEmailContact{
				StaffID:   staffID,
				FirstName: "Mila",
				LastName:  "Muster",
			}, nil
		},
		ApproversFn: func(context.Context) ([]*AbsenceEmailContact, error) {
			return []*AbsenceEmailContact{
				{StaffID: int64(44), FirstName: "Lena", LastName: "Leitung", Email: "lena@example.test"},
			}, nil
		},
	}
	svc, mailer := newAbsenceNotificationTestService(t, absSettingsMock{enabled: true}, staffRepo)
	ctx, _ := tenant.WithAfterCommitHooksForTest(context.Background())

	svc.notifyAbsenceRequested(ctx, notificationTestAbsence(AbsenceStatusRequested))

	assert.False(t, mailer.WaitForMessages(1, 150*time.Millisecond), "rollback must drop the queued email")
	assert.Empty(t, mailer.Messages())
}

func TestNotifyAbsenceDecision_CoversStatusesAndRecipientFailures(t *testing.T) {
	t.Parallel()

	t.Run("ignores unrelated status", func(t *testing.T) {
		staffRepo := &absenceEmailStaffStub{
			ContactFn: func(context.Context, int64) (*AbsenceEmailContact, error) {
				t.Fatal("unrelated status must not load the requester")
				return nil, nil
			},
		}
		svc, mailer := newAbsenceNotificationTestService(t, absSettingsMock{enabled: true}, staffRepo)

		svc.notifyAbsenceDecision(context.Background(), notificationTestAbsence(AbsenceStatusRequested))

		assert.Empty(t, mailer.Messages())
	})

	t.Run("requester lookup fails", func(t *testing.T) {
		staffRepo := &absenceEmailStaffStub{
			ContactFn: func(context.Context, int64) (*AbsenceEmailContact, error) {
				return nil, errors.New("requester unavailable")
			},
		}
		svc, mailer := newAbsenceNotificationTestService(t, absSettingsMock{enabled: true}, staffRepo)

		svc.notifyAbsenceDecision(context.Background(), notificationTestAbsence(AbsenceStatusApproved))

		assert.Empty(t, mailer.Messages())
	})

	t.Run("requester has no email", func(t *testing.T) {
		staffRepo := &absenceEmailStaffStub{
			ContactFn: func(context.Context, int64) (*AbsenceEmailContact, error) {
				return &AbsenceEmailContact{FirstName: "Mila", LastName: "Muster"}, nil
			},
		}
		svc, mailer := newAbsenceNotificationTestService(t, absSettingsMock{enabled: true}, staffRepo)

		svc.notifyAbsenceDecision(context.Background(), notificationTestAbsence(AbsenceStatusDeclined))

		assert.Empty(t, mailer.Messages())
	})

	for _, tt := range []struct {
		status   string
		template string
		subject  string
	}{
		{
			status:   AbsenceStatusApproved,
			template: "absence-request-approved.html",
			subject:  "Dein Abwesenheitsantrag wurde genehmigt",
		},
		{
			status:   AbsenceStatusDeclined,
			template: "absence-request-declined.html",
			subject:  "Dein Abwesenheitsantrag wurde abgelehnt",
		},
	} {
		t.Run(tt.status, func(t *testing.T) {
			staffRepo := &absenceEmailStaffStub{
				ContactFn: func(context.Context, int64) (*AbsenceEmailContact, error) {
					return &AbsenceEmailContact{
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
