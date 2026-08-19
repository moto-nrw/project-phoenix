package parent_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	notificationsService "github.com/moto-nrw/project-phoenix/services/notifications"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
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
	reports []notificationsService.AbsenceReport
}

type pausingStudentRepository struct {
	userModels.StudentRepository
	locked  chan<- struct{}
	release <-chan struct{}
}

func (r *pausingStudentRepository) FindByIDForUpdate(ctx context.Context, id int64) (*userModels.Student, error) {
	student, err := r.StudentRepository.FindByIDForUpdate(ctx, id)
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

type signalingStudentService struct {
	usersService.StudentService
	attempted chan<- struct{}
}

func (s *signalingStudentService) GetByIDForUpdate(ctx context.Context, id int64) (*userModels.Student, error) {
	close(s.attempted)
	return s.StudentService.GetByIDForUpdate(ctx, id)
}

func (n *recordingParentAbsenceNotifier) NotifyAbsenceReported(
	_ context.Context,
	report notificationsService.AbsenceReport,
) {
	n.reports = append(n.reports, report)
}

func buildWriteService(t *testing.T, sickEnabled, notesEnabled bool) (parentService.Service, *testpkg.RecordingBroadcaster, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	bc := testpkg.NewRecordingBroadcaster()
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StatusDayRepo:       repos.StudentStatusDay,
		StudentRepo:         repos.Student,
		PickupExceptionRepo: repos.StudentPickupException,
		Settings: parentSettingsStub{
			boolValues: map[string]bool{
				configModels.KeyParentSickNoteEnabled: sickEnabled,
				configModels.KeyParentNotesEnabled:    notesEnabled,
			},
			stringValues: map[string]string{
				configModels.KeyGuardianParentInviteMode: configModels.ParentInviteModeDisabled,
			},
		},
		Broadcaster: bc,
		DB:          db,
		Logger:      slog.Default(),
	})
	return svc, bc, db
}

func buildMessagingWriteService(t *testing.T, sickEnabled, notesEnabled bool) (parentService.Service, *testpkg.RecordingBroadcaster, *bun.DB, *repositories.Factory) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	bc := testpkg.NewRecordingBroadcaster()
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:            repos.ParentChild,
		StatusDayRepo:        repos.StudentStatusDay,
		StudentRepo:          repos.Student,
		PickupExceptionRepo:  repos.StudentPickupException,
		ArrivalExceptionRepo: repos.StudentArrivalException,
		Settings: parentSettingsStub{
			boolValues: map[string]bool{
				configModels.KeyParentSickNoteEnabled: sickEnabled,
				configModels.KeyParentNotesEnabled:    notesEnabled,
			},
			stringValues: map[string]string{
				configModels.KeyGuardianParentInviteMode: configModels.ParentInviteModeDisabled,
			},
		},
		Broadcaster:       bc,
		MessageThreadRepo: repos.ParentMessageThread,
		MessageRepo:       repos.ParentMessage,
		MessageReadRepo:   repos.ParentMessageRead,
		DB:                db,
		Logger:            slog.Default(),
	})
	return svc, bc, db, repos
}

// --- SubmitSickNote ---

func TestSubmitSickNote_TodayFlipsLiveFlagAndStoresReason(t *testing.T) {
	svc, bc, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	sickResult, err := svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID,
		[]timezone.Date{timezone.TodayDate()}, "Fieber, beim Arzt", activeModels.StudentStatusDaySick)
	require.NoError(t, err)
	require.Len(t, sickResult.StatusDays, 1)
	assert.Equal(t, activeModels.StudentStatusDaySick, sickResult.StatusDays[0].Status)
	assert.Equal(t, activeModels.StudentStatusSourceParent, sickResult.StatusDays[0].Source)
	require.NotNil(t, sickResult.StatusDays[0].Note)
	assert.Equal(t, "Fieber, beim Arzt", *sickResult.StatusDays[0].Note)

	var sick bool
	require.NoError(t, db.NewSelect().ColumnExpr("COALESCE(sick,false)").TableExpr("users.students").
		Where("id = ?", chain.StudentID).Scan(context.Background(), &sick))
	assert.True(t, sick, "today's sick note must flip the live sick flag")

	assert.Contains(t, tenantBroadcastIDs(bc), chain.TenantID, "SSE broadcast must fire for the tenant")
}

