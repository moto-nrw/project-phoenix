package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	portalCompose "github.com/moto-ogs/project-phoenix/workflows/parentportal/compose"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The parent child write contract of ADR 0161 (Nachtrag Schreibrouten,
// #5862), observed over HTTP on a real database: request sharing (A), the
// master-data request edit (B) and the photo consent grant (C) and withdrawal
// (D). Behaviour assertions (status, code, persistence, after-commit work)
// hold on the code before the handoff as well; the frame-shape and fault
// tests name the one transaction explicitly.

// writeDeadline bounds every request of these tests.
const writeDeadline = 20 * time.Second

// photoUnlinks records each photo the withdrawal schedules for deletion and
// whether the withdrawal was committed when the deletion ran.
type photoUnlinks struct {
	mu   sync.Mutex
	db   *bun.DB
	runs []photoUnlink
}

type photoUnlink struct {
	path      string
	committed bool
}

// ScheduleUnlinkAfterCommit is the production contract: the file goes after
// the commit, never before and never after a rollback.
func (p *photoUnlinks) ScheduleUnlinkAfterCommit(ctx context.Context, path string) error {
	tenant.RegisterAfterCommit(ctx, func() {
		// Another connection sees the photo gone only once the withdrawal
		// committed.
		var holders int
		err := p.db.NewRaw(`SELECT count(*) FROM users.student_profiles WHERE photo_path = ?`, path).Scan(context.Background(), &holders)
		p.mu.Lock()
		defer p.mu.Unlock()
		p.runs = append(p.runs, photoUnlink{path: path, committed: err == nil && holders == 0})
	})
	return nil
}

func (p *photoUnlinks) all() []photoUnlink {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]photoUnlink(nil), p.runs...)
}

// writeRig is the read rig with the photo deletion observed.
type writeRig struct {
	childReadRig
	photos *photoUnlinks
}

func newWriteRig(t *testing.T) writeRig {
	t.Helper()
	return newWriteRigOn(t, testpkg.SetupIsolatedTestDB(t))
}

func newWriteRigOn(t *testing.T, db *bun.DB) writeRig {
	t.Helper()
	photos := &photoUnlinks{db: db}
	rig := newChildReadRigOn(t, db, func(deps *portalCompose.Dependencies) { deps.StudentPhotos = photos })
	return writeRig{childReadRig: rig, photos: photos}
}

// childWrite is one write request.
type childWrite struct {
	method, path string
	body         any
}

func (r writeRig) send(t *testing.T, w childWrite, token string) (*httptest.ResponseRecorder, []string) {
	t.Helper()
	counter := testpkg.CaptureQueries(t, r.db)
	rec := r.serveWrite(t, context.Background(), w, token)
	queries := counter.Queries()
	counter.Stop()
	return rec, withoutProbe(queries)
}

// withoutProbe drops the photo-deletion probe, which runs on its own
// connection after the commit.
func withoutProbe(queries []string) []string {
	out := queries[:0:0]
	for _, q := range queries {
		if !strings.HasPrefix(q, "SELECT count(*) FROM users.student_profiles WHERE photo_path") {
			out = append(out, q)
		}
	}
	return out
}

// serveWrite answers w within writeDeadline; ctx may carry a cancellation.
func (r writeRig) serveWrite(t *testing.T, ctx context.Context, w childWrite, token string) *httptest.ResponseRecorder {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, writeDeadline)
	defer cancel()
	req := testutil.NewAuthenticatedRequest(t, w.method, w.path, w.body, testutil.WithJWTBearer(token)).WithContext(ctx)
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- testutil.ExecuteRequest(r.router, req) }()
	select {
	case rec := <-done:
		return rec
	case <-time.After(writeDeadline + 5*time.Second):
		t.Fatalf("%s %s did not answer in time", w.method, w.path)
		return nil
	}
}

func sharingWrite(studentID, requestID int64, recipients ...int64) childWrite {
	ids := make([]string, 0, len(recipients))
	for _, id := range recipients {
		ids = append(ids, fmt.Sprint(id))
	}
	return childWrite{http.MethodPut, fmt.Sprintf("/parent/me/children/%d/request-sharing/master_data/%d", studentID, requestID), map[string]any{"recipient_guardian_profile_ids": ids}}
}

