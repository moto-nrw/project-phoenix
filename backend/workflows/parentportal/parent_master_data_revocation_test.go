package parentportal_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

// Issue #6496: a Track A edit checks parent_portal.master_data.edit before
// its school transaction. These tests park the edit between that check and
// the write, change the relationship through the owner's regular write paths
// and require the write to decide on the relationship it holds locked.

const (
	permMasterDataEdit = "parent_portal.master_data.edit"
	permPortalAccess   = "parent_portal.access"
)

// pausingChildReads parks the first child check after it has decided, the
// point between ResolvePermittedChild and the school transaction.
type pausingChildReads struct {
	parentService.ChildReads
	once    *sync.Once
	reached chan<- struct{}
	release <-chan struct{}
}

func (r pausingChildReads) FindForAccount(ctx context.Context, accountID, studentID int64) (*parentService.ChildSummary, error) {
	child, err := r.ChildReads.FindForAccount(ctx, accountID, studentID)
	r.once.Do(func() {
		r.reached <- struct{}{}
		<-r.release
	})
	return child, err
}

type trackAEditCase struct {
	target, field, value string
}

var (
	healthInfoEdit = trackAEditCase{parentService.DataChangeTargetStudent, "health_info", "Allergie: Nüsse"}
	emailEdit      = trackAEditCase{"guardian_profile", "email", "entzogen@example.test"}
	phoneEdit      = trackAEditCase{"guardian_phone", "primary", "+49 211 1234567"}
)

func (c trackAEditCase) run(ctx context.Context, svc *parentService.Portal, chain testpkg.ParentChain) error {
	_, err := svc.UpdateMasterDataField(ctx, chain.AccountID, chain.StudentID, c.target, c.field, json.RawMessage(strconv.Quote(c.value)))
	return err
}

// relationshipChange is one regular owner write that ends or narrows the
// edit right of the chain's relationship.
type relationshipChange struct {
	name    string
	apply   func(t *testing.T, ctx context.Context, db *bun.DB, people peoplePorts.GuardianService, chain testpkg.ParentChain) error
	refusal error
}

func revokePermission(permission string) func(*testing.T, context.Context, *bun.DB, peoplePorts.GuardianService, testpkg.ParentChain) error {
	return func(t *testing.T, ctx context.Context, db *bun.DB, _ peoplePorts.GuardianService, chain testpkg.ParentChain) error {
		return prepareRevocation(t, ctx, db, chain, permission)()
	}
}

// prepareRevocation reads the relationship and returns the owner write that
// sets permission to false. The returned write may run on another goroutine.
func prepareRevocation(t *testing.T, ctx context.Context, db *bun.DB, chain testpkg.ParentChain, permission string) func() error {
	t.Helper()
	store := owners.NewStudentGuardianRepository(testpkg.NewDatabase(t, db))
	rel, err := testpkg.TenantQuery1(t, ctx, db, store.FindByID, any(relationshipID(t, db, chain)))
	require.NoError(t, err)
	var permissions map[string]any
	require.NoError(t, json.Unmarshal(rel.Permissions, &permissions))
	require.Equal(t, true, permissions[permission], "fixture must grant %s", permission)
	permissions[permission] = false
	rel.Permissions, err = json.Marshal(permissions)
	require.NoError(t, err)
	return func() error {
		_, err := store.UpdateColumns(ctx, rel, "permissions")
		return err
	}
}

var relationshipChanges = []relationshipChange{
	{name: "Schreibrecht entzogen", apply: revokePermission(permMasterDataEdit), refusal: parentService.ErrGuardianPermissionDenied},
	{name: "Portalzugang entzogen", apply: revokePermission(permPortalAccess), refusal: parentService.ErrChildNotLinked},
	{name: "Rolle geändert", apply: func(t *testing.T, ctx context.Context, db *bun.DB, people peoplePorts.GuardianService, chain testpkg.ParentChain) error {
		role := "pickup_only"
		id := relationshipID(t, db, chain)
		return inSchoolTx(t, ctx, db, chain.TenantID, func(txCtx context.Context) error {
			return people.UpdateStudentGuardianRelationship(txCtx, id, peoplePorts.StudentGuardianUpdateRequest{GuardianRole: &role})
		})
	}, refusal: parentService.ErrChildNotLinked},
	{name: "Beziehung entfernt", apply: func(_ *testing.T, ctx context.Context, _ *bun.DB, people peoplePorts.GuardianService, chain testpkg.ParentChain) error {
		return people.RemoveGuardianFromStudent(ctx, chain.StudentID, chain.GuardianProfileID, chain.AccountID, false)
	}, refusal: parentService.ErrChildNotLinked},
}