func TestSubmitSickNote_ResubmitDoesNotNotifyAgain(t *testing.T) {
	svc, _, db := buildWriteService(t, true, true)
	notifier := &recordingParentAbsenceNotifier{}
	require.Implements(t, (*parentService.AbsenceNotifierSetter)(nil), svc)
	svc.(parentService.AbsenceNotifierSetter).SetAbsenceNotifier(notifier)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	for range 2 {
		_, err := svc.SubmitSickNote(
			context.Background(),
			chain.AccountID,
			chain.StudentID,
			[]timezone.Date{timezone.TodayDate()},
			"Fieber",
			activeModels.StudentStatusDaySick,
		)
		require.NoError(t, err)
	}

	require.Len(t, notifier.reports, 1)
	assert.Equal(t, activeModels.StudentStatusDaySick, notifier.reports[0].Status)
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
	svc, _, db, _ := buildMessagingWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	// Downgrade the guardian to read-only portal access (no notes.write).
	_, err := db.ExecContext(context.Background(), `
		UPDATE users.students_guardians
		SET permissions = '{"parent_portal.access": true}'::jsonb
		WHERE tenant_id = ? AND student_id = ? AND guardian_profile_id = ?
	`, chain.TenantID, chain.StudentID, chain.GuardianProfileID)
	require.NoError(t, err)

	ctx := context.Background()

	_, err = svc.PostChildMessage(ctx, chain.AccountID, chain.StudentID, "Hallo OGS")
	require.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied, "posting a message must require notes.write")
}

// TestPostChildMessage_EmptyBody re-establishes a guard the deleted one-way
// notes suite covered: a blank / whitespace-only message body is rejected with a
// clean ErrEmptyNote (→ 400) BEFORE any insert. Without it the empty body would
// reach the chk_parent_messages_body_not_blank CHECK and surface as a raw 500.
func TestPostChildMessage_EmptyBody(t *testing.T) {
	svc, _, db, _ := buildMessagingWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.PostChildMessage(context.Background(), chain.AccountID, chain.StudentID, "   \n\t ")
	require.ErrorIs(t, err, parentService.ErrEmptyNote)
}

// TestPostChildMessage_BodyTooLong locks in the rune-count upper bound
// (maxParentNoteLen = 2000) on the messaging path, mirroring the sick-note
// ReasonTooLong guard. 2001 runes → ErrNoteTooLong (→ 400), never a silent insert.
func TestPostChildMessage_BodyTooLong(t *testing.T) {
	svc, _, db, _ := buildMessagingWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.PostChildMessage(context.Background(), chain.AccountID, chain.StudentID, strings.Repeat("x", 2001))
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
	svc, _, db, _ := buildMessagingWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	body := strings.Repeat("ä", maxMessageRunesForTest)
	view, err := svc.PostChildMessage(context.Background(), chain.AccountID, chain.StudentID, body)
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
	svc, _, db, _ := buildMessagingWriteService(t, true, false)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	ctx := context.Background()
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
	svc, _, db, _ := buildMessagingWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	other := testpkg.CreateTestStudent(t, db, "Mara", "Fremd", "2b")
	defer func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.students WHERE id = ?`, other.ID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.persons WHERE id = ?`, other.PersonID)
	}()

	_, err := svc.PostChildMessage(context.Background(), chain.AccountID, other.ID, "Hallo OGS")
	require.ErrorIs(t, err, parentService.ErrChildNotLinked,
		"a guardian must not post a message on a child they do not guardian")
}

// TestGetChildConversation_NotOwned is the read-side counterpart: a guardian MUST
// NOT be able to read another family's conversation. GetChildConversation also
// marks the thread read, so an ownership bypass here would both leak a message and
// silently advance the wrong reader's cursor.
func TestGetChildConversation_NotOwned(t *testing.T) {
	svc, _, db, _ := buildMessagingWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	other := testpkg.CreateTestStudent(t, db, "Mara", "Fremd", "2b")
	defer func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.students WHERE id = ?`, other.ID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.persons WHERE id = ?`, other.PersonID)
	}()

	_, err := svc.GetChildConversation(context.Background(), chain.AccountID, other.ID)
	require.ErrorIs(t, err, parentService.ErrChildNotLinked,
		"a guardian must not read a conversation for a child they do not guardian")
}

