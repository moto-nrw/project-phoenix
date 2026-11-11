package parentportal_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// consentRecordingUnlinker records the files a withdrawal removes. onUnlink,
// when set, runs at the moment the bytes would go, after the commit.
type consentRecordingUnlinker struct {
	urls     []string
	onUnlink func(url string)
}

func (r *consentRecordingUnlinker) UnlinkStored(url string) {
	if r.onUnlink != nil {
		r.onUnlink(url)
	}
	r.urls = append(r.urls, url)
}

// consentPhotoUnlinker is the only part of the photo lifecycle this portal
// reaches: a withdrawal detaches the stored file after the commit. Like the
// real binding it queues the removal on the write's transaction, so a
// rolled-back withdrawal removes nothing.
type consentPhotoUnlinker struct{ unlinker *consentRecordingUnlinker }

func (u consentPhotoUnlinker) ScheduleUnlinkAfterCommit(ctx context.Context, storedURL string) error {
	if storedURL == "" {
		return nil
	}
	if !tenant.HasAfterCommitHooks(ctx) {
		return errors.New("photo unlink scheduled outside the write's transaction")
	}
	tenant.RegisterAfterCommit(ctx, func() { u.unlinker.UnlinkStored(storedURL) })
	return nil
}

func buildConsentService(t *testing.T) (*parentService.Portal, *bun.DB, *api.PeopleRepositorySuiteFactory, *consentRecordingUnlinker) {
	t.Helper()
	return buildConsentServiceWith(t, nil)
}

// buildConsentServiceWith lets a test adjust the portal's dependencies
// before they are composed.
func buildConsentServiceWith(t *testing.T, adjust func(*parentportalcompose.Dependencies)) (*parentService.Portal, *bun.DB, *api.PeopleRepositorySuiteFactory, *consentRecordingUnlinker) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	unlinker := &consentRecordingUnlinker{}
	photos := consentPhotoUnlinker{unlinker: unlinker}
	deps := parentportalcompose.Dependencies{
		Memberships:         parentMemberships(t, db),
		GuardianProfileRepo: guardProfiles(t, db),
		GuardianChanges:     owners.ParentGuardianChanges(),
		DeparturePolicy:     owners.ParentDeparturePolicy(),
		GuardianPolicy:      owners.NewParentGuardianPolicy(),
		Fingerprints:        owners.SecurityFingerprints{},
		Declarations:        owners.ParentDeclarationPolicy(),
		Locales:             owners.ParentPortalLocales(),
		Runtime:             owners.ParentPortalRuntime{},
		CarePlan:            owners.NewParentCarePlanCommands(repos.CarePlan(), nil),
		People:              owners.MustNewPeopleDirectory(testpkg.MustDatabase(db)),
		CareProfiles:        careProfiles(t, db),
		ChildRepo:           repos.ParentChildReads(),
		StudentRepo:         owners.NewParentStudentReads(reposPeople.Student),
		StudentGuardianRepo: owners.NewParentStudentGuardianReads(reposPeople.StudentGuardian),
		StudentConsents:     owners.NewParentStudentConsents(api.NewStudentConsents(testpkg.NewDatabase(t, db))),
		StudentPhotos:       photos,
		StudentUpdates:      owners.BindParentStudentUpdates(testpkg.NewRecordingBroadcaster()),
		Logger:              slog.Default(),

		DataRequests: owners.NewParentDataRequestCommands(repos.CarePlan()),

		CarePlanPolicy: owners.NewParentCarePlanPolicy(),
	}
	if adjust != nil {
		adjust(&deps)
	}
	return parentportalcompose.New(deps), db, reposPeople, unlinker
}

