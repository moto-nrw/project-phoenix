package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
	portalCompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// Issue #6529: every parent write for an existing child holds the guardian's
// school membership, relationship, portal access and action permissions
// until it commits. For each write route these tests park the request after
// its check before the unit of work, withdraw one of these facts through the
// owner's regular path and require the write to refuse without changing a
// row of the child, the guardian profile or the account. The other direction
// parks a write that holds the guard's locks and requires the withdrawal to
// wait for its commit. Sleeps are not used; the tests wait on channels and on
// pg_blocking_pids.

// guardPortalAccess is the portal access every parent write needs.
const guardPortalAccess = "parent_portal.access"

// guardedWrite is one parent write route with what it needs to succeed.
type guardedWrite struct {
	name string
	// permissions are the action permissions the route needs besides portal
	// access; each is withdrawn on its own.
	permissions []string
	// prepare arranges what the write needs and returns the request.
	prepare func(t *testing.T, r *guardRig, f childReadFamily) childWrite
}

// guardRig is the parents portal over the read rig's composition, with a
// gate after the child check and a hook after the guard's relationship lock.
type guardRig struct {
	childReadRig
	people   peopledirectory.GuardianRelationshipCommand
	settings childReadSettings
	checked  *requestGate
	locked   *lockGate
}

// requestGate parks the first child check after it decided, while armed.
type requestGate struct {
	armed   atomic.Bool
	once    sync.Once
	reached chan struct{}
	release chan struct{}
}

func newRequestGate() *requestGate {
	return &requestGate{reached: make(chan struct{}), release: make(chan struct{})}
}

type gatedChildReads struct {
	parentportal.ChildReads
	gate *requestGate
}

func (r gatedChildReads) FindForAccount(ctx context.Context, accountID, studentID int64) (*parentportal.ChildSummary, error) {
	child, err := r.ChildReads.FindForAccount(ctx, accountID, studentID)
	if r.gate.armed.Load() {
		r.gate.once.Do(func() {
			close(r.gate.reached)
			<-r.gate.release
		})
	}
	return child, err
}

// lockGate parks the first relationship lock while armed and reports the
// backend that holds it.
type lockGate struct {
	armed   atomic.Bool
	once    sync.Once
	holder  chan lockHolder
	release chan struct{}
}

type lockHolder struct {
	pid int
	err error
}

func newLockGate() *lockGate {
	return &lockGate{holder: make(chan lockHolder, 1), release: make(chan struct{})}
}

func (g *lockGate) afterLock(ctx context.Context) {
	if !g.armed.Load() {
		return
	}
	g.once.Do(func() {
		var held lockHolder
		if tx, ok := tenant.TransactionFromContext(ctx); !ok {
			held.err = errors.New("the relationship lock runs outside the school transaction")
		} else {
			held.err = tx.(bun.IDB).NewRaw("SELECT pg_backend_pid()").Scan(ctx, &held.pid)
		}
		g.holder <- held
		<-g.release
	})
}