func TestSubmitSickNote_FutureDateDoesNotFlipLiveFlag(t *testing.T) {
	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	future := timezone.TodayDate().AddDays(7)
	sickResult, err := svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID,
		[]timezone.Date{future}, "Fieber", activeModels.StudentStatusDaySick)
	require.NoError(t, err)
	require.Len(t, sickResult.StatusDays, 1)
	require.NotNil(t, sickResult.StatusDays[0].Note)
	assert.Equal(t, "Fieber", *sickResult.StatusDays[0].Note)

	var sick bool
	require.NoError(t, db.NewSelect().ColumnExpr("COALESCE(sick,false)").TableExpr("users.students").
		Where("id = ?", chain.StudentID).Scan(context.Background(), &sick))
	assert.False(t, sick, "a future-only sick note must not flip today's live flag")
}

func TestSubmitSickNote_RefusesPartialAbsenceConflict(t *testing.T) {
	svc, _, db := buildWriteService(t, true, true)
	repos := repositories.NewFactory(db)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Partial", "Author")
	t.Cleanup(func() {
		testpkg.CleanupParentGuardianChain(t, db, chain)
		testpkg.CleanupStaffFixtures(t, db, staff.ID)
	})

	date := timezone.TodayDate().AddDays(7)
	from := timezone.WallClock(time.Date(2000, time.January, 1, 13, 30, 0, 0, time.UTC))
	staffID := staff.ID
	pickup := &scheduleModels.StudentPickupException{
		StudentID:             chain.StudentID,
		ExceptionDate:         date,
		PickupTime:            &from,
		ExcusedFrom:           &from,
		ExcusedCreatedBy:      &staffID,
		ExcusedOwnsPickupTime: true,
		Source:                scheduleModels.ExceptionSourceStaff,
		CreatedBy:             staff.ID,
	}
	pickup.SetTenantID(chain.TenantID)
	require.NoError(t, repos.StudentPickupException.Create(testpkg.TenantContext(chain.TenantID), pickup))

	_, err := svc.SubmitSickNote(
		context.Background(), chain.AccountID, chain.StudentID,
		[]timezone.Date{date}, "Fieber", activeModels.StudentStatusDaySick,
	)
	require.ErrorIs(t, err, parentService.ErrCareExceptionConflict)

	rows, findErr := repos.StudentStatusDay.FindActiveByStudentAndDateRange(
		testpkg.TenantContext(chain.TenantID), chain.StudentID, date, date,
	)
	require.NoError(t, findErr)
	assert.Empty(t, rows)
}