func TestGetChildConsentsShowsRecordedStatesAndWithdrawalCapability(t *testing.T) {
	t.Parallel()

	svc, db, repos, _ := buildConsentService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	now := time.Date(2026, time.August, 31, 9, 30, 0, 0, time.UTC)
	student, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, repos.Student.FindByID, any(chain.StudentID))
	require.NoError(t, err)
	student.AGBAcceptedAt = &now
	student.DataProcessingAcceptedAt = &now
	student.PhotoConsentGivenAt = &now
	require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), db, repos.Student.Update, student))

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
	student, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, repos.Student.FindByID, any(chain.StudentID))
	require.NoError(t, err)
	student.PhotoConsentGivenAt = &now
	student.PhotoPath = &storedURL
	require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), db, repos.Student.Update, student))

	consents, err := svc.WithdrawPhotoConsent(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.Equal(t, ChildConsentState(consents, "photo"), "withdrawn")
	for _, consent := range consents {
		if consent.Key == "photo" {
			assert.True(t, consent.CanGrant)
		}
	}
	assert.Equal(t, []string{storedURL}, unlinker.urls)

	// DELETE is idempotent: the current state stays withdrawn and the audit
	// history receives no duplicate event.
	_, err = svc.WithdrawPhotoConsent(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	rows, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, owners.NewTestAuditPersistence(testpkg.NewDatabase(t, db), nil).StudentConsentChange.ListByStudentID, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "withdrawn", rows[0].Action)
	assert.Equal(t, "parent_portal", rows[0].Source)
	assert.Equal(t, chain.AccountID, *rows[0].ActorAccountID)
}

func TestGrantPhotoConsentAfterWithdrawalRecordsNewStateWithoutRestoringPhoto(t *testing.T) {
	t.Parallel()

	svc, db, repos, unlinker := buildConsentService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	now := time.Date(2026, time.August, 31, 9, 30, 0, 0, time.UTC)
	storedURL := "/uploads/students/portrait.jpg"
	student, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, repos.Student.FindByID, any(chain.StudentID))
	require.NoError(t, err)
	student.PhotoConsentGivenAt = &now
	student.PhotoPath = &storedURL
	require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), db, repos.Student.Update, student))

	_, err = svc.WithdrawPhotoConsent(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	consents, err := svc.GrantPhotoConsent(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.Equal(t, "granted", ChildConsentState(consents, "photo"))
	assert.Equal(t, []string{storedURL}, unlinker.urls)

	updated, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, repos.Student.FindByID, any(chain.StudentID))
	require.NoError(t, err)
	require.NotNil(t, updated.PhotoConsentGivenAt)
	require.NotNil(t, updated.PhotoConsentGivenBy)
	assert.Equal(t, chain.AccountID, *updated.PhotoConsentGivenBy)
	assert.Nil(t, updated.PhotoPath)

	// PUT is idempotent: a retry keeps the current state and does not append a
	// second grant event.
	_, err = svc.GrantPhotoConsent(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	rows, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, owners.NewTestAuditPersistence(testpkg.NewDatabase(t, db), nil).StudentConsentChange.ListByStudentID, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "granted", rows[0].Action)
	assert.Equal(t, "withdrawn", rows[1].Action)
	assert.Equal(t, "parent_portal", rows[0].Source)
	assert.Equal(t, chain.AccountID, *rows[0].ActorAccountID)
}

func TestGrantPhotoConsentRejectsNeverRecordedConsent(t *testing.T) {
	t.Parallel()

	svc, db, repos, _ := buildConsentService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := svc.GrantPhotoConsent(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.ErrorIs(t, err, parentService.ErrPhotoConsentNotWithdrawn)

	student, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, repos.Student.FindByID, any(chain.StudentID))
	require.NoError(t, err)
	assert.Nil(t, student.PhotoConsentGivenAt)
	assert.Nil(t, student.PhotoConsentGivenBy)

	rows, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, owners.NewTestAuditPersistence(testpkg.NewDatabase(t, db), nil).StudentConsentChange.ListByStudentID, chain.StudentID)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