func newGuardRig(t *testing.T, db *bun.DB, with ...func(*portalCompose.Dependencies)) *guardRig {
	t.Helper()
	rig := &guardRig{checked: newRequestGate(), locked: newLockGate()}
	database := testpkg.NewDatabase(t, db)
	adjust := append([]func(*portalCompose.Dependencies){func(deps *portalCompose.Dependencies) {
		deps.StatusDayRepo = NewRepositoryTestFactory(database).StudentStatusDay
		deps.StudentPhotos = &photoUnlinks{db: db}
		deps.OfferingChanges = markingOfferings{OfferingChangeRequests: deps.OfferingChanges}
		deps.GuardianInvites = markingInvites{}
		if deps.ArrivalSchedules == nil || deps.PickupSchedules == nil {
			_, schedules, _, _, _, _, err := NewTestCareRequests(database, slog.Default())
			require.NoError(t, err)
			deps.ArrivalSchedules = owners.NewParentArrivalSchedules(schedules.Arrivals)
			deps.PickupSchedules = owners.NewParentPickupSchedules(schedules.Pickups)
		}
		deps.InvalidateInvites = owners.NewGuardianInvitationInvalidation(database)
		deps.ChildRepo = gatedChildReads{ChildReads: deps.ChildRepo, gate: rig.checked}
		deps.StudentGuardianRepo = parentportal.StudentGuardianReadsAfterLock(deps.StudentGuardianRepo, rig.locked.afterLock)
	}}, with...)
	rig.childReadRig = newChildReadRigOn(t, db, adjust...)
	guardians, err := NewGuardianTestModule(database, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	rig.people = guardians.PeopleDirectory
	settings, err := owners.NewSettingsTestModule(database, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	rig.settings = settings.Settings
	return rig
}

func offeringChange(f childReadFamily, method, suffix string) childWrite {
	return childWrite{method, childPath(f, suffix), map[string]any{
		"offerings":      []map[string]any{{"offering_id": "1", "selected_days": []string{"mon"}}},
		"effective_from": "2026-10-01", "note": "Bitte",
	}}
}

// secondGuardian links a second account holder of another family as legal
// guardian of the family's child and returns the profile.
func (r *guardRig) secondGuardian(t *testing.T, f childReadFamily) int64 {
	t.Helper()
	other := testpkg.CreateTestParentGuardianChain(t, r.db)
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), r.db, f.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		_, err := r.people.LinkGuardianToStudent(txCtx, peopledirectory.LinkGuardian{
			StudentID: f.StudentID, GuardianProfileID: other.GuardianProfileID,
			RelationshipType: "parent", GuardianRole: "legal_guardian",
		})
		return err
	}))
	return other.GuardianProfileID
}

func (r *guardRig) enableCareScheduleRequests(t *testing.T) {
	t.Helper()
	r.set(t, map[string]any{
		parentportal.KeyParentCarePickupRequestEnabled: true,
		parentportal.KeyParentCareModeRequestEnabled:   true, parentportal.KeyEnrollmentBookingsAuthoritative: false,
	})
}

func careScheduleRequest(f childReadFamily) childWrite {
	return childWrite{http.MethodPost, childPath(f, "/care-schedule/requests"), map[string]any{"payload": map[string]any{"weekdays": []any{map[string]any{"weekday": 1, "pickup": "15:30"}}}}}
}

// markOwnerWrite is the row an owner-command double writes in the request's
// transaction: a pending Stammdaten request of the child, so the fingerprint
// sees a committed write and a rolled-back one leaves nothing.
func markOwnerWrite(ctx context.Context, studentID, accountID int64) error {
	tx, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return errors.New("owner command outside the school transaction")
	}
	_, err := tx.(bun.IDB).NewRaw(`INSERT INTO users.student_data_change_requests (tenant_id, student_id, submitted_by, target, field_key, new_value, status)
		VALUES (current_setting('app.current_tenant_id')::bigint, ?, ?, 'person', 'birthday', '"2018-01-01"'::jsonb, 'pending')`, studentID, accountID).Exec(ctx)
	return err
}

// markingOfferings keeps the reads of Enrollment and writes a marker row for
// the request commands, whose own preconditions are Enrollment's tests.
type markingOfferings struct {
	parentportal.OfferingChangeRequests
}

func (m markingOfferings) SubmitOfferingChange(ctx context.Context, input parentportal.CreateOfferingChangeInputSnapshot) (*parentportal.OfferingChangeRequestSnapshot, error) {
	return &parentportal.OfferingChangeRequestSnapshot{}, markOwnerWrite(ctx, input.StudentID, input.AccountID)
}

func (m markingOfferings) Edit(ctx context.Context, _ int64, input parentportal.CreateOfferingChangeInputSnapshot, _ string) (*parentportal.OfferingChangeRequestSnapshot, error) {
	return &parentportal.OfferingChangeRequestSnapshot{}, markOwnerWrite(ctx, input.StudentID, input.AccountID)
}

