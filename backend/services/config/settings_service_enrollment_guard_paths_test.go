package config

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file covers the defensive and operational-failure branches of the three
// #1663 enrollment probes wired onto the settings service. The happy paths and
// the rejection messages live in settings_service_test.go; what is exercised
// here is the behaviour that separates a "your value is invalid" 400 from an
// operational 500, plus the early returns that keep an unwired or non-comparable
// write from reaching a probe at all.

// errGuardProbe is the sentinel a failing probe returns so the assertions can
// prove the original cause is wrapped rather than swallowed.
var errGuardProbe = errors.New("phase probe query failed")

// setFailingGradeCapGuard wires a highest-restricted-grade probe that fails,
// standing in for a broken phase query rather than a real grade restriction.
func setFailingGradeCapGuard(t *testing.T, svc SettingsService, err error) {
	t.Helper()
	guarded, ok := svc.(interface {
		SetGradeCapGuard(func(context.Context) (int, error))
	})
	require.True(t, ok, "settings service must expose SetGradeCapGuard")
	guarded.SetGradeCapGuard(func(context.Context) (int, error) { return 0, err })
}

// setFailingClassRestrictionGuard is the class-probe counterpart of
// setFailingGradeCapGuard.
func setFailingClassRestrictionGuard(t *testing.T, svc SettingsService, err error) {
	t.Helper()
	guarded, ok := svc.(interface {
		SetClassRestrictionGuard(func(context.Context) (bool, error))
	})
	require.True(t, ok, "settings service must expose SetClassRestrictionGuard")
	guarded.SetClassRestrictionGuard(func(context.Context) (bool, error) { return false, err })
}

// setFailingGradeRestrictionGuard is the grade-probe counterpart of
// setFailingGradeCapGuard.
func setFailingGradeRestrictionGuard(t *testing.T, svc SettingsService, err error) {
	t.Helper()
	guarded, ok := svc.(interface {
		SetGradeRestrictionGuard(func(context.Context) (bool, error))
	})
	require.True(t, ok, "settings service must expose SetGradeRestrictionGuard")
	guarded.SetGradeRestrictionGuard(func(context.Context) (bool, error) { return false, err })
}

// assertOperationalFailure proves an error is the operational kind: the probe's
// cause is wrapped, and it is NOT an InvalidValueError — the API layer maps the
// latter to 400, which would wrongly tell the admin their value was invalid when
// in truth the check could not run.
func assertOperationalFailure(t *testing.T, err error, wantMessage string) {
	t.Helper()
	require.Error(t, err)
	assert.Contains(t, err.Error(), wantMessage)
	assert.ErrorIs(t, err, errGuardProbe, "the probe's cause must stay wrapped for the 500 log")

	var invalid *InvalidValueError
	assert.False(t, errors.As(err, &invalid),
		"a failed probe is an operational failure (500), not an invalid value (400)")
}

// TestSetValue_GradeLevelCapGuard_UnreachableProbePaths covers the two branches
// that return before the cap probe runs at all.
func TestSetValue_GradeLevelCapGuard_UnreachableProbePaths(t *testing.T) {
	t.Run("no cap guard wired leaves the cap free", func(t *testing.T) {
		setupTest(t)
		registerTestSetting(config.KeyEnrollmentGradeLevelMax, config.FieldNumber, 4)
		svc := createService(newMockValueRepo(), &mockAuditRepo{})

		require.NoError(t, svc.SetValue(tenantCtx(1), config.KeyEnrollmentGradeLevelMax, 1, nil, nil),
			"without the enrollment probe the settings service must not block the cap")
	})

	t.Run("a non-numeric cap value is left to per-field validation", func(t *testing.T) {
		setupTest(t)
		// Registered as text so validateValue accepts a string and the write
		// reaches the cross-field guard, which has nothing numeric to compare.
		registerTestSetting(config.KeyEnrollmentGradeLevelMax, config.FieldText, "4")
		svc := createService(newMockValueRepo(), &mockAuditRepo{})
		setGradeCapGuard(t, svc, 6)

		require.NoError(t, svc.SetValue(tenantCtx(1), config.KeyEnrollmentGradeLevelMax, "vier", nil, nil),
			"a value the guard cannot compare numerically must not be rejected by the guard")
	})
}

