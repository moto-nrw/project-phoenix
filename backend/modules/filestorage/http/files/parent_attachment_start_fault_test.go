//go:build testhooks

package files_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	rootAPI "github.com/moto-nrw/project-phoenix/api"
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

// The attachment list's answer when a school unit cannot start (ADR 0161
// step 3, #5894), for every state of the request's settings cache and every
// entry of ADR 0161 point 7. The expected answers are the order before the
// bundle: the switch from the cache or from a unit of its own (fail closed),
// then the row read in a unit of its own. So a cached open switch meets the
// row read's start error (500), an empty cache meets it at the switch and
// hides the announcement (404), a cached closed switch never starts a school
// unit (404). The audience runs in an administrative unit first, which a
// school unit cannot join (500).

const newsSwitch = "operations.parent_news_enabled"

type startFaultRig struct {
	db             *bun.DB
	router         chi.Router
	settings       owners.SettingsTestModule
	announcementID int64
	parentToken    string
	schoolA        int64
	schoolB        int64
	fault          *startFault
}

// startFault makes the next unit after the audience wait for the pool's only
// connection, which a waiting goroutine takes first and keeps until release.
type startFault struct {
	db    *bun.DB
	armed bool
	held  chan *bun.Conn
}

func (f *startFault) arm() {
	if !f.armed {
		return
	}
	f.armed = false
	waits := f.db.Stats().WaitCount
	go func() {
		conn, err := f.db.Conn(context.Background())
		if err != nil {
			f.held <- nil
			return
		}
		f.held <- &conn
	}()
	for f.db.Stats().WaitCount == waits {
		time.Sleep(time.Millisecond)
	}
}

func (f *startFault) release() {
	if conn := <-f.held; conn != nil {
		_ = conn.Close()
	}
}