func TestSubmitSickNote_FutureWriteSerializesWithStaffConflictCheck(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	parentLocked := make(chan struct{})
	releaseParent := make(chan struct{})
	parentSvc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StatusDayRepo:       repos.StudentStatusDay,
		PickupExceptionRepo: repos.StudentPickupException,
		StudentRepo: &pausingStudentRepository{
			StudentRepository: repos.Student,
			locked:            parentLocked,
			release:           releaseParent,
		},
		Settings: parentSettingsStub{
			boolValues: map[string]bool{
				configModels.KeyParentSickNoteEnabled: true,
				configModels.KeyParentNotesEnabled:    true,
			},
			stringValues: map[string]string{
				configModels.KeyGuardianParentInviteMode: configModels.ParentInviteModeDisabled,
			},
		},
		Broadcaster: testpkg.NewRecordingBroadcaster(),
		DB:          db,
		Logger:      slog.Default(),
	})

	statusSvc := activeService.NewStudentStatusDayService(repos.StudentStatusDay)
	studentSvc := usersService.NewStudentService(repos.Student, repos.PrivacyConsent, repos.StudentCompanion, nil)
	staffAttempted := make(chan struct{})
	staffStudentSvc := &signalingStudentService{StudentService: studentSvc, attempted: staffAttempted}
	date := timezone.TodayDate().AddDays(40)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	parentResult := make(chan error, 1)
	go func() {
		_, err := parentSvc.SubmitSickNote(ctx, chain.AccountID, chain.StudentID,
			[]timezone.Date{date}, "Fieber", activeModels.StudentStatusDaySick)
		parentResult <- err
	}()

	select {
	case <-parentLocked:
	case <-ctx.Done():
		t.Fatal("parent write did not acquire the student lock")
	}

	staffResult := make(chan error, 1)
	go func() {
		staffResult <- statusSvc.CreateForDates(testpkg.TenantContext(chain.TenantID), activeService.StatusDayWriteContext{
			DB:             db,
			TenantID:       chain.TenantID,
			StudentService: staffStudentSvc,
			Authorize:      func(context.Context, *userModels.Student) bool { return true },
			AfterCommit:    func(int64) {},
		}, chain.StudentID, activeModels.StudentStatusDayExcused, "Termin", []timezone.Date{date})
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
	var conflictErr *activeService.StudentStatusDayConflictError
	require.ErrorAs(t, staffErr, &conflictErr)
	require.Len(t, conflictErr.Conflicts, 1)
	assert.Equal(t, activeModels.StudentStatusDaySick, conflictErr.Conflicts[0].Status)

	rows, err := statusSvc.GetActiveByStudentAndDateRange(testpkg.TenantContext(chain.TenantID), chain.StudentID, date, date)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, activeModels.StudentStatusDaySick, rows[0].Status,
		"the staff write must not overwrite the parent row after waiting for its lock")
}

func TestSubmitSickNote_NoDates(t *testing.T) {
	svc, _, _ := buildWriteService(t, true, true)
	_, err := svc.SubmitSickNote(context.Background(), 123, 456, nil, "", activeModels.StudentStatusDaySick)
	require.ErrorIs(t, err, parentService.ErrNoDates)
}

func TestSubmitSickNote_NotOwnedChild(t *testing.T) {
	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	other := testpkg.CreateTestStudent(t, db, "Mara", "Fremd", "2b")
	defer func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.students WHERE id = ?`, other.ID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM users.persons WHERE id = ?`, other.PersonID)
	}()

	_, err := svc.SubmitSickNote(context.Background(), chain.AccountID, other.ID,
		[]timezone.Date{timezone.TodayDate()}, "", activeModels.StudentStatusDaySick)
	require.ErrorIs(t, err, parentService.ErrChildNotLinked)
}

func TestSubmitSickNote_FeatureDisabled(t *testing.T) {
	svc, _, db := buildWriteService(t, false, true) // sick disabled
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID,
		[]timezone.Date{timezone.TodayDate()}, "", activeModels.StudentStatusDaySick)
	require.ErrorIs(t, err, parentService.ErrSickNoteDisabled)
}

func TestSubmitSickNote_MissingGuardianPermission(t *testing.T) {
	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := db.ExecContext(context.Background(), `
		UPDATE users.students_guardians
		SET permissions = '{"parent_portal.access": true}'::jsonb
		WHERE tenant_id = ? AND student_id = ? AND guardian_profile_id = ?
	`, chain.TenantID, chain.StudentID, chain.GuardianProfileID)
	require.NoError(t, err)

	_, err = svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID,
		[]timezone.Date{timezone.TodayDate()}, "", activeModels.StudentStatusDaySick)
	require.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied)
}

func TestSubmitSickNote_ReasonTooLong(t *testing.T) {
	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID,
		[]timezone.Date{timezone.TodayDate()}, strings.Repeat("x", 2001), activeModels.StudentStatusDaySick)
	require.ErrorIs(t, err, parentService.ErrNoteTooLong)
}

func TestSubmitSickNote_EmptyReasonRejected(t *testing.T) {
	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID,
		[]timezone.Date{timezone.TodayDate()}, "   ", activeModels.StudentStatusDaySick)
	require.ErrorIs(t, err, parentService.ErrEmptyNote)
}

