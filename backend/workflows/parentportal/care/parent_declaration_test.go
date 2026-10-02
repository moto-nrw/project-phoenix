package care_test

// Integration tests for Erklärungen (#3430): the guardian's declaration per
// child with its permission, version, deadline and idempotency guards, the
// feed block the portal renders, the unread badge and the guardian's proof.
// Real guardian->student chains, so the relationship permission and the
// audience resolution run through the repository SQL.

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/messaging"
)

type declarationSetup struct {
	svc          *messaging.Service
	db           *bun.DB
	repo         usersModels.ParentAnnouncementRepository
	chain        testpkg.ParentChain
	seedCtx      context.Context
	ctx          context.Context
	announcement *usersModels.ParentAnnouncement
	version      *usersModels.DeclarationVersion
}

func newDeclarationSetup(t *testing.T, settings usersModels.AnnouncementDeclarationSettings, deadline *time.Time) *declarationSetup {
	t.Helper()
	testpkg.OwnTenant(t)
	svc, db, repos := buildAnnouncementServiceOn(t, testpkg.SetupIsolatedTestDB(t), true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	setup := &declarationSetup{
		svc: svc, db: db, repo: repos.ParentAnnouncement, chain: chain,
		seedCtx: tenant.WithTenantID(testpkg.WithTestTenantRuntime(t, context.Background()), chain.TenantID),
		ctx:     testpkg.WithTestTenantRuntime(t, context.Background()),
	}
	setup.announcement, setup.version = seedDeclaration(t, setup.seedCtx, setup.repo, chain.AccountID, chain.TenantID,
		[]*usersModels.ParentAnnouncementTarget{{TargetType: usersModels.AnnouncementTargetStudent, TargetRefID: &chain.StudentID}},
		settings, deadline)
	return setup
}

// seedDeclaration creates a published Erklärung with its first frozen
// version, the state the staff publish leaves behind.
func seedDeclaration(
	t *testing.T, ctx context.Context, repo usersModels.ParentAnnouncementRepository, createdBy, tenantID int64,
	targets []*usersModels.ParentAnnouncementTarget, settings usersModels.AnnouncementDeclarationSettings, deadline *time.Time,
) (*usersModels.ParentAnnouncement, *usersModels.DeclarationVersion) {
	t.Helper()
	a := &usersModels.ParentAnnouncement{
		Title:            "Ausflug in den Zoo",
		Body:             "Wir fahren am Freitag mit dem Bus in den Zoo.",
		Priority:         usersModels.ParentAnnouncementPriorityInfo,
		DeliveryMode:     usersModels.ParentAnnouncementDeliveryDeclaration,
		ResponseDeadline: deadline,
		Declaration:      settings,
		Active:           true,
		CreatedBy:        createdBy,
	}
	a.SetTenantID(tenantID)
	require.NoError(t, repo.Create(ctx, a))
	require.NoError(t, repo.ReplaceTargets(ctx, tenantID, a.ID, targets))
	version := freezeTestVersion(t, ctx, repo, a, 1)
	now := time.Now().Add(-time.Minute)
	require.NoError(t, repo.SetPublished(ctx, a.ID, &now))
	return a, version
}

func freezeTestVersion(t *testing.T, ctx context.Context, repo usersModels.ParentAnnouncementRepository, a *usersModels.ParentAnnouncement, number int) *usersModels.DeclarationVersion {
	t.Helper()
	version := &usersModels.DeclarationVersion{
		TenantID: a.GetTenantID(), AnnouncementID: a.ID, VersionNo: number, Title: a.Title,
		Body: a.Body + " " + string(rune('A'+number)), Kind: a.Declaration.Kind, PublishedAt: time.Now(),
	}
	version.ContentHash = messaging.DeclarationContentHash(version)
	require.NoError(t, repo.InsertDeclarationVersion(ctx, version))
	return version
}

func consent(signers string, revocable, password bool) usersModels.AnnouncementDeclarationSettings {
	return usersModels.AnnouncementDeclarationSettings{
		Kind: usersModels.DeclarationKindConsent, Signers: signers, Revocable: revocable, RequiresPassword: password,
	}
}

func (s *declarationSetup) submit(accountID int64, action string) (*usersModels.DeclarationSubmission, bool, error) {
	return s.svc.SubmitDeclaration(s.ctx, accountID, s.announcement.ID, messaging.DeclarationInput{
		StudentID: s.chain.StudentID, VersionID: s.version.ID, Action: action,
	}, nil)
}

func (s *declarationSetup) feedDeclaration(t *testing.T, accountID int64) *usersModels.AnnouncementFeedDeclaration {
	t.Helper()
	feed, err := s.svc.ListAnnouncements(s.ctx, accountID)
	require.NoError(t, err)
	item := findFeedItem(t, feed, s.announcement.ID)
	require.NotNil(t, item.Declaration)
	return item.Declaration
}

func (s *declarationSetup) history(t *testing.T) []*usersModels.DeclarationSubmission {
	t.Helper()
	rows, err := s.repo.ListDeclarationSubmissions(s.seedCtx, s.chain.TenantID, s.announcement.ID)
	require.NoError(t, err)
	return rows
}

func TestDeclaration_ConsentIsRecordedOnceAndShownInFeed(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(usersModels.DeclarationSignersAny, false, false), nil)

	before := s.feedDeclaration(t, s.chain.AccountID)
	require.Len(t, before.Children, 1)
	assert.True(t, before.Children[0].CanSubmit)
	assert.Equal(t, usersModels.DeclarationStateOpen, before.Children[0].State)
	assert.Equal(t, []string{"agreed", "declined"}, before.Children[0].AllowedActions)
	require.NotNil(t, before.Version)
	assert.Equal(t, s.version.ID, before.Version.ID)

	first, created, err := s.submit(s.chain.AccountID, usersModels.DeclarationActionAgreed)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, 1, first.VersionNo)
	assert.Equal(t, s.version.ContentHash, first.ContentHash)
	assert.Equal(t, messaging.DeclarationRecordHash(first), first.RecordHash)

	// A double click stores nothing new and answers with the same proof.
	again, created, err := s.submit(s.chain.AccountID, usersModels.DeclarationActionAgreed)
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, first.ID, again.ID)

	history := s.history(t)
	require.Len(t, history, 1)
	stored := history[0]
	assert.Equal(t, "Sabine Schneider", stored.SignerName)
	assert.Equal(t, "primary_guardian", stored.GuardianRole)
	assert.Equal(t, usersModels.DeclarationMethodSimpleElectronic, stored.Method)
	assert.Equal(t, stored.RecordHash, messaging.DeclarationRecordHash(stored),
		"the record hash must survive the database round trip")

	after := s.feedDeclaration(t, s.chain.AccountID)
	assert.Equal(t, usersModels.DeclarationStateAgreed, after.Children[0].State)
	require.NotNil(t, after.Children[0].MyAction)
	assert.Equal(t, usersModels.DeclarationActionAgreed, *after.Children[0].MyAction)
	assert.Equal(t, []string{"declined"}, after.Children[0].AllowedActions)
}