func masterDataEdit(studentID, requestID int64, value, version string) childWrite {
	raw, _ := json.Marshal(value)
	return childWrite{http.MethodPut, fmt.Sprintf("/parent/me/children/%d/master-data/requests/%d", studentID, requestID), map[string]any{"new_value": json.RawMessage(raw), "expected_version": version}}
}

func photoConsent(method string, studentID int64) childWrite {
	return childWrite{method, fmt.Sprintf("/parent/me/children/%d/consents/photo", studentID), nil}
}

// writeState is what the four routes persist for one family.
type writeState struct {
	Shares       int
	Ledger       int
	Value        string
	Status       string
	PhotoPath    *string
	PhotoGivenAt *time.Time
	ConsentRows  int
}

func (r writeRig) state(t *testing.T, f childReadFamily) writeState {
	t.Helper()
	s, err := r.stateOf(f)
	require.NoError(t, err)
	return s
}

// stateOf reads the persisted state without failing the test, for polls.
func (r writeRig) stateOf(f childReadFamily) (writeState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var s writeState
	for _, read := range []func() error{
		func() error {
			return r.db.NewRaw(`SELECT count(*) FROM users.parent_request_share_events WHERE request_type = 'master_data' AND request_id = ?`, f.requestID).Scan(ctx, &s.Shares)
		},
		func() error {
			return r.db.NewRaw(`SELECT count(*) FROM users.parent_request_events WHERE request_type = 'master_data' AND request_id = ?`, f.requestID).Scan(ctx, &s.Ledger)
		},
		func() error {
			return r.db.NewRaw(`SELECT new_value #>> '{}', status FROM users.student_data_change_requests WHERE id = ?`, f.requestID).Scan(ctx, &s.Value, &s.Status)
		},
		func() error {
			return r.db.NewRaw(`SELECT photo_path, photo_consent_given_at FROM users.student_profiles WHERE id = ?`, f.StudentID).Scan(ctx, &s.PhotoPath, &s.PhotoGivenAt)
		},
		func() error {
			return r.db.NewRaw(`SELECT count(*) FROM audit.student_consent_changes WHERE student_id = ?`, f.StudentID).Scan(ctx, &s.ConsentRows)
		},
	} {
		if err := read(); err != nil {
			return writeState{}, err
		}
	}
	return s, nil
}

// unchanged polls until the persisted state equals want: a connection the
// pool dropped may still hold its transaction for a moment.
func (r writeRig) unchanged(t *testing.T, f childReadFamily, want writeState, msg string) {
	t.Helper()
	require.Eventually(t, func() bool {
		got, err := r.stateOf(f)
		return err == nil && equalState(got, want)
	}, 10*time.Second, 50*time.Millisecond, msg)
}

// givePhotoConsent stores a granted photo consent with a photo.
func (r writeRig) givePhotoConsent(t *testing.T, f childReadFamily) {
	t.Helper()
	_, err := r.db.NewRaw(`UPDATE users.student_profiles SET photo_path = ?, photo_consent_given_at = now(), photo_consent_given_by = ? WHERE id = ?`,
		fmt.Sprintf("students/%d/photo.jpg", f.StudentID), f.AccountID, f.StudentID).Exec(context.Background())
	require.NoError(t, err)
}

func (r writeRig) revoke(t *testing.T, f childReadFamily, permission string) {
	t.Helper()
	_, err := r.db.NewRaw(`UPDATE auth.guardian_student_access SET permissions = permissions - ? WHERE account_id = ?`, permission, f.AccountID).Exec(context.Background())
	require.NoError(t, err)
}

func answer(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	if rec.Code == http.StatusOK {
		return "200"
	}
	return fmt.Sprintf("%d %s", rec.Code, childReadCode(t, rec))
}

