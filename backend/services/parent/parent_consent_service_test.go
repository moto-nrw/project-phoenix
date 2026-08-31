package parent_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type consentRecordingUnlinker struct{ urls []string }

func (r *consentRecordingUnlinker) UnlinkStored(url string) { r.urls = append(r.urls, url) }

func buildConsentService(t *testing.T) (parentService.Service, *bun.DB, *repositories.Factory, *consentRecordingUnlinker) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	unlinker := &consentRecordingUnlinker{}
	photos := usersService.NewStudentPhotoService(usersService.StudentPhotoServiceDependencies{
		StudentRepo: repos.Student,
		Broadcaster: testpkg.NewRecordingBroadcaster(),
		Unlinker:    unlinker,
		DB:          db,
		Logger:      slog.Default(),
	})
	return parentService.NewService(parentService.ServiceConfig{
		ChildRepo:             repos.ParentChild,
		StudentRepo:           repos.Student,
		StudentGuardianRepo:   repos.StudentGuardian,
		StudentConsentChanges: repos.StudentConsentChange,
		StudentConsents:       usersService.NewStudentConsentRecorder(repos.StudentConsentChange),
		StudentPhotos:         photos,
		Broadcaster:           testpkg.NewRecordingBroadcaster(),
		DB:                    db,
		Logger:                slog.Default(),
	}), db, repos, unlinker
}

func TestGetChildConsentsShowsRecordedStatesAndWithdrawalCapability(t *testing.T) {
	t.Parallel()

	svc, db, repos, _ := buildConsentService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	now := time.Date(2026, time.August, 31, 9, 30, 0, 0, time.UTC)
	student, err := repos.Student.FindByID(testpkg.Ctx(t), chain.StudentID)
	require.NoError(t, err)
	student.AGBAcceptedAt = &now
	student.DataProcessingAcceptedAt = &now
	student.PhotoConsentGivenAt = &now
	require.NoError(t, repos.Student.Update(testpkg.Ctx(t), student))

	consents, err := svc.GetChildConsents(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, consents, 4)

	byKey := make(map[string]parentService.ChildConsent, len(consents))
	for _, consent := range consents {
		byKey[consent.Key] = consent
	}
	assert.Equal(t, "granted", byKey["agb"].State)
	assert.Equal(t, "granted", byKey["data_processing"].State)
	assert.Equal(t, "not_recorded", byKey["email_contact"].State)
	assert.Equal(t, "granted", byKey["photo"].State)
	assert.True(t, byKey["photo"].CanWithdraw)
	assert.False(t, byKey["agb"].CanWithdraw)
	assert.True(t, byKey["photo"].ChangedAt.Equal(now))
}

func TestWithdrawPhotoConsentRemovesPhotoAndRecordsOneHistoryEntry(t *testing.T) {
	t.Parallel()

	svc, db, repos, unlinker := buildConsentService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	now := time.Date(2026, time.August, 31, 9, 30, 0, 0, time.UTC)
	storedURL := "/uploads/students/portrait.jpg"
	student, err := repos.Student.FindByID(testpkg.Ctx(t), chain.StudentID)
	require.NoError(t, err)
	student.PhotoConsentGivenAt = &now
	student.PhotoPath = &storedURL
	require.NoError(t, repos.Student.Update(testpkg.Ctx(t), student))

	consents, err := svc.WithdrawPhotoConsent(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.Equal(t, ChildConsentState(consents, "photo"), "withdrawn")
	assert.Equal(t, []string{storedURL}, unlinker.urls)

	// DELETE is idempotent: the current state stays withdrawn and the audit
	// history receives no duplicate event.
	_, err = svc.WithdrawPhotoConsent(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	rows, err := repos.StudentConsentChange.ListByStudentID(testpkg.Ctx(t), chain.StudentID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "withdrawn", rows[0].Action)
	assert.Equal(t, "parent_portal", rows[0].Source)
	assert.Equal(t, chain.AccountID, *rows[0].ActorAccountID)
}

func ChildConsentState(consents []parentService.ChildConsent, key string) string {
	for _, consent := range consents {
		if consent.Key == key {
			return consent.State
		}
	}
	return ""
}
