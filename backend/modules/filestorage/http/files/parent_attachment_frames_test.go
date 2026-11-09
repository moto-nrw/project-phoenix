//go:build testhooks

package files_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	rootAPI "github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	filestorage "github.com/moto-nrw/project-phoenix/modules/filestorage"
	filesHTTP "github.com/moto-nrw/project-phoenix/modules/filestorage/http/files"
	settingsModule "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The parent attachment routes of ADR 0161 (#5894) observed over HTTP on a
// real, isolated database, with the frame's own failures injected at the
// driver (test.BeginFault): the answer, the BEGINs the request attempted (no
// retry, no second way after a failure) and that nothing is written. The
// invariant table of PR #6388 names every row; "E9/E15" rows are deliberate
// deviations from the code before the bundle, "E4" the cancelled refusal.

const (
	attachmentNewsSwitch = "operations.parent_news_enabled"
	answerOK             = "200"
	answerRefused        = "404 general.input"
	answerFailed         = "500 general.server"

	tenantRoleSwitch = "SELECT set_config('role', 'phoenix_tenant'"
	audienceLast     = `SELECT "pds".*` // the last audience statement for an account outside it
	switchRead       = `SELECT "setting_value"`
)

type attachmentFrameRig struct {
	db       *bun.DB // the isolated clone, for fixtures
	pool     *bun.DB // the routes' pool, with the fault connector
	fault    *testpkg.BeginFault
	counter  *testpkg.QueryCounter // the pool's statements while a request runs
	router   chi.Router
	settings owners.SettingsTestModule

	schoolA, schoolB, organization int64
	announcementID, systemID       int64
	attachmentID                   string
	parentToken, strangerToken     string
}