// TestParentChildWritesAnswerLikeBefore walks the rows of the invariant
// table in #6394: each refusal answers with its status and code and persists
// nothing, each success persists exactly its write.
func TestParentChildWritesAnswerLikeBefore(t *testing.T) {
	t.Parallel()
	rig := newWriteRig(t)
	other := rig.family(t)

	t.Run("A request sharing", func(t *testing.T) {
		f := rig.family(t)
		before := rig.state(t, f)
		unknownType := childWrite{http.MethodPut, fmt.Sprintf("/parent/me/children/%d/request-sharing/bogus/%d", f.StudentID, f.requestID), map[string]any{"recipient_guardian_profile_ids": []string{}}}
		rec, queries := rig.send(t, unknownType, f.token)
		require.Equal(t, "400 care.request_sharing_invalid", answer(t, rec))
		require.Empty(t, queries, "the type is refused before any unit")

		for name, c := range map[string]struct {
			write childWrite
			want  string
		}{
			"foreign child":     {sharingWrite(other.StudentID, other.requestID), "404 care.request_not_found"},
			"foreign request":   {sharingWrite(f.StudentID, other.requestID), "404 care.request_not_found"},
			"foreign recipient": {sharingWrite(f.StudentID, f.requestID, other.StudentID+1_000_000), "400 care.request_sharing_invalid"},
		} {
			rec, _ := rig.send(t, c.write, f.token)
			require.Equal(t, c.want, answer(t, rec), name)
			require.Equal(t, before, rig.state(t, f), name)
		}

		co := testpkg.CreateTestCoGuardianForStudent(t, rig.db, f.StudentID, "Jonas", "Teiler")
		var coProfile int64
		require.NoError(t, rig.db.NewRaw(`SELECT id FROM users.guardian_profiles WHERE account_id = ?`, co.AccountID).Scan(context.Background(), &coProfile))
		_, err := rig.db.NewRaw(`INSERT INTO users.student_family_protection_events (tenant_id, student_id, enabled, reason, actor_account_id) VALUES (?, ?, true, 'Schutz', ?)`, f.TenantID, f.StudentID, f.AccountID).Exec(context.Background())
		require.NoError(t, err)
		rec, _ = rig.send(t, sharingWrite(f.StudentID, f.requestID, coProfile), f.token)
		require.Equal(t, "403 care.family_protection", answer(t, rec))
		require.Equal(t, before, rig.state(t, f))
		_, err = rig.db.NewRaw(`INSERT INTO users.student_family_protection_events (tenant_id, student_id, enabled, reason, actor_account_id) VALUES (?, ?, false, 'Ende', ?)`, f.TenantID, f.StudentID, f.AccountID).Exec(context.Background())
		require.NoError(t, err)

		rec, _ = rig.send(t, sharingWrite(f.StudentID, f.requestID, coProfile), f.token)
		require.Equal(t, "200", answer(t, rec), rec.Body.String())
		after := rig.state(t, f)
		require.Equal(t, before.Shares+1, after.Shares)
		require.Equal(t, before.Ledger+1, after.Ledger)

		rig.revoke(t, f, "parent_portal.master_data.request")
		rec, _ = rig.send(t, sharingWrite(f.StudentID, f.requestID), f.token)
		require.Equal(t, "403 care.guardian_permission_denied", answer(t, rec))
		require.Equal(t, after, rig.state(t, f))
	})

	t.Run("B master-data request edit", func(t *testing.T) {
		f := rig.family(t)
		before := rig.state(t, f)
		for name, c := range map[string]struct {
			write childWrite
			want  string
		}{
			"foreign child":   {masterDataEdit(other.StudentID, other.requestID, "Paul", ""), "403 care.child_not_linked"},
			"foreign request": {masterDataEdit(f.StudentID, other.requestID, "Paul", ""), "404 care.request_not_found"},
			"stale version":   {masterDataEdit(f.StudentID, f.requestID, "Paul", "1"), "409 students.change_request_stale"},
			"no change":       {masterDataEdit(f.StudentID, f.requestID, "Felix", ""), "400 care.master_data_no_changes"},
		} {
			rec, _ := rig.send(t, c.write, f.token)
			require.Equal(t, c.want, answer(t, rec), name)
			require.Equal(t, before, rig.state(t, f), name)
		}

		rec, _ := rig.send(t, masterDataEdit(f.StudentID, f.requestID, "Paul", ""), f.token)
		require.Equal(t, "200", answer(t, rec), rec.Body.String())
		after := rig.state(t, f)
		require.Equal(t, "Paul", after.Value)
		require.Equal(t, before.Ledger+1, after.Ledger)

		_, err := rig.db.NewRaw(`UPDATE users.student_data_change_requests SET status = 'rejected' WHERE id = ?`, f.requestID).Exec(context.Background())
		require.NoError(t, err)
		decided := rig.state(t, f)
		rec, _ = rig.send(t, masterDataEdit(f.StudentID, f.requestID, "Emil", ""), f.token)
		require.Equal(t, "409 care.request_not_open", answer(t, rec))
		require.Equal(t, decided, rig.state(t, f))

		ended := calendar.NewDate(2026, 8, 1)
		testpkg.SetStudentLifecycle(t, rig.db, f.StudentID, "active", nil, &ended)
		rec, _ = rig.send(t, masterDataEdit(f.StudentID, f.requestID, "Emil", ""), f.token)
		require.Equal(t, "403 care.child_care_ended", answer(t, rec))

		g := rig.family(t)
		rig.revoke(t, g, "parent_portal.master_data.request")
		rec, _ = rig.send(t, masterDataEdit(g.StudentID, g.requestID, "Paul", ""), g.token)
		require.Equal(t, "403 care.guardian_permission_denied", answer(t, rec))
	})

	t.Run("C and D photo consent", func(t *testing.T) {
		f := rig.family(t)
		before := rig.state(t, f)
		rec, _ := rig.send(t, photoConsent(http.MethodPut, f.StudentID), f.token)
		require.Equal(t, "409 care.photo_consent_not_withdrawn", answer(t, rec))
		require.Equal(t, before, rig.state(t, f))
		for _, method := range []string{http.MethodPut, http.MethodDelete} {
			rec, _ := rig.send(t, photoConsent(method, other.StudentID), f.token)
			require.Equal(t, "403 care.child_not_linked", answer(t, rec), method)
		}

		rig.givePhotoConsent(t, f)
		given := rig.state(t, f)
		rec, _ = rig.send(t, photoConsent(http.MethodDelete, f.StudentID), f.token)
		require.Equal(t, "200", answer(t, rec), rec.Body.String())
		withdrawn := rig.state(t, f)
		require.Nil(t, withdrawn.PhotoPath)
		require.Nil(t, withdrawn.PhotoGivenAt)
		require.Greater(t, withdrawn.ConsentRows, given.ConsentRows)
		require.Equal(t, []photoUnlink{{path: *given.PhotoPath, committed: true}}, rig.photos.forPath(*given.PhotoPath))

		rec, _ = rig.send(t, photoConsent(http.MethodDelete, f.StudentID), f.token)
		require.Equal(t, "200", answer(t, rec), "a repeated withdrawal changes nothing")
		require.Equal(t, withdrawn, rig.state(t, f))

		rec, _ = rig.send(t, photoConsent(http.MethodPut, f.StudentID), f.token)
		require.Equal(t, "200", answer(t, rec), rec.Body.String())
		granted := rig.state(t, f)
		require.NotNil(t, granted.PhotoGivenAt)
		require.Greater(t, granted.ConsentRows, withdrawn.ConsentRows)

		g := rig.family(t)
		rig.revoke(t, g, "parent_portal.consent.manage")
		for _, method := range []string{http.MethodPut, http.MethodDelete} {
			rec, _ := rig.send(t, photoConsent(method, g.StudentID), g.token)
			require.Equal(t, "403 care.guardian_permission_denied", answer(t, rec), method)
		}
		// #6494, decision A: the end of care does not end the photo consent.
		// The withdrawal clears it with one history row and the file after the
		// commit; giving it again stays refused.
		ended := calendar.NewDate(2026, 8, 1)
		h := rig.family(t)
		rig.givePhotoConsent(t, h)
		testpkg.SetStudentLifecycle(t, rig.db, h.StudentID, "active", nil, &ended)
		held := rig.state(t, h)
		rec, _ = rig.send(t, photoConsent(http.MethodDelete, h.StudentID), h.token)
		require.Equal(t, "200", answer(t, rec), rec.Body.String())
		endedWithdrawn := rig.state(t, h)
		require.Nil(t, endedWithdrawn.PhotoPath)
		require.Nil(t, endedWithdrawn.PhotoGivenAt)
		require.Equal(t, held.ConsentRows+1, endedWithdrawn.ConsentRows)
		require.Equal(t, []photoUnlink{{path: *held.PhotoPath, committed: true}}, rig.photos.forPath(*held.PhotoPath))
		rec, _ = rig.send(t, photoConsent(http.MethodPut, h.StudentID), h.token)
		require.Equal(t, "403 care.child_care_ended", answer(t, rec))
		require.Equal(t, endedWithdrawn, rig.state(t, h))
	})
}

