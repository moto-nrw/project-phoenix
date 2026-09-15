package auth_test

// Coverage for the admin-override branches and helpers that the existing
// suites only graze. The follow-ups file covers the happy paths and the
// permission gates; this file targets the value transitions (force_off
// vs force_on, "none" clears, previous-value tracking) and the user-agent
// shortening helper used by the trusted-device email.

import (
	"context"
	"database/sql"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// --- OperatorSetGlobalMFAOverride branches -----------------------------

func TestMFAService_OperatorSetGlobalMFAOverride_RejectsInvalidValue(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _, db := newTestMFAService(t)
	acc := testpkg.CreateTestAccount(t, db, "global-invalid")

	require.ErrorIs(t,
		svc.OperatorSetGlobalMFAOverride(ctx, 99, acc.ID, "force_maybe", "any reason"),
		auth.ErrMFAInvalidOverride,
	)
}

func TestMFAService_OperatorSetGlobalMFAOverride_RejectsEmptyReason(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _, db := newTestMFAService(t)
	acc := testpkg.CreateTestAccount(t, db, "global-empty-reason")

	for _, reason := range []string{"", "   ", "\t\n"} {
		err := svc.OperatorSetGlobalMFAOverride(ctx, 99, acc.ID, authModel.MFAAdminOverrideForceOff, reason)
		require.Error(t, err, "whitespace-only reason %q must be rejected", reason)
		require.Contains(t, err.Error(), "reason")
	}
}

func TestMFAService_OperatorSetGlobalMFAOverride_NoneOnEmptyState(t *testing.T) {
	t.Parallel()

	// "none" on a fresh account must succeed even though there's no row
	// to delete — DeleteGlobal is idempotent and the audit row still fires.
	ctx := context.Background()
	svc, repos, db := newTestMFAService(t)
	acc := testpkg.CreateTestAccount(t, db, "global-none-noop")

	require.NoError(t, svc.OperatorSetGlobalMFAOverride(ctx, 99, acc.ID, authModel.MFAAdminOverrideNone, "no-op"))

	row, err := repos.MFAOverride.FindGlobal(ctx, acc.ID)
	require.NoError(t, err)
	assert.Nil(t, row)
}

func TestMFAService_OperatorSetGlobalMFAOverride_ForceOnKeepsExistingDevice(t *testing.T) {
	t.Parallel()

	// force_on is a hardening override — it must NOT revoke trusted
	// devices (that's the force_off branch). This locks in the asymmetry
	// so a future refactor that copies the revoke into force_on would
	// break this test.
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "global-force-on-keeps-device")
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	require.NoError(t, svc.Enroll(ctx, acc.ID))
	_, _, err := svc.IssueTrustedDevice(ctx, acc.ID, tenantID, "UA-test", net.ParseIP("203.0.113.10"))
	require.NoError(t, err)

	require.NoError(t, svc.OperatorSetGlobalMFAOverride(ctx, 99, acc.ID, authModel.MFAAdminOverrideForceOn, "hardening"))

	devices, err := svc.ListTrustedDevices(ctx, acc.ID, tenantID)
	require.NoError(t, err)
	assert.Len(t, devices, 1, "force_on must not revoke trusted devices — that's force_off's job")
}

func TestMFAService_OperatorSetGlobalMFAOverride_ForceOffRevokesAcrossAllTenants(t *testing.T) {
	t.Parallel()

	// The platform-wide "emergency switch" must yank trust EVERYWHERE,
	// not just in the tenant the operator happens to be looking at.
	// Two distinct tenants make this visible — both must drop to zero
	// devices after a single global force_off.
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc := testpkg.CreateTestAccount(t, db, "global-force-off-cross-tenant")

	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)

	require.NoError(t, svc.Enroll(ctx, acc.ID))
	_, _, err := svc.IssueTrustedDevice(ctx, acc.ID, tenantA, "UA-A", net.ParseIP("203.0.113.21"))
	require.NoError(t, err)
	_, _, err = svc.IssueTrustedDevice(ctx, acc.ID, tenantB, "UA-B", net.ParseIP("203.0.113.22"))
	require.NoError(t, err)

	require.NoError(t, svc.OperatorSetGlobalMFAOverride(ctx, 99, acc.ID, authModel.MFAAdminOverrideForceOff, "mailbox lockout"))

	for _, tid := range []int64{tenantA, tenantB} {
		devices, err := svc.ListTrustedDevices(ctx, acc.ID, tid)
		require.NoError(t, err)
		assert.Empty(t, devices, "global force_off must clear trust in every tenant (tenant=%d)", tid)
	}
}

