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

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/realtime"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// stubSettings implements only the bool resolution the write service uses;
// every other SettingsService method is inherited from the embedded nil
// interface and would panic if (unexpectedly) called.
type stubSettings struct {
	configService.SettingsService
	sickEnabled  bool
	notesEnabled bool
}

func (s stubSettings) ResolveBoolForTenant(_ context.Context, _ int64, key string) (bool, error) {
	switch key {
	case configModels.KeyParentSickNoteEnabled:
		return s.sickEnabled, nil
	case configModels.KeyParentNotesEnabled:
		return s.notesEnabled, nil
	default:
		return false, nil
	}
}

// ResolveStringForTenant answers the related-accounts invite-mode lookup that
// ChildFeatures now performs. Defaults to "disabled" so the existing feature
// tests are unaffected (they predate the invite/remove capability flags).
func (s stubSettings) ResolveStringForTenant(_ context.Context, _ int64, key string) (string, error) {
	if key == configModels.KeyGuardianParentInviteMode {
		return configModels.ParentInviteModeDisabled, nil
	}
	return "", nil
}

// captureBroadcaster records tenant broadcasts so tests can assert the SSE
// fan-out fired after a sick note.
type captureBroadcaster struct {
	tenantEvents []int64
}

func (c *captureBroadcaster) BroadcastToGroup(_ int64, _ string, _ realtime.Event) error {
	return nil
}
func (c *captureBroadcaster) BroadcastToTenant(tenantID int64, _ realtime.Event) error {
	c.tenantEvents = append(c.tenantEvents, tenantID)
	return nil
}
func (c *captureBroadcaster) BroadcastToAll(_ realtime.Event) error { return nil }

func buildWriteService(t *testing.T, sickEnabled, notesEnabled bool) (parentService.Service, *captureBroadcaster, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	repos := repositories.NewFactory(db)
	bc := &captureBroadcaster{}
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:     repos.ParentChild,
		StatusDayRepo: repos.StudentStatusDay,
		StudentRepo:   repos.Student,
		NoteRepo:      repos.StudentParentNote,
		Settings:      stubSettings{sickEnabled: sickEnabled, notesEnabled: notesEnabled},
		Broadcaster:   bc,
		DB:            db,
		Logger:        slog.Default(),
	})
	return svc, bc, db
}

// --- SubmitSickNote ---

func TestSubmitSickNote_TodayFlipsLiveFlagAndStoresReason(t *testing.T) {
	svc, bc, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	rows, err := svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID,
		[]timezone.Date{timezone.TodayDate()}, "Fieber, beim Arzt")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, activeModels.StudentStatusDaySick, rows[0].Status)
	assert.Equal(t, activeModels.StudentStatusSourceParent, rows[0].Source)
	require.NotNil(t, rows[0].Note)
	assert.Equal(t, "Fieber, beim Arzt", *rows[0].Note)

	var sick bool
	require.NoError(t, db.NewSelect().ColumnExpr("COALESCE(sick,false)").TableExpr("users.students").
		Where("id = ?", chain.StudentID).Scan(context.Background(), &sick))
	assert.True(t, sick, "today's sick note must flip the live sick flag")

	assert.Contains(t, bc.tenantEvents, chain.TenantID, "SSE broadcast must fire for the tenant")
}

func TestSubmitSickNote_FutureDateDoesNotFlipLiveFlag(t *testing.T) {
	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	future := timezone.TodayDate().AddDays(7)
	rows, err := svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID,
		[]timezone.Date{future}, "")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Nil(t, rows[0].Note, "no reason supplied → note stays nil")

	var sick bool
	require.NoError(t, db.NewSelect().ColumnExpr("COALESCE(sick,false)").TableExpr("users.students").
		Where("id = ?", chain.StudentID).Scan(context.Background(), &sick))
	assert.False(t, sick, "a future-only sick note must not flip today's live flag")
}

func TestSubmitSickNote_NoDates(t *testing.T) {
	svc, _, _ := buildWriteService(t, true, true)
	_, err := svc.SubmitSickNote(context.Background(), 123, 456, nil, "")
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
		[]timezone.Date{timezone.TodayDate()}, "")
	require.ErrorIs(t, err, parentService.ErrChildNotLinked)
}

func TestSubmitSickNote_FeatureDisabled(t *testing.T) {
	svc, _, db := buildWriteService(t, false, true) // sick disabled
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID,
		[]timezone.Date{timezone.TodayDate()}, "")
	require.ErrorIs(t, err, parentService.ErrSickNoteDisabled)
}

func TestSubmitSickNote_ReasonTooLong(t *testing.T) {
	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID,
		[]timezone.Date{timezone.TodayDate()}, strings.Repeat("x", 2001))
	require.ErrorIs(t, err, parentService.ErrNoteTooLong)
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

	rows, err := svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID, []timezone.Date{date}, "")
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, activeModels.StudentStatusDaySick, rows[0].Status)

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
		[]timezone.Date{day}, "")
	require.NoError(t, err)

	from := timezone.TodayDate()
	to := timezone.NewDate(from.Year, from.Month+1, from.Day)
	sick, err := svc.ListSickDays(context.Background(), chain.AccountID, chain.StudentID, from, to)
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

