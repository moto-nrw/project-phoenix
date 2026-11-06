package compose_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

func phoneReplacementOwner(t *testing.T, db *bun.DB) peopledirectory.Owner {
	t.Helper()
	directory, err := compose.New(compose.Dependencies{
		DB: db, StudentDirectory: testpkg.StudentDirectory{}, Observe: func(compose.Observation) {},
	}, nil)
	require.NoError(t, err)
	return directory.Owner
}

func phoneSequence(t *testing.T, db *bun.DB) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`SELECT last_value FROM users.guardian_phone_numbers_id_seq`).Scan(context.Background(), &id))
	return id
}

type replacementPhone struct {
	ID          int64
	TenantID    int64
	PhoneNumber string
	PhoneType   string
	Label       *string
	IsPrimary   bool
	Priority    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func replacementPhones(t *testing.T, db *bun.DB, guardianID int64) []replacementPhone {
	t.Helper()
	var rows []replacementPhone
	require.NoError(t, db.NewRaw(`SELECT id, tenant_id, phone_number, phone_type::text, label, is_primary, priority, created_at, updated_at
 FROM users.guardian_phone_numbers WHERE guardian_profile_id = ? ORDER BY id`, guardianID).Scan(context.Background(), &rows))
	return rows
}

func replacementInput(n int) []peopledirectory.GuardianPhoneRecord {
	phones := make([]peopledirectory.GuardianPhoneRecord, n)
	for i := range phones {
		// The first two keys become identical after trimming. Both rows must survive.
		number := fmt.Sprintf(" 030 555%03d ", max(i, 1))
		label := " Büro "
		phones[i] = peopledirectory.GuardianPhoneRecord{
			PhoneNumber: number, PhoneType: peopledirectory.PhoneTypeWork,
			Label: &label, Priority: -i, IsPrimary: i%2 == 0,
		}
	}
	return phones
}

func TestGuardianPhoneReplacementPreservesOrderDuplicatesAndEmpty(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t) // The sequence is database-global.
	owner := phoneReplacementOwner(t, db)
	ctx := testpkg.Ctx(t)
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, testpkg.Tenant(t), "Order", "Phone", "replace-order")
	_, err := owner.AddGuardianPhoneRecord(ctx, guardian.ID, peopledirectory.GuardianPhoneRecord{
		PhoneNumber: "030 999", PhoneType: peopledirectory.PhoneTypeHome,
	})
	require.NoError(t, err)
	for _, n := range []int{3, 8} {
		before := phoneSequence(t, db)
		require.NoError(t, owner.ReplaceGuardianPhones(ctx, guardian.ID, replacementInput(n)))
		rows := replacementPhones(t, db, guardian.ID)
		require.Len(t, rows, n)
		for i, row := range rows {
			assert.Equal(t, before+int64(i)+1, row.ID)
			assert.Equal(t, testpkg.Tenant(t), row.TenantID)
			assert.Equal(t, fmt.Sprintf("030 555%03d", max(i, 1)), row.PhoneNumber)
			assert.Equal(t, "work", row.PhoneType)
			require.NotNil(t, row.Label)
			assert.Equal(t, "Büro", *row.Label)
			assert.Equal(t, 1, row.Priority)
			assert.Equal(t, i%2 == 0, row.IsPrimary)
			assert.Equal(t, rows[0].CreatedAt, row.CreatedAt)
			assert.Equal(t, row.CreatedAt, row.UpdatedAt)
		}
		assert.Equal(t, before+int64(n), phoneSequence(t, db))
		invalid := append(replacementInput(n), peopledirectory.GuardianPhoneRecord{PhoneNumber: "invalid", PhoneType: peopledirectory.PhoneTypeHome})
		require.ErrorIs(t, owner.ReplaceGuardianPhones(ctx, guardian.ID, invalid), peopledirectory.ErrInvalidGuardian)
		assert.Equal(t, rows, replacementPhones(t, db, guardian.ID), "normalization finishes before any deletion")
		assert.Equal(t, before+int64(n), phoneSequence(t, db))
		t.Logf("N%d: ordered IDs consumed=%d; normalized duplicate keys retained; invalid replacement writes nothing", n, n)
	}
	before := phoneSequence(t, db)
	require.NoError(t, owner.ReplaceGuardianPhones(ctx, guardian.ID, nil))
	assert.Empty(t, replacementPhones(t, db, guardian.ID))
	assert.Equal(t, before, phoneSequence(t, db), "empty replacement deletes without consuming IDs")
}

// PostgreSQL sequences do not roll back. A rejected first row must not consume
// the IDs of later rows, even though the whole phone replacement rolls back.
func TestGuardianPhoneReplacementForeignKeyConsumesOnlyFirstID(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	owner := phoneReplacementOwner(t, db)
	ctx := testpkg.Ctx(t)
	guardian := testpkg.CreateTestGuardianProfileForTenant(t, db, testpkg.Tenant(t), "Failure", "Phone", "replace-failure")
	first, err := owner.AddGuardianPhoneRecord(ctx, guardian.ID, peopledirectory.GuardianPhoneRecord{
		PhoneNumber: "030 111", PhoneType: peopledirectory.PhoneTypeHome,
	})
	require.NoError(t, err)
	original := replacementPhones(t, db, guardian.ID)
	var missing int64
	require.NoError(t, db.NewRaw(`SELECT nextval('users.guardian_profiles_id_seq')`).Scan(ctx, &missing))
	before := phoneSequence(t, db)
	require.Equal(t, first, before)
	err = owner.ReplaceGuardianPhones(ctx, missing, replacementInput(3))
	require.Error(t, err)
	var pgErr pgdriver.Error
	require.True(t, errors.As(err, &pgErr), "the real FK database error stays in the public capability's error chain")
	assert.Equal(t, "23503", pgErr.Field('C'))
	assert.Equal(t, "people directory postgres: add guardian phone: "+pgErr.Error(), err.Error())
	assert.NotEmpty(t, pgErr.Field('n'))
	assert.NotEmpty(t, pgErr.Field('D'))
	assert.Equal(t, original, replacementPhones(t, db, guardian.ID))
	assert.Empty(t, replacementPhones(t, db, missing))
	after := phoneSequence(t, db)
	t.Logf("real FK failure: code=%s constraint=%s detail=%s sequence_delta=%d error=%s", pgErr.Field('C'), pgErr.Field('n'), pgErr.Field('D'), after-before, err)
	require.Equal(t, before+1, after, "the first rejected insert consumes exactly one ID, never the remaining input IDs")
	next, err := owner.AddGuardianPhoneRecord(ctx, guardian.ID, peopledirectory.GuardianPhoneRecord{
		PhoneNumber: "030 222", PhoneType: peopledirectory.PhoneTypeMobile,
	})
	require.NoError(t, err)
	assert.Equal(t, first+2, next, "the next successful public write exposes exactly one failed-row sequence gap")
}