func (m markingOfferings) CreateCourseRequest(ctx context.Context, input parentportal.CreateCourseRequestInputSnapshot) (*parentportal.OfferingChangeRequestSnapshot, error) {
	return &parentportal.OfferingChangeRequestSnapshot{}, markOwnerWrite(ctx, input.StudentID, input.AccountID)
}

func (m markingOfferings) WithdrawCourseRequest(ctx context.Context, _ int64, accountID, studentID int64) error {
	return markOwnerWrite(ctx, studentID, accountID)
}

// markingInvites stands for Identity & Access's relative-access flows.
type markingInvites struct{}

func (markingInvites) InviteToStudent(ctx context.Context, request parentportal.GuardianInviteRequest) (parentportal.GuardianInviteOutcome, error) {
	return parentportal.GuardianInviteOutcome{Outcome: "invited"}, markOwnerWrite(ctx, request.StudentID, request.RequestedByAccountID)
}

func (markingInvites) RevokeAccess(ctx context.Context, revocation parentportal.GuardianAccessRevocation) error {
	return markOwnerWrite(ctx, revocation.StudentID, revocation.ActorAccountID)
}

// givePhoto stores a granted photo consent with a photo.
func (r *guardRig) givePhoto(t *testing.T, f childReadFamily) {
	t.Helper()
	_, err := r.db.NewRaw(`UPDATE users.student_profiles SET photo_path = ?, photo_consent_given_at = now(), photo_consent_given_by = ? WHERE id = ?`,
		fmt.Sprintf("students/%d/photo.jpg", f.StudentID), f.AccountID, f.StudentID).Exec(context.Background())
	require.NoError(t, err)
}

func (r *guardRig) set(t *testing.T, values map[string]any) {
	t.Helper()
	for key, value := range values {
		require.NoError(t, r.settings.SetValue(testpkg.Ctx(t), key, value, nil, nil), key)
	}
}

// withdrawal is one owner write that ends a fact the guard holds.
type withdrawal struct {
	name string
	run  func(t *testing.T, r *guardRig, f childReadFamily) func() error
}

// withdrawPermission sets one permission of the relationship to false
// through the relationship store, which locks the relationship first.
func withdrawPermission(permission string) withdrawal {
	return withdrawal{name: "Recht " + permission + " entzogen", run: func(t *testing.T, r *guardRig, f childReadFamily) func() error {
		store := owners.NewStudentGuardianRepository(testpkg.NewDatabase(t, r.db))
		rel, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), r.db, store.FindByID, any(guardRelationshipID(t, r.db, f.ParentChain)))
		require.NoError(t, err)
		var permissions map[string]any
		require.NoError(t, json.Unmarshal(rel.Permissions, &permissions))
		require.Equal(t, true, permissions[permission], "fixture must grant %s", permission)
		permissions[permission] = false
		rel.Permissions, err = json.Marshal(permissions)
		require.NoError(t, err)
		ctx := testpkg.Ctx(t)
		return func() error {
			_, err := store.UpdateColumns(ctx, rel, "permissions")
			return err
		}
	}}
}

var removeRelationship = withdrawal{name: "Beziehung entfernt", run: func(t *testing.T, r *guardRig, f childReadFamily) func() error {
	ctx := testpkg.Ctx(t)
	return func() error {
		return r.people.RemoveGuardianFromStudent(ctx, peopledirectory.RemoveGuardian{
			StudentID: f.StudentID, GuardianProfileID: f.GuardianProfileID, ActorAccountID: f.AccountID,
		})
	}
}}