// --- AddParentNote / ListParentNotes ---

func TestAddParentNote_PersistsAndReturnsNewestFirst(t *testing.T) {
	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	ctx := context.Background()
	_, err := svc.AddParentNote(ctx, chain.AccountID, chain.StudentID, "Erste Nachricht")
	require.NoError(t, err)
	_, err = svc.AddParentNote(ctx, chain.AccountID, chain.StudentID, "Zweite Nachricht")
	require.NoError(t, err)
	notes, err := svc.AddParentNote(ctx, chain.AccountID, chain.StudentID, "Dritte Nachricht")
	require.NoError(t, err)

	require.Len(t, notes, 3)
	assert.Equal(t, "Dritte Nachricht", notes[0].Body, "newest first")
	assert.Equal(t, chain.AccountID, notes[0].GuardianAccountID)

	count, err := db.NewSelect().TableExpr("users.student_parent_notes").
		Where("student_id = ?", chain.StudentID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestAddParentNote_LimitedToNewestThree(t *testing.T) {
	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, err := svc.AddParentNote(ctx, chain.AccountID, chain.StudentID, "note")
		require.NoError(t, err)
	}
	notes, err := svc.ListParentNotes(ctx, chain.AccountID, chain.StudentID, 0)
	require.NoError(t, err)
	assert.Len(t, notes, parentService.ParentNoteDisplayLimit, "default display limit is honoured")
}

func TestAddParentNote_EmptyBody(t *testing.T) {
	svc, _, _ := buildWriteService(t, true, true)
	_, err := svc.AddParentNote(context.Background(), 1, 2, "   ")
	require.ErrorIs(t, err, parentService.ErrEmptyNote)
}

func TestAddParentNote_TooLong(t *testing.T) {
	svc, _, _ := buildWriteService(t, true, true)
	_, err := svc.AddParentNote(context.Background(), 1, 2, strings.Repeat("y", 2001))
	require.ErrorIs(t, err, parentService.ErrNoteTooLong)
}

func TestAddParentNote_FeatureDisabled(t *testing.T) {
	svc, _, db := buildWriteService(t, true, false) // notes disabled
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.AddParentNote(context.Background(), chain.AccountID, chain.StudentID, "Hallo")
	require.ErrorIs(t, err, parentService.ErrNotesDisabled)
}

func TestAddParentNote_NotOwned(t *testing.T) {
	svc, _, _ := buildWriteService(t, true, true)
	_, err := svc.AddParentNote(context.Background(), 777777, 666666, "Hallo")
	require.ErrorIs(t, err, parentService.ErrChildNotLinked)
}

func TestListParentNotes_ReadsRegardlessOfSetting(t *testing.T) {
	svcWrite, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	ctx := context.Background()
	_, err := svcWrite.AddParentNote(ctx, chain.AccountID, chain.StudentID, "bleibt sichtbar")
	require.NoError(t, err)

	svcRead, _, _ := buildWriteService(t, true, false) // notes now disabled
	notes, err := svcRead.ListParentNotes(ctx, chain.AccountID, chain.StudentID, 3)
	require.NoError(t, err)
	require.Len(t, notes, 1)
	assert.Equal(t, "bleibt sichtbar", notes[0].Body)
}

func TestListParentNotes_NotOwned(t *testing.T) {
	svc, _, _ := buildWriteService(t, true, true)
	_, err := svc.ListParentNotes(context.Background(), 555555, 444444, 3)
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
	tctx := testpkg.TenantContext(1)

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

	rows, err := svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID,
		[]timezone.Date{mon, wed}, "")
	require.NoError(t, err)
	require.Len(t, rows, 2, "only the two submitted sick days, not the Tuesday excused row")
	for _, r := range rows {
		assert.Equal(t, activeModels.StudentStatusDaySick, r.Status)
		assert.NotEqual(t, tue, r.Date, "Tuesday excused row must be excluded")
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

func TestChildFeatures_NotOwned(t *testing.T) {
	svc, _, _ := buildWriteService(t, true, true)
	_, err := svc.ChildFeatures(context.Background(), 999999, 888888)
	require.ErrorIs(t, err, parentService.ErrChildNotLinked)
}

// --- length limit counts characters, not bytes ---

func TestAddParentNote_AllowsMultibyteUpToRuneLimit(t *testing.T) {
	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	// 2000 umlauts = 2000 characters but 4000 UTF-8 bytes. The old len()
	// check would have rejected this; the rune count must accept it.
	body := strings.Repeat("ä", maxNoteRunesForTest)
	notes, err := svc.AddParentNote(context.Background(), chain.AccountID, chain.StudentID, body)
	require.NoError(t, err)
	require.Len(t, notes, 1)
}

func TestAddParentNote_RejectsOverRuneLimit(t *testing.T) {
	svc, _, db := buildWriteService(t, true, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.AddParentNote(context.Background(), chain.AccountID, chain.StudentID,
		strings.Repeat("ä", maxNoteRunesForTest+1))
	require.ErrorIs(t, err, parentService.ErrNoteTooLong)
}

// maxNoteRunesForTest mirrors the service's maxParentNoteLen (kept local so
// the test doesn't depend on an exported constant).
const maxNoteRunesForTest = 2000
