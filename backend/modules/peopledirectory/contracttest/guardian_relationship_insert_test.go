package contracttest_test

import (
	"context"
	"errors"
	"testing"
	"time"

	people "github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

type guardianRetentionState struct {
	ChildlessSince *time.Time
	UpdatedAt      time.Time
}

func retentionState(t *testing.T, db *bun.DB, guardianID int64) guardianRetentionState {
	t.Helper()
	var state guardianRetentionState
	require.NoError(t, db.NewRaw(`SELECT childless_since, updated_at FROM users.guardian_profiles WHERE id = ?`, guardianID).Scan(context.Background(), &state))
	return state
}

func markRetainedGuardian(t *testing.T, db *bun.DB, guardianID int64) guardianRetentionState {
	t.Helper()
	stamp := time.Date(2025, time.January, 2, 3, 4, 5, 0, time.UTC)
	_, err := db.NewRaw(`UPDATE users.guardian_profiles SET childless_since = ? WHERE id = ?`, stamp, guardianID).Exec(context.Background())
	require.NoError(t, err)
	return retentionState(t, db, guardianID)
}

func nonPrimaryLink(studentID, guardianID int64) *people.GuardianRelationshipSnapshot {
	link := newLink(studentID, guardianID)
	link.IsPrimary = false
	return link
}

func TestNonPrimaryGuardianRelationshipRetentionAndConflict(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Retention", "Child", "1a")
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, testpkg.Tenant(t), "Retention", "Guardian", "relationship-retention")
	store := newRelationshipStore(db, &sqlGuardianOwners{db: db})
	markRetainedGuardian(t, db, guardian.ID)
	link := nonPrimaryLink(student.ID, guardian.ID)
	require.NoError(t, store.Create(ctx, link))
	require.Positive(t, link.ID)
	require.False(t, link.CreatedAt.IsZero())
	require.Equal(t, link.CreatedAt, link.UpdatedAt)
	require.Nil(t, retentionState(t, db, guardian.ID).ChildlessSince)
	saved, err := testpkg.TenantQuery1(t, ctx, db, store.FindByID, any(link.ID))
	require.NoError(t, err)
	require.Equal(t, link.GuardianRole, saved.GuardianRole)
	require.Equal(t, link.CanPickup, saved.CanPickup)
	require.Equal(t, link.PickupNotes, saved.PickupNotes)
	require.JSONEq(t, string(link.Permissions), string(saved.Permissions))

	before := markRetainedGuardian(t, db, guardian.ID)
	duplicate := nonPrimaryLink(student.ID, guardian.ID)
	duplicate.GuardianRole = "pickup_only"
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
		err := store.Create(ctx, duplicate)
		var pgErr pgdriver.Error
		require.ErrorAs(t, err, &pgErr)
		require.Equal(t, "23505", pgErr.Field('C'))
		return nil // A handled failure must roll back the retention write too.
	}))
	require.Equal(t, before, retentionState(t, db, guardian.ID))
	inserted, err := store.LinkIfNotExists(ctx, duplicate)
	require.NoError(t, err)
	require.False(t, inserted)
	require.Zero(t, duplicate.ID)
	require.Nil(t, retentionState(t, db, guardian.ID).ChildlessSince)
	after, err := testpkg.TenantQuery1(t, ctx, db, store.FindByID, any(link.ID))
	require.NoError(t, err)
	require.Equal(t, saved, after, "conflict leaves all three owner halves and timestamps unchanged")
	// Even an already-NULL retention mark still performs the UPDATE and fires
	// the existing updated_at trigger with the caller's transaction timestamp.
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
		var stamp time.Time
		require.NoError(t, tx.NewRaw(`SELECT transaction_timestamp()`).Scan(ctx, &stamp))
		inserted, err := store.LinkIfNotExists(ctx, nonPrimaryLink(student.ID, guardian.ID))
		require.NoError(t, err)
		require.False(t, inserted)
		var updated time.Time
		require.NoError(t, tx.NewRaw(`SELECT updated_at FROM users.guardian_profiles WHERE id = ?`, guardian.ID).Scan(ctx, &updated))
		require.Equal(t, stamp, updated)
		return nil
	}))
}