func newAttachmentFrameRig(t *testing.T) *attachmentFrameRig {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	sqlDB, fault := testpkg.OpenPostgresSQLWithBeginFault(testpkg.IsolatedTestDSN(t))
	counter := testpkg.NewQueryCounter()
	counter.Stop()
	pool := testpkg.NewBunDB(sqlDB).WithQueryHook(counter)
	t.Cleanup(func() { _ = pool.Close() })
	ctx := testpkg.Ctx(t)

	database := testpkg.NewDatabase(t, pool)
	unit := testpkg.TenantRuntime(t, pool)
	store, err := objects.PrivateUploadsBackend()
	require.NoError(t, err)
	files, _, err := rootAPI.NewFileStoreTestModule(database, unit, store)
	require.NoError(t, err)
	settings, err := owners.NewSettingsTestModule(database, unit)
	require.NoError(t, err)
	repos := rootAPI.NewParentMessagingSuiteRepositories(database)
	portal, err := rootAPI.NewParentCareScheduleTestService(database, rootAPI.StudentTestModule{}, rootAPI.ActiveTestModule{}, rootAPI.ActiveSettings{Settings: settings.Settings})
	require.NoError(t, err)
	portal.AnnouncementRepo = owners.BindParentAnnouncementAccess(repos.ParentAnnouncement)
	reads, err := filestorage.TenantReads(files)
	require.NoError(t, err)
	guardian, err := owners.NewParentGuardianAttachments(portal, reads)
	require.NoError(t, err)
	resource := filesHTTP.NewResource(files, slog.Default())
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, pool))
	router.Use(rootAPI.RequestSettingsCacheMiddleware, rootAPI.RequestIdentityCacheMiddleware)
	router.Mount("/api/announcement-attachments", resource.AnnouncementAttachmentRouter(rootAPI.ProtectedTenantRoutes))
	router.Mount("/parent-news-attachments", resource.ParentAnnouncementAttachmentRouter(rootAPI.ProtectedParentGroup, guardian))

	rig := &attachmentFrameRig{db: db, pool: pool, fault: fault, counter: counter, router: router, settings: settings, schoolA: testpkg.Tenant(t)}
	_, admin := testpkg.CreateTestStaffWithAccount(t, db, "Attachment", "Frames")
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	stranger := testpkg.CreateTestParentGuardianChain(t, db)
	rig.parentToken = communication18AttachmentToken(t, chain.AccountID, chain.Email, 0, "parent", []string{"parent"}, nil)
	rig.strangerToken = communication18AttachmentToken(t, stranger.AccountID, stranger.Email, 0, "parent", []string{"parent"}, nil)
	staffToken := communication18AttachmentToken(t, admin.ID, admin.Email, rig.schoolA, "", []string{"user"}, []string{permissions.AdminWildcard})
	t.Cleanup(func() { require.NoError(t, Communication18AttachmentCleanupForTest(rig.schoolA)) })

	announcement := testpkg.CreateTestParentAnnouncement(t, db, admin.ID, "Attachment frames")
	system := testpkg.CreateTestParentAnnouncement(t, db, admin.ID, "Attachment frames system")
	rig.announcementID, rig.systemID = announcement.ID, system.ID
	for _, id := range []int64{announcement.ID, system.ID} {
		_, err = db.NewRaw(`INSERT INTO users.parent_announcement_targets (tenant_id,announcement_id,target_type,target_ref_id) VALUES (?,?,'student',?)`, rig.schoolA, id, chain.StudentID).Exec(ctx)
		require.NoError(t, err)
	}
	rig.attachmentID = rig.upload(t, staffToken, announcement.ID)
	rig.upload(t, staffToken, system.ID)
	_, err = db.NewRaw(`UPDATE users.parent_announcements SET system_kind = 'care_cancellation' WHERE id = ?`, system.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE users.parent_announcements SET published_at = ? WHERE id IN (?, ?)`, time.Now().Add(-time.Hour), announcement.ID, system.ID).Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, rig.schoolA).Scan(ctx, &rig.organization))

	t.Run("school B", func(t *testing.T) {
		rig.schoolB = testpkg.OwnTenant(t)
		testpkg.EnsureTestTenant(t, db, rig.schoolB)
	})
	return rig
}

func (r *attachmentFrameRig) upload(t *testing.T, token string, announcementID int64) string {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "Elternbrief.pdf")
	require.NoError(t, err)
	_, err = part.Write([]byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF\n"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/announcement-attachments/%d", announcementID), &body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	r.router.ServeHTTP(rec, req.WithContext(testpkg.WithTestTenantRuntime(t, context.Background())))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var created struct{ Data struct{ ID string } }
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	return created.Data.ID
}

// setSwitch stores the school's news switch.
func (r *attachmentFrameRig) setSwitch(t *testing.T, on bool) {
	t.Helper()
	school := tenant.WithTenantID(testpkg.WithTestTenantRuntime(t, context.Background()), r.schoolA)
	require.NoError(t, r.settings.Settings.SetValue(school, attachmentNewsSwitch, on, nil, nil))
}

// requestContext is an unbound request context whose settings cache holds the
// switch open, closed or nothing ("empty", "open", "closed"). The stored
// value equals the cached one; warming happens before any fault is armed.
func (r *attachmentFrameRig) requestContext(t *testing.T, cache string) context.Context {
	t.Helper()
	ctx := settingsModule.WithSettingsRequestCache(testpkg.WithTestTenantRuntime(t, context.Background()))
	r.setSwitch(t, cache != "closed")
	if cache != "empty" {
		on, err := r.settings.Settings.ResolveBoolForTenant(ctx, r.schoolA, attachmentNewsSwitch)
		require.NoError(t, err)
		require.Equal(t, cache == "open", on)
	}
	return ctx
}

func (r *attachmentFrameRig) listPath(announcementID int64) string {
	return fmt.Sprintf("/parent-news-attachments/%d", announcementID)
}

func (r *attachmentFrameRig) downloadPath(attachmentID string) string {
	return fmt.Sprintf("/parent-news-attachments/%d/%s/download", r.announcementID, attachmentID)
}

// attachmentAnswer is what one request did: its answer, the BEGINs it
// attempted, the statements it issued and the rows it wrote.
type attachmentAnswer struct {
	answer     string
	begins     int64
	statements []string
	writes     int64
}

// serve runs one request in ctx with a hard deadline.
func (r *attachmentFrameRig) serve(t *testing.T, ctx context.Context, path, token string) attachmentAnswer {
	t.Helper()
	return r.serveWith(t, ctx, path, token, false)
}

// serveWith runs one request in ctx with a hard deadline of 10 s; a request
// the test cancels itself only fails on that deadline.
func (r *attachmentFrameRig) serveWith(t *testing.T, ctx context.Context, path, token string, cancellable bool) attachmentAnswer {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	counter := r.counter
	counter.Reset()
	counter.Start()
	defer counter.Stop()
	begins := r.fault.Begins()
	req := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	r.router.ServeHTTP(rec, req)
	if cancellable {
		require.NotErrorIs(t, ctx.Err(), context.DeadlineExceeded, "the request ran into its deadline")
	} else {
		require.NoError(t, ctx.Err(), "the request ran into its deadline")
	}
	got := attachmentAnswer{begins: r.fault.Begins() - begins, statements: counter.Queries(), writes: counter.WriteRows()}
	if rec.Code == http.StatusOK {
		got.answer = answerOK
		return got
	}
	var body struct{ Code string }
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	got.answer = fmt.Sprintf("%d %s", rec.Code, body.Code)
	return got
}

// noWrites asserts that a request wrote nothing.
func noWrites(t *testing.T, got attachmentAnswer, name string) {
	t.Helper()
	require.Zero(t, got.writes, name)
	for _, q := range got.statements {
		head := strings.ToUpper(strings.TrimSpace(q))
		require.False(t, strings.HasPrefix(head, "INSERT") || strings.HasPrefix(head, "UPDATE") || strings.HasPrefix(head, "DELETE"), "%s wrote: %s", name, q)
	}
}

// frameShape names the scope of every unit of a request, in order.
func frameShape(statements []string) string {
	var units []string
	open := false
	for _, q := range statements {
		switch {
		case q == "BEGIN":
			units = append(units, "tenant")
			open = true
		case !open:
		case q == "SET LOCAL ROLE phoenix_admin" && units[len(units)-1] == "tenant":
			units[len(units)-1] = "admin"
		case strings.HasPrefix(q, tenantRoleSwitch) && units[len(units)-1] == "admin":
			units[len(units)-1] = "admin>tenant"
		case q == "COMMIT" || q == "ROLLBACK":
			units[len(units)-1] += " " + strings.ToLower(q)
			open = false
		}
	}
	return strings.Join(units, ", ")
}

// afterFailure returns the statement right after the first one matching
// prefix, or "" when there is none.
func afterFailure(statements []string, prefix string) string {
	for i, q := range statements {
		if strings.HasPrefix(q, prefix) && i+1 < len(statements) {
			return statements[i+1]
		}
	}
	return ""
}

// attachmentExpectation selects the column a run checks: "now" is this
// code's contract; the counter-check on the code before the bundle sets it to
// "old" and must then pass unchanged (see PR #6388).
const attachmentExpectation = "now"

// attachmentWant is the expected answer, the BEGINs the request attempted and
// whether the armed injection never fired (the frame it aims at did not run).
type attachmentWant struct {
	answer  string
	begins  int64
	pending bool
}

type attachmentFaultCase struct {
	name     string
	path     func(r *attachmentFrameRig) string
	stranger bool
	cache    string // "empty", "open", "closed"
	arm      func(f *testpkg.BeginFault, cancel context.CancelFunc)
	old, now attachmentWant
}

func (c attachmentFaultCase) want() attachmentWant {
	if attachmentExpectation == "old" {
		return c.old
	}
	return c.now
}

func listOf(r *attachmentFrameRig) string   { return r.listPath(r.announcementID) }
func systemOf(r *attachmentFrameRig) string { return r.listPath(r.systemID) }
func missingRowOf(r *attachmentFrameRig) string {
	return r.downloadPath("987654321")
}

func armBegin(skip int) func(*testpkg.BeginFault, context.CancelFunc) {
	return func(f *testpkg.BeginFault, _ context.CancelFunc) { f.ArmBegin(skip) }
}

func armCommit(skip int) func(*testpkg.BeginFault, context.CancelFunc) {
	return func(f *testpkg.BeginFault, _ context.CancelFunc) { f.ArmCommit(skip) }
}

func armRollback(skip int) func(*testpkg.BeginFault, context.CancelFunc) {
	return func(f *testpkg.BeginFault, _ context.CancelFunc) { f.ArmRollback(skip) }
}

func armStatement(prefix string) func(*testpkg.BeginFault, context.CancelFunc) {
	return func(f *testpkg.BeginFault, _ context.CancelFunc) { f.ArmStatement(prefix, 0) }
}

func cancelAfter(prefix string) func(*testpkg.BeginFault, context.CancelFunc) {
	return func(f *testpkg.BeginFault, cancel context.CancelFunc) { f.AfterStatement(prefix, cancel) }
}

// TestParentAttachmentFrameFaultsAnswerPerTable injects every failure of the
// table's frame rows once and checks the answer, that the request attempted
// no BEGIN beyond the failed unit (no retry, no second way) and wrote nothing.
func TestParentAttachmentFrameFaultsAnswerPerTable(t *testing.T) {
	t.Parallel()
	rig := newAttachmentFrameRig(t)
	failed := func(begins int64) attachmentWant { return attachmentWant{answer: answerFailed, begins: begins} }
	refused := func(begins int64) attachmentWant { return attachmentWant{answer: answerRefused, begins: begins} }
	refusedPending := func(begins int64) attachmentWant {
		return attachmentWant{answer: answerRefused, begins: begins, pending: true}
	}
	cases := []attachmentFaultCase{
		// BEGIN of the first unit: before the bundle the audience, now the
		// bundled handoff.
		{name: "BEGIN 1, cache empty", path: listOf, cache: "empty", arm: armBegin(0), old: failed(1), now: failed(1)},
		{name: "BEGIN 1, cache open", path: listOf, cache: "open", arm: armBegin(0), old: failed(1), now: failed(1)},
		{name: "BEGIN 1, cache closed", path: listOf, cache: "closed", arm: armBegin(0), old: failed(1), now: failed(1)},
		{name: "BEGIN 1, system", path: systemOf, cache: "empty", arm: armBegin(0), old: failed(1), now: failed(1)},
		// BEGIN of the second unit: before the bundle the switch unit (or the
		// row unit), now the check after the read.
		{name: "BEGIN 2, cache empty (E9/E15)", path: listOf, cache: "empty", arm: armBegin(1), old: refused(2), now: failed(2)},
		{name: "BEGIN 2, cache open", path: listOf, cache: "open", arm: armBegin(1), old: failed(2), now: failed(2)},
		{name: "BEGIN 2, cache closed", path: listOf, cache: "closed", arm: armBegin(1), old: refusedPending(1), now: refusedPending(1)},
		{name: "BEGIN 2, system", path: systemOf, cache: "empty", arm: armBegin(1), old: failed(2), now: failed(2)},
		// The school role switch: before the bundle in the switch (or row)
		// unit, now in the handoff.
		{name: "tenant set_config, cache empty (E9/E15)", path: listOf, cache: "empty", arm: armStatement(tenantRoleSwitch), old: refused(2), now: failed(1)},
		{name: "tenant set_config, cache open", path: listOf, cache: "open", arm: armStatement(tenantRoleSwitch), old: failed(2), now: failed(1)},
		{name: "tenant set_config, cache closed (E9/E15)", path: listOf, cache: "closed", arm: armStatement(tenantRoleSwitch), old: refusedPending(1), now: failed(1)},
		{name: "tenant set_config, system", path: systemOf, cache: "empty", arm: armStatement(tenantRoleSwitch), old: failed(2), now: failed(1)},
		// The administrative role switch of the first unit.
		{name: "admin role, cache empty", path: listOf, cache: "empty", arm: armStatement("SET LOCAL ROLE phoenix_admin"), old: failed(1), now: failed(1)},
		// The first COMMIT: before the bundle the audience's, now the
		// handoff's (the old switch unit's COMMIT failed closed, see COMMIT 2).
		{name: "COMMIT 1, cache empty", path: listOf, cache: "empty", arm: armCommit(0), old: failed(1), now: failed(1)},
		// The second COMMIT: before the bundle the switch (or row) unit's, now
		// the check after the read.
		{name: "COMMIT 2, cache empty (E9/E15)", path: listOf, cache: "empty", arm: armCommit(1), old: refused(2), now: failed(2)},
		{name: "COMMIT 2, cache open", path: listOf, cache: "open", arm: armCommit(1), old: failed(2), now: failed(2)},
		{name: "COMMIT 2, cache closed", path: listOf, cache: "closed", arm: armCommit(1), old: refusedPending(1), now: refusedPending(1)},
		{name: "COMMIT 2, system", path: systemOf, cache: "empty", arm: armCommit(1), old: failed(2), now: failed(2)},
		// A ROLLBACK that fails after a refusal is a failure of the unit.
		{name: "ROLLBACK after switch off, cache closed (E9/E15)", path: listOf, cache: "closed", arm: armRollback(0), old: refusedPending(1), now: failed(1)},
		{name: "ROLLBACK after negative audience (E9/E15)", path: listOf, stranger: true, cache: "empty", arm: armRollback(0), old: refusedPending(1), now: failed(1)},
		// File Storage keeps classifying its own not-found with a joined
		// rollback error, as before.
		{name: "ROLLBACK after missing row", path: missingRowOf, cache: "empty", arm: armRollback(0), old: refused(3), now: refused(1)},
		// A cancelled request after a refusal.
		{name: "cancel after negative audience (E4)", path: listOf, stranger: true, cache: "empty", arm: cancelAfter(audienceLast), old: failed(1), now: refused(1)},
		{name: "cancel after switch read, switch off", path: listOf, cache: "switch off", arm: cancelAfter(switchRead), old: refused(2), now: refused(1)},
	}
	for _, c := range cases {
		ctx := rig.contextFor(t, c.cache)
		ctx, cancel := context.WithCancel(ctx)
		token := rig.parentToken
		if c.stranger {
			token = rig.strangerToken
		}
		c.arm(rig.fault, cancel)
		got := rig.serveCancellable(t, ctx, c.path(rig), token)
		cancel()
		want := c.want()
		require.Equal(t, want.answer, got.answer, "%s: %s", c.name, strings.Join(got.statements, " | "))
		require.Equal(t, want.begins, got.begins, "%s: BEGINs (no retry, no second way): %s", c.name, frameShape(got.statements))
		require.Equal(t, want.pending, rig.fault.Pending(), "%s: injection fired", c.name)
		noWrites(t, got, c.name)
		rig.disarm()
	}
}

// contextFor is requestContext plus "switch off": the switch stored off and
// nothing cached.
func (r *attachmentFrameRig) contextFor(t *testing.T, cache string) context.Context {
	t.Helper()
	if cache != "switch off" {
		return r.requestContext(t, cache)
	}
	r.setSwitch(t, false)
	return settingsModule.WithSettingsRequestCache(testpkg.WithTestTenantRuntime(t, context.Background()))
}

// disarm drops an injection that did not fire, so the next case starts clean.
func (r *attachmentFrameRig) disarm() { r.fault.Reset() }

// serveCancellable is serve for a request whose context the test may cancel:
// only the deadline counts as hanging.
func (r *attachmentFrameRig) serveCancellable(t *testing.T, ctx context.Context, path, token string) attachmentAnswer {
	t.Helper()
	return r.serveWith(t, ctx, path, token, true)
}

type attachmentRouteCase struct {
	name     string
	path     func(r *attachmentFrameRig) string
	stranger bool
	cache    string
	revoke   string // a table the school role may not read for this case
	old, now [2]string
}

// TestParentAttachmentRoutesAnswerPerTable covers the table's rows on a
// healthy database: answer and units of every refusal, success and SQL
// failure, and that nothing is written.
func TestParentAttachmentRoutesAnswerPerTable(t *testing.T) {
	t.Parallel()
	rig := newAttachmentFrameRig(t)
	const (
		oldList  = "admin commit, tenant commit, tenant commit, admin commit"
		handoff  = "admin>tenant commit, admin commit"
		refusing = "admin>tenant rollback"
	)
	cases := []attachmentRouteCase{
		{name: "list", path: listOf, cache: "empty", old: [2]string{answerOK, oldList}, now: [2]string{answerOK, handoff}},
		{name: "download", path: func(r *attachmentFrameRig) string { return r.downloadPath(r.attachmentID) }, cache: "empty",
			old: [2]string{answerOK, oldList + ", tenant commit"}, now: [2]string{answerOK, handoff + ", tenant commit"}},
		{name: "system announcement", path: systemOf, cache: "empty", old: [2]string{answerOK, "admin commit, tenant commit, admin commit"}, now: [2]string{answerOK, handoff}},
		{name: "outside the audience", path: listOf, stranger: true, cache: "empty", old: [2]string{answerRefused, "admin commit"}, now: [2]string{answerRefused, "admin rollback"}},
		{name: "switch off", path: listOf, cache: "switch off", old: [2]string{answerRefused, "admin commit, tenant commit"}, now: [2]string{answerRefused, refusing}},
		{name: "switch off, cached", path: listOf, cache: "closed", old: [2]string{answerRefused, "admin commit"}, now: [2]string{answerRefused, refusing}},
		{name: "missing row", path: missingRowOf, cache: "empty", old: [2]string{answerRefused, "admin commit, tenant commit, tenant rollback"}, now: [2]string{answerRefused, refusing}},
		{name: "switch unreadable", path: listOf, cache: "empty", revoke: "config.setting_values", old: [2]string{answerRefused, "admin commit, tenant rollback"}, now: [2]string{answerRefused, refusing}},
		{name: "rows unreadable", path: listOf, cache: "empty", revoke: "documents.announcement_attachments", old: [2]string{answerFailed, "admin commit, tenant commit, tenant rollback"}, now: [2]string{answerFailed, refusing}},
	}
	for _, c := range cases {
		token := rig.parentToken
		if c.stranger {
			token = rig.strangerToken
		}
		ctx := rig.contextFor(t, c.cache)
		if c.revoke != "" {
			rig.revoke(t, c.revoke)
		}
		got := rig.serve(t, ctx, c.path(rig), token)
		if c.revoke != "" {
			rig.grant(t, c.revoke)
		}
		want := c.now
		if attachmentExpectation == "old" {
			want = c.old
		}
		require.Equal(t, want[0], got.answer, "%s: %s", c.name, strings.Join(got.statements, " | "))
		require.Equal(t, want[1], frameShape(got.statements), c.name)
		if c.revoke == "config.setting_values" {
			require.Equal(t, "ROLLBACK", afterFailure(got.statements, switchRead), "%s: an unreadable switch ends the unit", c.name)
		}
		noWrites(t, got, c.name)
	}
}

func (r *attachmentFrameRig) revoke(t *testing.T, table string) {
	t.Helper()
	_, err := r.db.NewRaw("REVOKE SELECT ON " + table + " FROM phoenix_tenant").Exec(context.Background())
	require.NoError(t, err)
}

func (r *attachmentFrameRig) grant(t *testing.T, table string) {
	t.Helper()
	_, err := r.db.NewRaw("GRANT SELECT ON " + table + " TO phoenix_tenant").Exec(context.Background())
	require.NoError(t, err)
}

// TestParentAttachmentUnitsLeaveNoScopeOnThePool: with a single pooled
// connection, the transaction after a committed and after a rolled-back
// attachment unit sees neither the school role nor the school.
func TestParentAttachmentUnitsLeaveNoScopeOnThePool(t *testing.T) {
	t.Parallel()
	rig := newAttachmentFrameRig(t)
	rig.pool.SetMaxOpenConns(1)
	rig.pool.SetMaxIdleConns(1)
	var login string
	require.NoError(t, rig.pool.NewRaw("SELECT current_user").Scan(context.Background(), &login))
	for name, cache := range map[string]string{"committed": "empty", "rolled back": "closed"} {
		got := rig.serve(t, rig.contextFor(t, cache), listOf(rig), rig.parentToken)
		require.NotEqual(t, answerFailed, got.answer, name)
		var role, school string
		require.NoError(t, rig.pool.RunInTx(context.Background(), nil, func(ctx context.Context, tx bun.Tx) error {
			return tx.NewRaw("SELECT current_user, coalesce(current_setting('app.current_tenant_id', true), '')").Scan(ctx, &role, &school)
		}), name)
		require.Equal(t, login, role, name)
		require.Empty(t, school, name)
	}
}

// TestParentAttachmentAmbientEntriesAnswerAsBefore: called inside an open
// unit or from a context bound to a school, the attachment list answers
// exactly as the unbundled way did (ADR 0161 point 7). The table is the
// answer before the bundle; it holds unchanged on both codes.
func TestParentAttachmentAmbientEntriesAnswerAsBefore(t *testing.T) {
	t.Parallel()
	rig := newAttachmentFrameRig(t)
	inUnit := func(open func(context.Context, func(context.Context) error) error) func(context.Context, func(context.Context)) {
		return func(ctx context.Context, run func(context.Context)) {
			require.NoError(t, open(ctx, func(txCtx context.Context) error { run(txCtx); return nil }))
		}
	}
	school := func(id int64) func(context.Context, func(context.Context) error) error {
		return func(ctx context.Context, fn func(context.Context) error) error {
			tid, err := tenant.NewTenantID(id)
			require.NoError(t, err)
			return tenant.WithinTenant(ctx, tid, fn)
		}
	}
	bound := func(id int64) func(context.Context, func(context.Context)) {
		return func(ctx context.Context, run func(context.Context)) { run(tenant.WithTenantID(ctx, id)) }
	}
	entries := map[string]func(context.Context, func(context.Context)){
		"none":          func(ctx context.Context, run func(context.Context)) { run(ctx) },
		"admin unit":    inUnit(tenant.WithinAdmin),
		"unit school A": inUnit(school(rig.schoolA)),
		"unit school B": inUnit(school(rig.schoolB)),
		"Träger unit": inUnit(func(ctx context.Context, fn func(context.Context) error) error {
			return tenant.WithinTraeger(ctx, rig.organization, fn)
		}),
		"bound school A": bound(rig.schoolA),
		"bound school B": bound(rig.schoolB),
	}
	// Columns: cache empty, open, closed, system announcement.
	want := map[string][4]string{
		"none":           {answerOK, answerOK, answerRefused, answerOK},
		"admin unit":     {answerRefused, answerFailed, answerRefused, answerFailed},
		"unit school A":  {answerFailed, answerFailed, answerFailed, answerFailed},
		"unit school B":  {answerFailed, answerFailed, answerFailed, answerFailed},
		"Träger unit":    {answerFailed, answerFailed, answerFailed, answerFailed},
		"bound school A": {answerOK, answerOK, answerRefused, answerOK},
		"bound school B": {answerRefused, answerRefused, answerRefused, answerFailed},
	}
	for name, enter := range entries {
		for i, cache := range []string{"empty", "open", "closed", "system"} {
			path := listOf(rig)
			if cache == "system" {
				cache, path = "empty", systemOf(rig)
			}
			var got attachmentAnswer
			enter(rig.contextFor(t, cache), func(ctx context.Context) { got = rig.serve(t, ctx, path, rig.parentToken) })
			require.Equal(t, want[name][i], got.answer, "%s, column %d: %s", name, i, strings.Join(got.statements, " | "))
			noWrites(t, got, name)
		}
	}
}
