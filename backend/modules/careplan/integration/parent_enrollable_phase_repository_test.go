package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// insertEnrollablePhase makes a minimal phase row directly so we don't
// pull in the enrollment repo here. windowOpen/Close = nil means
// "always open".
func insertEnrollablePhase(t *testing.T, db *bun.DB, tenantID int64, name string, windowOpen, windowClose *time.Time, active bool) int64 {
	t.Helper()
	bg := context.Background()
	var id int64
	err := db.NewRaw(`
		INSERT INTO enrollment.phases
		  (tenant_id, name, kind, service_start_date, service_end_date,
		   enrollment_open_at, enrollment_close_at, is_active, care_overflow_mode,
		   show_status_reason_to_parent)
		VALUES (?, ?, 'school_year', '2026-09-01', '2027-07-31',
		        ?, ?, ?, 'waitlist', FALSE)
		RETURNING id
	`, tenantID, name, windowOpen, windowClose, active).Scan(bg, &id)
	require.NoError(t, err)
	return id
}

func wipePhasesForTenant(db *bun.DB, tenantID int64, namePrefix string) {
	_, _ = db.NewDelete().Table("enrollment.phases").
		Where("tenant_id = ? AND name LIKE ?", tenantID, namePrefix+"%").
		Exec(context.Background())
}

// --- ListEnrollable ---------------------------------------------------

func TestEnrollablePhaseRepository_ListEnrollable_RejectsNonPositiveAccount(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := newSchoolProjectionFactory(t, db).ParentEnrollablePhase

	_, err := repo.ListEnrollable(context.Background(), 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be positive")

	_, err = repo.ListEnrollable(context.Background(), -1)
	require.Error(t, err)
}

func TestEnrollablePhaseRepository_ListEnrollable_HappyPath(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	account := testpkg.CreateTestAccount(t, db, "enrollable-happy")
	// The assertion below is "no account_tenants mapping → AlreadyLinked is
	// false", so the mapping CreateTestAccount adds for the test's own tenant
	// has to go (#2419).
	testpkg.UnclaimTestAccount(t, db, account.ID)
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.account_tenants").
			Where("account_id = ?", account.ID).Exec(context.Background())
		_, _ = db.NewDelete().Table("auth.accounts").
			Where("id = ?", account.ID).Exec(context.Background())
	})

	name := fmt.Sprintf("enrollable-happy-%d", time.Now().UnixNano())
	defer wipePhasesForTenant(db, tenantID, "enrollable-happy")
	insertEnrollablePhase(t, db, tenantID, name, nil, nil, true)

	repo := newSchoolProjectionFactory(t, db).ParentEnrollablePhase
	var list []*parentModels.EnrollablePhase
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListEnrollable(ctx, account.ID)
		return lErr
	}))

	found := false
	for _, p := range list {
		if p.PhaseName == name {
			found = true
			assert.False(t, p.AlreadyLinked,
				"no account_tenants mapping → AlreadyLinked must be false")
			assert.Equal(t, tenantID, p.SchoolID)
		}
	}
	assert.True(t, found, "phase with enrollment enabled + active + open window MUST appear")
}

func TestEnrollablePhaseRepository_ListEnrollable_AlreadyLinkedFlag(t *testing.T) {
	t.Parallel()

	// Parent has an active account_tenants mapping → already_linked
	// must be TRUE for that tenant's phases.
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	account := testpkg.CreateTestAccount(t, db, "enrollable-linked")
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.account_tenants").
			Where("account_id = ?", account.ID).Exec(context.Background())
		_, _ = db.NewDelete().Table("auth.accounts").
			Where("id = ?", account.ID).Exec(context.Background())
	})

	name := fmt.Sprintf("enrollable-linked-%d", time.Now().UnixNano())
	defer wipePhasesForTenant(db, tenantID, "enrollable-linked")
	insertEnrollablePhase(t, db, tenantID, name, nil, nil, true)

	repo := newSchoolProjectionFactory(t, db).ParentEnrollablePhase
	var list []*parentModels.EnrollablePhase
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListEnrollable(ctx, account.ID)
		return lErr
	}))

	for _, p := range list {
		if p.PhaseName == name {
			assert.True(t, p.AlreadyLinked,
				"active account_tenants mapping must surface as AlreadyLinked=true")
			return
		}
	}
	t.Fatal("phase not found in result")
}

func TestEnrollablePhaseRepository_ListEnrollable_DoesNotReadEnrollmentSetting(t *testing.T) {
	t.Parallel()

	// Settings filtering belongs to services/parent. The repository returns
	// eligible phases even when no config.setting_values row exists.
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	account := testpkg.CreateTestAccount(t, db, "enrollable-nosetting")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.accounts").
			Where("id = ?", account.ID).Exec(context.Background())
	})

	name := fmt.Sprintf("enrollable-nosetting-%d", time.Now().UnixNano())
	defer wipePhasesForTenant(db, tenantID, "enrollable-nosetting")
	insertEnrollablePhase(t, db, tenantID, name, nil, nil, true)

	repo := newSchoolProjectionFactory(t, db).ParentEnrollablePhase
	var list []*parentModels.EnrollablePhase
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListEnrollable(ctx, account.ID)
		return lErr
	}))
	found := false
	for _, phase := range list {
		found = found || phase.PhaseName == name
	}
	assert.True(t, found, "repository must not filter on enrollment.enabled")
}