func TestDeclaration_ConcurrentSubmissionsStoreOneRow(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(usersModels.DeclarationSignersAny, false, false), nil)

	var wg sync.WaitGroup
	createdCount := 0
	var mu sync.Mutex
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, created, err := s.submit(s.chain.AccountID, usersModels.DeclarationActionAgreed)
			assert.NoError(t, err)
			if created {
				mu.Lock()
				createdCount++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, 1, createdCount)
	assert.Len(t, s.history(t), 1)
}

func TestDeclaration_ChangedVersionIsRefusedAndKeepsOldProof(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(usersModels.DeclarationSignersAny, false, false), nil)
	_, _, err := s.submit(s.chain.AccountID, usersModels.DeclarationActionAgreed)
	require.NoError(t, err)

	// A correction publishes version 2: the declaration on version 1 stays
	// stored, but no longer settles the child, and a request for version 1 is
	// refused so the guardian sees the new wording first.
	second := freezeTestVersion(t, s.seedCtx, s.repo, s.announcement, 2)
	_, _, err = s.submit(s.chain.AccountID, usersModels.DeclarationActionAgreed)
	require.ErrorIs(t, err, messaging.ErrDeclarationVersionChanged)

	feed := s.feedDeclaration(t, s.chain.AccountID)
	assert.Equal(t, second.ID, feed.Version.ID)
	assert.Equal(t, usersModels.DeclarationStateOpen, feed.Children[0].State)

	s.version = second
	_, created, err := s.submit(s.chain.AccountID, usersModels.DeclarationActionAgreed)
	require.NoError(t, err)
	assert.True(t, created)
	history := s.history(t)
	require.Len(t, history, 2)
	assert.Equal(t, 2, history[0].VersionNo)
	assert.Equal(t, 1, history[1].VersionNo)
}