func (p *photoUnlinks) forPath(path string) []photoUnlink {
	var out []photoUnlink
	for _, run := range p.all() {
		if run.path == path {
			out = append(out, run)
		}
	}
	return out
}

// childWrites are one successful request per route for a family whose photo
// consent is given: D withdraws it, C grants it again.
func childWrites(f childReadFamily) []struct {
	name  string
	write childWrite
} {
	return []struct {
		name  string
		write childWrite
	}{
		{"A request sharing", sharingWrite(f.StudentID, f.requestID)},
		{"B master-data request edit", masterDataEdit(f.StudentID, f.requestID, "Paul", "")},
		{"D photo consent withdrawal", photoConsent(http.MethodDelete, f.StudentID)},
		{"C photo consent grant", photoConsent(http.MethodPut, f.StudentID)},
	}
}

func isRowLock(q string) bool {
	upper := strings.ToUpper(q)
	return strings.Contains(upper, " FOR UPDATE") || strings.Contains(upper, " FOR SHARE") || strings.Contains(upper, " FOR NO KEY UPDATE") || strings.Contains(upper, " FOR KEY SHARE") || strings.Contains(upper, "PG_ADVISORY")
}

// TestParentChildWritesAreOneTransaction: each write checks the child and
// writes in one transaction. The administrative part locks no row, and the
// first lock follows the switch to the school.
func TestParentChildWritesAreOneTransaction(t *testing.T) {
	t.Parallel()
	rig := newWriteRig(t)
	f := rig.family(t)
	rig.givePhotoConsent(t, f)

	for _, w := range childWrites(f) {
		rec, queries := rig.send(t, w.write, f.token)
		require.Equal(t, http.StatusOK, rec.Code, "%s: %s", w.name, rec.Body.String())
		t.Logf("statements %s: %d", w.name, len(queries))
		require.Equal(t, "BEGIN", queries[0], w.name)
		require.True(t, isChildAdminRole(queries[1]), "%s: %q", w.name, queries)
		require.Equal(t, 1, childStatementCount(queries, func(q string) bool { return q == "BEGIN" }), "%s: one transaction: %q", w.name, queries)
		require.Equal(t, 1, childStatementCount(queries, isChildAdminRole), "%s: %q", w.name, queries)
		require.Equal(t, "COMMIT", queries[len(queries)-1], w.name)
		require.Equal(t, 1, childStatementCount(queries, func(q string) bool { return q == "COMMIT" }), "%s: %q", w.name, queries)
		for _, q := range discovery(queries) {
			require.False(t, isRowLock(q), "%s: the administrative part locks nothing: %q", w.name, q)
		}
		switchAt := childStatementIndex(queries, isChildTenantSwitch)
		lockAt := childStatementIndex(queries, isRowLock)
		require.Positive(t, switchAt, w.name)
		require.Greater(t, lockAt, switchAt, "%s: %q", w.name, queries)
	}

	// A refusal between the parts rolls the one transaction back before any
	// statement of the school.
	g := rig.family(t)
	ended := calendar.NewDate(2026, 8, 1)
	testpkg.SetStudentLifecycle(t, rig.db, g.StudentID, "active", nil, &ended)
	rec, queries := rig.send(t, masterDataEdit(g.StudentID, g.requestID, "Paul", ""), g.token)
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Equal(t, "ROLLBACK", queries[len(queries)-1], "%q", queries)
	require.Equal(t, -1, childStatementIndex(queries, isChildTenantSwitch), "%q", queries)
}