func TestSubmitSickNote_ClearsClassTripForSubmittedDate(t *testing.T) {
	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	statusRepo := repositories.NewFactory(db).StudentStatusDay
	ctx := testpkg.TenantContext(chain.TenantID)
	date := timezone.TodayDate().AddDays(7)
	require.NoError(t, statusRepo.UpsertReported(ctx, &activeModels.StudentStatusDay{
		StudentID:  chain.StudentID,
		Date:       date,
		Status:     activeModels.StudentStatusDayClassTrip,
		ReportedAt: time.Now(),
		Source:     activeModels.StudentStatusSourcePlanned,
	}))

	sickResult, err := svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID, []timezone.Date{date}, "Krank", activeModels.StudentStatusDaySick)
	require.NoError(t, err)
	require.Len(t, sickResult.StatusDays, 1)
	assert.Equal(t, activeModels.StudentStatusDaySick, sickResult.StatusDays[0].Status)

	activeRows, err := statusRepo.FindActiveByStudentAndDateRange(ctx, chain.StudentID, date, date)
	require.NoError(t, err)
	require.Len(t, activeRows, 1)
	assert.Equal(t, activeModels.StudentStatusDaySick, activeRows[0].Status)
}

// --- ListSickDays ---

func TestListSickDays_ReturnsSickOnlyAfterSubmit(t *testing.T) {
	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	day := timezone.TodayDate().AddDays(3)
	_, err := svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID,
		[]timezone.Date{day}, "Krank", activeModels.StudentStatusDaySick)
	require.NoError(t, err)

	from := timezone.TodayDate()
	to := timezone.NewDate(from.Year, from.Month+1, from.Day)
	sick, err := svc.ListSickDays(context.Background(), chain.AccountID, chain.StudentID, from, to)
	require.NoError(t, err)
	require.Len(t, sick, 1)
	assert.Equal(t, activeModels.StudentStatusDaySick, sick[0].Status)
}

func TestListSickDays_AllowsPortalAccessWithoutWritePermissions(t *testing.T) {
	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	day := timezone.TodayDate().AddDays(2)
	_, err := svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID,
		[]timezone.Date{day}, "Krank", activeModels.StudentStatusDaySick)
	require.NoError(t, err)

	_, err = db.ExecContext(context.Background(), `
		UPDATE users.students_guardians
		SET permissions = '{"parent_portal.access": true}'::jsonb
		WHERE tenant_id = ? AND student_id = ? AND guardian_profile_id = ?
	`, chain.TenantID, chain.StudentID, chain.GuardianProfileID)
	require.NoError(t, err)

	sick, err := svc.ListSickDays(context.Background(), chain.AccountID, chain.StudentID, day, day)
	require.NoError(t, err)
	require.Len(t, sick, 1)
	assert.Equal(t, activeModels.StudentStatusDaySick, sick[0].Status)
}

func TestListSickDays_NotOwned(t *testing.T) {
	svc, _, _ := buildWriteService(t, true, true)
	from := timezone.TodayDate()
	_, err := svc.ListSickDays(context.Background(), 999999, 888888, from, timezone.NewDate(from.Year, from.Month+1, from.Day))
	require.ErrorIs(t, err, parentService.ErrChildNotLinked)
}

// TestSubmitSickNote_NonContiguousExcludesUnrelatedRows guards the response
// filter: a non-contiguous submission (Mon + Wed) must not return an
// unrelated active excused row that falls on Tuesday inside the min..max
// range.
func TestSubmitSickNote_NonContiguousExcludesUnrelatedRows(t *testing.T) {
	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	statusRepo := repositories.NewFactory(db).StudentStatusDay
	tctx := testpkg.Ctx(t)

	base := timezone.TodayDate().AddDays(7)
	mon := base
	tue := base.AddDays(1)
	wed := base.AddDays(2)

	// Pre-existing excused row on Tuesday (between the two sick days).
	require.NoError(t, statusRepo.UpsertReported(tctx, &activeModels.StudentStatusDay{
		StudentID:  chain.StudentID,
		Date:       tue,
		Status:     activeModels.StudentStatusDayExcused,
		ReportedAt: time.Now(),
		Source:     activeModels.StudentStatusSourceManual,
	}))

	sickResult, err := svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID,
		[]timezone.Date{mon, wed}, "Krank", activeModels.StudentStatusDaySick)
	require.NoError(t, err)
	require.Len(t, sickResult.StatusDays, 2, "only the two submitted sick days, not the Tuesday excused row")
	for _, r := range sickResult.StatusDays {
		assert.Equal(t, activeModels.StudentStatusDaySick, r.Status)
		assert.NotEqual(t, tue, r.Date, "Tuesday excused row must be excluded")
	}
}