// #6494, decision A: the end of care is no end of the voluntary photo
// consent. A guardian with portal access and the consent permission withdraws
// it after the care ended exactly as before: consent and photo reference go
// in one write with one history entry, the file only after the commit, and a
// repeated withdrawal adds nothing. Giving the consent again stays refused.
func TestPhotoConsentWithdrawalAfterCareEndClearsConsentAndPhoto(t *testing.T) {
	t.Parallel()

	svc, db, repos, unlinker := buildConsentService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	now := time.Date(2026, time.August, 31, 9, 30, 0, 0, time.UTC)
	storedURL := "/uploads/students/portrait.jpg"
	student, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, repos.Student.FindByID, any(chain.StudentID))
	require.NoError(t, err)
	student.PhotoConsentGivenAt = &now
	student.PhotoPath = &storedURL
	require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), db, repos.Student.Update, student))
	endCareFor(t, db, chain.StudentID)

	// The bytes go only once the cleared row is committed: at that moment a
	// separate connection already reads neither the photo nor the consent.
	var photoAtUnlink, consentAtUnlink bool
	unlinker.onUnlink = func(string) {
		committed, readErr := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, repos.Student.FindByID, any(chain.StudentID))
		assert.NoError(t, readErr)
		if committed != nil {
			photoAtUnlink, consentAtUnlink = committed.PhotoPath != nil, committed.PhotoConsentGivenAt != nil
		}
	}
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	consents, err := svc.WithdrawPhotoConsent(ctx, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	photo := photoConsent(t, consents)
	assert.Equal(t, "withdrawn", photo.State)
	assert.False(t, photo.CanGrant, "after the care ended the portal offers no new consent")
	assert.Equal(t, []string{storedURL}, unlinker.urls)
	assert.False(t, photoAtUnlink, "the file is removed only after the cleared photo reference is committed")
	assert.False(t, consentAtUnlink, "the file is removed only after the cleared consent is committed")

	updated, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, repos.Student.FindByID, any(chain.StudentID))
	require.NoError(t, err)
	assert.Nil(t, updated.PhotoPath)
	assert.Nil(t, updated.PhotoConsentGivenAt)
	assert.Nil(t, updated.PhotoConsentGivenBy)

	_, err = svc.WithdrawPhotoConsent(ctx, chain.AccountID, chain.StudentID)
	require.NoError(t, err, "a repeated withdrawal stays successful")
	_, err = svc.GrantPhotoConsent(ctx, chain.AccountID, chain.StudentID)
	require.ErrorIs(t, err, parentService.ErrChildCareEnded, "giving the consent again stays refused")

	rows, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, owners.NewTestAuditPersistence(testpkg.NewDatabase(t, db), nil).StudentConsentChange.ListByStudentID, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "withdrawn", rows[0].Action)
	assert.Equal(t, chain.AccountID, *rows[0].ActorAccountID)
	assert.Equal(t, []string{storedURL}, unlinker.urls)
	still, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, repos.Student.FindByID, any(chain.StudentID))
	require.NoError(t, err)
	assert.Nil(t, still.PhotoConsentGivenAt)
}

// A consent withdrawn while the child was still in care cannot be given
// again after the care ended.
func TestPhotoConsentGrantAfterCareEndStaysRefused(t *testing.T) {
	t.Parallel()

	svc, db, repos, unlinker := buildConsentService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	now := time.Date(2026, time.August, 31, 9, 30, 0, 0, time.UTC)
	student, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, repos.Student.FindByID, any(chain.StudentID))
	require.NoError(t, err)
	student.PhotoConsentGivenAt = &now
	require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), db, repos.Student.Update, student))
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	_, err = svc.WithdrawPhotoConsent(ctx, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	endCareFor(t, db, chain.StudentID)

	consents, err := svc.GetChildConsents(ctx, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.False(t, photoConsent(t, consents).CanGrant)
	_, err = svc.GrantPhotoConsent(ctx, chain.AccountID, chain.StudentID)
	require.ErrorIs(t, err, parentService.ErrChildCareEnded)

	updated, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, repos.Student.FindByID, any(chain.StudentID))
	require.NoError(t, err)
	assert.Nil(t, updated.PhotoConsentGivenAt)
	assert.Empty(t, unlinker.urls)
	rows, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, owners.NewTestAuditPersistence(testpkg.NewDatabase(t, db), nil).StudentConsentChange.ListByStudentID, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, rows, 1, "only the withdrawal before the end of care")
}

// The check before the write unit read the child while its care still ran;
// the care ends before the grant locks the child. The grant re-checks the
// care under the child lock and is refused.
func TestPhotoConsentGrantRechecksTheCareEndUnderTheChildLock(t *testing.T) {
	t.Parallel()

	var armed atomic.Bool
	reached, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { close(release) })
	svc, db, repos, _ := buildConsentServiceWith(t, func(deps *parentportalcompose.Dependencies) {
		deps.ChildRepo = owners.ParentChildReadsAfterCheck(deps.ChildRepo, func() {
			if armed.Load() {
				once.Do(func() {
					close(reached)
					<-release
				})
			}
		})
	})
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	now := time.Date(2026, time.August, 31, 9, 30, 0, 0, time.UTC)
	student, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, repos.Student.FindByID, any(chain.StudentID))
	require.NoError(t, err)
	student.PhotoConsentGivenAt = &now
	require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), db, repos.Student.Update, student))
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	_, err = svc.WithdrawPhotoConsent(ctx, chain.AccountID, chain.StudentID)
	require.NoError(t, err)

	armed.Store(true)
	granted := make(chan error, 1)
	go func() {
		_, grantErr := svc.GrantPhotoConsent(ctx, chain.AccountID, chain.StudentID)
		granted <- grantErr
	}()
	select {
	case <-reached:
	case err := <-granted:
		t.Fatalf("the grant answered before its child check: %v", err)
	case <-time.After(20 * time.Second):
		t.Fatal("the grant never reached its child check")
	}
	endCareFor(t, db, chain.StudentID)
	release <- struct{}{}

	select {
	case err := <-granted:
		require.ErrorIs(t, err, parentService.ErrChildCareEnded)
	case <-time.After(20 * time.Second):
		t.Fatal("the grant did not finish")
	}
	updated, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, repos.Student.FindByID, any(chain.StudentID))
	require.NoError(t, err)
	assert.Nil(t, updated.PhotoConsentGivenAt)
}

