package application

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The kiosk (PyrePortal) maps these texts; they are a cross-repo contract.
func TestStaffPINErrorTextsAreTheKioskContract(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "invalid staff PIN credentials", domain.ErrInvalidStaffPINCredentials.Error())
	assert.Equal(t, "staff PIN is temporarily locked", domain.ErrStaffPINLocked.Error())
	assert.Equal(t, "account not found", domain.ErrStaffPINAccountNotFound.Error())
	assert.Equal(t, "account is temporarily locked due to failed PIN attempts", domain.ErrStaffPINSelfServiceLocked.Error())
	assert.Equal(t, "current PIN is required when updating existing PIN", domain.ErrStaffPINCurrentRequired.Error())
	assert.Equal(t, "current PIN is incorrect", domain.ErrStaffPINCurrentWrong.Error())
}

func TestAuthenticateStaffPINAcceptsMatchingAccountPIN(t *testing.T) {
	t.Parallel()

	f := newLifecycleFixture(t)
	staffID := f.seedPINStaff(10, "2468")

	staff, err := f.lifecycle.AuthenticateStaffPIN(context.Background(), lifecycleTenant, staffID, "2468")

	require.NoError(t, err)
	assert.Equal(t, staffID, staff.ID)
	assert.Equal(t, lifecycleTenant, staff.TenantID)
	assert.Zero(t, f.store.pinAttempts[10])
	assert.Equal(t, 1, f.store.pinResets[10])
}

func TestAuthenticateStaffPINCommitsFailedAttemptForWrongPIN(t *testing.T) {
	t.Parallel()

	f := newLifecycleFixture(t)
	staffID := f.seedPINStaff(10, "2468")

	_, err := f.lifecycle.AuthenticateStaffPIN(context.Background(), lifecycleTenant, staffID, "9999")

	// A credential refusal is not a transaction failure: the recorded
	// attempt commits, and the caller sees the bare sentinel.
	require.Equal(t, domain.ErrInvalidStaffPINCredentials, err)
	assert.Equal(t, 1, f.store.pinAttempts[10])
	assert.Zero(t, f.store.pinResets[10])
}

func TestAuthenticateStaffPINRejectsLockedAccount(t *testing.T) {
	t.Parallel()

	f := newLifecycleFixture(t)
	staffID := f.seedPINStaff(10, "2468")
	lockedUntil := time.Now().Add(time.Hour)
	account := f.store.pins[10]
	account.PINLockedUntil = &lockedUntil
	f.store.pins[10] = account

	_, err := f.lifecycle.AuthenticateStaffPIN(context.Background(), lifecycleTenant, staffID, "2468")

	require.Equal(t, domain.ErrStaffPINLocked, err)
	assert.Zero(t, f.store.pinAttempts[10])
	assert.Zero(t, f.store.pinResets[10])
}

func TestAuthenticateStaffPINRejectsNonActiveTenantMapping(t *testing.T) {
	t.Parallel()

	f := newLifecycleFixture(t)
	staffID := f.seedPINStaff(10, "2468")
	f.store.inactive[mappingKey{10, lifecycleTenant}] = true

	_, err := f.lifecycle.AuthenticateStaffPIN(context.Background(), lifecycleTenant, staffID, "2468")

	require.ErrorIs(t, err, domain.ErrInvalidStaffPINCredentials)
	assert.Zero(t, f.store.pinAttempts[10])
	assert.Zero(t, f.store.pinResets[10])
}