// inSchoolTx runs a staff command in the school transaction the tenant
// middleware opens for its route.
func inSchoolTx(t *testing.T, ctx context.Context, db *bun.DB, tenantID int64, fn func(context.Context) error) error {
	return testpkg.WithTenantTx(t, ctx, db, tenantID, func(txCtx context.Context, _ bun.Tx) error { return fn(txCtx) })
}

func relationshipID(t *testing.T, db *bun.DB, chain testpkg.ParentChain) int64 {
	var id int64
	err := db.NewRaw(`SELECT id FROM users.student_guardian_relationships WHERE student_id = ? AND guardian_profile_id = ?`,
		chain.StudentID, chain.GuardianProfileID).Scan(testpkg.WithPackageTenantRuntime(context.Background()), &id)
	require.NoError(t, err)
	return id
}

// trackAState is everything a Track A edit may change: the live field, the
// auto-applied history and the child's field-edit audit.
type trackAState struct {
	HealthData   *string
	Email        *string
	Phones       int
	AutoApplied  int
	StudentEdits int
}

func readTrackAState(t *testing.T, db *bun.DB, chain testpkg.ParentChain) trackAState {
	t.Helper()
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	var state trackAState
	require.NoError(t, db.NewRaw(`SELECT
		(SELECT md5(care.health_data::text) FROM users.student_care_profiles AS care
			JOIN users.student_school_memberships AS membership ON membership.id = care.membership_id
			WHERE membership.student_profile_id = ? AND membership.deleted_at IS NULL) AS health_data,
		(SELECT email FROM users.guardian_profiles WHERE id = ?) AS email,
		(SELECT count(*) FROM users.guardian_phone_numbers WHERE guardian_profile_id = ?) AS phones,
		(SELECT count(*) FROM users.student_data_change_requests WHERE student_id = ? AND status = 'auto_applied') AS auto_applied,
		(SELECT count(*) FROM audit.student_field_edits WHERE student_id = ?) AS student_edits`,
		chain.StudentID, chain.GuardianProfileID, chain.GuardianProfileID, chain.StudentID, chain.StudentID).Scan(ctx, &state))
	return state
}

func editorContext(chain testpkg.ParentChain) context.Context {
	return context.WithValue(testpkg.WithPackageTenantRuntime(context.Background()), sessiontoken.CtxClaims, sessiontoken.AppClaims{
		ID: int(chain.AccountID), FirstName: "Sabine", LastName: "Schneider",
	})
}

func TestUpdateMasterDataField_RevocationBeforeWriteRefusesStaleCheck(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	services, _, err := api.NewTestPeopleServices(testpkg.NewDatabase(t, db), slog.Default())
	require.NoError(t, err)
	people := services.Guardian

	type scenario struct {
		change relationshipChange
		edit   trackAEditCase
	}
	scenarios := make([]scenario, 0, len(relationshipChanges)+2)
	for _, change := range relationshipChanges {
		scenarios = append(scenarios, scenario{change, healthInfoEdit})
	}
	scenarios = append(scenarios, scenario{relationshipChanges[0], emailEdit}, scenario{relationshipChanges[0], phoneEdit})

	for _, sc := range scenarios {
		t.Run(sc.change.name+"/"+sc.edit.target+"/"+sc.edit.field, func(t *testing.T) {
			chain := testpkg.CreateTestParentGuardianChain(t, db)
			reached, release := make(chan struct{}), make(chan struct{})
			deps := masterDataServiceDependencies(t, db, true)
			deps.ChildRepo = pausingChildReads{ChildReads: deps.ChildRepo, once: &sync.Once{}, reached: reached, release: release}
			svc := parentportalcompose.New(deps)
			before := readTrackAState(t, db, chain)

			ctx, cancel := context.WithTimeout(editorContext(chain), 20*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- sc.edit.run(ctx, svc, chain) }()
			select {
			case <-reached:
			case <-ctx.Done():
				t.Fatal("the edit never reached its permission check")
			}

			// The owner change commits while the edit waits behind its check.
			require.NoError(t, sc.change.apply(t, testpkg.Ctx(t), db, people, chain))
			close(release)

			var editErr error
			select {
			case editErr = <-done:
			case <-ctx.Done():
				t.Fatal("the edit did not finish")
			}
			require.ErrorIs(t, editErr, sc.change.refusal)
			require.Equal(t, before, readTrackAState(t, db, chain), "a refused edit leaves field, history and audit untouched")
		})
	}
}