func TestNonPrimaryGuardianRelationshipErrorsAndRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	school := testpkg.Tenant(t)
	student := testpkg.CreateTestStudent(t, db, "Rollback", "Child", "1a")
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, testpkg.Tenant(t), "Rollback", "Guardian", "relationship-rollback")
	before := markRetainedGuardian(t, db, guardian.ID)
	var missingGuardian, missingStudent int64
	require.NoError(t, db.NewRaw(`SELECT nextval('users.guardian_profiles_id_seq'), nextval(pg_get_serial_sequence('users.student_profiles', 'id'))`).Scan(ctx, &missingGuardian, &missingStudent))
	store := newRelationshipStore(db, &sqlGuardianOwners{db: db})
	for _, ifAbsent := range []bool{false, true} {
		write := func(link *people.GuardianRelationshipSnapshot) error {
			if ifAbsent {
				_, err := store.LinkIfNotExists(ctx, link)
				return err
			}
			return store.Create(ctx, link)
		}
		require.ErrorIs(t, write(nonPrimaryLink(missingStudent, missingGuardian)), people.ErrGuardianNotFound, "missing guardian wins over missing child")
		foreign, _ := testpkg.CreateTestTenant(t, db)
		foreignGuardian := testpkg.CreateTestGuardianProfileForTenant(t, db, foreign, "Foreign", "Guardian", "relationship-foreign")
		foreignState := markRetainedGuardian(t, db, foreignGuardian.ID)
		link := nonPrimaryLink(student.ID, foreignGuardian.ID)
		require.ErrorIs(t, write(link), people.ErrGuardianNotFound, "explicit tenant filter hides foreign guardian")
		link.TenantID = foreign
		require.ErrorIs(t, write(link), people.ErrGuardianNotFound, "RLS hides foreign guardian even when row tenant names it")
		require.Equal(t, foreignState, retentionState(t, db, foreignGuardian.ID))
		err := write(nonPrimaryLink(missingStudent, guardian.ID))
		var pgErr pgdriver.Error
		require.ErrorAs(t, err, &pgErr)
		require.Equal(t, "23503", pgErr.Field('C'))
		require.Equal(t, before, retentionState(t, db, guardian.ID))
	}
	for _, owners := range []*sqlGuardianOwners{{db: db, failPickupCreate: true}, {db: db, failAccessGrant: true}} {
		failing := newRelationshipStore(db, owners)
		require.NoError(t, testpkg.WithTenantTx(t, ctx, db, school, func(ctx context.Context, _ bun.Tx) error {
			_, err := failing.LinkIfNotExists(ctx, nonPrimaryLink(student.ID, guardian.ID))
			require.ErrorIs(t, err, errInjectedOwnerFailure)
			return nil
		}))
		require.Equal(t, before, retentionState(t, db, guardian.ID))
		relationships, pickups, accesses := linkHalves(t, db, student.ID, guardian.ID)
		require.Zero(t, relationships+pickups+accesses)
	}
	rollback := errors.New("outer rollback")
	err := testpkg.WithTenantTx(t, ctx, db, school, func(ctx context.Context, _ bun.Tx) error {
		inserted, err := store.LinkIfNotExists(ctx, nonPrimaryLink(student.ID, guardian.ID))
		require.NoError(t, err)
		require.True(t, inserted)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	require.Equal(t, before, retentionState(t, db, guardian.ID))
	relationships, pickups, accesses := linkHalves(t, db, student.ID, guardian.ID)
	require.Zero(t, relationships+pickups+accesses)
}

func TestNonPrimaryGuardianRelationshipWaitsForGuardianVersion(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	for _, deleted := range []bool{false, true} {
		name := "updated account"
		if deleted {
			name = "deleted guardian"
		}
		t.Run(name, func(t *testing.T) {
			ctx := testpkg.Ctx(t)
			school := testpkg.Tenant(t)
			student := testpkg.CreateTestStudent(t, db, "Wait", "Child", "1a")
			guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, school, "Wait", "Guardian", "relationship-wait")
			account := testpkg.CreateTestAccount(t, db, "relationship-wait-account")
			store := newRelationshipStore(db, &sqlGuardianOwners{db: db})
			holder, err := db.BeginTx(ctx, nil)
			require.NoError(t, err)
			t.Cleanup(func() { _ = holder.Rollback() })
			var holderPID int
			require.NoError(t, holder.NewRaw(`SELECT pg_backend_pid()`).Scan(ctx, &holderPID))
			if deleted {
				_, err = holder.NewRaw(`DELETE FROM users.guardian_profiles WHERE id = ?`, guardian.ID).Exec(ctx)
			} else {
				_, err = holder.NewRaw(`UPDATE users.guardian_profiles SET account_id = ?, has_account = TRUE, childless_since = NOW() WHERE id = ?`, account.ID, guardian.ID).Exec(ctx)
			}
			require.NoError(t, err)
			type result struct {
				link  *people.GuardianRelationshipSnapshot
				stamp time.Time
				err   error
			}
			done := make(chan result, 1)
			go func() {
				defer close(done)
				output := result{link: nonPrimaryLink(student.ID, guardian.ID)}
				output.err = testpkg.WithTenantTx(t, ctx, db, school, func(ctx context.Context, tx bun.Tx) error {
					if err := tx.NewRaw(`SELECT transaction_timestamp()`).Scan(ctx, &output.stamp); err != nil {
						return err
					}
					inserted, err := store.LinkIfNotExists(ctx, output.link)
					if err == nil && !inserted {
						return errors.New("new pair was not inserted")
					}
					return err
				})
				done <- output
			}()
			t.Cleanup(func() { _ = holder.Rollback(); <-done })
			require.Eventually(t, func() bool {
				var waiting bool
				err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE ?::int = ANY (pg_blocking_pids(pid)))`, holderPID).Scan(ctx, &waiting)
				return err == nil && waiting
			}, 5*time.Second, 10*time.Millisecond, "link must wait for the guardian before inserting the relationship")
			relationships, pickups, accesses := linkHalves(t, db, student.ID, guardian.ID)
			require.Zero(t, relationships+pickups+accesses)
			require.NoError(t, holder.Commit())
			select {
			case output := <-done:
				if deleted {
					require.ErrorIs(t, output.err, people.ErrGuardianNotFound)
					relationships, pickups, accesses := linkHalves(t, db, student.ID, guardian.ID)
					require.Zero(t, relationships+pickups+accesses)
					return
				}
				require.NoError(t, output.err)
				var linkedAccount *int64
				require.NoError(t, db.NewRaw(`SELECT account_id FROM auth.guardian_student_access WHERE relationship_id = ?`, output.link.ID).Scan(ctx, &linkedAccount))
				require.NotNil(t, linkedAccount)
				require.Equal(t, account.ID, *linkedAccount, "bind the committed guardian version, not the pre-wait statement snapshot")
				state := retentionState(t, db, guardian.ID)
				require.Nil(t, state.ChildlessSince)
				require.Equal(t, output.stamp, state.UpdatedAt)
			case <-time.After(10 * time.Second):
				t.Fatal("link did not finish after guardian commit")
			}
		})
	}
}