func TestDeclaration_WithdrawnPermissionRefusesTheDeclaration(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(usersModels.DeclarationSignersAny, false, false), nil)
	revokeDeclarationPermission(t, s.db, s.chain.AccountID)

	_, _, err := s.submit(s.chain.AccountID, usersModels.DeclarationActionAgreed)
	require.ErrorIs(t, err, messaging.ErrDeclarationNotPermitted)

	// The guardian still sees the Erklärung, but gets no buttons.
	feed := s.feedDeclaration(t, s.chain.AccountID)
	require.Len(t, feed.Children, 1)
	assert.False(t, feed.Children[0].CanSubmit)
	assert.Empty(t, feed.Children[0].AllowedActions)
	assert.Equal(t, usersModels.DeclarationStateNoSigner, feed.Children[0].State)
	assert.Empty(t, s.history(t))
}

func TestDeclaration_WithdrawnPermissionAllowsOwnRevocation(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(usersModels.DeclarationSignersAny, true, false), nil)

	_, created, err := s.submit(s.chain.AccountID, usersModels.DeclarationActionAgreed)
	require.NoError(t, err)
	require.True(t, created)
	revokeDeclarationPermission(t, s.db, s.chain.AccountID)

	revoked, created, err := s.submit(s.chain.AccountID, usersModels.DeclarationActionRevoked)
	require.NoError(t, err)
	assert.True(t, created)
	assert.Equal(t, usersModels.DeclarationActionRevoked, revoked.Action)
}

func TestDeclaration_ForeignChildAndForeignSchoolAreNotFound(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(usersModels.DeclarationSignersAny, false, false), nil)

	// Another family's child in the same school, not reached for this account.
	other := testpkg.CreateTestParentGuardianChain(t, s.db)
	_, _, err := s.svc.SubmitDeclaration(s.ctx, other.AccountID, s.announcement.ID, messaging.DeclarationInput{
		StudentID: s.chain.StudentID, VersionID: s.version.ID, Action: usersModels.DeclarationActionAgreed,
	}, nil)
	require.ErrorIs(t, err, messaging.ErrAnnouncementNotFound)

	// An Erklärung of another school the account has no child at.
	foreignTenant, _ := testpkg.CreateTestTenant(t, s.db)
	foreignCtx := tenant.WithTenantID(testpkg.WithTestTenantRuntime(t, context.Background()), foreignTenant)
	foreign, foreignVersion := seedDeclaration(t, foreignCtx, s.repo, s.chain.AccountID, foreignTenant,
		[]*usersModels.ParentAnnouncementTarget{{TargetType: usersModels.AnnouncementTargetSchoolAll}},
		consent(usersModels.DeclarationSignersAny, false, false), nil)
	_, _, err = s.svc.SubmitDeclaration(s.ctx, s.chain.AccountID, foreign.ID, messaging.DeclarationInput{
		StudentID: s.chain.StudentID, VersionID: foreignVersion.ID, Action: usersModels.DeclarationActionAgreed,
	}, nil)
	require.ErrorIs(t, err, messaging.ErrAnnouncementNotFound)
	assert.Empty(t, s.history(t))
}