// The withdrawal after the end of care still needs the relationship's
// consent permission; another family's child and a child of another school
// stay out of reach. None of these refusals changes the consent or the photo.
func TestPhotoConsentWithdrawalAfterCareEndKeepsRelationshipRights(t *testing.T) {
	t.Parallel()

	svc, db, repos, unlinker := buildConsentService(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	now := time.Date(2026, time.August, 31, 9, 30, 0, 0, time.UTC)
	storedURL := "/uploads/students/portrait.jpg"
	student, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, repos.Student.FindByID, any(chain.StudentID))
	require.NoError(t, err)
	student.PhotoConsentGivenAt = &now
	student.PhotoPath = &storedURL
	require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), db, repos.Student.Update, student))
	endCareFor(t, db, chain.StudentID)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	t.Run("ohne Einwilligungsrecht", func(t *testing.T) {
		limited := testpkg.CreateTestCoGuardianForStudent(t, db, chain.StudentID, "Jonas", "Schneider")
		_, err := db.ExecContext(testpkg.WithPackageTenantRuntime(context.Background()), `
			UPDATE auth.guardian_student_access AS a
			SET permissions = '{"parent_portal.access": true}'::jsonb
			FROM users.student_guardian_relationships AS r
			WHERE r.tenant_id = a.tenant_id AND r.id = a.relationship_id
			  AND r.tenant_id = ? AND r.student_id = ? AND r.guardian_profile_id = ?
		`, limited.TenantID, chain.StudentID, limited.GuardianProfileID)
		require.NoError(t, err)
		_, err = svc.WithdrawPhotoConsent(ctx, limited.AccountID, chain.StudentID)
		require.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied)
	})
	t.Run("fremde Familie", func(t *testing.T) {
		stranger := testpkg.CreateTestParentGuardianChain(t, db)
		_, err := svc.WithdrawPhotoConsent(ctx, stranger.AccountID, chain.StudentID)
		require.ErrorIs(t, err, parentService.ErrChildNotLinked)
	})
	t.Run("fremde Schule", func(t *testing.T) {
		testpkg.OwnTenant(t)
		stranger := testpkg.CreateTestParentGuardianChain(t, db)
		_, err := svc.WithdrawPhotoConsent(ctx, stranger.AccountID, chain.StudentID)
		require.ErrorIs(t, err, parentService.ErrChildNotLinked)
	})

	updated, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, repos.Student.FindByID, any(chain.StudentID))
	require.NoError(t, err)
	require.NotNil(t, updated.PhotoConsentGivenAt)
	require.NotNil(t, updated.PhotoPath)
	assert.Equal(t, storedURL, *updated.PhotoPath)
	assert.Empty(t, unlinker.urls)
	rows, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, owners.NewTestAuditPersistence(testpkg.NewDatabase(t, db), nil).StudentConsentChange.ListByStudentID, chain.StudentID)
	require.NoError(t, err)
	assert.Empty(t, rows)
}

func photoConsent(t *testing.T, consents []parentService.ChildConsent) parentService.ChildConsent {
	t.Helper()
	for _, consent := range consents {
		if consent.Key == "photo" {
			return consent
		}
	}
	t.Fatal("no photo consent in the projection")
	return parentService.ChildConsent{}
}

func ChildConsentState(consents []parentService.ChildConsent, key string) string {
	for _, consent := range consents {
		if consent.Key == key {
			return consent.State
		}
	}
	return ""
}