// --- SetMFAOverride (tenant-scoped) branches ---------------------------

func TestMFAService_SetMFAOverride_RejectsEmptyReason(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _, db := newTestMFAService(t)
	acc, tenantID := tenantMappedAccount(t, db, "tenant-empty-reason")

	perms := []string{"users:manage"}
	err := svc.SetMFAOverride(ctx, 11, tenantID, acc.ID, authModel.MFAAdminOverrideForceOff, "  ", perms)
	require.Error(t, err)
	require.Contains(t, err.Error(), "reason")
}

func TestMFAService_SetMFAOverride_NoneClearsExistingRow(t *testing.T) {
	t.Parallel()

	// Tenant-scoped "none" maps to a DELETE on the (account, tenant) row.
	// After clearing, FindByAccountAndTenant must return nil (not the
	// stale row, not an error).
	ctx := context.Background()
	svc, repos, db := newTestMFAService(t)

	acc, tenantID := tenantMappedAccount(t, db, "tenant-none-clears")

	perms := []string{"users:manage"}
	require.NoError(t, svc.SetMFAOverride(ctx, 11, tenantID, acc.ID, authModel.MFAAdminOverrideForceOn, "harden", perms))

	row, err := repos.MFAOverride.FindByAccountAndTenant(ctx, acc.ID, tenantID)
	require.NoError(t, err)
	require.NotNil(t, row)

	require.NoError(t, svc.SetMFAOverride(ctx, 11, tenantID, acc.ID, authModel.MFAAdminOverrideNone, "policy reverted", perms))

	row, err = repos.MFAOverride.FindByAccountAndTenant(ctx, acc.ID, tenantID)
	require.NoError(t, err)
	assert.Nil(t, row, "none must DELETE the row, not leave it with a stale override value")
}

func TestMFAService_SetMFAOverride_ForceOnKeepsTrustedDevices(t *testing.T) {
	t.Parallel()

	// Same asymmetry as the global path: only force_off clears trust.
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc, tenantID := tenantMappedAccount(t, db, "tenant-force-on-keeps-device")

	require.NoError(t, svc.Enroll(ctx, acc.ID))
	_, _, err := svc.IssueTrustedDevice(ctx, acc.ID, tenantID, "UA-keep", net.ParseIP("203.0.113.30"))
	require.NoError(t, err)

	perms := []string{"users:manage"}
	require.NoError(t, svc.SetMFAOverride(ctx, 11, tenantID, acc.ID, authModel.MFAAdminOverrideForceOn, "harden", perms))

	devices, err := svc.ListTrustedDevices(ctx, acc.ID, tenantID)
	require.NoError(t, err)
	assert.Len(t, devices, 1)
}

func TestMFAService_SetMFAOverride_ForceOffOnlyAffectsTargetTenant(t *testing.T) {
	t.Parallel()

	// Tenant-scoped force_off must NOT touch trusted devices in a
	// different tenant the same account also belongs to. Mirror of the
	// global force_off test — confirms the two scopes don't bleed.
	ctx := context.Background()
	svc, _, db := newTestMFAService(t)

	acc, tenantA := tenantMappedAccount(t, db, "tenant-force-off-isolation")
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureAccountTenant(t, db, acc.ID, tenantB)

	require.NoError(t, svc.Enroll(ctx, acc.ID))
	_, _, err := svc.IssueTrustedDevice(ctx, acc.ID, tenantA, "UA-A", net.ParseIP("203.0.113.41"))
	require.NoError(t, err)
	_, _, err = svc.IssueTrustedDevice(ctx, acc.ID, tenantB, "UA-B", net.ParseIP("203.0.113.42"))
	require.NoError(t, err)

	// Tenant-A admin disables MFA on tenant A only.
	perms := []string{"users:manage"}
	require.NoError(t, svc.SetMFAOverride(ctx, 11, tenantA, acc.ID, authModel.MFAAdminOverrideForceOff, "tenant-A lockout", perms))

	devicesA, err := svc.ListTrustedDevices(ctx, acc.ID, tenantA)
	require.NoError(t, err)
	assert.Empty(t, devicesA, "tenant-A force_off must clear tenant-A devices")

	devicesB, err := svc.ListTrustedDevices(ctx, acc.ID, tenantB)
	require.NoError(t, err)
	assert.Len(t, devicesB, 1, "tenant-A override must not touch tenant-B devices")
}

