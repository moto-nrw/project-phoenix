package compose_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The three tests below moved here with users.privacy_consents' application
// logic (#3349). They previously drove the retired
// database/repositories consent adapter; the owner capability is now the only
// path to the table.

func TestPrivacyConsentRejectsIncompleteRecords(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Consent", "Validation", "1a")
	cases := []struct {
		name  string
		value studentpresence.PrivacyConsent
	}{
		{"missing student", studentpresence.PrivacyConsent{PolicyVersion: "v1.0", DataRetentionDays: 30}},
		{"missing policy", studentpresence.PrivacyConsent{StudentID: student.ID, DataRetentionDays: 30}},
		{"invalid retention", studentpresence.PrivacyConsent{StudentID: student.ID, PolicyVersion: "v1.0", DataRetentionDays: 100}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
				_, err := module.RecordPrivacyConsent(txCtx, tc.value)
				return err
			})
			require.ErrorIs(t, err, studentpresence.ErrInvalidPrivacyConsent)
		})
	}
}

func TestPrivacyConsentLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Consent", "Lifecycle", "1a")
	other := testpkg.CreateTestStudent(t, db, "Consent", "Empty", "1b")
	found, err := module.ListPrivacyConsents(ctx, other.ID)
	require.NoError(t, err)
	require.Empty(t, found)

	var consent studentpresence.PrivacyConsent
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		consent, err = module.RecordPrivacyConsent(txCtx, studentpresence.PrivacyConsent{
			StudentID: student.ID, PolicyVersion: "v1.0", DataRetentionDays: 30,
		})
		return err
	}))
	require.NotZero(t, consent.ID)
	found, err = module.ListPrivacyConsents(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, found, 1)
	require.Equal(t, consent.ID, found[0].ID)
	require.Equal(t, student.ID, found[0].StudentID)
	require.Equal(t, "v1.0", found[0].PolicyVersion)
	require.False(t, found[0].Accepted)

	now := time.Now()
	expires := now.AddDate(1, 0, 0)
	consent.Accepted = true
	consent.AcceptedAt = &now
	consent.ExpiresAt = &expires
	consent.RenewalRequired = true
	consent.DataRetentionDays = 15
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, err := module.RevisePrivacyConsent(txCtx, consent)
		return err
	}))
	found, err = module.ListPrivacyConsents(ctx, student.ID)
	require.NoError(t, err)
	require.Len(t, found, 1)
	require.True(t, found[0].Accepted)
	require.NotNil(t, found[0].AcceptedAt)
	require.WithinDuration(t, now, *found[0].AcceptedAt, time.Microsecond)
	require.NotNil(t, found[0].ExpiresAt)
	require.WithinDuration(t, expires, *found[0].ExpiresAt, time.Microsecond)
	require.True(t, found[0].RenewalRequired)
	require.Equal(t, 15, found[0].DataRetentionDays)

	consent.Accepted = false
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, err := module.RevisePrivacyConsent(txCtx, consent)
		return err
	}))
	found, err = module.ListPrivacyConsents(ctx, student.ID)
	require.NoError(t, err)
	require.False(t, found[0].Accepted)
}

func TestPrivacyConsentKeepsExpiryAndRejectsMissingRevision(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Consent", "Expiry", "1a")
	now := time.Now()
	expires := now.AddDate(1, 0, 0)
	value := studentpresence.PrivacyConsent{
		StudentID: student.ID, PolicyVersion: "v1.0", Accepted: true,
		AcceptedAt: &now, ExpiresAt: &expires, DataRetentionDays: 30,
	}
	require.Error(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, err := module.RevisePrivacyConsent(txCtx, value)
		return err
	}))
	var recorded studentpresence.PrivacyConsent
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		recorded, err = module.RecordPrivacyConsent(txCtx, value)
		return err
	}))
	require.NotZero(t, recorded.ID)
	require.NotNil(t, recorded.ExpiresAt)
	require.WithinDuration(t, expires, *recorded.ExpiresAt, time.Microsecond)
}
