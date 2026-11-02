package migrations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func careProfileColumnExists(t *testing.T, ctx context.Context, db *bun.DB, column string) bool {
	t.Helper()
	var exists bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_schema = 'users' AND table_name = 'student_care_profiles' AND column_name = ?)`, column).Scan(ctx, &exists))
	return exists
}

// TestStudentHealthDataSealsEveryHealthField runs the migration of #5472 over
// children written before it: afterwards no health field stays in plaintext,
// each sealed value opens only with the context of its care row, a child
// without a health field gets no sealed value, and the down path restores
// the plaintext.
func TestStudentHealthDataSealsEveryHealthField(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	cipher := testHolidayHealthCipher(t)
	withHealth := testpkg.CreateTestStudent(t, db, "Lia", "Gesund", "1A")
	without := testpkg.CreateTestStudent(t, db, "Ben", "Ohne", "1A")
	require.NoError(t, studentHealthDataDownWith(ctx, db, cipher))

	membership := func(studentID int64) (tenantID, membershipID int64) {
		t.Helper()
		var row struct {
			ID       int64 `bun:"id"`
			TenantID int64 `bun:"tenant_id"`
		}
		require.NoError(t, db.NewRaw(`SELECT id, tenant_id FROM users.student_school_memberships
			WHERE student_profile_id = ? AND deleted_at IS NULL`, studentID).Scan(ctx, &row))
		return row.TenantID, row.ID
	}
	tenantID, healthMembership := membership(withHealth.ID)
	_, quietMembership := membership(without.ID)
	_, err := db.NewRaw(`UPDATE users.student_care_profiles SET health_info = 'Erdnussallergie, Notfallspray' WHERE membership_id = ?`,
		healthMembership).Exec(ctx)
	require.NoError(t, err)

	require.Error(t, studentHealthDataUpWith(ctx, db, nil), "a health field to seal needs the key ring")
	require.True(t, careProfileColumnExists(t, ctx, db, "health_info"), "the refused migration leaves the plaintext untouched")
	require.NoError(t, studentHealthDataUpWith(ctx, db, cipher))
	assert.False(t, careProfileColumnExists(t, ctx, db, "health_info"), "the plaintext column leaves the table")

	var whole string
	require.NoError(t, db.NewRaw(`SELECT jsonb_agg(to_jsonb(care))::text FROM users.student_care_profiles AS care
		WHERE membership_id IN (?, ?)`, healthMembership, quietMembership).Scan(ctx, &whole))
	assert.NotContains(t, whole, "Erdnuss", "no plaintext health field stays in the table")

	sealedOf := func(membershipID int64) []byte {
		t.Helper()
		var sealed []byte
		require.NoError(t, db.NewRaw(`SELECT health_data FROM users.student_care_profiles WHERE membership_id = ?`,
			membershipID).Scan(ctx, &sealed))
		return sealed
	}
	assert.Nil(t, sealedOf(quietMembership), "a child without a health field gets no sealed value")
	sealed := sealedOf(healthMembership)
	opened, err := cipher.Open(sealed, studentHealthContext(tenantID, healthMembership))
	require.NoError(t, err)
	assert.Equal(t, "Erdnussallergie, Notfallspray", string(opened))
	_, err = cipher.Open(sealed, studentHealthContext(tenantID, quietMembership))
	assert.Error(t, err, "a sealed value opens only for its own care row")

	require.Error(t, studentHealthDataDownWith(ctx, db, nil), "a sealed value to open needs the key ring")
	require.NoError(t, studentHealthDataDownWith(ctx, db, cipher))
	var restored *string
	require.NoError(t, db.NewRaw(`SELECT health_info FROM users.student_care_profiles WHERE membership_id = ?`,
		healthMembership).Scan(ctx, &restored))
	require.NotNil(t, restored)
	assert.Equal(t, "Erdnussallergie, Notfallspray", *restored)
	assert.False(t, careProfileColumnExists(t, ctx, db, "health_data"), "down drops the sealed column")
	require.NoError(t, studentHealthDataUpWith(ctx, db, cipher))
}
