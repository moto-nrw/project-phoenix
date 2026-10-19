package parentportal_test

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/communication/communicationtest"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// tenantBroadcastIDs extracts the tenant IDs from every BroadcastToTenant call
// recorded by bc, in call order — the shared RecordingBroadcaster equivalent of
// the old captureBroadcaster.tenantEvents slice.
func tenantBroadcastIDs(bc *testpkg.RecordingBroadcaster) []int64 {
	calls := bc.CallsByMethod("tenant")
	ids := make([]int64, len(calls))
	for i, c := range calls {
		ids[i] = c.TenantID
	}
	return ids
}

type recordingParentAbsenceNotifier struct {
	reports []parentService.AbsenceReport
}

// studentReads is the student surface the portal reads; the test may not name
// the portal's own port.
type studentReads[T any] interface {
	FindByID(ctx context.Context, id int64) (*T, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*T, error)
}

type pausingStudentRepository[T any] struct {
	studentReads[T]
	locked  chan<- struct{}
	release <-chan struct{}
}

func (r *pausingStudentRepository[T]) FindByIDForUpdate(ctx context.Context, id int64) (*T, error) {
	student, err := r.studentReads.FindByIDForUpdate(ctx, id)
	if err != nil {
		return nil, err
	}
	close(r.locked)
	select {
	case <-r.release:
		return student, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// signalingStudentService takes the staff write's student lock through the
// People Directory and signals the attempt.
type signalingStudentService[T any] struct {
	directory interface {
		LockStudent(ctx context.Context, id int64) (*T, error)
	}
	students interface {
		Update(ctx context.Context, student *T) error
	}
	attempted chan<- struct{}
}

func (s *signalingStudentService[T]) GetByIDForUpdate(ctx context.Context, id int64) (*T, error) {
	close(s.attempted)
	return s.directory.LockStudent(ctx, id)
}

func (s *signalingStudentService[T]) Update(ctx context.Context, student *T) error {
	return s.students.Update(ctx, student)
}

func (n *recordingParentAbsenceNotifier) NotifyAbsenceReported(
	_ context.Context,
	report parentService.AbsenceReport,
) error {
	n.reports = append(n.reports, report)
	return nil
}

func buildWriteService(t *testing.T, sickEnabled, notesEnabled bool, reportFlags ...map[string]bool) (*parentService.Portal, *testpkg.RecordingBroadcaster, *bun.DB) {
	t.Helper()
	cfg, bc, db := writeServiceConfig(t, sickEnabled, notesEnabled, reportFlags...)
	return parentportalcompose.New(cfg), bc, db
}

func writeServiceConfig(t *testing.T, sickEnabled, notesEnabled bool, reportFlags ...map[string]bool) (parentportalcompose.Dependencies, *testpkg.RecordingBroadcaster, *bun.DB) {
	t.Helper()
	values := map[string]bool{
		parentService.KeyParentSickNoteEnabled: sickEnabled,
		parentService.KeyParentNotesEnabled:    notesEnabled,
	}
	for _, flags := range reportFlags {
		for key, value := range flags {
			values[key] = value
		}
	}
	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	bc := testpkg.NewRecordingBroadcaster()
	cfg := parentportalcompose.Dependencies{
		GuardianChanges: owners.ParentGuardianChanges(),
		DeparturePolicy: owners.ParentDeparturePolicy(),
		GuardianPolicy:  owners.NewParentGuardianPolicy(),
		Fingerprints:    owners.SecurityFingerprints{},
		Declarations:    owners.ParentDeclarationPolicy(),
		Locales:         owners.ParentPortalLocales(),
		Runtime:         owners.ParentPortalRuntime{},
		CarePlan:        owners.NewParentCarePlanCommands(repos.CarePlan(), nil),
		People:          owners.MustNewPeopleDirectory(testpkg.MustDatabase(db)),
		CareProfiles:    careProfiles(t, db),
		ChildRepo:       repos.ParentChildReads(),
		StatusDayRepo:   repos.StudentStatusDay,
		StudentRepo:     owners.NewParentStudentReads(reposPeople.Student),
		CareExceptions:  owners.NewParentCareExceptions(repos.CarePlan()),
		Settings: parentSettingsStub{
			boolValues: values,
			stringValues: map[string]string{
				parentService.KeyGuardianParentInviteMode: parentService.ParentInviteModeDisabled,
			},
		},
		MealPlan:       availableMealPlan(false),
		StudentUpdates: owners.BindParentStudentUpdates(bc),
		Logger:         slog.Default(),

		DataRequests: owners.NewParentDataRequestCommands(repos.CarePlan()),

		CarePlanPolicy: owners.NewParentCarePlanPolicy(),
	}
	return cfg, bc, db
}

func buildMessagingWriteService(t *testing.T, sickEnabled, notesEnabled bool) (*parentService.Portal, *testpkg.RecordingBroadcaster, *bun.DB, *api.RepositoryTestFactory) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	reposMessages := api.NewParentMessagingSuiteRepositories(testpkg.NewDatabase(t, db))
	bc := testpkg.NewRecordingBroadcaster()
	settings := parentSettingsStub{
		boolValues: map[string]bool{
			parentService.KeyParentSickNoteEnabled: sickEnabled,
			parentService.KeyParentNotesEnabled:    notesEnabled,
		},
		stringValues: map[string]string{
			parentService.KeyGuardianParentInviteMode: parentService.ParentInviteModeDisabled,
		},
	}
	svc := parentportalcompose.New(parentportalcompose.Dependencies{
		GuardianChanges:   owners.ParentGuardianChanges(),
		DeparturePolicy:   owners.ParentDeparturePolicy(),
		GuardianPolicy:    owners.NewParentGuardianPolicy(),
		Fingerprints:      owners.SecurityFingerprints{},
		Declarations:      owners.ParentDeclarationPolicy(),
		Locales:           owners.ParentPortalLocales(),
		Runtime:           owners.ParentPortalRuntime{},
		MessagingEnabled:  owners.BindParentMessagingGate(settings, slog.Default()),
		CarePlan:          owners.NewParentCarePlanCommands(repos.CarePlan(), nil),
		People:            owners.MustNewPeopleDirectory(testpkg.MustDatabase(db)),
		CareProfiles:      careProfiles(t, db),
		ChildRepo:         repos.ParentChildReads(),
		StatusDayRepo:     repos.StudentStatusDay,
		StudentRepo:       owners.NewParentStudentReads(reposPeople.Student),
		CareExceptions:    owners.NewParentCareExceptions(repos.CarePlan()),
		Settings:          settings,
		StudentUpdates:    owners.BindParentStudentUpdates(bc),
		MessageThreadRepo: owners.BindParentMessageThreadAccess(reposMessages.ParentMessageThread),
		MessageRepo:       owners.BindParentMessageReads(communicationtest.NewParentMessageRepository(db)),
		MessageReadRepo:   owners.BindParentMessageInboxReads(reposMessages.ParentMessageRead),
		Conversations:     api.NewParentConversationCore(reposMessages.ParentMessageThread, communicationtest.NewParentMessageRepository(db), reposMessages.ParentMessageRead, bc, slog.Default()),
		Logger:            slog.Default(),

		DataRequests: owners.NewParentDataRequestCommands(repos.CarePlan()),

		CarePlanPolicy: owners.NewParentCarePlanPolicy(),
	})
	return svc, bc, db, repos
}

// --- SubmitSickNote ---

func TestSubmitSickNote_TodayFlipsLiveFlagAndStoresReason(t *testing.T) {
	t.Parallel()

	svc, bc, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	sickResult, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]calendar.Date{calendar.TodayDate()}, "Fieber, beim Arzt", absencerecords.StudentStatusDaySick, nil)
	require.NoError(t, err)
	require.Len(t, sickResult.StatusDays, 1)
	assert.Equal(t, absencerecords.StudentStatusDaySick, sickResult.StatusDays[0].Status)
	assert.Equal(t, absencerecords.StudentStatusSourceParent, sickResult.StatusDays[0].Source)
	require.NotNil(t, sickResult.StatusDays[0].Note)
	assert.Equal(t, "Fieber, beim Arzt", *sickResult.StatusDays[0].Note)

	var sick bool
	require.NoError(t, db.NewSelect().ColumnExpr("COALESCE(sick,false)").TableExpr("users.student_care_profiles").
		Where("membership_id IN (SELECT id FROM users.student_school_memberships WHERE student_profile_id = ? AND deleted_at IS NULL)", chain.StudentID).Scan(testpkg.WithPackageTenantRuntime(context.Background()), &sick))
	assert.True(t, sick, "today's sick note must flip the live sick flag")

	assert.Contains(t, tenantBroadcastIDs(bc), chain.TenantID, "SSE broadcast must fire for the tenant")
}

