package migrations

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// TestStoredNamesNFCComposesDecomposedNames pins #5791: a name stored
// decomposed before #4949 normalized names on write is composed, so the
// searches and the duplicate check that normalize only their input find it.
func TestStoredNamesNFCComposesDecomposedNames(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	tenantID := testpkg.Tenant(t)
	const decomposedFirst, decomposedLast = "Zoë", "Ćirić"

	person := testpkg.CreateTestPerson(t, db, "Kim", "Becker")
	_, err := db.ExecContext(ctx, `UPDATE users.persons SET first_name = ?, last_name = ? WHERE id = ?`, decomposedFirst, decomposedLast, person.ID)
	require.NoError(t, err)
	guardian := testpkg.CreateTestGuardianProfileNamed(t, db, "Kim", "Becker", fmt.Sprintf("nfc-%d@example.test", tenantID))
	_, err = db.ExecContext(ctx, `UPDATE users.guardian_profiles SET first_name = ?, last_name = ? WHERE id = ?`, decomposedFirst, decomposedLast, guardian.ID)
	require.NoError(t, err)
	phaseID := insertNotificationModeTestPhase(t, db, tenantID, "nfc")
	requestID := insertNotificationModeTestRequest(t, db, tenantID, phaseID, fmt.Sprintf("nfc-%d", tenantID), "under_review")
	_, err = db.ExecContext(ctx, `UPDATE enrollment.request_children SET first_name = ?, last_name = ? WHERE request_id = ?`, decomposedFirst, decomposedLast, requestID)
	require.NoError(t, err)

	require.NoError(t, storedNamesNFCUp(ctx, db))

	for table, id := range map[string]string{
		"users.persons":               fmt.Sprintf("id = %d", person.ID),
		"users.guardian_profiles":     fmt.Sprintf("id = %d", guardian.ID),
		"enrollment.request_children": fmt.Sprintf("request_id = %d", requestID),
	} {
		var first, last string
		require.NoError(t, db.QueryRowContext(ctx, `SELECT first_name, last_name FROM `+table+` WHERE `+id).Scan(&first, &last), table)
		assert.Equal(t, "Zoë", first, table)
		assert.Equal(t, "Ćirić", last, table)
	}
	require.NoError(t, storedNamesNFCUp(ctx, db), "a second run finds nothing left to compose")
}