func TestDeclaration_DeadlineAllowsOnlyRevocation(t *testing.T) {
	t.Parallel()
	deadline := time.Now().Add(time.Hour)
	s := newDeclarationSetup(t, consent(usersModels.DeclarationSignersAny, true, false), &deadline)
	_, _, err := s.submit(s.chain.AccountID, usersModels.DeclarationActionRevoked)
	require.ErrorIs(t, err, messaging.ErrDeclarationActionNotAllowed, "nothing to revoke before consenting")
	_, _, err = s.submit(s.chain.AccountID, usersModels.DeclarationActionAgreed)
	require.NoError(t, err)

	moveDeclarationDeadline(t, s.db, s.announcement.ID, time.Now().Add(-time.Minute))

	_, _, err = s.submit(s.chain.AccountID, usersModels.DeclarationActionDeclined)
	require.ErrorIs(t, err, messaging.ErrDeclarationClosed)
	revoked, created, err := s.submit(s.chain.AccountID, usersModels.DeclarationActionRevoked)
	require.NoError(t, err, "a consent can always be withdrawn")
	assert.True(t, created)
	assert.Equal(t, usersModels.DeclarationActionRevoked, revoked.Action)

	feed := s.feedDeclaration(t, s.chain.AccountID)
	assert.True(t, feed.Closed)
	assert.Equal(t, usersModels.DeclarationStateRevoked, feed.Children[0].State)
	assert.Len(t, s.history(t), 2, "the consent and its revocation both stay on record")
}

func TestDeclaration_ExpiredAllowsOnlyRevocation(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(usersModels.DeclarationSignersAny, true, false), nil)
	_, _, err := s.submit(s.chain.AccountID, usersModels.DeclarationActionAgreed)
	require.NoError(t, err)

	_, err = s.db.NewUpdate().TableExpr("users.parent_announcements").
		Set("created_at = created_at - interval '2 hours', expires_at = created_at - interval '1 hour'").
		Where("id = ?", s.announcement.ID).
		Exec(testpkg.WithPackageTenantRuntime(context.Background()))
	require.NoError(t, err)

	_, _, err = s.submit(s.chain.AccountID, usersModels.DeclarationActionDeclined)
	require.ErrorIs(t, err, messaging.ErrAnnouncementNotFound)
	revoked, created, err := s.submit(s.chain.AccountID, usersModels.DeclarationActionRevoked)
	require.NoError(t, err, "a consent can be withdrawn after the declaration expires")
	assert.True(t, created)
	assert.Equal(t, usersModels.DeclarationActionRevoked, revoked.Action)
	assert.Len(t, s.history(t), 2)
}

