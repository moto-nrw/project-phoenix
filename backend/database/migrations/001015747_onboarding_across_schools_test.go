package migrations

import (
	"context"
	"fmt"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A person's rows from before #5659 become one state: every finished step, a
// skip only where no school finished the step, and hidden if hidden anywhere.
// Another person's single row is left alone.
func TestOnboardingAcrossSchoolsMergesEachPersonsRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	require.NoError(t, onboardingAcrossSchoolsDown(ctx, db))

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	var organizationID int64
	_, err := db.NewRaw(`INSERT INTO platform.organizations (name, slug, active, created_at, updated_at)
		VALUES (?, ?, true, NOW(), NOW()) RETURNING id`, "Merge Org "+suffix, "merge-org-"+suffix).Exec(ctx, &organizationID)
	require.NoError(t, err)
	school := func(label string) int64 {
		slug := "merge-" + label + "-" + suffix
		var id int64
		_, err := db.NewRaw(`INSERT INTO platform.schools (name, slug, subdomain, organization_id, active, created_at, updated_at)
			VALUES (?, ?, ?, ?, true, NOW(), NOW()) RETURNING id`, slug, slug, slug, organizationID).Exec(ctx, &id)
		require.NoError(t, err)
		return id
	}
	account := func(label string) int64 {
		var id int64
		_, err := db.NewRaw(`INSERT INTO auth.accounts (email, active) VALUES (?, TRUE) RETURNING id`,
			label+"-"+suffix+"@ogs-beispiel.de").Exec(ctx, &id)
		require.NoError(t, err)
		return id
	}
	schoolA, schoolB := school("a"), school("b")
	both, single := account("both"), account("single")
	dismissedAt := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)

	insert := func(tenantID, accountID int64, done, skipped string, dismissed *time.Time) {
		_, err := db.NewRaw(`INSERT INTO config.staff_onboardings (tenant_id, account_id, done_steps, skipped_steps, dismissed_at)
			VALUES (?, ?, ?::text[], ?::text[], ?)`, tenantID, accountID, done, skipped, dismissed).Exec(ctx)
		require.NoError(t, err)
	}
	insert(schoolA, both, "{groups}", "{students}", &dismissedAt)
	insert(schoolB, both, "{students}", "{calendar}", nil)
	insert(schoolB, single, "{groups}", "{}", nil)

	require.NoError(t, onboardingAcrossSchoolsUp(ctx, db))

	type row struct {
		Done      []string   `bun:"done_steps,array"`
		Skipped   []string   `bun:"skipped_steps,array"`
		Dismissed *time.Time `bun:"dismissed_at"`
	}
	read := func(accountID int64) []row {
		var rows []row
		require.NoError(t, db.NewRaw(`SELECT done_steps, skipped_steps, dismissed_at
			FROM config.staff_onboardings WHERE account_id = ?`, accountID).Scan(ctx, &rows))
		return rows
	}

	merged := read(both)
	require.Len(t, merged, 2)
	for _, got := range merged {
		assert.Equal(t, []string{"groups", "students"}, got.Done)
		assert.Equal(t, []string{"calendar"}, got.Skipped, "a finished step is no skip")
		require.NotNil(t, got.Dismissed)
		assert.True(t, got.Dismissed.Equal(dismissedAt))
	}

	untouched := read(single)
	require.Len(t, untouched, 1)
	assert.Equal(t, []string{"groups"}, untouched[0].Done)
	assert.Nil(t, untouched[0].Dismissed)
}

// The request role reaches other schools' onboarding rows through the two
// functions only, and neither is open to everyone.
func TestOnboardingAcrossSchoolsFunctionsAreForTheRequestRoleOnly(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	for _, function := range []string{
		"config.staff_onboarding_across_schools(bigint)",
		"config.school_setup_dismissed_in_traeger(bigint)",
	} {
		var tenantRole, authRole, definer bool
		require.NoError(t, db.NewRaw(`SELECT
				has_function_privilege('phoenix_tenant', ?::regprocedure, 'EXECUTE'),
				has_function_privilege('phoenix_auth', ?::regprocedure, 'EXECUTE'),
				(SELECT prosecdef FROM pg_proc WHERE oid = ?::regprocedure)`,
			function, function, function).Scan(context.Background(), &tenantRole, &authRole, &definer))
		assert.True(t, tenantRole, function)
		assert.False(t, authRole, function)
		assert.True(t, definer, function)
	}
}