func TestSubmitSickNote_ResubmitDoesNotNotifyAgain(t *testing.T) {
	t.Parallel()

	cfg, _, db := writeServiceConfig(t, true, true)
	notifier := &recordingParentAbsenceNotifier{}
	cfg.AbsenceNotifier = notifier
	svc := parentportalcompose.New(cfg)

	chain := testpkg.CreateTestParentGuardianChain(t, db)

	for range 2 {
		_, err := svc.SubmitSickNote(
			testpkg.WithPackageTenantRuntime(context.Background()),
			chain.AccountID,
			chain.StudentID,
			[]calendar.Date{calendar.TodayDate()},
			"Fieber",
			absencerecords.StudentStatusDaySick,
			nil,
		)
		require.NoError(t, err)
	}

	require.Len(t, notifier.reports, 1)
	assert.Equal(t, absencerecords.StudentStatusDaySick, notifier.reports[0].Status)
}

// TestChildMessaging_RequiresNotesWritePermission is the regression guard for the
// messaging write paths: a guardian with parent_portal.access but NOT
// parent_portal.notes.write (e.g. a pickup_only / emergency_contact preset) may
// read the conversation but must not post messages or submit/withdraw change
// requests. It replaces the deleted TestAddParentNote_MissingGuardianPermission
// that guarded the old one-way notes path, and asserts the API enforces what the
// UI already hides (ChildFeatures.NotesEnabled gates on the same permission). See
// .claude/rules/guardian-parent-permissions.md.
func TestChildMessaging_RequiresNotesWritePermission(t *testing.T) {
	t.Parallel()

	svc, _, db, _ := buildMessagingWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	// Downgrade the guardian to read-only portal access (no notes.write).
	_, err := db.ExecContext(testpkg.WithPackageTenantRuntime(context.Background()), `
		UPDATE auth.guardian_student_access AS a
		SET permissions = '{"parent_portal.access": true}'::jsonb
		FROM users.student_guardian_relationships AS r
		WHERE r.tenant_id = a.tenant_id AND r.id = a.relationship_id
		  AND r.tenant_id = ? AND r.student_id = ? AND r.guardian_profile_id = ?
	`, chain.TenantID, chain.StudentID, chain.GuardianProfileID)
	require.NoError(t, err)

	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	_, err = svc.PostChildMessage(ctx, chain.AccountID, chain.StudentID, "Hallo OGS")
	require.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied, "posting a message must require notes.write")
}