// Every miss answers with the same sentinel, so a kiosk cannot probe which
// staff ids or accounts exist.
func TestAuthenticateStaffPINHidesWhichPartIsMissing(t *testing.T) {
	t.Parallel()

	cases := map[string]func(f *lifecycleFixture, staffID int64) (int64, int64, string){
		"invalid input": func(_ *lifecycleFixture, staffID int64) (int64, int64, string) { return lifecycleTenant, staffID, "" },
		"unknown staff": func(_ *lifecycleFixture, _ int64) (int64, int64, string) { return lifecycleTenant, 424242, "2468" },
		"staff of other school": func(_ *lifecycleFixture, staffID int64) (int64, int64, string) {
			return lifecycleTenant + 1, staffID, "2468"
		},
		"inactive account": func(f *lifecycleFixture, staffID int64) (int64, int64, string) {
			account := f.store.pins[10]
			account.Active = false
			f.store.pins[10] = account
			return lifecycleTenant, staffID, "2468"
		},
		"account without PIN": func(f *lifecycleFixture, staffID int64) (int64, int64, string) {
			account := f.store.pins[10]
			account.PINHash = ""
			f.store.pins[10] = account
			return lifecycleTenant, staffID, "2468"
		},
		"person without account": func(f *lifecycleFixture, staffID int64) (int64, int64, string) {
			member := f.staff.staff[staffID]
			person := f.staff.persons[member.PersonID]
			person.AccountID = nil
			f.staff.persons[member.PersonID] = person
			return lifecycleTenant, staffID, "2468"
		},
	}
	for name, arrange := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newLifecycleFixture(t)
			staffID := f.seedPINStaff(10, "2468")
			tenantID, requested, pin := arrange(f, staffID)

			_, err := f.lifecycle.AuthenticateStaffPIN(context.Background(), tenantID, requested, pin)

			require.Equal(t, domain.ErrInvalidStaffPINCredentials, err)
			assert.Zero(t, f.store.pinAttempts[10])
		})
	}
}

func TestAuthenticateStaffPINWrapsLookupFailures(t *testing.T) {
	t.Parallel()

	f := newLifecycleFixture(t)
	staffID := f.seedPINStaff(10, "2468")
	f.staff.findStaffErr = errLifecycleBoom

	_, err := f.lifecycle.AuthenticateStaffPIN(context.Background(), lifecycleTenant, staffID, "2468")

	var operation *OperationError
	require.ErrorAs(t, err, &operation)
	assert.Equal(t, "authenticate staff PIN", operation.Op)
	require.ErrorIs(t, err, errLifecycleBoom)
	assert.NotErrorIs(t, err, domain.ErrInvalidStaffPINCredentials)
}

func TestPINLocked(t *testing.T) {
	t.Parallel()

	now := time.Now()
	past, future := now.Add(-time.Hour), now.Add(time.Hour)
	assert.False(t, domain.PINLocked(nil, now))
	assert.False(t, domain.PINLocked(&past, now))
	assert.True(t, domain.PINLocked(&future, now))
}

func TestRecordFailedPINAttemptUsesTenantLockoutSettings(t *testing.T) {
	t.Parallel()

	f := newLifecycleFixture(t)
	f.lockout.threshold = 3
	f.lockout.duration = 22 * time.Minute
	before := time.Now()

	require.NoError(t, f.lifecycle.RecordFailedPINAttempt(context.Background(), 42))

	assert.Equal(t, 3, f.store.lastLockout.threshold)
	assert.WithinDuration(t, before.Add(22*time.Minute), f.store.lastLockout.lockedUntil, 5*time.Second)
}

func TestRecordFailedPINAttemptFallsBackWhenSettingsMissing(t *testing.T) {
	t.Parallel()

	f := newLifecycleFixture(t)
	before := time.Now()

	require.NoError(t, f.lifecycle.RecordFailedPINAttempt(context.Background(), 42))

	assert.Equal(t, domain.PINLockoutThreshold, f.store.lastLockout.threshold)
	assert.WithinDuration(t, before.Add(domain.PINLockoutDuration), f.store.lastLockout.lockedUntil, 5*time.Second)
}