// withdrawMembership deactivates the account's mapping at the school with
// the statement of Identity & Access's DeactivateTenantMapping. No operator
// flow removes a guardian's mapping (it refuses the guardian role), so the
// tests drive the statement every deactivation ends in.
var withdrawMembership = withdrawal{name: "Schulmitgliedschaft deaktiviert", run: func(t *testing.T, r *guardRig, f childReadFamily) func() error {
	return func() error {
		_, err := r.db.NewRaw(`UPDATE auth.account_tenants SET status = 'inactive', deactivated_at = NOW(), updated_at = NOW()
			WHERE account_id = ? AND tenant_id = ? AND status = 'active'`, f.AccountID, f.TenantID).Exec(context.Background())
		return err
	}
}}

// deleteMembership removes the mapping physically, as an account erasure does.
var deleteMembership = withdrawal{name: "Schulmitgliedschaft gelöscht", run: func(t *testing.T, r *guardRig, f childReadFamily) func() error {
	return func() error {
		_, err := r.db.NewRaw(`DELETE FROM auth.account_tenants WHERE account_id = ? AND tenant_id = ?`, f.AccountID, f.TenantID).Exec(context.Background())
		return err
	}
}}

func (w guardedWrite) withdrawals() []withdrawal {
	out := make([]withdrawal, 0, len(w.permissions)+3)
	for _, permission := range w.permissions {
		out = append(out, withdrawPermission(permission))
	}
	return append(out, withdrawPermission(guardPortalAccess), removeRelationship, withdrawMembership, deleteMembership)
}

func guardRelationshipID(t *testing.T, db *bun.DB, chain testpkg.ParentChain) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`SELECT id FROM users.student_guardian_relationships WHERE student_id = ? AND guardian_profile_id = ?`,
		chain.StudentID, chain.GuardianProfileID).Scan(context.Background(), &id))
	return id
}

// fingerprintColumns are the columns a parent write keys its rows by.
var fingerprintColumns = []string{"student_id", "guardian_profile_id", "account_id", "guardian_account_id", "submitted_by", "created_student_id"}

// fingerprint hashes every row of every table that names the family's
// child, guardian profile or account, so a refused write proves it changed
// nothing of theirs.
func (r *guardRig) fingerprint(t *testing.T, f childReadFamily) map[string]string {
	t.Helper()
	ctx := context.Background()
	var columns []struct {
		Schema string `bun:"table_schema"`
		Table  string `bun:"table_name"`
		Column string `bun:"column_name"`
	}
	require.NoError(t, r.db.NewRaw(`SELECT c.table_schema, c.table_name, c.column_name
		FROM information_schema.columns c
		JOIN information_schema.tables tb ON tb.table_schema = c.table_schema AND tb.table_name = c.table_name AND tb.table_type = 'BASE TABLE'
		WHERE c.column_name IN (?) AND c.table_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY 1, 2, 3`, bun.List(fingerprintColumns)).Scan(ctx, &columns))
	ids := map[string]int64{
		"student_id": f.StudentID, "created_student_id": f.StudentID, "guardian_profile_id": f.GuardianProfileID,
		"account_id": f.AccountID, "guardian_account_id": f.AccountID, "submitted_by": f.AccountID,
	}
	out := make(map[string]string, len(columns))
	for _, c := range columns {
		var digest string
		query := fmt.Sprintf(`SELECT md5(coalesce(string_agg(t::text, '|' ORDER BY t::text), '')) FROM %q.%q t WHERE %q = ?`, c.Schema, c.Table, c.Column)
		require.NoError(t, r.db.NewRaw(query, ids[c.Column]).Scan(ctx, &digest), query)
		out[c.Schema+"."+c.Table+"."+c.Column] = digest
	}
	return out
}

// withdrawalTables hold the rows the withdrawals themselves change.
var withdrawalTables = []string{"auth.account_tenants.", "auth.guardian_student_access.", "users.student_guardian_relationships."}

func withoutWithdrawalRows(changed []string) []string {
	out := changed[:0:0]
	for _, key := range changed {
		owned := false
		for _, prefix := range withdrawalTables {
			owned = owned || strings.HasPrefix(key, prefix)
		}
		if !owned {
			out = append(out, key)
		}
	}
	return out
}