// TestPostChildMessage_EmptyBody re-establishes a guard the deleted one-way
// notes suite covered: a blank / whitespace-only message body is rejected with a
// clean ErrEmptyNote (→ 400) BEFORE any insert. Without it the empty body would
// reach the chk_parent_messages_body_not_blank CHECK and surface as a raw 500.
func TestPostChildMessage_EmptyBody(t *testing.T) {
	t.Parallel()

	svc, _, db, _ := buildMessagingWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := svc.PostChildMessage(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, "   \n\t ")
	require.ErrorIs(t, err, parentService.ErrEmptyNote)
}

// TestPostChildMessage_BodyTooLong locks in the rune-count upper bound
// (maxParentNoteLen = 2000) on the messaging path, mirroring the sick-note
// ReasonTooLong guard. 2001 runes → ErrNoteTooLong (→ 400), never a silent insert.
func TestPostChildMessage_BodyTooLong(t *testing.T) {
	t.Parallel()

	svc, _, db, _ := buildMessagingWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := svc.PostChildMessage(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, strings.Repeat("x", 2001))
	require.ErrorIs(t, err, parentService.ErrNoteTooLong)
}

// TestPostChildMessage_AllowsMultibyteUpToRuneLimit re-establishes the
// at-the-limit accept guard the deleted one-way notes suite carried
// (TestAddParentNote_AllowsMultibyteUpToRuneLimit): the cap counts CHARACTERS
// (utf8.RuneCountInString), not bytes. 2000 umlauts = 2000 runes but ~4000 UTF-8
// bytes, so a regression to len(body) (or a varchar(2000) column cap) would
// silently reject legitimate German messages. Only the over-limit case
// (BodyTooLong above) was re-tested after the rewrite; this pins the boundary
// from the accepting side so the rune-count semantics can't quietly revert.
func TestPostChildMessage_AllowsMultibyteUpToRuneLimit(t *testing.T) {
	t.Parallel()

	svc, _, db, _ := buildMessagingWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	body := strings.Repeat("ä", maxMessageRunesForTest)
	view, err := svc.PostChildMessage(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, body)
	require.NoError(t, err)
	require.NotNil(t, view)
	require.Len(t, view.Messages, 1, "the just-sent at-limit message is persisted and returned")
	require.Equal(t, body, view.Messages[0].Body)
}

// maxMessageRunesForTest mirrors the service's maxParentNoteLen (kept local so
// the test doesn't depend on an unexported constant).
const maxMessageRunesForTest = 2000

// TestChildMessaging_FeatureDisabled re-establishes the "messaging turned off →
// 403" guard for BOTH write paths: with operations.parent_notes_enabled off, a
// fully-permitted guardian still cannot post a message or submit a request. The
// frontend hides the composer on the same flag; this asserts the API agrees.
func TestChildMessaging_FeatureDisabled(t *testing.T) {
	t.Parallel()

	svc, _, db, _ := buildMessagingWriteService(t, true, false)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	_, err := svc.PostChildMessage(ctx, chain.AccountID, chain.StudentID, "Hallo OGS")
	require.ErrorIs(t, err, parentService.ErrNotesDisabled, "posting must be refused when messaging is disabled")
}

// TestPostChildMessage_NotOwned re-establishes the cross-family ownership guard
// the deleted one-way notes suite carried (TestAddParentNote_NotOwned): a guardian
// MUST NOT be able to inject a message onto a child they do not guardian. This is
// the single most security-critical invariant of the messaging feature (account A
// writing into account B's family thread); enforcement lives in resolveOwnedChild,
// so this pins it directly against a future refactor of that resolve path.
func TestPostChildMessage_NotOwned(t *testing.T) {
	t.Parallel()

	svc, _, db, _ := buildMessagingWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	other := testpkg.CreateTestStudent(t, db, "Mara", "Fremd", "2b")
	defer func() {
		_, _ = db.ExecContext(testpkg.WithPackageTenantRuntime(context.Background()), `DELETE FROM users.student_profiles WHERE id = ?`, other.ID)
		_, _ = db.ExecContext(testpkg.WithPackageTenantRuntime(context.Background()), `DELETE FROM users.persons WHERE id = ?`, other.PersonID)
	}()

	_, err := svc.PostChildMessage(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, other.ID, "Hallo OGS")
	require.ErrorIs(t, err, parentService.ErrChildNotLinked,
		"a guardian must not post a message on a child they do not guardian")
}

// TestGetChildConversation_NotOwned is the read-side counterpart: a guardian MUST
// NOT be able to read another family's conversation. GetChildConversation also
// marks the thread read, so an ownership bypass here would both leak a message and
// silently advance the wrong reader's cursor.
func TestGetChildConversation_NotOwned(t *testing.T) {
	t.Parallel()

	svc, _, db, _ := buildMessagingWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	other := testpkg.CreateTestStudent(t, db, "Mara", "Fremd", "2b")
	defer func() {
		_, _ = db.ExecContext(testpkg.WithPackageTenantRuntime(context.Background()), `DELETE FROM users.student_profiles WHERE id = ?`, other.ID)
		_, _ = db.ExecContext(testpkg.WithPackageTenantRuntime(context.Background()), `DELETE FROM users.persons WHERE id = ?`, other.PersonID)
	}()

	_, err := svc.GetChildConversation(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, other.ID)
	require.ErrorIs(t, err, parentService.ErrChildNotLinked,
		"a guardian must not read a conversation for a child they do not guardian")
}

func TestSubmitSickNote_FutureDateDoesNotFlipLiveFlag(t *testing.T) {
	t.Parallel()

	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	future := calendar.TodayDate().AddDays(7)
	sickResult, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]calendar.Date{future}, "Fieber", absencerecords.StudentStatusDaySick, nil)
	require.NoError(t, err)
	require.Len(t, sickResult.StatusDays, 1)
	require.NotNil(t, sickResult.StatusDays[0].Note)
	assert.Equal(t, "Fieber", *sickResult.StatusDays[0].Note)

	var sick bool
	require.NoError(t, db.NewSelect().ColumnExpr("COALESCE(sick,false)").TableExpr("users.student_care_profiles").
		Where("membership_id IN (SELECT id FROM users.student_school_memberships WHERE student_profile_id = ? AND deleted_at IS NULL)", chain.StudentID).Scan(testpkg.WithPackageTenantRuntime(context.Background()), &sick))
	assert.False(t, sick, "a future-only sick note must not flip today's live flag")
}