// TestParentChildWritesStatementCounts logs the statements of each write; it
// runs unchanged on the code before the handoff for the measurement in #6394.
func TestParentChildWritesStatementCounts(t *testing.T) {
	t.Parallel()
	rig := newWriteRig(t)
	f := rig.family(t)
	rig.givePhotoConsent(t, f)
	for _, w := range childWrites(f) {
		rec, queries := rig.send(t, w.write, f.token)
		require.Equal(t, http.StatusOK, rec.Code, "%s: %s", w.name, rec.Body.String())
		t.Logf("statements %s: %d", w.name, len(queries))
	}
}

// TestParentChildWritesFailTheirFrameOnce injects a real failure into the
// unit's own frame: a dropped connection at BEGIN, at the switch to the
// school and at COMMIT. Each answers 500 once, starts no second unit and
// leaves nothing persisted.
func TestParentChildWritesFailTheirFrameOnce(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	sqlDB, fault := testpkg.OpenPostgresSQLWithBeginFault(testpkg.IsolatedTestDSN(t))
	db := testpkg.NewBunDB(sqlDB)
	t.Cleanup(func() { _ = db.Close() })
	rig := newWriteRigOn(t, db)

	for _, point := range []struct {
		name string
		arm  func()
	}{
		{"BEGIN", fault.Arm},
		{"switch to the school", func() { fault.ArmStatement("SELECT set_config('role'", 0) }},
		{"COMMIT", func() { fault.ArmCommit(0) }},
	} {
		f := rig.family(t)
		rig.givePhotoConsent(t, f)
		for _, w := range childWrites(f) {
			before := rig.state(t, f)
			begins := fault.Begins()
			point.arm()
			rec := rig.serveWrite(t, context.Background(), w.write, f.token)
			require.False(t, fault.Pending(), "%s %s: the injection fired", point.name, w.name)
			require.Equal(t, "500 general.server", answer(t, rec), "%s %s: %s", point.name, w.name, rec.Body.String())
			require.EqualValues(t, 1, fault.Begins()-begins, "%s %s: exactly one BEGIN", point.name, w.name)
			rig.unchanged(t, f, before, point.name+" "+w.name+": nothing persisted")
			// The write itself still succeeds afterwards, so the next route
			// starts from the state the success table expects.
			ok := rig.serveWrite(t, context.Background(), w.write, f.token)
			require.Equal(t, http.StatusOK, ok.Code, "%s %s: %s", point.name, w.name, ok.Body.String())
		}
	}
}