func changedTables(before, after map[string]string) []string {
	var changed []string
	for key, digest := range after {
		if before[key] != digest {
			changed = append(changed, key)
		}
	}
	sort.Strings(changed)
	return changed
}

// sendParked sends w, parks it at the gate g reached, and returns the channel
// of its answer.
func (r *guardRig) sendAsync(t *testing.T, w childWrite, token string) <-chan *httptest.ResponseRecorder {
	t.Helper()
	done := make(chan *httptest.ResponseRecorder, 1)
	req := testutil.NewAuthenticatedRequest(t, w.method, w.path, w.body, testutil.WithJWTBearer(token))
	ctx, cancel := context.WithTimeout(context.Background(), writeDeadline)
	t.Cleanup(cancel)
	go func() { done <- testutil.ExecuteRequest(r.router, req.WithContext(ctx)) }()
	return done
}

func awaitAnswer(t *testing.T, done <-chan *httptest.ResponseRecorder, what string) *httptest.ResponseRecorder {
	t.Helper()
	select {
	case rec := <-done:
		return rec
	case <-time.After(writeDeadline + 5*time.Second):
		t.Fatalf("%s did not answer", what)
		return nil
	}
}

func (w guardedWrite) run(t *testing.T, db *bun.DB) {
	for _, change := range w.withdrawals() {
		t.Run(w.name+"/"+change.name, func(t *testing.T) {
			rig := newGuardRig(t, db)
			f := rig.family(t)
			request := w.prepare(t, rig, f)
			apply := change.run(t, rig, f)

			rig.checked.armed.Store(true)
			done := rig.sendAsync(t, request, f.token)
			select {
			case <-rig.checked.reached:
			case rec := <-done:
				t.Fatalf("the write answered before its child check: %d %s", rec.Code, rec.Body.String())
			case <-time.After(writeDeadline):
				t.Fatal("the write never reached its child check")
			}
			require.NoError(t, apply(), "the owner withdrawal commits while the write waits behind its check")
			before := rig.fingerprint(t, f)
			close(rig.checked.release)

			rec := awaitAnswer(t, done, request.path)
			require.GreaterOrEqual(t, rec.Code, 400, "the write must be refused after %s: %s", change.name, rec.Body.String())
			require.Less(t, rec.Code, 500, "a withdrawal is a refusal, not a failure: %s", rec.Body.String())
			require.Empty(t, changedTables(before, rig.fingerprint(t, f)), "a refused write changes nothing of the family")
		})
	}
	for _, change := range []withdrawal{withdrawPermission(firstOr(w.permissions, guardPortalAccess)), withdrawMembership} {
		t.Run(w.name+"/gesichert vor "+change.name, func(t *testing.T) {
			rig := newGuardRig(t, db)
			f := rig.family(t)
			request := w.prepare(t, rig, f)
			apply := change.run(t, rig, f)

			rig.locked.armed.Store(true)
			done := rig.sendAsync(t, request, f.token)
			var held lockHolder
			select {
			case held = <-rig.locked.holder:
				require.NoError(t, held.err)
			case rec := <-done:
				t.Fatalf("the write answered before it locked its relationship: %d %s", rec.Code, rec.Body.String())
			case <-time.After(writeDeadline):
				t.Fatal("the write never locked its relationship")
			}
			before := rig.fingerprint(t, f)
			withdrawn := make(chan error, 1)
			go func() { withdrawn <- apply() }()
			require.Eventually(t, func() bool {
				var waiting bool
				err := rig.db.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))", held.pid).
					Scan(context.Background(), &waiting)
				return err == nil && waiting
			}, 10*time.Second, 10*time.Millisecond, "the withdrawal must wait for the write that holds the guard")
			select {
			case err := <-withdrawn:
				t.Fatalf("the withdrawal committed while the write held the guard: %v", err)
			default:
			}
			close(rig.locked.release)
			rec := awaitAnswer(t, done, request.path)
			require.Less(t, rec.Code, 500, "the guarded write commits first: %s", rec.Body.String())
			select {
			case err := <-withdrawn:
				require.NoError(t, err)
			case <-time.After(writeDeadline):
				t.Fatal("the withdrawal did not finish after the write")
			}
			// A route that reads its view in a second unit after the write
			// may already see the withdrawal; its write itself committed.
			if rec.Code >= 300 {
				require.NotEmpty(t, withoutWithdrawalRows(changedTables(before, rig.fingerprint(t, f))),
					"the guarded write commits before the withdrawal: %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// do sends a request the test arranges with, outside the gates.
func (r *guardRig) do(t *testing.T, w childWrite, token string) *httptest.ResponseRecorder {
	t.Helper()
	rec := testutil.ExecuteRequest(r.router, testutil.NewAuthenticatedRequest(t, w.method, w.path, w.body, testutil.WithJWTBearer(token)))
	require.Less(t, rec.Code, 300, "arrange %s %s: %s", w.method, w.path, rec.Body.String())
	return rec
}

func (r *guardRig) scalar(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var id int64
	require.NoError(t, r.db.NewRaw(query, args...).Scan(context.Background(), &id), query)
	return id
}

func firstOr(values []string, fallback string) string {
	if len(values) > 0 {
		return values[0]
	}
	return fallback
}

func childPath(f childReadFamily, suffix string) string {
	return fmt.Sprintf("/parent/me/children/%d%s", f.StudentID, suffix)
}

const (
	permSickNote       = "parent_portal.sick_note.submit"
	permMasterRequest  = "parent_portal.master_data.request"
	permMasterEdit     = "parent_portal.master_data.edit"
	permConsent        = "parent_portal.consent.manage"
	permGuardianEdit   = "parent_portal.guardian.edit"
	permPickupManage   = "parent_portal.pickup.manage"
	permMealManage     = "parent_portal.meal_participation.manage"
	permRequestSubmit  = "parent_portal.request.submit"
	permNotesWrite     = "parent_portal.notes.write"
	permRelatedManage  = "parent_portal.related_accounts.manage"
	permEnrollSubmit   = "parent_portal.enrollment.submit"
	permEnrollmentView = "parent_portal.enrollments.view"
)

// guardedWrites are the parent write routes for an existing child.
func guardedWrites() []guardedWrite {
	return []guardedWrite{
		{
			name:        "Abwesenheitsantrag bearbeiten",
			permissions: []string{permSickNote},
			prepare: func(t *testing.T, r *guardRig, f childReadFamily) childWrite {
				r.set(t, map[string]any{parentportal.KeyParentSickNoteEnabled: true, parentportal.KeyParentExcusedReportsEnabled: true, parentportal.KeyParentExcusedRequiresApproval: true})
				r.do(t, childWrite{http.MethodPost, childPath(f, "/sick-note"), map[string]any{"dates": []string{"2026-09-03"}, "status": "excused", "reason": "Termin"}}, f.token)
				id := r.scalar(t, `SELECT id FROM active.excused_absence_requests WHERE student_id = ? ORDER BY id DESC LIMIT 1`, f.StudentID)
				return childWrite{http.MethodPut, childPath(f, fmt.Sprintf("/excused-requests/%d", id)), map[string]any{"dates": []string{"2026-09-04"}, "note": "Termin verschoben"}}
			},
		},
		{
			name:        "Stammdatenantrag",
			permissions: []string{permMasterRequest},
			prepare: func(t *testing.T, r *guardRig, f childReadFamily) childWrite {
				r.set(t, map[string]any{parentportal.KeyParentMasterDataRequestEnabled: true})
				return childWrite{http.MethodPost, childPath(f, "/master-data/requests"), map[string]any{"changes": []map[string]any{{"target": "person", "field_key": "last_name", "value": "Neumann"}}}}
			},
		},
		{
			name:        "Stammdatenantrag bearbeiten",
			permissions: []string{permMasterRequest},
			prepare: func(t *testing.T, _ *guardRig, f childReadFamily) childWrite {
				return masterDataEdit(f.StudentID, f.requestID, "Maxi", "")
			},
		},
		{
			name:        "Stammdaten direkt ändern",
			permissions: []string{permMasterEdit},
			prepare: func(t *testing.T, r *guardRig, f childReadFamily) childWrite {
				r.set(t, map[string]any{parentportal.KeyParentMasterDataEditEnabled: true})
				return childWrite{http.MethodPatch, childPath(f, "/master-data/student/health_info"), map[string]any{"value": "Allergie: Nüsse"}}
			},
		},
		{
			name:        "Fotoeinwilligung widerrufen",
			permissions: []string{permConsent},
			prepare: func(t *testing.T, r *guardRig, f childReadFamily) childWrite {
				r.givePhoto(t, f)
				return photoConsent(http.MethodDelete, f.StudentID)
			},
		},
		{
			name:        "Fotoeinwilligung erteilen",
			permissions: []string{permConsent},
			prepare: func(t *testing.T, r *guardRig, f childReadFamily) childWrite {
				r.givePhoto(t, f)
				r.do(t, photoConsent(http.MethodDelete, f.StudentID), f.token)
				return photoConsent(http.MethodPut, f.StudentID)
			},
		},
		{
			name:        "Anfrage teilen",
			permissions: []string{permMasterRequest},
			prepare: func(t *testing.T, _ *guardRig, f childReadFamily) childWrite {
				return sharingWrite(f.StudentID, f.requestID)
			},
		},
		{
			name:        "Kontakt anlegen",
			permissions: []string{permGuardianEdit},
			prepare: func(t *testing.T, r *guardRig, f childReadFamily) childWrite {
				r.set(t, map[string]any{parentportal.KeyParentGuardianManagementEnabled: true})
				return childWrite{http.MethodPost, childPath(f, "/guardians"), map[string]any{"first_name": "Oma", "last_name": "Kontakt", "relationship_type": "relative", "phones": []any{}}}
			},
		},
		{
			name:        "eigenen Kontakt ändern",
			permissions: []string{permGuardianEdit},
			prepare: func(t *testing.T, r *guardRig, f childReadFamily) childWrite {
				r.set(t, map[string]any{parentportal.KeyParentGuardianManagementEnabled: true})
				return childWrite{http.MethodPut, childPath(f, fmt.Sprintf("/guardians/%d/contact", f.GuardianProfileID)), map[string]any{"first_name": "Sabine", "last_name": "Geändert", "phones": []any{}}}
			},
		},
		{
			name:        "Abholhinweis ändern",
			permissions: []string{permGuardianEdit},
			prepare: func(t *testing.T, r *guardRig, f childReadFamily) childWrite {
				r.set(t, map[string]any{parentportal.KeyParentGuardianManagementEnabled: true})
				return childWrite{http.MethodPut, childPath(f, fmt.Sprintf("/guardians/%d/pickup", f.GuardianProfileID)), map[string]any{"pickup_notes": "Nur nach Absprache"}}
			},
		},
		{
			name:        "Betreuungsplan-Antrag",
			permissions: []string{permRequestSubmit},
			prepare: func(t *testing.T, r *guardRig, f childReadFamily) childWrite {
				r.enableCareScheduleRequests(t)
				return careScheduleRequest(f)
			},
		},
		{
			name:        "Betreuungsplan-Antrag bearbeiten",
			permissions: []string{permRequestSubmit},
			prepare: func(t *testing.T, r *guardRig, f childReadFamily) childWrite {
				r.enableCareScheduleRequests(t)
				r.do(t, careScheduleRequest(f), f.token)
				id := r.scalar(t, `SELECT id FROM schedule.care_schedule_change_requests WHERE student_id = ? ORDER BY id DESC LIMIT 1`, f.StudentID)
				return childWrite{http.MethodPut, childPath(f, fmt.Sprintf("/care-schedule/requests/%d", id)), map[string]any{"payload": map[string]any{"weekdays": []any{map[string]any{"weekday": 2, "pickup": "16:00"}}}}}
			},
		},
		{
			name:        "Betreuungsangebot ändern",
			permissions: []string{permRequestSubmit},
			prepare: func(t *testing.T, _ *guardRig, f childReadFamily) childWrite {
				return offeringChange(f, http.MethodPost, "/care-offerings/requests")
			},
		},
		{
			name:        "Betreuungsangebot-Antrag bearbeiten",
			permissions: []string{permRequestSubmit},
			prepare: func(t *testing.T, _ *guardRig, f childReadFamily) childWrite {
				return offeringChange(f, http.MethodPut, fmt.Sprintf("/care-offerings/requests/%d", f.requestID))
			},
		},
		{
			name:        "Kurs anfragen",
			permissions: []string{permEnrollSubmit, permEnrollmentView},
			prepare: func(t *testing.T, _ *guardRig, f childReadFamily) childWrite {
				return childWrite{http.MethodPost, childPath(f, "/courses/requests"), map[string]any{"course_id": "1", "note": "Bitte"}}
			},
		},
		{
			name:        "Kursanfrage zurücknehmen",
			permissions: []string{permEnrollSubmit, permEnrollmentView},
			prepare: func(t *testing.T, _ *guardRig, f childReadFamily) childWrite {
				return childWrite{http.MethodPost, childPath(f, fmt.Sprintf("/courses/requests/%d/withdraw", f.requestID)), nil}
			},
		},
		{
			name:        "Mitbenutzer einladen",
			permissions: []string{permRelatedManage},
			prepare: func(t *testing.T, r *guardRig, f childReadFamily) childWrite {
				r.set(t, map[string]any{parentportal.KeyGuardianParentInviteMode: parentportal.ParentInviteModeDirect})
				return childWrite{http.MethodPost, childPath(f, "/related-accounts"), map[string]any{"email": fmt.Sprintf("mitbenutzer-%d@example.test", testpkg.UniqueSuffix()), "first_name": "Max", "last_name": "Mitbenutzer"}}
			},
		},
		{
			name:        "Mitbenutzer entfernen",
			permissions: []string{permRelatedManage},
			prepare: func(t *testing.T, r *guardRig, f childReadFamily) childWrite {
				r.set(t, map[string]any{parentportal.KeyGuardianParentInviteMode: parentportal.ParentInviteModeDirect, parentportal.KeyGuardianParentCanRemove: true})
				other := r.secondGuardian(t, f)
				return childWrite{http.MethodDelete, childPath(f, fmt.Sprintf("/related-accounts/%d", other)), nil}
			},
		},
		{
			name:        "Essensplan der Woche",
			permissions: []string{permMealManage},
			prepare: func(t *testing.T, _ *guardRig, f childReadFamily) childWrite {
				return childWrite{http.MethodPut, childPath(f, "/meal-participation"), map[string]any{"weekdays": []int{1, 3}}}
			},
		},
		{
			name:        "Krankmeldung",
			permissions: []string{permSickNote},
			prepare: func(t *testing.T, r *guardRig, f childReadFamily) childWrite {
				r.set(t, map[string]any{parentportal.KeyParentSickNoteEnabled: true, parentportal.KeyParentSickReportsEnabled: true, parentportal.KeyParentSickRequiresApproval: false})
				return childWrite{http.MethodPost, childPath(f, "/sick-note"), map[string]any{"dates": []string{"2026-09-02"}, "status": "sick", "reason": "Fieber"}}
			},
		},
	}
}

func TestParentChildWritesHoldTheGuardUntilCommit(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	for _, w := range guardedWrites() {
		w.run(t, db)
	}
}