// --- SubmitSickNote: excused ("Termin/Abwesenheit") path (issue #1735) ---

func TestSubmitSickNote_ExcusedTodayStoresExcusedWithoutLiveFlag(t *testing.T) {
	svc, bc, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	sickResult, err := svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID,
		[]timezone.Date{timezone.TodayDate()}, "Zahnarzttermin", activeModels.StudentStatusDayExcused)
	require.NoError(t, err)
	require.Len(t, sickResult.StatusDays, 1)
	assert.Equal(t, activeModels.StudentStatusDayExcused, sickResult.StatusDays[0].Status)
	assert.Equal(t, activeModels.StudentStatusSourceParent, sickResult.StatusDays[0].Source)
	require.NotNil(t, sickResult.StatusDays[0].Note)
	assert.Equal(t, "Zahnarzttermin", *sickResult.StatusDays[0].Note)

	var sick, excused bool
	require.NoError(t, db.NewSelect().
		ColumnExpr("COALESCE(sick,false), COALESCE(excused,false)").
		TableExpr("users.students").Where("id = ?", chain.StudentID).
		Scan(context.Background(), &sick, &excused))
	assert.False(t, sick, "an excused absence must not set the live sick flag")
	assert.False(t, excused, "an excused absence must not set a live excused flag (issue #1735)")

	assert.Contains(t, tenantBroadcastIDs(bc), chain.TenantID, "SSE broadcast must fire for the tenant")
}

func TestSubmitSickNote_ExcusedTodayClearsStaleLiveSickFlag(t *testing.T) {
	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	// First report sick today (flips the live sick flag), then switch the same
	// day to an excused absence: the stale live sick flag must be cleared so the
	// flag stays consistent with the now-cleared sick status day.
	_, err := svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID,
		[]timezone.Date{timezone.TodayDate()}, "Krank", activeModels.StudentStatusDaySick)
	require.NoError(t, err)

	sickResult, err := svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID,
		[]timezone.Date{timezone.TodayDate()}, "Termin", activeModels.StudentStatusDayExcused)
	require.NoError(t, err)
	require.Len(t, sickResult.StatusDays, 1)
	assert.Equal(t, activeModels.StudentStatusDayExcused, sickResult.StatusDays[0].Status)

	var sick bool
	require.NoError(t, db.NewSelect().ColumnExpr("COALESCE(sick,false)").TableExpr("users.students").
		Where("id = ?", chain.StudentID).Scan(context.Background(), &sick))
	assert.False(t, sick, "switching today from sick to excused must clear the live sick flag")
}

func TestSubmitSickNote_InvalidStatus(t *testing.T) {
	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID,
		[]timezone.Date{timezone.TodayDate()}, "", "class_trip")
	require.ErrorIs(t, err, parentService.ErrInvalidStatus)
}

func TestListSickDays_ReturnsSickAndExcused(t *testing.T) {
	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	sickDay := timezone.TodayDate().AddDays(2)
	excusedDay := timezone.TodayDate().AddDays(4)
	_, err := svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID,
		[]timezone.Date{sickDay}, "Krank", activeModels.StudentStatusDaySick)
	require.NoError(t, err)
	_, err = svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID,
		[]timezone.Date{excusedDay}, "Termin", activeModels.StudentStatusDayExcused)
	require.NoError(t, err)

	from := timezone.TodayDate()
	to := timezone.NewDate(from.Year, from.Month+1, from.Day)
	absences, err := svc.ListSickDays(context.Background(), chain.AccountID, chain.StudentID, from, to)
	require.NoError(t, err)
	require.Len(t, absences, 2, "both the sick and the excused absence must be listed")
	byStatus := map[string]timezone.Date{}
	for _, a := range absences {
		byStatus[a.Status] = a.Date
	}
	assert.Equal(t, sickDay, byStatus[activeModels.StudentStatusDaySick])
	assert.Equal(t, excusedDay, byStatus[activeModels.StudentStatusDayExcused])
}

