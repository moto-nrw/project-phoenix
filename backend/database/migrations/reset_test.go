package migrations

import (
	"context"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

// TestDroppablePublicTypesSkipsUndroppableTypes pins the filter behind
// `migrate reset` (#3299). Types owned by an extension cannot be dropped at
// all, and array and table row types are dropped together with what they belong
// to. Listing any of them made the reset log 16 "Failed to drop type" warnings.
func TestDroppablePublicTypesSkipsUndroppableTypes(t *testing.T) {
	// Safe in parallel: TestMain enables PerTestDatabases, so this test
	// mutates its own disposable clone.
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	// btree_gist owns the gbtreekey* types in public; a migration installs it in
	// real databases, but create it here so the test does not depend on that.
	mustExec(t, db, `CREATE EXTENSION IF NOT EXISTS btree_gist`)
	mustExec(t, db, `DROP TYPE IF EXISTS public.reset_filter_enum CASCADE`)
	mustExec(t, db, `DROP TABLE IF EXISTS public.reset_filter_table CASCADE`)
	mustExec(t, db, `CREATE TYPE public.reset_filter_enum AS ENUM ('a', 'b')`)
	mustExec(t, db, `CREATE TABLE public.reset_filter_table (id INT PRIMARY KEY)`)
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = db.ExecContext(cleanupCtx, `DROP TABLE IF EXISTS public.reset_filter_table CASCADE`)
		_, _ = db.ExecContext(cleanupCtx, `DROP TYPE IF EXISTS public.reset_filter_enum CASCADE`)
	})

	typeNames, err := droppablePublicTypes(ctx, db)
	if err != nil {
		t.Fatalf("list droppable public types: %v", err)
	}

	listed := make(map[string]bool, len(typeNames))
	for _, name := range typeNames {
		listed[name] = true
	}

	if !listed["reset_filter_enum"] {
		t.Errorf("standalone type reset_filter_enum must stay droppable, got %v", typeNames)
	}
	if listed["_reset_filter_enum"] {
		t.Error("array type _reset_filter_enum must not be listed; it is dropped with its element type")
	}
	if listed["reset_filter_table"] {
		t.Error("row type reset_filter_table must not be listed; it is dropped with its table")
	}
	for _, name := range typeNames {
		if strings.HasPrefix(name, "gbtreekey") {
			t.Errorf("extension-owned type %s must not be listed; Postgres refuses to drop it", name)
		}
	}
}

// TestResetDropsEveryListedPublicType runs the drop loop against a live
// database and fails if any listed type survives or errors — the end state
// acceptance criterion of #3299: a reset without drop warnings.
func TestResetDropsEveryListedPublicType(t *testing.T) {
	// Safe in parallel: TestMain enables PerTestDatabases, so this test
	// mutates its own disposable clone.
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	mustExec(t, db, `CREATE EXTENSION IF NOT EXISTS btree_gist`)
	mustExec(t, db, `DROP TYPE IF EXISTS public.reset_drop_enum CASCADE`)
	mustExec(t, db, `CREATE TYPE public.reset_drop_enum AS ENUM ('a', 'b')`)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DROP TYPE IF EXISTS public.reset_drop_enum CASCADE`)
	})

	typeNames, err := droppablePublicTypes(ctx, db)
	if err != nil {
		t.Fatalf("list droppable public types: %v", err)
	}
	for _, typeName := range typeNames {
		if _, err := db.ExecContext(ctx, "DROP TYPE IF EXISTS ? CASCADE", bun.Ident(typeName)); err != nil {
			t.Errorf("drop type %s: %v", typeName, err)
		}
	}

	remaining, err := droppablePublicTypes(ctx, db)
	if err != nil {
		t.Fatalf("re-list droppable public types: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("types still droppable after the drop loop: %v", remaining)
	}
}

func mustExec(t *testing.T, db *bun.DB, query string) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), query); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}