func TestEnrollablePhaseRepository_ListEnrollable_OmitsInactivePhases(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	account := testpkg.CreateTestAccount(t, db, "enrollable-inactive")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.accounts").
			Where("id = ?", account.ID).Exec(context.Background())
	})

	name := fmt.Sprintf("enrollable-inactive-%d", time.Now().UnixNano())
	defer wipePhasesForTenant(db, tenantID, "enrollable-inactive")
	insertEnrollablePhase(t, db, tenantID, name, nil, nil, false) // is_active = false

	repo := newSchoolProjectionFactory(t, db).ParentEnrollablePhase
	var list []*parentModels.EnrollablePhase
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListEnrollable(ctx, account.ID)
		return lErr
	}))
	for _, p := range list {
		assert.NotEqual(t, name, p.PhaseName,
			"is_active=false phase MUST NOT appear")
	}
}

func TestEnrollablePhaseRepository_ListEnrollable_RespectsEnrollmentWindow(t *testing.T) {
	t.Parallel()

	// A phase whose window has closed must NOT appear; one whose
	// window opens in the future must NOT appear.
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	account := testpkg.CreateTestAccount(t, db, "enrollable-window")
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.accounts").
			Where("id = ?", account.ID).Exec(context.Background())
	})

	defer wipePhasesForTenant(db, tenantID, "enrollable-window")

	past := time.Now().Add(-2 * time.Hour)
	morePast := time.Now().Add(-1 * time.Hour)
	future := time.Now().Add(2 * time.Hour)
	moreFuture := time.Now().Add(3 * time.Hour)

	openName := fmt.Sprintf("enrollable-window-open-%d", time.Now().UnixNano())
	closedName := fmt.Sprintf("enrollable-window-closed-%d", time.Now().UnixNano())
	futureName := fmt.Sprintf("enrollable-window-future-%d", time.Now().UnixNano())

	insertEnrollablePhase(t, db, tenantID, openName, &past, &future, true)
	insertEnrollablePhase(t, db, tenantID, closedName, &past, &morePast, true)
	insertEnrollablePhase(t, db, tenantID, futureName, &future, &moreFuture, true)

	repo := newSchoolProjectionFactory(t, db).ParentEnrollablePhase
	var list []*parentModels.EnrollablePhase
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListEnrollable(ctx, account.ID)
		return lErr
	}))

	seen := map[string]bool{}
	for _, p := range list {
		seen[p.PhaseName] = true
	}
	assert.True(t, seen[openName], "open + active phase MUST appear")
	assert.False(t, seen[closedName], "closed-window phase MUST NOT appear")
	assert.False(t, seen[futureName], "future-window phase MUST NOT appear")
}

func TestEnrollablePhaseRepository_ListEnrollable_OrdersLinkedFirst(t *testing.T) {
	t.Parallel()

	// Linked schools come before unlinked in the result order so the
	// parent dashboard puts familiar schools at the top.
	db := testpkg.SetupTestDB(t)

	tenantLinked := testpkg.UniqueTestTenantID(t)
	tenantUnlinked := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantLinked)
	testpkg.EnsureTestTenant(t, db, tenantUnlinked)

	account := testpkg.CreateTestAccount(t, db, "enrollable-order")
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantLinked) // linked here only
	t.Cleanup(func() {
		_, _ = db.NewDelete().Table("auth.account_tenants").
			Where("account_id = ?", account.ID).Exec(context.Background())
		_, _ = db.NewDelete().Table("auth.accounts").
			Where("id = ?", account.ID).Exec(context.Background())
	})

	linkedName := fmt.Sprintf("enrollable-order-linked-%d", time.Now().UnixNano())
	unlinkedName := fmt.Sprintf("enrollable-order-unlinked-%d", time.Now().UnixNano())
	defer wipePhasesForTenant(db, tenantLinked, "enrollable-order-linked")
	defer wipePhasesForTenant(db, tenantUnlinked, "enrollable-order-unlinked")
	insertEnrollablePhase(t, db, tenantLinked, linkedName, nil, nil, true)
	insertEnrollablePhase(t, db, tenantUnlinked, unlinkedName, nil, nil, true)

	repo := newSchoolProjectionFactory(t, db).ParentEnrollablePhase
	var list []*parentModels.EnrollablePhase
	require.NoError(t, runAsAdmin(t, db, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListEnrollable(ctx, account.ID)
		return lErr
	}))

	// Filter to ours and verify ordering.
	linkedIdx, unlinkedIdx := -1, -1
	for i, p := range list {
		switch p.PhaseName {
		case linkedName:
			linkedIdx = i
		case unlinkedName:
			unlinkedIdx = i
		}
	}
	require.NotEqual(t, -1, linkedIdx, "linked phase must appear")
	require.NotEqual(t, -1, unlinkedIdx, "unlinked phase must appear")
	assert.Less(t, linkedIdx, unlinkedIdx,
		"AlreadyLinked DESC must put linked schools BEFORE unlinked ones in the result")
}