func TestDeclaration_EndedActivityEnrollmentKeepsHistoryAndRevocation(t *testing.T) {
	t.Parallel()
	testpkg.OwnTenant(t)
	svc, db, repos := buildAnnouncementServiceOn(t, testpkg.SetupIsolatedTestDB(t), true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	seedCtx := tenant.WithTenantID(testpkg.WithTestTenantRuntime(t, context.Background()), chain.TenantID)
	ctx := testpkg.WithTestTenantRuntime(t, context.Background())
	group := testpkg.CreateTestActivityGroupForTenant(t, db, chain.TenantID, "Nachweis-AG")
	enrollment := &testpkg.StudentEnrollment{
		StudentID:       chain.StudentID,
		ActivityGroupID: group.ID,
		ValidFrom:       testpkg.ActivityDate(calendar.TodayDate().AddDays(-1)),
	}
	enrollment.SetTenantID(chain.TenantID)
	_, err := db.NewInsert().
		Model(enrollment).
		ModelTableExpr(`activities.student_enrollments AS "enrollment"`).
		Exec(context.Background())
	require.NoError(t, err)

	announcement, version := seedDeclaration(t, seedCtx, repos.ParentAnnouncement, chain.AccountID, chain.TenantID,
		[]*usersModels.ParentAnnouncementTarget{{TargetType: usersModels.AnnouncementTargetActivityGroup, TargetRefID: &group.ID}},
		consent(usersModels.DeclarationSignersAny, true, false), nil)
	_, created, err := svc.SubmitDeclaration(ctx, chain.AccountID, announcement.ID, messaging.DeclarationInput{
		StudentID: chain.StudentID, VersionID: version.ID, Action: usersModels.DeclarationActionAgreed,
	}, nil)
	require.NoError(t, err)
	require.True(t, created)

	_, err = db.NewUpdate().
		Model((*testpkg.StudentEnrollment)(nil)).
		ModelTableExpr("activities.student_enrollments").
		Set("valid_until = ?", testpkg.ActivityDate(calendar.TodayDate())).
		Where("id = ?", enrollment.ID).
		Exec(context.Background())
	require.NoError(t, err)

	children, err := repos.ParentAnnouncement.DeclarationChildren(seedCtx, chain.TenantID, announcement.ID)
	require.NoError(t, err)
	require.Len(t, children, 1, "staff status and export keep the recorded child")
	assert.Equal(t, chain.StudentID, children[0].StudentID)

	feed, err := svc.ListAnnouncements(ctx, chain.AccountID)
	require.NoError(t, err)
	item := findFeedItem(t, feed, announcement.ID)
	require.NotNil(t, item.Declaration)
	require.Len(t, item.Declaration.Children, 1)
	persisted, err := repos.ParentAnnouncement.FindByID(seedCtx, announcement.ID)
	require.NoError(t, err)
	require.NotNil(t, persisted.PublishedAt)
	require.NoError(t, svc.MarkAnnouncementRead(ctx, chain.AccountID, announcement.ID, *persisted.PublishedAt))

	_, _, err = svc.SubmitDeclaration(ctx, chain.AccountID, announcement.ID, messaging.DeclarationInput{
		StudentID: chain.StudentID, VersionID: version.ID, Action: usersModels.DeclarationActionDeclined,
	}, nil)
	require.ErrorIs(t, err, messaging.ErrDeclarationNotPermitted, "a past activity enrollment does not allow another declaration")

	_, created, err = svc.SubmitDeclaration(ctx, chain.AccountID, announcement.ID, messaging.DeclarationInput{
		StudentID: chain.StudentID, VersionID: version.ID, Action: usersModels.DeclarationActionRevoked,
	}, nil)
	require.NoError(t, err)
	assert.True(t, created, "a recorded agreement remains revocable after enrollment ends")

	_, err = db.NewUpdate().TableExpr("users.persons").
		Set("deleted_at = NOW()").
		Where("id = ?", chain.PersonID).
		Exec(context.Background())
	require.NoError(t, err)
	children, err = repos.ParentAnnouncement.DeclarationChildren(seedCtx, chain.TenantID, announcement.ID)
	require.NoError(t, err)
	assert.Empty(t, children, "a deleted person must not reappear through declaration history")
}

func TestDeclaration_AllGuardiansMustDeclare(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(usersModels.DeclarationSignersAll, false, false), nil)
	co := testpkg.CreateTestCoGuardianForStudent(t, s.db, s.chain.StudentID, "Anna", "Schneider")

	_, _, err := s.submit(s.chain.AccountID, usersModels.DeclarationActionAgreed)
	require.NoError(t, err)
	feed := s.feedDeclaration(t, co.AccountID)
	require.Len(t, feed.Children, 1)
	assert.Equal(t, usersModels.DeclarationStatePartial, feed.Children[0].State)
	require.Len(t, feed.Children[0].OtherSigners, 1)
	require.NotNil(t, feed.Children[0].OtherSigners[0].Action)
	assert.Equal(t, usersModels.DeclarationActionAgreed, *feed.Children[0].OtherSigners[0].Action)
	assert.NotNil(t, feed.Children[0].OtherSigners[0].SubmittedAt)

	_, _, err = s.submit(co.AccountID, usersModels.DeclarationActionDeclined)
	require.NoError(t, err)
	feed = s.feedDeclaration(t, s.chain.AccountID)
	assert.Equal(t, usersModels.DeclarationStateDeclined, feed.Children[0].State,
		"one guardian's refusal wins over the other's consent")
}