func TestSubmitSickNote_RefusesPartialAbsenceConflict(t *testing.T) {
	t.Parallel()

	svc, _, db := buildWriteService(t, true, true)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Partial", "Author")

	date := calendar.NewDate(2026, 8, 24).AddDays(7)
	from := calendar.NormalizeWallClock(time.Date(2000, time.January, 1, 13, 30, 0, 0, time.UTC))
	staffID := staff.ID
	pickup := &careplan.PickupException{
		StudentID:             chain.StudentID,
		ExceptionDate:         careplan.Date(date),
		PickupTime:            &from,
		ExcusedFrom:           &from,
		ExcusedCreatedBy:      &staffID,
		ExcusedOwnsPickupTime: true,
		Source:                careplan.ExceptionSourceStaff,
		CreatedBy:             staff.ID,
	}
	pickup.TenantID = chain.TenantID
	require.NoError(t, api.CreateParentPickupExceptionFixture(testpkg.TenantContext(chain.TenantID), repos.CarePlan(), pickup))

	_, err := svc.SubmitSickNote(
		testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]calendar.Date{date}, "Fieber", absencerecords.StudentStatusDaySick,
		nil,
	)
	require.ErrorIs(t, err, parentService.ErrCareExceptionConflict)

	rows, findErr := repos.StudentStatusDay.FindActiveByStudentAndDateRange(
		testpkg.TenantContext(chain.TenantID), chain.StudentID, date, date,
	)
	require.NoError(t, findErr)
	assert.Empty(t, rows)
}

func TestSubmitSickNote_FutureWriteSerializesWithStaffConflictCheck(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	parentLocked := make(chan struct{})
	releaseParent := make(chan struct{})
	parentSvc := parentportalcompose.New(parentportalcompose.Dependencies{
		GuardianChanges: owners.ParentGuardianChanges(),
		DeparturePolicy: owners.ParentDeparturePolicy(),
		GuardianPolicy:  owners.NewParentGuardianPolicy(),
		Fingerprints:    owners.SecurityFingerprints{},
		Declarations:    owners.ParentDeclarationPolicy(),
		Locales:         owners.ParentPortalLocales(),
		Runtime:         owners.ParentPortalRuntime{},
		CarePlan:        owners.NewParentCarePlanCommands(repos.CarePlan(), nil),
		People:          owners.MustNewPeopleDirectory(testpkg.MustDatabase(db)),
		CareProfiles:    careProfiles(t, db),
		ChildRepo:       repos.ParentChildReads(),
		StatusDayRepo:   repos.StudentStatusDay,
		CareExceptions:  owners.NewParentCareExceptions(repos.CarePlan()),
		StudentRepo:     newPausingStudentRepository(owners.NewParentStudentReads(reposPeople.Student), parentLocked, releaseParent),
		Settings: parentSettingsStub{
			boolValues: map[string]bool{
				parentService.KeyParentSickNoteEnabled: true,
				parentService.KeyParentNotesEnabled:    true,
			},
			stringValues: map[string]string{
				parentService.KeyGuardianParentInviteMode: parentService.ParentInviteModeDisabled,
			},
		},
		StudentUpdates: owners.BindParentStudentUpdates(testpkg.NewRecordingBroadcaster()),
		Logger:         slog.Default(),

		DataRequests: owners.NewParentDataRequestCommands(repos.CarePlan()),

		CarePlanPolicy: owners.NewParentCarePlanPolicy(),
	})

	statusSvc := presenceservice.NewStatusDays(repos.StudentStatusDay, nil, nil, repos.CarePlan().LockExceptionDay)
	staffAttempted := make(chan struct{})
	staffStudentSvc := newSignalingStudentService(api.NewStudentDirectory(owners.MustNewPeopleDirectory(testpkg.MustDatabase(db))), reposPeople.Student, staffAttempted)
	date := calendar.NewDate(2026, 8, 24).AddDays(40)

	ctx, cancel := context.WithTimeout(testpkg.WithPackageTenantRuntime(context.Background()), 5*time.Second)
	defer cancel()
	parentResult := make(chan error, 1)
	go func() {
		_, err := parentSvc.SubmitSickNote(ctx, chain.AccountID, chain.StudentID,
			[]calendar.Date{date}, "Fieber", absencerecords.StudentStatusDaySick, nil)
		parentResult <- err
	}()

	select {
	case <-parentLocked:
	case <-ctx.Done():
		t.Fatal("parent write did not acquire the student lock")
	}

	staffResult := make(chan error, 1)
	go func() {
		staffResult <- statusSvc.CreateForDates(testpkg.TenantContext(chain.TenantID), studentpresence.StatusDayWriteContext{
			DB:             db,
			TenantID:       chain.TenantID,
			StudentService: api.StatusDayStudents(staffStudentSvc, api.AllowAllStatusDayWrites),
			AfterCommit:    func(int64) {},
		}, chain.StudentID, absencerecords.StudentStatusDayExcused, "Termin", []calendar.Date{date})
	}()

	select {
	case <-staffAttempted:
	case <-ctx.Done():
		t.Fatal("staff write did not attempt the student lock")
	}
	select {
	case err := <-staffResult:
		t.Fatalf("staff write returned before the parent released the shared student lock: %v", err)
	default:
	}

	close(releaseParent)
	require.NoError(t, <-parentResult)
	staffErr := <-staffResult
	var conflictErr *studentpresence.StudentStatusDayConflictError
	require.ErrorAs(t, staffErr, &conflictErr)
	require.Len(t, conflictErr.Conflicts, 1)
	assert.Equal(t, absencerecords.StudentStatusDaySick, conflictErr.Conflicts[0].Status)

	rows, err := statusSvc.GetActiveByStudentAndDateRange(testpkg.TenantContext(chain.TenantID), chain.StudentID, date, date)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, absencerecords.StudentStatusDaySick, rows[0].Status,
		"the staff write must not overwrite the parent row after waiting for its lock")
}

