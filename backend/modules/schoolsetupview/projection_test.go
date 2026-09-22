package schoolsetupview_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolsetupview"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// ambientTransaction resolves the ambient transaction exactly like the
// Settings Platform composition does.
func ambientTransaction(ctx context.Context) (bun.IDB, error) {
	transaction, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, errors.New("transaction is required")
	}
	tx, ok := transaction.(bun.Tx)
	if !ok {
		return nil, fmt.Errorf("unsupported transaction %T", transaction)
	}
	return tx, nil
}

func progress(t *testing.T, db *bun.DB, tenantID int64) schoolsetupview.Facts {
	t.Helper()
	projection := schoolsetupview.New(ambientTransaction)
	var facts schoolsetupview.Facts
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, tenantID, func(ctx context.Context) error {
		var err error
		facts, err = projection.Progress(ctx, tenantID)
		return err
	}))
	return facts
}

func TestProgressOfANewSchoolIsEmpty(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	assert.Equal(t, schoolsetupview.Facts{}, progress(t, db, testpkg.Tenant(t)))
}

func TestProgressReportsTheSchoolsOwnData(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	admin := testpkg.CreateTestAccount(t, db, "setup-admin")
	role := testpkg.CreateTestRole(t, db, "setup-staff")
	testpkg.CreateTestInvitationToken(t, db, "setup-staff", role.ID, admin.ID, time.Now().Add(48*time.Hour))
	testpkg.CreateTestRoom(t, db, "Setup Raum")
	testpkg.CreateTestEducationGroup(t, db, "Setup Gruppe")
	testpkg.CreateTestStudent(t, db, "Setup", "Kind", "1a")
	guardian := testpkg.CreateTestGuardianProfile(t, db, "setup-guardian")
	testpkg.InsertTestGuardianInvitation(t, db, &testpkg.GuardianInvitation{
		Token:             fmt.Sprintf("setup-guardian-%d", guardian.ID),
		GuardianProfileID: guardian.ID,
		CreatedBy:         admin.ID,
	})

	assert.Equal(t, schoolsetupview.Facts{
		StaffInvited:    true,
		RoomCreated:     true,
		GroupCreated:    true,
		StudentEnrolled: true,
		GuardianInvited: true,
	}, progress(t, db, testpkg.Tenant(t)))
}

// TestProgressIgnoresRowsTheSchoolDidNotCreate pins the two rows that exist in
// a freshly provisioned school without any admin work: the operator's
// invitation of the first admin (no creator) and the system rooms moto
// creates itself (Schulhof, WC).
func TestProgressIgnoresRowsTheSchoolDidNotCreate(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)

	role := testpkg.CreateTestRole(t, db, "setup-first-admin")
	testpkg.CreateTestInvitationToken(t, db, "setup-first-admin", role.ID, 0, time.Now().Add(48*time.Hour))
	room := testpkg.CreateTestRoom(t, db, "Schulhof")
	_, err := db.NewUpdate().
		TableExpr("facilities.rooms").
		Set("is_system = TRUE").
		Where("id = ?", room.ID).
		Exec(context.Background())
	require.NoError(t, err)

	facts := progress(t, db, testpkg.Tenant(t))
	assert.False(t, facts.StaffInvited, "the operator's invitation must not finish the team step")
	assert.False(t, facts.RoomCreated, "a system room must not finish the room step")
}

// TestProgressIsTenantScoped pins the tenant_safe invariant of ADR 0040: data
// of another school never counts.
func TestProgressIsTenantScoped(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ownTenant := testpkg.Tenant(t)

	t.Run("other school", func(t *testing.T) {
		testpkg.OwnTenant(t)
		testpkg.CreateTestRoom(t, db, "Fremder Raum")
		testpkg.CreateTestEducationGroup(t, db, "Fremde Gruppe")
		testpkg.CreateTestStudent(t, db, "Fremdes", "Kind", "2b")
	})

	assert.Equal(t, schoolsetupview.Facts{}, progress(t, db, ownTenant))
}

func TestProgressRequiresTenantAndTransaction(t *testing.T) {
	t.Parallel()
	projection := schoolsetupview.New(ambientTransaction)

	_, err := projection.Progress(context.Background(), 0)
	require.Error(t, err)

	_, err = projection.Progress(context.Background(), testpkg.Tenant(t))
	require.ErrorContains(t, err, "transaction is required")
}