func TestDeclaration_PasswordIsCheckedWhenTheSchoolAsksForIt(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(usersModels.DeclarationSignersAny, false, true), nil)
	const correctSecret, wrongSecret = "richtig", "falsch"
	confirm := func(_ context.Context, given string) error {
		if given != correctSecret {
			return messaging.ErrDeclarationPasswordIncorrect
		}
		return nil
	}
	input := messaging.DeclarationInput{StudentID: s.chain.StudentID, VersionID: s.version.ID, Action: usersModels.DeclarationActionAgreed}

	_, _, err := s.svc.SubmitDeclaration(s.ctx, s.chain.AccountID, s.announcement.ID, input, confirm)
	require.ErrorIs(t, err, messaging.ErrDeclarationPasswordRequired)
	input.Password = wrongSecret
	_, _, err = s.svc.SubmitDeclaration(s.ctx, s.chain.AccountID, s.announcement.ID, input, confirm)
	require.ErrorIs(t, err, messaging.ErrDeclarationPasswordIncorrect)
	assert.Empty(t, s.history(t))

	input.Password = correctSecret
	submission, created, err := s.svc.SubmitDeclaration(s.ctx, s.chain.AccountID, s.announcement.ID, input, confirm)
	require.NoError(t, err)
	assert.True(t, created)
	assert.True(t, submission.PasswordConfirmed)
}

func TestDeclaration_OpenedButUnansweredKeepsTheBadge(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(usersModels.DeclarationSignersAny, false, false), nil)
	persisted, err := s.repo.FindByID(s.seedCtx, s.announcement.ID)
	require.NoError(t, err)

	count, err := s.svc.UnreadAnnouncementCount(s.ctx, s.chain.AccountID)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	require.NoError(t, s.svc.MarkAnnouncementRead(s.ctx, s.chain.AccountID, s.announcement.ID, *persisted.PublishedAt))
	count, err = s.svc.UnreadAnnouncementCount(s.ctx, s.chain.AccountID)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "reading an Erklärung does not answer it")

	_, _, err = s.submit(s.chain.AccountID, usersModels.DeclarationActionDeclined)
	require.NoError(t, err)
	count, err = s.svc.UnreadAnnouncementCount(s.ctx, s.chain.AccountID)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestDeclaration_ProofContainsOnlyTheOwnDeclarations(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(usersModels.DeclarationSignersAny, false, false), nil)
	co := testpkg.CreateTestCoGuardianForStudent(t, s.db, s.chain.StudentID, "Anna", "Schneider")

	_, err := s.svc.DeclarationProof(s.ctx, s.chain.AccountID, s.announcement.ID, s.chain.StudentID)
	require.ErrorIs(t, err, messaging.ErrAnnouncementNotFound, "no proof before a declaration")

	_, _, err = s.submit(s.chain.AccountID, usersModels.DeclarationActionAgreed)
	require.NoError(t, err)
	_, _, err = s.submit(co.AccountID, usersModels.DeclarationActionAgreed)
	require.NoError(t, err)

	proof, err := s.svc.DeclarationProof(s.ctx, s.chain.AccountID, s.announcement.ID, s.chain.StudentID)
	require.NoError(t, err)
	require.Len(t, proof.Submissions, 1)
	assert.Equal(t, s.chain.AccountID, *proof.Submissions[0].AccountID)
	require.Contains(t, proof.Versions, s.version.ID)
	assert.Equal(t, s.version.Body, proof.Versions[s.version.ID].Body)
	assert.True(t, proof.IntegrityOK)

	// A valid answer record must still point to the frozen content it declares
	// on. Recompute its own checksum to isolate that linkage from record-hash
	// validation.
	tampered := *proof.Submissions[0]
	tampered.ContentHash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	tampered.RecordHash = messaging.DeclarationRecordHash(&tampered)
	_, err = s.db.ExecContext(s.ctx,
		`UPDATE users.parent_announcement_declaration_submissions SET content_hash = ?, record_hash = ? WHERE id = ?`,
		tampered.ContentHash, tampered.RecordHash, tampered.ID)
	require.NoError(t, err)
	proof, err = s.svc.DeclarationProof(s.ctx, s.chain.AccountID, s.announcement.ID, s.chain.StudentID)
	require.NoError(t, err)
	assert.False(t, proof.IntegrityOK)

	stranger := testpkg.CreateTestParentGuardianChain(t, s.db)
	_, err = s.svc.DeclarationProof(s.ctx, stranger.AccountID, s.announcement.ID, s.chain.StudentID)
	require.True(t, errors.Is(err, messaging.ErrAnnouncementNotFound))
}