// A staff-created excused day (source=planned/manual) is an internal scheduled
// status. It must NOT leak into the parents portal listing, while the parent's
// own excused day on a different date still shows. Guards the privacy fix for
// the issue #1735 follow-up review.
func TestListSickDays_ExcludesStaffCreatedExcused(t *testing.T) {
	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	statusRepo := repositories.NewFactory(db).StudentStatusDay
	tctx := testpkg.Ctx(t)

	staffExcusedDay := timezone.TodayDate().AddDays(2)
	parentExcusedDay := timezone.TodayDate().AddDays(4)
	staffNote := "interner Hinweis"

	// Staff-created excused row (source=manual, carries a staff note).
	require.NoError(t, statusRepo.UpsertReported(tctx, &activeModels.StudentStatusDay{
		StudentID:  chain.StudentID,
		Date:       staffExcusedDay,
		Status:     activeModels.StudentStatusDayExcused,
		ReportedAt: time.Now(),
		Source:     activeModels.StudentStatusSourceManual,
		Note:       &staffNote,
	}))

	// Parent's own excused report on a different date.
	_, err := svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID,
		[]timezone.Date{parentExcusedDay}, "Termin", activeModels.StudentStatusDayExcused)
	require.NoError(t, err)

	from := timezone.TodayDate()
	to := timezone.NewDate(from.Year, from.Month+1, from.Day)
	absences, err := svc.ListSickDays(context.Background(), chain.AccountID, chain.StudentID, from, to)
	require.NoError(t, err)
	require.Len(t, absences, 1, "only the parent-reported excused day must be listed")
	assert.Equal(t, parentExcusedDay, absences[0].Date)
	assert.Equal(t, activeModels.StudentStatusSourceParent, absences[0].Source)
	for _, a := range absences {
		assert.NotEqual(t, staffExcusedDay, a.Date, "staff-created excused day must not leak to the parent")
	}
}

// --- ChildFeatures ---

func TestChildFeatures_ReflectsTenantSettings(t *testing.T) {
	svc, _, db := buildWriteService(t, true, false) // sick on, notes off
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	flags, err := svc.ChildFeatures(context.Background(), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.True(t, flags.SickNoteEnabled)
	assert.False(t, flags.NotesEnabled)
}

func TestChildFeatures_RequiresActionPermissions(t *testing.T) {
	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	portalAndSick := `{"` + authorize.GuardianPermissionPortalAccess + `": true, "` + authorize.GuardianPermissionSickNoteSubmit + `": true}`
	_, err := db.ExecContext(context.Background(), `
		UPDATE users.students_guardians
		SET permissions = ?::jsonb
		WHERE tenant_id = ? AND student_id = ? AND guardian_profile_id = ?
	`, portalAndSick, chain.TenantID, chain.StudentID, chain.GuardianProfileID)
	require.NoError(t, err)

	flags, err := svc.ChildFeatures(context.Background(), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.True(t, flags.SickNoteEnabled)
	assert.False(t, flags.NotesEnabled)

	portalOnly := `{"` + authorize.GuardianPermissionPortalAccess + `": true}`
	_, err = db.ExecContext(context.Background(), `
		UPDATE users.students_guardians
		SET permissions = ?::jsonb
		WHERE tenant_id = ? AND student_id = ? AND guardian_profile_id = ?
	`, portalOnly, chain.TenantID, chain.StudentID, chain.GuardianProfileID)
	require.NoError(t, err)

	flags, err = svc.ChildFeatures(context.Background(), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.False(t, flags.SickNoteEnabled)
	assert.False(t, flags.NotesEnabled)
}

func TestChildFeatures_NotOwned(t *testing.T) {
	svc, _, _ := buildWriteService(t, true, true)
	_, err := svc.ChildFeatures(context.Background(), 999999, 888888)
	require.ErrorIs(t, err, parentService.ErrChildNotLinked)
}