func TestSubmitSickNote_NoDates(t *testing.T) {
	t.Parallel()

	svc, _, _ := buildWriteService(t, true, true)
	_, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), 123, 456, nil, "", absencerecords.StudentStatusDaySick, nil)
	require.ErrorIs(t, err, parentService.ErrNoDates)
}

func TestSubmitSickNote_NotOwnedChild(t *testing.T) {
	t.Parallel()

	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	other := testpkg.CreateTestStudent(t, db, "Mara", "Fremd", "2b")
	defer func() {
		_, _ = db.ExecContext(testpkg.WithPackageTenantRuntime(context.Background()), `DELETE FROM users.student_profiles WHERE id = ?`, other.ID)
		_, _ = db.ExecContext(testpkg.WithPackageTenantRuntime(context.Background()), `DELETE FROM users.persons WHERE id = ?`, other.PersonID)
	}()

	_, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, other.ID,
		[]calendar.Date{calendar.TodayDate()}, "", absencerecords.StudentStatusDaySick, nil)
	require.ErrorIs(t, err, parentService.ErrChildNotLinked)
}

func TestSubmitSickNote_FeatureDisabled(t *testing.T) {
	t.Parallel()

	svc, _, db := buildWriteService(t, false, true) // sick disabled
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]calendar.Date{calendar.TodayDate()}, "", absencerecords.StudentStatusDaySick, nil)
	require.ErrorIs(t, err, parentService.ErrSickNoteDisabled)
}

func TestSubmitSickNote_MissingGuardianPermission(t *testing.T) {
	t.Parallel()

	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := db.ExecContext(testpkg.WithPackageTenantRuntime(context.Background()), `
		UPDATE auth.guardian_student_access AS a
		SET permissions = '{"parent_portal.access": true}'::jsonb
		FROM users.student_guardian_relationships AS r
		WHERE r.tenant_id = a.tenant_id AND r.id = a.relationship_id
		  AND r.tenant_id = ? AND r.student_id = ? AND r.guardian_profile_id = ?
	`, chain.TenantID, chain.StudentID, chain.GuardianProfileID)
	require.NoError(t, err)

	_, err = svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]calendar.Date{calendar.TodayDate()}, "", absencerecords.StudentStatusDaySick, nil)
	require.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied)
}

func TestSubmitSickNote_ReasonTooLong(t *testing.T) {
	t.Parallel()

	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]calendar.Date{calendar.TodayDate()}, strings.Repeat("x", 2001), absencerecords.StudentStatusDaySick, nil)
	require.ErrorIs(t, err, parentService.ErrNoteTooLong)
}

func TestSubmitSickNote_EmptyReasonRejected(t *testing.T) {
	t.Parallel()

	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]calendar.Date{calendar.TodayDate()}, "   ", absencerecords.StudentStatusDaySick, nil)
	require.ErrorIs(t, err, parentService.ErrEmptyNote)
}

func TestSubmitSickNote_ClearsClassTripForSubmittedDate(t *testing.T) {
	t.Parallel()

	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	statusRepo := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db)).StudentStatusDay
	ctx := testpkg.TenantContext(chain.TenantID)
	date := calendar.NewDate(2026, 8, 24).AddDays(7)
	require.NoError(t, statusRepo.UpsertReported(ctx, &absencerecords.StudentStatusDay{
		StudentID:  chain.StudentID,
		Date:       date,
		Status:     absencerecords.StudentStatusDayClassTrip,
		ReportedAt: time.Now(),
		Source:     absencerecords.StudentStatusSourcePlanned,
	}))

	sickResult, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, []calendar.Date{date}, "Krank", absencerecords.StudentStatusDaySick, nil)
	require.NoError(t, err)
	require.Len(t, sickResult.StatusDays, 1)
	assert.Equal(t, absencerecords.StudentStatusDaySick, sickResult.StatusDays[0].Status)

	activeRows, err := statusRepo.FindActiveByStudentAndDateRange(ctx, chain.StudentID, date, date)
	require.NoError(t, err)
	require.Len(t, activeRows, 1)
	assert.Equal(t, absencerecords.StudentStatusDaySick, activeRows[0].Status)
}

// --- ListSickDays ---

func TestListSickDays_ReturnsSickOnlyAfterSubmit(t *testing.T) {
	t.Parallel()

	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	day := calendar.TodayDate().AddDays(3)
	_, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]calendar.Date{day}, "Krank", absencerecords.StudentStatusDaySick, nil)
	require.NoError(t, err)

	from := calendar.TodayDate()
	to := calendar.NewDate(from.Year(), from.Month()+1, from.Day())
	sick, err := svc.ListSickDays(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, from, to)
	require.NoError(t, err)
	require.Len(t, sick, 1)
	assert.Equal(t, absencerecords.StudentStatusDaySick, sick[0].Status)
}