func TestDeclarationProofAttachmentAccessRequiresTheOwnFrozenVersion(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(usersModels.DeclarationSignersAny, false, false), nil)
	s.version.Attachments = []usersModels.DeclarationAttachmentDigest{{
		AttachmentID: 91, Filename: "Ausflug.pdf", ContentType: "application/pdf", SizeBytes: 2048,
	}}
	s.version.ContentHash = messaging.DeclarationContentHash(s.version)
	attachments, err := json.Marshal(s.version.Attachments)
	require.NoError(t, err)
	_, err = s.db.ExecContext(s.seedCtx,
		`UPDATE users.parent_announcement_declaration_versions SET attachments = ?, content_hash = ? WHERE id = ?`,
		string(attachments), s.version.ContentHash, s.version.ID)
	require.NoError(t, err)

	_, _, err = s.submit(s.chain.AccountID, usersModels.DeclarationActionAgreed)
	require.NoError(t, err)

	tenantID, err := s.svc.GuardianDeclarationProofAttachmentTenant(s.ctx, s.chain.AccountID, s.announcement.ID, s.chain.StudentID, 91)
	require.NoError(t, err)
	assert.Equal(t, s.chain.TenantID, tenantID)

	newer := freezeTestVersion(t, s.seedCtx, s.repo, s.announcement, 2)
	newer.Attachments = []usersModels.DeclarationAttachmentDigest{{
		AttachmentID: 92, Filename: "Spätere Fassung.pdf", ContentType: "application/pdf", SizeBytes: 1024,
	}}
	newer.ContentHash = messaging.DeclarationContentHash(newer)
	attachments, err = json.Marshal(newer.Attachments)
	require.NoError(t, err)
	_, err = s.db.ExecContext(s.seedCtx,
		`UPDATE users.parent_announcement_declaration_versions SET attachments = ?, content_hash = ? WHERE id = ?`,
		string(attachments), newer.ContentHash, newer.ID)
	require.NoError(t, err)

	tenantID, err = s.svc.GuardianDeclarationProofAttachmentTenant(s.ctx, s.chain.AccountID, s.announcement.ID, s.chain.StudentID, 92)
	require.NoError(t, err)
	assert.Zero(t, tenantID, "an attachment from an undeclared version must not be exposed")

	co := testpkg.CreateTestCoGuardianForStudent(t, s.db, s.chain.StudentID, "Anna", "Schneider")
	tenantID, err = s.svc.GuardianDeclarationProofAttachmentTenant(s.ctx, co.AccountID, s.announcement.ID, s.chain.StudentID, 91)
	require.NoError(t, err)
	assert.Zero(t, tenantID, "another guardian needs their own declaration proof")
}
