package test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestEnsureTestTenant_IsConcurrentSafe(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)
	tenantID := UniqueTestTenantID(t)
	start := make(chan struct{})
	errs := make(chan error, 16)
	var wg sync.WaitGroup

	for range cap(errs) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			errs <- ensureTestTenant(ctx, db, tenantID)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		require.NoError(t, err)
	}
}

func TestCleanupTenantTestDataRemovesPlatformOwners(t *testing.T) {
	t.Parallel()

	db := SetupTestDB(t)
	tenantID, _ := CreateTestTenant(t, db)

	cleanupTenantTestData(t, db, tenantID)

	for _, table := range []string{"platform.schools", "platform.organizations"} {
		var exists bool
		err := db.NewRaw("SELECT EXISTS (SELECT 1 FROM "+table+" WHERE id = ?)", tenantID).
			Scan(context.Background(), &exists)
		require.NoError(t, err)
		require.False(t, exists, "%s row must be removed", table)
	}
}