func TestListSickDays_HidesAnotherGuardiansReason(t *testing.T) {
	t.Parallel()

	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	otherAccount := testpkg.CreateTestAccount(t, db, "other-absence-parent")
	otherProfile := &testpkg.GuardianProfile{
		FirstName: "Andere", LastName: "Person", Email: &otherAccount.Email,
		AccountID: &otherAccount.ID, HasAccount: true,
		PreferredContactMethod: "email", LanguagePreference: "de",
	}
	otherProfile.TenantID = chain.TenantID
	_, err := db.NewInsert().Model(otherProfile).ModelTableExpr(`users.guardian_profiles`).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	link := &testpkg.StudentGuardian{StudentID: chain.StudentID, GuardianProfileID: otherProfile.ID, RelationshipType: "parent"}
	link.GuardianRole, link.Permissions = api.ParentGuardianRoleFixture("legal_guardian")
	link.TenantID = chain.TenantID
	err = testpkg.InsertTestStudentGuardian(testpkg.Ctx(t), db, link)
	require.NoError(t, err)

	day := calendar.NewDate(2026, 8, 24).AddDays(3)
	_, err = svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]calendar.Date{day}, "Vertraulicher Grund", absencerecords.StudentStatusDaySick, nil)
	require.NoError(t, err)
	var storedAuthor *int64
	require.NoError(t, db.NewRaw(`
		SELECT guardian_account_id FROM active.student_status_days
		WHERE tenant_id = ? AND student_id = ? AND date = ?
	`, chain.TenantID, chain.StudentID, day).Scan(testpkg.Ctx(t), &storedAuthor))
	require.NotNil(t, storedAuthor)
	assert.Equal(t, chain.AccountID, *storedAuthor)

	own, err := svc.ListSickDays(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, day, day)
	require.NoError(t, err)
	require.Len(t, own, 1)
	require.NotNil(t, own[0].Note)
	assert.Equal(t, "Vertraulicher Grund", *own[0].Note)

	other, err := svc.ListSickDays(testpkg.WithPackageTenantRuntime(context.Background()), otherAccount.ID, chain.StudentID, day, day)
	require.NoError(t, err)
	require.Len(t, other, 1, "the effective absence remains shared")
	assert.Nil(t, other[0].Note, "the submitting guardian's free text stays private")

	_, err = svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), otherAccount.ID, chain.StudentID,
		[]calendar.Date{day}, "Neuer vertraulicher Grund", absencerecords.StudentStatusDaySick, nil)
	require.NoError(t, err)
	other, err = svc.ListSickDays(testpkg.WithPackageTenantRuntime(context.Background()), otherAccount.ID, chain.StudentID, day, day)
	require.NoError(t, err)
	require.Len(t, other, 1)
	require.NotNil(t, other[0].Note)
	assert.Equal(t, "Neuer vertraulicher Grund", *other[0].Note)
	own, err = svc.ListSickDays(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, day, day)
	require.NoError(t, err)
	require.Len(t, own, 1)
	assert.Nil(t, own[0].Note, "a re-report transfers authorship with the new note")
}

func TestListSickDays_AllowsPortalAccessWithoutWritePermissions(t *testing.T) {
	t.Parallel()

	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	day := calendar.NewDate(2026, 8, 24).AddDays(2)
	_, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]calendar.Date{day}, "Krank", absencerecords.StudentStatusDaySick, nil)
	require.NoError(t, err)

	_, err = db.ExecContext(testpkg.WithPackageTenantRuntime(context.Background()), `
		UPDATE auth.guardian_student_access AS a
		SET permissions = '{"parent_portal.access": true}'::jsonb
		FROM users.student_guardian_relationships AS r
		WHERE r.tenant_id = a.tenant_id AND r.id = a.relationship_id
		  AND r.tenant_id = ? AND r.student_id = ? AND r.guardian_profile_id = ?
	`, chain.TenantID, chain.StudentID, chain.GuardianProfileID)
	require.NoError(t, err)

	sick, err := svc.ListSickDays(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, day, day)
	require.NoError(t, err)
	require.Len(t, sick, 1)
	assert.Equal(t, absencerecords.StudentStatusDaySick, sick[0].Status)
}

func TestListSickDays_NotOwned(t *testing.T) {
	t.Parallel()

	svc, _, _ := buildWriteService(t, true, true)
	from := calendar.NewDate(2026, 8, 24)
	_, err := svc.ListSickDays(testpkg.WithPackageTenantRuntime(context.Background()), 999999, 888888, from, calendar.NewDate(from.Year(), from.Month()+1, from.Day()))
	require.ErrorIs(t, err, parentService.ErrChildNotLinked)
}

// TestSubmitSickNote_NonContiguousExcludesUnrelatedRows guards the response
// filter: a non-contiguous submission (Mon + Wed) must not return an
// unrelated active excused row that falls on Tuesday inside the min..max
// range.
func TestSubmitSickNote_NonContiguousExcludesUnrelatedRows(t *testing.T) {
	t.Parallel()

	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	statusRepo := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db)).StudentStatusDay
	tctx := testpkg.Ctx(t)

	base := calendar.NewDate(2026, 8, 24).AddDays(7)
	mon := base
	tue := base.AddDays(1)
	wed := base.AddDays(2)

	// Pre-existing excused row on Tuesday (between the two sick days).
	require.NoError(t, statusRepo.UpsertReported(tctx, &absencerecords.StudentStatusDay{
		StudentID:  chain.StudentID,
		Date:       tue,
		Status:     absencerecords.StudentStatusDayExcused,
		ReportedAt: time.Now(),
		Source:     absencerecords.StudentStatusSourceManual,
	}))

	sickResult, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]calendar.Date{mon, wed}, "Krank", absencerecords.StudentStatusDaySick, nil)
	require.NoError(t, err)
	require.Len(t, sickResult.StatusDays, 2, "only the two submitted sick days, not the Tuesday excused row")
	for _, r := range sickResult.StatusDays {
		assert.Equal(t, absencerecords.StudentStatusDaySick, r.Status)
		assert.NotEqual(t, tue, r.Date, "Tuesday excused row must be excluded")
	}
}

