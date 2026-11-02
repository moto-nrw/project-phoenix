package test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestQueryCountersOnSharedPoolCountOnlyTheirOwnQueries reproduces #5220:
// parallel tests attach and drop counters on the package pool while other
// statements are in flight. Under -race, registration must not touch the hook
// list bun reads, and every counter must see each of its own statements
// exactly once and none of its neighbours'.
func TestQueryCountersOnSharedPoolCountOnlyTheirOwnQueries(t *testing.T) {
	t.Parallel()
	db := SetupTestDB(t)
	tenantID := Tenant(t)

	loadCtx, stopLoad := context.WithCancel(Ctx(t))
	var load sync.WaitGroup
	for range 2 {
		load.Add(1)
		go func() {
			defer load.Done()
			for loadCtx.Err() == nil {
				var one int
				_ = db.NewRaw("SELECT 1 /* qc-load */").Scan(loadCtx, &one)
			}
		}()
	}
	t.Cleanup(func() {
		stopLoad()
		load.Wait()
	})

	const counters, ownQueries = 8, 5
	t.Run("counters", func(t *testing.T) {
		for i := range counters {
			t.Run(fmt.Sprintf("counter-%d", i), func(t *testing.T) {
				t.Parallel()
				token := fmt.Sprintf("qc-%d ", i)
				plain := CaptureQueriesForContext(t, db)
				nested := CaptureQueriesForContext(t, db)

				var values []int
				for range ownQueries {
					query := fmt.Sprintf("SELECT generate_series(1, %d) /* %s*/", i+1, token)
					require.NoError(t, db.NewRaw(query).Scan(plain.Context(Ctx(t)), &values))
					require.NoError(t, db.NewRaw(query).Scan(Ctx(t), &values), "an unmarked statement")
				}
				require.NoError(t, WithinTenantContext(t, nested.Context(Ctx(t)), db, tenantID, func(txCtx context.Context) error {
					tx, err := tenant.CurrentTransaction[bun.IDB](txCtx)
					if err != nil {
						return err
					}
					for range 2 {
						if err := tx.NewRaw("SELECT 1 /* "+token+"*/").Scan(txCtx, &values); err != nil {
							return err
						}
					}
					return nil
				}))

				assert.Equal(t, ownQueries, plain.Total())
				assert.Len(t, plain.Matching(containsToken(token)), ownQueries)
				rows, statements := plain.Rows()
				assert.EqualValues(t, ownQueries*(i+1), rows)
				assert.Equal(t, ownQueries, statements)
				assert.Len(t, nested.Matching(containsToken(token)), 2, "statements inside the tenant transaction")
				for _, counter := range []*QueryCounter{plain, nested} {
					for _, query := range counter.Queries() {
						if strings.Contains(query, "qc-") {
							assert.Contains(t, query, token, "a counter recorded a foreign statement")
						}
					}
				}
			})
		}
	})
}

func containsToken(token string) func(string) bool {
	return func(sql string) bool { return strings.Contains(sql, token) }
}
