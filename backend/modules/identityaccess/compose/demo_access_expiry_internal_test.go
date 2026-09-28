package compose

import (
	"context"
	"fmt"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The expiry of demo accesses (#3470) on an injected clock: an access past
// its end is deleted, and the schools no remaining access enters are named
// once each, so the demo process can hide them.

func insertDemoAccess(t *testing.T, db *bun.DB, slug string, expiresAt time.Time) {
	t.Helper()
	tokenHash := fmt.Sprintf("%s-%d", slug, expiresAt.UnixNano())
	_, err := db.NewRaw(`INSERT INTO auth.demo_accesses (email, first_name, last_name, school_name, token_hash, expires_at, school_slug)
		VALUES (?, 'Kim', 'Beispiel', 'OGS Beispiel', ?, ?, ?)`, "kim-"+slug+"@ogs-beispiel.de", tokenHash, expiresAt, slug).Exec(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.NewRaw(`DELETE FROM auth.demo_accesses WHERE token_hash = ?`, tokenHash).Exec(context.Background())
		require.NoError(t, err)
	})
}

func TestDemoAccessExpiryDeletesExpiredAccessesAndNamesOrphanedSchools(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	clock := time.Date(2026, 9, 22, 8, 0, 0, 0, time.UTC)
	expiry, err := NewDemoAccessExpiry(db, func() time.Time { return clock })
	require.NoError(t, err)
	nord, sued, west := fmt.Sprintf("ogs-nord-%d", testpkg.Tenant(t)), fmt.Sprintf("ogs-sued-%d", testpkg.Tenant(t)), fmt.Sprintf("ogs-west-%d", testpkg.Tenant(t))
	// Nord keeps a link; Süd and West lose their last one, West twice over.
	insertDemoAccess(t, db, nord, clock.Add(-time.Hour))
	insertDemoAccess(t, db, nord, clock.Add(time.Hour))
	insertDemoAccess(t, db, sued, clock.Add(-time.Hour))
	insertDemoAccess(t, db, west, clock.Add(-2*time.Hour))
	insertDemoAccess(t, db, west, clock.Add(-time.Minute))

	deleted, orphaned, err := expiry.ExpireDemoAccesses(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 4, deleted)
	assert.Equal(t, []string{sued, west}, orphaned, "a school with a link left is not named; a school named twice is named once")
	var remaining []string
	require.NoError(t, db.NewRaw(`SELECT school_slug FROM auth.demo_accesses WHERE school_slug IN (?, ?, ?)`, nord, sued, west).Scan(t.Context(), &remaining))
	assert.Equal(t, []string{nord}, remaining, "the unexpired access stays")

	// Nothing is due until the clock passes the remaining end.
	deleted, orphaned, err = expiry.ExpireDemoAccesses(t.Context())
	require.NoError(t, err)
	assert.Zero(t, deleted)
	assert.Empty(t, orphaned)
	clock = clock.Add(2 * time.Hour)
	deleted, orphaned, err = expiry.ExpireDemoAccesses(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, deleted)
	assert.Equal(t, []string{nord}, orphaned)
}

func TestDemoAccessExpiryRequiresADatabaseAndAClock(t *testing.T) {
	t.Parallel()
	_, err := NewDemoAccessExpiry(nil, time.Now)
	assert.Error(t, err)
	_, err = NewDemoAccessExpiry(testpkg.SetupTestDB(t), nil)
	assert.Error(t, err)
}