// --- SubmitSickNote: excused ("Termin/Abwesenheit") path (issue #1735) ---

func TestSubmitSickNote_ExcusedTodayStoresExcusedWithoutLiveFlag(t *testing.T) {
	t.Parallel()

	svc, bc, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	sickResult, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]calendar.Date{calendar.TodayDate()}, "Zahnarzttermin", absencerecords.StudentStatusDayExcused, nil)
	require.NoError(t, err)
	require.Len(t, sickResult.StatusDays, 1)
	assert.Equal(t, absencerecords.StudentStatusDayExcused, sickResult.StatusDays[0].Status)
	assert.Equal(t, absencerecords.StudentStatusSourceParent, sickResult.StatusDays[0].Source)
	require.NotNil(t, sickResult.StatusDays[0].Note)
	assert.Equal(t, "Zahnarzttermin", *sickResult.StatusDays[0].Note)

	var sick, excused bool
	require.NoError(t, db.NewSelect().
		ColumnExpr("COALESCE(sick,false), COALESCE(excused,false)").
		TableExpr("users.student_care_profiles").Where("membership_id IN (SELECT id FROM users.student_school_memberships WHERE student_profile_id = ? AND deleted_at IS NULL)", chain.StudentID).
		Scan(testpkg.WithPackageTenantRuntime(context.Background()), &sick, &excused))
	assert.False(t, sick, "an excused absence must not set the live sick flag")
	assert.False(t, excused, "an excused absence must not set a live excused flag (issue #1735)")

	assert.Contains(t, tenantBroadcastIDs(bc), chain.TenantID, "SSE broadcast must fire for the tenant")
}

func TestSubmitSickNote_ExcusedTodayClearsStaleLiveSickFlag(t *testing.T) {
	t.Parallel()

	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	// First report sick today (flips the live sick flag), then switch the same
	// day to an excused absence: the stale live sick flag must be cleared so the
	// flag stays consistent with the now-cleared sick status day.
	_, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]calendar.Date{calendar.TodayDate()}, "Krank", absencerecords.StudentStatusDaySick, nil)
	require.NoError(t, err)

	sickResult, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]calendar.Date{calendar.TodayDate()}, "Termin", absencerecords.StudentStatusDayExcused, nil)
	require.NoError(t, err)
	require.Len(t, sickResult.StatusDays, 1)
	assert.Equal(t, absencerecords.StudentStatusDayExcused, sickResult.StatusDays[0].Status)

	var sick bool
	require.NoError(t, db.NewSelect().ColumnExpr("COALESCE(sick,false)").TableExpr("users.student_care_profiles").
		Where("membership_id IN (SELECT id FROM users.student_school_memberships WHERE student_profile_id = ? AND deleted_at IS NULL)", chain.StudentID).Scan(testpkg.WithPackageTenantRuntime(context.Background()), &sick))
	assert.False(t, sick, "switching today from sick to excused must clear the live sick flag")
}

func TestSubmitSickNote_InvalidStatus(t *testing.T) {
	t.Parallel()

	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]calendar.Date{calendar.TodayDate()}, "", "class_trip", nil)
	require.ErrorIs(t, err, parentService.ErrInvalidStatus)
}

func TestListSickDays_ReturnsSickAndExcused(t *testing.T) {
	t.Parallel()

	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	sickDay := calendar.NewDate(2026, 8, 24).AddDays(2)
	excusedDay := calendar.NewDate(2026, 8, 24).AddDays(4)
	_, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]calendar.Date{sickDay}, "Krank", absencerecords.StudentStatusDaySick, nil)
	require.NoError(t, err)
	_, err = svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]calendar.Date{excusedDay}, "Termin", absencerecords.StudentStatusDayExcused, nil)
	require.NoError(t, err)

	from := calendar.NewDate(2026, 8, 24)
	to := calendar.NewDate(from.Year(), from.Month()+1, from.Day())
	absences, err := svc.ListSickDays(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, from, to)
	require.NoError(t, err)
	require.Len(t, absences, 2, "both the sick and the excused absence must be listed")
	byStatus := map[string]calendar.Date{}
	for _, a := range absences {
		byStatus[a.Status] = a.Date
	}
	assert.Equal(t, sickDay, byStatus[absencerecords.StudentStatusDaySick])
	assert.Equal(t, excusedDay, byStatus[absencerecords.StudentStatusDayExcused])
}

// A staff-created excused day (source=planned/manual) is an internal scheduled
// status. It must NOT leak into the parents portal listing, while the parent's
// own excused day on a different date still shows. Guards the privacy fix for
// the issue #1735 follow-up review.
func TestListSickDays_ExcludesStaffCreatedExcused(t *testing.T) {
	t.Parallel()

	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	statusRepo := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db)).StudentStatusDay
	tctx := testpkg.Ctx(t)

	staffExcusedDay := calendar.NewDate(2026, 8, 24).AddDays(2)
	parentExcusedDay := calendar.NewDate(2026, 8, 24).AddDays(4)
	staffNote := "interner Hinweis"

	// Staff-created excused row (source=manual, carries a staff note).
	require.NoError(t, statusRepo.UpsertReported(tctx, &absencerecords.StudentStatusDay{
		StudentID:  chain.StudentID,
		Date:       staffExcusedDay,
		Status:     absencerecords.StudentStatusDayExcused,
		ReportedAt: time.Now(),
		Source:     absencerecords.StudentStatusSourceManual,
		Note:       &staffNote,
	}))

	// Parent's own excused report on a different date.
	_, err := svc.SubmitSickNote(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID,
		[]calendar.Date{parentExcusedDay}, "Termin", absencerecords.StudentStatusDayExcused, nil)
	require.NoError(t, err)

	from := calendar.NewDate(2026, 8, 24)
	to := calendar.NewDate(from.Year(), from.Month()+1, from.Day())
	absences, err := svc.ListSickDays(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, from, to)
	require.NoError(t, err)
	require.Len(t, absences, 1, "only the parent-reported excused day must be listed")
	assert.Equal(t, parentExcusedDay, absences[0].Date)
	assert.Equal(t, absencerecords.StudentStatusSourceParent, absences[0].Source)
	for _, a := range absences {
		assert.NotEqual(t, staffExcusedDay, a.Date, "staff-created excused day must not leak to the parent")
	}
}