func newStartFaultRig(t *testing.T) startFaultRig {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	database := testpkg.NewDatabase(t, db)
	unit := testpkg.TenantRuntime(t, db)
	store, err := objects.PrivateUploadsBackend()
	require.NoError(t, err)
	files, _, err := rootAPI.NewFileStoreTestModule(database, unit, store)
	require.NoError(t, err)
	settings, err := owners.NewSettingsTestModule(database, unit)
	require.NoError(t, err)
	require.NoError(t, settings.Settings.SetValue(ctx, newsSwitch, true, nil, nil))
	repos := rootAPI.NewParentMessagingSuiteRepositories(database)
	portal, err := rootAPI.NewParentCareScheduleTestService(database, rootAPI.StudentTestModule{}, rootAPI.ActiveTestModule{}, rootAPI.ActiveSettings{Settings: settings.Settings})
	require.NoError(t, err)
	fault := &startFault{db: db, held: make(chan *bun.Conn, 1)}
	portal.AnnouncementRepo = owners.ParentAnnouncementAccessAfterAudience(owners.BindParentAnnouncementAccess(repos.ParentAnnouncement), fault.arm)
	reads, err := filestorage.TenantReads(files)
	require.NoError(t, err)
	guardian, err := owners.NewParentGuardianAttachments(portal, reads)
	require.NoError(t, err)
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Use(rootAPI.RequestSettingsCacheMiddleware, rootAPI.RequestIdentityCacheMiddleware)
	router.Mount("/parent-news-attachments", filesHTTP.NewResource(files, slog.Default()).ParentAnnouncementAttachmentRouter(rootAPI.ProtectedParentGroup, guardian))

	_, admin := testpkg.CreateTestStaffWithAccount(t, db, "Start", "Fault")
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	announcement := testpkg.CreateTestParentAnnouncement(t, db, admin.ID, "Start fault")
	_, err = db.NewRaw(`INSERT INTO users.parent_announcement_targets (tenant_id,announcement_id,target_type,target_ref_id) VALUES (?,?,'student',?)`, testpkg.Tenant(t), announcement.ID, chain.StudentID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE users.parent_announcements SET published_at = ? WHERE id = ?`, time.Now().Add(-time.Hour), announcement.ID).Exec(ctx)
	require.NoError(t, err)
	rig := startFaultRig{
		db: db, router: router, settings: settings, announcementID: announcement.ID, schoolA: testpkg.Tenant(t), fault: fault,
		parentToken: communication18AttachmentToken(t, chain.AccountID, chain.Email, 0, "parent", []string{"parent"}, nil),
	}
	t.Run("school B", func(t *testing.T) {
		rig.schoolB = testpkg.OwnTenant(t)
		testpkg.EnsureTestTenant(t, db, rig.schoolB)
	})
	return rig
}

// answer serves the parent attachment list in ctx and returns status and code.
func (r startFaultRig) answer(t *testing.T, ctx context.Context) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/parent-news-attachments/%d", r.announcementID), nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer "+r.parentToken)
	rec := httptest.NewRecorder()
	r.router.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		return "200"
	}
	var body struct{ Code string }
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	return fmt.Sprintf("%d %s", rec.Code, body.Code)
}

func TestParentAttachmentStartFaultsAnswerInTheOrderBeforeTheBundle(t *testing.T) {
	t.Parallel()
	rig := newStartFaultRig(t)
	rig.db.SetMaxIdleConns(1)
	rig.db.SetMaxOpenConns(1)

	const ok, refused, failed = "200", "404 general.input", "500 general.server"
	type cell struct{ cache, entry string }
	want := map[bool]map[cell]string{
		false: { // the school unit starts unless an entry prevents it
			{"empty", "none"}: ok, {"open", "none"}: ok, {"closed", "none"}: refused,
			{"empty", "admin"}: refused, {"open", "admin"}: failed, {"closed", "admin"}: refused,
			{"empty", "own school"}: failed, {"open", "own school"}: failed, {"closed", "own school"}: failed,
			{"empty", "other school"}: failed, {"open", "other school"}: failed, {"closed", "other school"}: failed,
		},
		true: { // the unit after the audience cannot start
			{"empty", "none"}: refused, {"open", "none"}: failed, {"closed", "none"}: refused,
			{"empty", "admin"}: refused, {"open", "admin"}: failed, {"closed", "admin"}: refused,
			{"empty", "own school"}: failed, {"open", "own school"}: failed, {"closed", "own school"}: failed,
			{"empty", "other school"}: failed, {"open", "other school"}: failed, {"closed", "other school"}: failed,
		},
	}
	entries := map[string]func(context.Context, func(context.Context)){
		"none": func(ctx context.Context, run func(context.Context)) { run(ctx) },
		"admin": func(ctx context.Context, run func(context.Context)) {
			require.NoError(t, tenant.WithinAdmin(ctx, func(txCtx context.Context) error { run(txCtx); return nil }))
		},
		"own school":   rig.inSchool(t, rig.schoolA),
		"other school": rig.inSchool(t, rig.schoolB),
	}
	for _, faulted := range []bool{false, true} {
		for _, cache := range []string{"empty", "open", "closed"} {
			for entryName, enter := range entries {
				name := fmt.Sprintf("fault=%t cache=%s entry=%s", faulted, cache, entryName)
				ctx := rig.cachedContext(t, cache)
				var got string
				rig.fault.armed = faulted
				callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				enter(callCtx, func(ctx context.Context) { got = rig.answer(t, ctx) })
				cancel()
				if faulted && !rig.fault.armed {
					rig.fault.release()
				}
				rig.fault.armed = false
				require.Equal(t, want[faulted][cell{cache, entryName}], got, name)
			}
		}
	}
}

// cachedContext is a request context whose settings cache holds the school's
// switch open, closed or nothing. The stored value equals the cached one.
func (r startFaultRig) cachedContext(t *testing.T, state string) context.Context {
	t.Helper()
	ctx := settingsModule.WithSettingsRequestCache(testpkg.WithTestTenantRuntime(t, context.Background()))
	stored := state != "closed"
	school := tenant.WithTenantID(testpkg.WithTestTenantRuntime(t, context.Background()), r.schoolA)
	require.NoError(t, r.settings.Settings.SetValue(school, newsSwitch, stored, nil, nil))
	if state != "empty" {
		on, err := r.settings.Settings.ResolveBoolForTenant(ctx, r.schoolA, newsSwitch)
		require.NoError(t, err)
		require.Equal(t, stored, on)
	}
	return ctx
}

func (r startFaultRig) inSchool(t *testing.T, id int64) func(context.Context, func(context.Context)) {
	t.Helper()
	return func(ctx context.Context, run func(context.Context)) {
		school, err := tenant.NewTenantID(id)
		require.NoError(t, err)
		require.NoError(t, tenant.WithinTenant(ctx, school, func(txCtx context.Context) error { run(txCtx); return nil }))
	}
}
