package users_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestStudentPrivacyConsentStoreValidation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store := repositories.NewStudentPrivacyConsentStore(db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Consent", "Validation", "1a")
	cases := []struct {
		name    string
		value   *users.PrivacyConsent
		message string
	}{
		{"nil", nil, "cannot be nil"},
		{"missing student", &users.PrivacyConsent{PolicyVersion: "v1.0", DataRetentionDays: 30}, "student ID"},
		{"missing policy", &users.PrivacyConsent{StudentID: student.ID, DataRetentionDays: 30}, "policy version"},
		{"invalid retention", &users.PrivacyConsent{StudentID: student.ID, PolicyVersion: "v1.0", DataRetentionDays: 100}, "data retention"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error { return store.Create(txCtx, tc.value) })
			require.ErrorContains(t, err, tc.message)
		})
	}
	err := tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error { return store.Update(txCtx, nil) })
	require.ErrorContains(t, err, "cannot be nil")
}

func TestStudentPrivacyConsentStoreLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store := repositories.NewStudentPrivacyConsentStore(db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Consent", "Lifecycle", "1a")
	other := testpkg.CreateTestStudent(t, db, "Consent", "Empty", "1b")
	found, err := store.FindByStudentID(ctx, other.ID)
	require.NoError(t, err)
	require.Empty(t, found)
	consent := &users.PrivacyConsent{StudentID: student.ID, PolicyVersion: "v1.0", DataRetentionDays: 30}
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error { return store.Create(txCtx, consent) }))
	require.NotZero(t, consent.ID)
	found, err = store.FindByStudentID(ctx, student.ID)
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
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error { return store.Update(txCtx, consent) }))
	found, err = store.FindByStudentID(ctx, student.ID)
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
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error { return store.Update(txCtx, consent) }))
	found, err = store.FindByStudentID(ctx, student.ID)
	require.NoError(t, err)
	require.False(t, found[0].Accepted)
}

func TestStudentPrivacyConsentStoreCreatesExpiryAndRejectsMissingRevision(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store := repositories.NewStudentPrivacyConsentStore(db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Consent", "Expiry", "1a")
	now := time.Now()
	expires := now.AddDate(1, 0, 0)
	consent := &users.PrivacyConsent{StudentID: student.ID, PolicyVersion: "v1.0", Accepted: true, AcceptedAt: &now, ExpiresAt: &expires, DataRetentionDays: 30}
	require.Error(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error { return store.Update(txCtx, consent) }))
	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error { return store.Create(txCtx, consent) }))
	require.NotZero(t, consent.ID)
	require.NotNil(t, consent.ExpiresAt)
	require.WithinDuration(t, expires, *consent.ExpiresAt, time.Microsecond)
}