// --- ChildFeatures ---

func TestParentReportKindsAreIndependent(t *testing.T) {
	t.Parallel()
	for _, sickEnabled := range []bool{false, true} {
		for _, excusedEnabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("sick=%t/excused=%t", sickEnabled, excusedEnabled), func(t *testing.T) {
				t.Parallel()
				ctx := testpkg.OwnCtx(t)
				svc, _, db := buildWriteService(t, true, true, map[string]bool{
					parentService.KeyParentSickReportsEnabled:    sickEnabled,
					parentService.KeyParentExcusedReportsEnabled: excusedEnabled,
				})
				chain := testpkg.CreateTestParentGuardianChain(t, db)
				flags, err := svc.ChildFeatures(ctx, chain.AccountID, chain.StudentID)
				require.NoError(t, err)
				assert.Equal(t, sickEnabled, flags.SickNoteEnabled)
				assert.Equal(t, excusedEnabled, flags.ExcusedNoteEnabled)
				for index, report := range []struct {
					status  string
					enabled bool
				}{
					{absencerecords.StudentStatusDaySick, sickEnabled},
					{absencerecords.StudentStatusDayExcused, excusedEnabled},
				} {
					result, err := svc.SubmitSickNote(ctx, chain.AccountID, chain.StudentID, []calendar.Date{calendar.TodayDate().AddDays(index)}, "Testmeldung", report.status, nil)
					if report.enabled {
						require.NoError(t, err)
						require.Len(t, result.StatusDays, 1)
						assert.Equal(t, absencerecords.StudentStatusSourceParent, result.StatusDays[0].Source)
					} else {
						require.ErrorIs(t, err, parentService.ErrSickNoteDisabled)
						assert.Nil(t, result)
					}
				}
				for _, table := range []string{"active.attendance", "active.visits"} {
					count, err := db.NewSelect().TableExpr(table).Where("student_id = ?", chain.StudentID).Count(ctx)
					require.NoError(t, err)
					assert.Zero(t, count, "parent reports must not create actual attendance or room check-ins")
				}
			})
		}
	}
}

func TestChildFeatures_ReflectsTenantSettings(t *testing.T) {
	t.Parallel()

	svc, _, db := buildWriteService(t, true, false) // sick on, notes off
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	flags, err := svc.ChildFeatures(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.True(t, flags.SickNoteEnabled)
	assert.False(t, flags.NotesEnabled)
}

func TestChildFeatures_RequiresActionPermissions(t *testing.T) {
	t.Parallel()

	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	portalAndSick := `{"` + "parent_portal.access" + `": true, "` + "parent_portal.sick_note.submit" + `": true}`
	_, err := db.ExecContext(testpkg.WithPackageTenantRuntime(context.Background()), `
		UPDATE auth.guardian_student_access AS a
		SET permissions = ?::jsonb
		FROM users.student_guardian_relationships AS r
		WHERE r.tenant_id = a.tenant_id AND r.id = a.relationship_id
		  AND r.tenant_id = ? AND r.student_id = ? AND r.guardian_profile_id = ?
	`, portalAndSick, chain.TenantID, chain.StudentID, chain.GuardianProfileID)
	require.NoError(t, err)

	flags, err := svc.ChildFeatures(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.True(t, flags.SickNoteEnabled)
	assert.False(t, flags.NotesEnabled)

	portalOnly := `{"` + "parent_portal.access" + `": true}`
	_, err = db.ExecContext(testpkg.WithPackageTenantRuntime(context.Background()), `
		UPDATE auth.guardian_student_access AS a
		SET permissions = ?::jsonb
		FROM users.student_guardian_relationships AS r
		WHERE r.tenant_id = a.tenant_id AND r.id = a.relationship_id
		  AND r.tenant_id = ? AND r.student_id = ? AND r.guardian_profile_id = ?
	`, portalOnly, chain.TenantID, chain.StudentID, chain.GuardianProfileID)
	require.NoError(t, err)

	flags, err = svc.ChildFeatures(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.False(t, flags.SickNoteEnabled)
	assert.False(t, flags.NotesEnabled)
}

func TestChildFeatures_NotOwned(t *testing.T) {
	t.Parallel()

	svc, _, _ := buildWriteService(t, true, true)
	_, err := svc.ChildFeatures(testpkg.WithPackageTenantRuntime(context.Background()), 999999, 888888)
	require.ErrorIs(t, err, parentService.ErrChildNotLinked)
}

// careProfiles builds Care Plan's care-profile commands the portal writes the
// child's health information and live absence flags through.
func careProfiles(t *testing.T, db *bun.DB) careplan.StudentProfileCommands {
	t.Helper()
	commands, err := api.NewParentPortalCareProfiles(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	return commands
}

func newPausingStudentRepository[T any](source studentReads[T], locked chan<- struct{}, release <-chan struct{}) *pausingStudentRepository[T] {
	return &pausingStudentRepository[T]{studentReads: source, locked: locked, release: release}
}
func newSignalingStudentService[T any](directory interface {
	LockStudent(context.Context, int64) (*T, error)
}, students interface {
	Update(context.Context, *T) error
}, attempted chan<- struct{}) *signalingStudentService[T] {
	return &signalingStudentService[T]{directory: directory, students: students, attempted: attempted}
}