func TestRepeatedWrongPINsLockTheAccount(t *testing.T) {
	t.Parallel()

	f := newLifecycleFixture(t)
	f.lockout.threshold = 2
	f.lockout.duration = time.Minute
	staffID := f.seedPINStaff(10, "2468")

	for range 2 {
		_, err := f.lifecycle.AuthenticateStaffPIN(context.Background(), lifecycleTenant, staffID, "0000")
		require.Equal(t, domain.ErrInvalidStaffPINCredentials, err)
	}
	_, err := f.lifecycle.AuthenticateStaffPIN(context.Background(), lifecycleTenant, staffID, "2468")
	require.Equal(t, domain.ErrStaffPINLocked, err, "the correct PIN is refused while the lockout runs")
}

func TestStaffPINSelfService(t *testing.T) {
	t.Parallel()

	t.Run("status reports a missing account", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		_, _, err := f.lifecycle.StaffPINStatus(context.Background(), 404)
		require.Equal(t, domain.ErrStaffPINAccountNotFound, err)
	})

	t.Run("status reports the PIN and its last change", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		f.seedPINStaff(10, "2468")
		hasPIN, changed, err := f.lifecycle.StaffPINStatus(context.Background(), 10)
		require.NoError(t, err)
		assert.True(t, hasPIN)
		require.NotNil(t, changed)
		assert.Equal(t, f.store.pins[10].UpdatedAt, *changed)

		account := f.store.pins[10]
		account.PINHash = ""
		f.store.pins[10] = account
		hasPIN, changed, err = f.lifecycle.StaffPINStatus(context.Background(), 10)
		require.NoError(t, err)
		assert.False(t, hasPIN)
		assert.Nil(t, changed)
	})

	t.Run("preflight refuses a locked account", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		f.seedPINStaff(10, "2468")
		require.NoError(t, f.lifecycle.StaffPINPreflight(context.Background(), 10))
		lockedUntil := time.Now().Add(time.Minute)
		account := f.store.pins[10]
		account.PINLockedUntil = &lockedUntil
		f.store.pins[10] = account
		require.Equal(t, domain.ErrStaffPINSelfServiceLocked, f.lifecycle.StaffPINPreflight(context.Background(), 10))
		require.Equal(t, domain.ErrStaffPINAccountNotFound, f.lifecycle.StaffPINPreflight(context.Background(), 404))
	})

	t.Run("an existing PIN must be confirmed", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		f.seedPINStaff(10, "2468")
		empty := ""
		require.Equal(t, domain.ErrStaffPINCurrentRequired, f.lifecycle.ChangeStaffPIN(tenantContext(), 10, nil, "1357"))
		require.Equal(t, domain.ErrStaffPINCurrentRequired, f.lifecycle.ChangeStaffPIN(tenantContext(), 10, &empty, "1357"))
		assert.Equal(t, "pin:2468", f.store.pins[10].PINHash)
	})

	t.Run("a wrong current PIN counts towards the lockout", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		f.seedPINStaff(10, "2468")
		wrong := "0000"
		require.Equal(t, domain.ErrStaffPINCurrentWrong, f.lifecycle.ChangeStaffPIN(tenantContext(), 10, &wrong, "1357"))
		assert.Equal(t, 1, f.store.pinAttempts[10])
		assert.Equal(t, "pin:2468", f.store.pins[10].PINHash)
	})

	t.Run("a verified change stores the new PIN and clears the lockout", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		f.seedPINStaff(10, "2468")
		f.store.pinAttempts[10] = 2
		current := "2468"
		require.NoError(t, f.lifecycle.ChangeStaffPIN(tenantContext(), 10, &current, "1357"))
		assert.Equal(t, "pin:1357", f.store.pins[10].PINHash)
		assert.Zero(t, f.store.pinAttempts[10])
		assert.Equal(t, 1, f.store.pinResets[10])
	})

	t.Run("a first PIN needs no confirmation", func(t *testing.T) {
		t.Parallel()
		f := newLifecycleFixture(t)
		f.seedPINStaff(10, "2468")
		account := f.store.pins[10]
		account.PINHash = ""
		f.store.pins[10] = account
		require.NoError(t, f.lifecycle.ChangeStaffPIN(tenantContext(), 10, nil, "1357"))
		assert.Equal(t, "pin:1357", f.store.pins[10].PINHash)
	})
}