func TestUpdateMasterDataField_LockedEditCommitsBeforeWaitingRevocation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	type lockHolder struct {
		pid int
		err error
	}
	var once sync.Once
	holder, release := make(chan lockHolder, 1), make(chan struct{})
	deps := masterDataServiceDependencies(t, db, true)
	deps.StudentGuardianRepo = parentService.StudentGuardianReadsAfterLock(deps.StudentGuardianRepo, func(ctx context.Context) {
		once.Do(func() {
			var held lockHolder
			if tx, ok := tenant.TransactionFromContext(ctx); !ok {
				held.err = errors.New("the relationship lock runs outside the school transaction")
			} else {
				held.err = tx.(bun.IDB).NewRaw("SELECT pg_backend_pid()").Scan(ctx, &held.pid)
			}
			holder <- held
			<-release
		})
	})
	svc := parentportalcompose.New(deps)

	ctx, cancel := context.WithTimeout(editorContext(chain), 20*time.Second)
	defer cancel()
	edited := make(chan error, 1)
	go func() { edited <- healthInfoEdit.run(ctx, svc, chain) }()
	var pid int
	select {
	case held := <-holder:
		require.NoError(t, held.err)
		pid = held.pid
	case <-ctx.Done():
		t.Fatal("the edit never locked its relationship")
	}

	revoke := prepareRevocation(t, testpkg.Ctx(t), db, chain, permMasterDataEdit)
	revoked := make(chan error, 1)
	go func() { revoked <- revoke() }()
	require.Eventually(t, func() bool {
		var waiting bool
		err := db.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))", pid).
			Scan(context.Background(), &waiting)
		return err == nil && waiting
	}, 10*time.Second, 10*time.Millisecond, "the revocation must wait for the edit that holds the relationship")
	select {
	case err := <-revoked:
		t.Fatalf("the revocation committed while the edit held the relationship: %v", err)
	default:
	}
	close(release)

	select {
	case err := <-edited:
		require.NoError(t, err, "the edit that held the relationship first commits")
	case <-ctx.Done():
		t.Fatal("the edit did not finish")
	}
	select {
	case err := <-revoked:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("the revocation did not finish")
	}
	view, err := svc.GetChildMasterData(editorContext(chain), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.NotNil(t, view.HealthInfo)
	require.Equal(t, healthInfoEdit.value, *view.HealthInfo)
	committed := readTrackAState(t, db, chain)

	// After the confirmed revocation no further edit commits.
	err = trackAEditCase{healthInfoEdit.target, healthInfoEdit.field, "nach dem Entzug"}.run(editorContext(chain), svc, chain)
	require.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied)
	require.Equal(t, committed, readTrackAState(t, db, chain))
}