func equalState(a, b writeState) bool {
	ra, _ := json.Marshal(a)
	rb, _ := json.Marshal(b)
	return string(ra) == string(rb)
}

// TestParentChildWritesKeepTheirAnswerWhenTheRollbackFails: a refusal whose
// ROLLBACK fails answers with the refusal, as the adapter joins the rollback
// error to it; nothing is persisted.
func TestParentChildWritesKeepTheirAnswerWhenTheRollbackFails(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	sqlDB, fault := testpkg.OpenPostgresSQLWithBeginFault(testpkg.IsolatedTestDSN(t))
	db := testpkg.NewBunDB(sqlDB)
	t.Cleanup(func() { _ = db.Close() })
	rig := newWriteRigOn(t, db)
	f := rig.family(t)
	other := rig.family(t)
	before := rig.state(t, f)

	for _, c := range []struct {
		write childWrite
		want  string
	}{
		{sharingWrite(f.StudentID, other.requestID), "404 care.request_not_found"},
		{masterDataEdit(f.StudentID, f.requestID, "Paul", "1"), "409 students.change_request_stale"},
		{photoConsent(http.MethodPut, f.StudentID), "409 care.photo_consent_not_withdrawn"},
	} {
		fault.ArmRollback(0)
		rec := rig.serveWrite(t, context.Background(), c.write, f.token)
		require.False(t, fault.Pending(), c.write.path)
		require.Equal(t, c.want, answer(t, rec), c.write.path)
		rig.unchanged(t, f, before, c.write.path)
	}
}

// TestParentChildWritesPersistNothingWhenCancelledUnderTheLock cancels the
// request right after the child's row lock: the write answers 500 and
// persists nothing.
func TestParentChildWritesPersistNothingWhenCancelledUnderTheLock(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	sqlDB, fault := testpkg.OpenPostgresSQLWithBeginFault(testpkg.IsolatedTestDSN(t))
	db := testpkg.NewBunDB(sqlDB)
	t.Cleanup(func() { _ = db.Close() })
	rig := newWriteRigOn(t, db)
	f := rig.family(t)
	rig.givePhotoConsent(t, f)

	for _, w := range childWrites(f) {
		before := rig.state(t, f)
		ctx, cancel := context.WithCancel(context.Background())
		fault.AfterStatement(lockStatementPrefix, cancel)
		rec := rig.serveWrite(t, ctx, w.write, f.token)
		cancel()
		require.False(t, fault.Pending(), "%s: the lock statement ran", w.name)
		require.Equal(t, http.StatusInternalServerError, rec.Code, "%s: %s", w.name, rec.Body.String())
		rig.unchanged(t, f, before, w.name)
		ok := rig.serveWrite(t, context.Background(), w.write, f.token)
		require.Equal(t, http.StatusOK, ok.Code, "%s: %s", w.name, ok.Body.String())
	}
}

// lockStatementPrefix starts the statement that takes the class-writes gate
// and the child's row lock, the first lock of every route here.
const lockStatementPrefix = "WITH class_write_gate"
