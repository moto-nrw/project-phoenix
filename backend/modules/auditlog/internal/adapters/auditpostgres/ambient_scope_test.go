package audit

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

func TestPersistenceRequiresAmbientTransaction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	for name, runtime := range map[string]Runtime{"ambient": NewAmbientRuntime(db), "explicit tenant": NewRuntime(db, auditTestTenantID), "handoff": NewHandoffRuntime(ContextHandoff)} {
		t.Run(name, func(t *testing.T) {
			testpkg.AssertAmbientDatabase(t, db, func(ctx context.Context) (bun.IDB, error) { tx, _, err := database(ctx, runtime); return tx, err })
		})
	}
}