// TestSetValue_GradeLevelCapGuard_ProbeFailureIsOperational pins that a broken
// cap probe surfaces as a 500, not as a rejection of the admin's value.
func TestSetValue_GradeLevelCapGuard_ProbeFailureIsOperational(t *testing.T) {
	setupTest(t)
	registerTestSetting(config.KeyEnrollmentGradeLevelMax, config.FieldNumber, 4)
	svc := createService(newMockValueRepo(), &mockAuditRepo{})
	setFailingGradeCapGuard(t, svc, errGuardProbe)

	err := svc.SetValue(tenantCtx(1), config.KeyEnrollmentGradeLevelMax, 1, nil, nil)
	assertOperationalFailure(t, err, "check grade-restricted phases")
}

// TestSetValue_ClassCollectionGuard_NonBoolValueSkipsComparison covers the
// defensive early return for a value the toggle guard cannot interpret.
func TestSetValue_ClassCollectionGuard_NonBoolValueSkipsComparison(t *testing.T) {
	setupTest(t)
	// Text-typed so a string survives validateValue and reaches the guard.
	registerTestSetting(config.KeyEnrollmentCollectSchoolClass, config.FieldText, "true")
	registerTestSetting(config.KeyEnrollmentCollectGradeLevel, config.FieldBoolean, true)
	svc := createService(newMockValueRepo(), &mockAuditRepo{})
	setClassRestrictionGuard(t, svc, true)

	require.NoError(t, svc.SetValue(tenantCtx(1), config.KeyEnrollmentCollectSchoolClass, "nein", nil, nil),
		"a non-bool toggle value carries no on/off decision for the guard to act on")
}

// TestSetValue_ClassCollectionGuard_ProbeFailuresAreOperational covers both
// probe failures reachable from the collection guard.
func TestSetValue_ClassCollectionGuard_ProbeFailuresAreOperational(t *testing.T) {
	registerCollectionKeys := func() {
		registerTestSetting(config.KeyEnrollmentCollectGradeLevel, config.FieldBoolean, true)
		registerTestSetting(config.KeyEnrollmentCollectSchoolClass, config.FieldBoolean, true)
	}

	t.Run("a failing grade probe blocks the grade toggle operationally", func(t *testing.T) {
		setupTest(t)
		registerCollectionKeys()
		svc := createService(newMockValueRepo(), &mockAuditRepo{})
		setFailingGradeRestrictionGuard(t, svc, errGuardProbe)

		err := svc.SetValue(tenantCtx(1), config.KeyEnrollmentCollectGradeLevel, false, nil, nil)
		assertOperationalFailure(t, err, "check grade-restricted phases")
	})

	t.Run("a failing class probe blocks the class toggle operationally", func(t *testing.T) {
		setupTest(t)
		registerCollectionKeys()
		svc := createService(newMockValueRepo(), &mockAuditRepo{})
		setFailingClassRestrictionGuard(t, svc, errGuardProbe)

		err := svc.SetValue(tenantCtx(1), config.KeyEnrollmentCollectSchoolClass, false, nil, nil)
		assertOperationalFailure(t, err, "check class-restricted phases")
	})
}

// TestSetValue_ClassCollectionGuard_UnresolvablePairedToggle pins the failure
// mode when the sibling toggle cannot be resolved. The guard needs BOTH toggles
// to decide whether collection stays effective, so it must fail loudly rather
// than assume the missing half is enabled and wave the write through.
func TestSetValue_ClassCollectionGuard_UnresolvablePairedToggle(t *testing.T) {
	setupTest(t)
	// Only the written key is registered — resolving its sibling fails.
	registerTestSetting(config.KeyEnrollmentCollectSchoolClass, config.FieldBoolean, true)
	svc := createService(newMockValueRepo(), &mockAuditRepo{})
	setClassRestrictionGuard(t, svc, true)

	err := svc.SetValue(tenantCtx(1), config.KeyEnrollmentCollectSchoolClass, false, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "resolve paired class-collection toggle")

	var invalid *InvalidValueError
	assert.False(t, errors.As(err, &invalid),
		"an unresolvable sibling toggle is operational, not an invalid value")
}

// TestLockClassCollectionPair_WithoutTransactionIsSkipped pins the best-effort
// contract of the shared advisory lock. A transaction-scoped lock taken outside
// a transaction would be released immediately and protect nothing, so it is
// skipped rather than failing — the settings and phase write paths always run
// inside a tenant transaction, where the lock does apply.
func TestLockClassCollectionPair_WithoutTransactionIsSkipped(t *testing.T) {
	setupTest(t)
	svc := createService(newMockValueRepo(), &mockAuditRepo{})

	require.NoError(t, svc.LockClassCollectionPair(tenantCtx(1)),
		"no ambient transaction means there is no xact lock to take")
}
