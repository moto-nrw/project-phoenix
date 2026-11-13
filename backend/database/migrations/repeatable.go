package migrations

import (
	"context"
	"fmt"
	"sort"

	"github.com/uptrace/bun"
)

// A repeatable definition is the one current definition of a replaceable
// database object: a function, written as CREATE OR REPLACE (ADR 0223).
//
// A numbered migration that redefines a function carries a full copy of it.
// Two branches that change the same function then each ship a copy made from
// the same older state, in two new files git never compares, and the copy that
// runs last silently drops the other change (#6391). A repeatable definition
// lives in one file instead: the two branches conflict in git, in the line
// they both touched.
//
// Every migrate run installs every definition after the numbered migrations,
// whether or not one was pending. A late numbered migration that still carries
// an older copy can therefore not leave it behind, and a definition changed by
// hand is restored by the next deployment. No checksum table is kept: the
// statements are idempotent and a run costs a few milliseconds.
//
// Rules for a definition:
//
//   - One file per object, named repeatable_<schema>_<object>.go, registering
//     under "<schema>.<object>".
//   - The SQL can run any number of times. A new function brings its REVOKE
//     and GRANT statements; CREATE OR REPLACE keeps the grants, owner and
//     comment of an existing one. A changed signature or return type needs
//     DROP FUNCTION IF EXISTS in front.
//   - Numbered migrations no longer define the object. A numbered migration
//     that calls it while it runs gets the definition of the previous run.
//   - The file runs its SQL itself, as a constant: the architecture check
//     reads the SQL of every new file in this package.
var repeatableDefinitions = make(map[string]repeatableInstall)

// repeatableInstall runs the SQL of one definition.
type repeatableInstall func(ctx context.Context, tx bun.Tx) error

// RegisterRepeatable adds what installs the current definition of the object name. It
// panics on an empty or a second definition, so the binary does not start
// with an ambiguous one.
func RegisterRepeatable(name string, install repeatableInstall) {
	if name == "" || install == nil {
		panic(fmt.Sprintf("repeatable definition %q needs a name and its SQL", name))
	}
	if _, exists := repeatableDefinitions[name]; exists {
		panic(fmt.Sprintf("REPEATABLE DEFINITION COLLISION: %s is registered twice", name))
	}
	repeatableDefinitions[name] = install
}

// RepeatableNames returns the objects with a repeatable definition, sorted.
// That is the order they are installed in.
func RepeatableNames() []string {
	names := make([]string, 0, len(repeatableDefinitions))
	for name := range repeatableDefinitions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// applyRepeatableDefinitions installs every definition in one transaction:
// a run either leaves all of them current or none of them changed. It logs
// nothing: with no migration pending a migrate run writes one line (#3300).
func applyRepeatableDefinitions(ctx context.Context, db *bun.DB) error {
	names := RepeatableNames()
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Two migrate runs that replace the same function at once fail with
		// "tuple concurrently updated"; the second waits for the first.
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('migrations:repeatable-definitions', 0))`); err != nil {
			return fmt.Errorf("lock the repeatable definitions: %w", err)
		}
		for _, name := range names {
			if err := repeatableDefinitions[name](ctx, tx); err != nil {
				return fmt.Errorf("install repeatable definition %s: %w", name, err)
			}
		}
		return nil
	})
}