func TestUpdateMasterDataField_RightsStayPerRelationship(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	services, _, err := api.NewTestPeopleServices(testpkg.NewDatabase(t, db), slog.Default())
	require.NoError(t, err)
	people := services.Guardian
	svc := parentportalcompose.New(masterDataServiceDependencies(t, db, true))

	first := testpkg.CreateTestParentGuardianChain(t, db)
	second := testpkg.CreateTestParentGuardianChain(t, db)
	// The second parent is also legal guardian of the first child.
	require.NoError(t, inSchoolTx(t, testpkg.Ctx(t), db, first.TenantID, func(txCtx context.Context) error {
		_, err := people.LinkGuardianToStudent(txCtx, peoplePorts.StudentGuardianCreateRequest{
			StudentID: first.StudentID, GuardianProfileID: second.GuardianProfileID,
			RelationshipType: "parent", GuardianRole: "legal_guardian",
		})
		return err
	}))
	secondOnFirst := testpkg.ParentChain{
		AccountID: second.AccountID, TenantID: second.TenantID, GuardianProfileID: second.GuardianProfileID,
		StudentID: first.StudentID, PersonID: first.PersonID, Email: second.Email,
	}
	require.NoError(t, revokePermission(permMasterDataEdit)(t, testpkg.Ctx(t), db, people, secondOnFirst))

	require.ErrorIs(t, healthInfoEdit.run(editorContext(second), svc, secondOnFirst), parentService.ErrGuardianPermissionDenied,
		"the narrowed relationship refuses the edit")
	require.NoError(t, healthInfoEdit.run(editorContext(second), svc, second),
		"the same account keeps the right on its other child")
	require.NoError(t, healthInfoEdit.run(editorContext(first), svc, first),
		"the other guardian of the child keeps the right")

	t.Run("fremde Familie", func(t *testing.T) {
		stranger := testpkg.CreateTestParentGuardianChain(t, db)
		foreign := testpkg.ParentChain{AccountID: stranger.AccountID, GuardianProfileID: stranger.GuardianProfileID, StudentID: first.StudentID}
		require.ErrorIs(t, healthInfoEdit.run(editorContext(stranger), svc, foreign), parentService.ErrChildNotLinked)
	})
	t.Run("fremde Schule", func(t *testing.T) {
		testpkg.OwnTenant(t)
		stranger := testpkg.CreateTestParentGuardianChain(t, db)
		foreign := testpkg.ParentChain{AccountID: stranger.AccountID, GuardianProfileID: stranger.GuardianProfileID, StudentID: first.StudentID}
		require.ErrorIs(t, healthInfoEdit.run(editorContext(stranger), svc, foreign), parentService.ErrChildNotLinked)
	})
	t.Run("Betreuung beendet", func(t *testing.T) {
		endCareFor(t, db, second.StudentID)
		require.ErrorIs(t, healthInfoEdit.run(editorContext(second), svc, second), parentService.ErrChildCareEnded)
	})
}

// The guardian's own fields lock the profile before the relationship, the
// order of every contact write of the portal: an edit that waits for the
// profile must not hold the relationship meanwhile.
func TestUpdateMasterDataField_GuardianFieldLocksProfileBeforeRelationship(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	svc := parentportalcompose.New(masterDataServiceDependencies(t, db, true))
	ctx, cancel := context.WithTimeout(editorContext(chain), 20*time.Second)
	defer cancel()

	edited := make(chan error, 1)
	err := testpkg.WithTenantTx(t, ctx, db, chain.TenantID, func(lockCtx context.Context, tx bun.Tx) error {
		var locked []int64
		if err := tx.NewRaw(`SELECT id FROM users.guardian_profiles WHERE id = ? FOR UPDATE`, chain.GuardianProfileID).Scan(lockCtx, &locked); err != nil {
			return err
		}
		var pid int
		require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(lockCtx, &pid))
		go func() { edited <- emailEdit.run(ctx, svc, chain) }()
		require.Eventually(t, func() bool {
			var waiting bool
			err := db.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))", pid).
				Scan(context.Background(), &waiting)
			return err == nil && waiting
		}, 10*time.Second, 10*time.Millisecond, "the edit must wait for the guardian profile")
		return tx.NewRaw(`SELECT id FROM users.student_guardian_relationships WHERE id = ? FOR UPDATE NOWAIT`,
			relationshipID(t, db, chain)).Scan(lockCtx, &locked)
	})
	require.NoError(t, err, "the waiting edit must not hold the relationship")
	select {
	case err := <-edited:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("the edit did not finish")
	}
}