// --- ShortenUserAgent — pure function ---------------------------------

func TestShortenUserAgent(t *testing.T) {
	t.Parallel()

	// Every branch of the parser. The full UA strings come from real
	// browsers (User-Agent headers as logged in development) so a future
	// edit that breaks one of them is caught here.
	cases := []struct {
		name     string
		ua       string
		expected string
	}{
		{"empty", "", "Unbekanntes Gerät"},
		{"whitespace", "   \n\t", "Unbekanntes Gerät"},
		{"unknown browser unknown os", "Mozilla/5.0 SomethingObscure", "Browser"},
		{"chrome on macOS",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Chrome auf macOS",
		},
		{"chrome on windows",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
			"Chrome auf Windows",
		},
		{"chrome on android",
			"Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 Chrome/120.0.0.0 Mobile Safari/537.36",
			"Chrome auf Android",
		},
		{"firefox on linux",
			"Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
			"Firefox auf Linux",
		},
		{"safari on iphone",
			"Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1",
			"Safari auf iPhone",
		},
		{"safari on ipad",
			"Mozilla/5.0 (iPad; CPU OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/604.1",
			"Safari auf iPad",
		},
		{"edge on windows",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
			"Edge auf Windows",
		},
		{"chrome detected when edge sub-token absent",
			"Mozilla/5.0 Chrome/120.0.0.0 Safari/537.36",
			"Chrome",
		},
		{"safari without chrome token stays safari",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Version/17.0 Safari/605.1.15",
			"Safari auf macOS",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := auth.ShortenUserAgent(tc.ua)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestShortenUserAgent_DoesNotMisidentifyChromeAsSafari(t *testing.T) {
	t.Parallel()

	// Regression guard: Chrome's UA contains "Safari/" — the parser
	// must NOT report Safari for a Chrome string. Pulled into its own
	// test because this is the easiest mistake to introduce while
	// reordering the switch cases.
	ua := "Mozilla/5.0 Chrome/120 Safari/537.36"
	got := auth.ShortenUserAgent(ua)
	require.True(t, strings.HasPrefix(got, "Chrome"), "expected Chrome prefix, got %q", got)
}

// --- override writes vs. an in-flight token mint -----------------------------

// TestMFAService_OperatorSetGlobalMFAOverride_WaitsForTheAccountRowLock is the
// #2207 review regression for the per-account half of the MFA policy. A session
// mint resolves the override and then holds auth.accounts FOR UPDATE until it
// commits; an override write that ignored that row could commit force_on in
// between, and the login would still hand out a session that never saw a second
// factor. The write therefore takes the same row first — account-first, the
// order every revocation path walks — and waits.
func TestMFAService_OperatorSetGlobalMFAOverride_WaitsForTheAccountRowLock(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _, db := newTestMFAService(t)
	acc := testpkg.CreateTestAccount(t, db, "global-override-lock-order")

	// An override row must already exist: the row is what makes this a real
	// test. Creating one INSERTs and therefore takes the foreign key's own lock
	// on auth.accounts, which would make the write wait no matter what this
	// code does. Flipping an EXISTING row skips the foreign-key re-check
	// (account_id does not change), so nothing but the explicit account lock
	// orders the flip against a mint that already read the old value.
	require.NoError(t, svc.OperatorSetGlobalMFAOverride(
		ctx, 99, acc.ID, authModel.MFAAdminOverrideForceOff, "seed"))

	// Stand in for the mint transaction, which owns the account row from its
	// precondition check until commit.
	mintTx, err := db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	defer func() { _ = mintTx.Rollback() }()
	_, err = mintTx.ExecContext(ctx, `SELECT id FROM auth.accounts WHERE id = ? FOR UPDATE`, acc.ID)
	require.NoError(t, err)

	written := make(chan error, 1)
	go func() {
		written <- svc.OperatorSetGlobalMFAOverride(
			context.Background(), 99, acc.ID, authModel.MFAAdminOverrideForceOn, "lock order")
	}()

	select {
	case writeErr := <-written:
		t.Fatalf("the override write did not wait for the mint's account row lock: %v", writeErr)
	case <-time.After(300 * time.Millisecond):
		// Blocked, which is the point.
	}

	require.NoError(t, mintTx.Rollback())

	select {
	case writeErr := <-written:
		require.NoError(t, writeErr, "the write must proceed once the mint released the row")
	case <-time.After(10 * time.Second):
		t.Fatal("the override write never completed after the account row was released")
	}
}
