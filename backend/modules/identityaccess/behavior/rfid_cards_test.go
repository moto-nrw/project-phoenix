package behavior_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestRFIDCardsRegistrationAndIsolation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	cards, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	tag := fmt.Sprintf("ABCD%016X", testpkg.UniqueTestTenantID(t))
	rawTag := " ab:cd-" + strings.ToLower(tag[4:]) + " "
	require.NoError(t, cards.RegisterRFIDCard(ctx, rawTag))
	id, active, found, err := cards.LookupRFIDCard(ctx, rawTag)
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, active)
	require.Equal(t, tag, id)
	require.NoError(t, cards.ValidateRFIDTag(rawTag))
	require.Error(t, cards.RegisterRFIDCard(ctx, tag), "registration does not overwrite an existing card")

	_, err = db.NewRaw(`UPDATE users.rfid_cards SET active = FALSE WHERE id = ? AND tenant_id = ?`, tag, testpkg.Tenant(t)).Exec(ctx)
	require.NoError(t, err)
	_, active, found, err = cards.LookupRFIDCard(ctx, tag)
	require.NoError(t, err)
	require.True(t, found, "inactive cards still exist")
	require.False(t, active)

	otherSchool := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherSchool)
	_, _, found, err = cards.LookupRFIDCard(tenant.WithTenantID(ctx, otherSchool), tag)
	require.NoError(t, err)
	require.False(t, found, "a card from another school is not visible")
	_, _, _, err = cards.LookupRFIDCard(context.Background(), tag)
	require.ErrorIs(t, err, identityaccess.ErrTenantRequired)
	require.ErrorIs(t, cards.RegisterRFIDCard(context.Background(), tag), identityaccess.ErrTenantRequired)

	rolledBackTag := fmt.Sprintf("ABCD%016X", testpkg.UniqueTestTenantID(t))
	rollback := errors.New("roll back RFID registration")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		require.NoError(t, cards.RegisterRFIDCard(txCtx, rolledBackTag))
		_, _, visible, readErr := cards.LookupRFIDCard(txCtx, rolledBackTag)
		require.NoError(t, readErr)
		require.True(t, visible)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	_, _, found, err = cards.LookupRFIDCard(ctx, rolledBackTag)
	require.NoError(t, err)
	require.False(t, found)

	legacy := testpkg.CreateTestRFIDCard(t, db, "LEGACY")
	_, _, found, err = cards.LookupRFIDCard(ctx, legacy.ID)
	require.NoError(t, err)
	require.True(t, found, "lookups retain old non-hex identifiers")
	require.ErrorContains(t, cards.ValidateRFIDTag(legacy.ID), "must be hexadecimal")
	require.ErrorContains(t, cards.RegisterRFIDCard(ctx, "invalid-tag"), "must be hexadecimal")
}

func TestRFIDCardsDatabaseFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	cards, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	ctx := tenant.WithUnitOfWork(testpkg.TenantContext(testpkg.Tenant(t)), testpkg.TenantRuntime(t, db))
	require.NoError(t, db.Close())
	_, _, _, err = cards.LookupRFIDCard(ctx, "ABCD1234")
	require.ErrorContains(t, err, "database is closed")
	require.ErrorContains(t, cards.RegisterRFIDCard(ctx, "ABCD1234"), "database is closed")
}
