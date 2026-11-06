package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const unaccentNameSearchVersion = "1.15.756"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     unaccentNameSearchVersion,
		Description: "Namen ohne Akzente suchen: unaccent und IMMUTABLE-Hülle public.f_unaccent (#4949)",
		DependsOn: []string{
			germanNameCollationVersion, // the last migration before this one
		},
	})
	Migrations.MustRegister(unaccentNameSearchUp, unaccentNameSearchDown)
}

// unaccentNameSearchUpSQL installs unaccent so a name search typed without
// accents finds "Gómez" for "gomez" and "Özdemir" for "ozdem" (#4949).
// unaccent only strips accents: "Müller" folds to "Muller", not "Mueller".
//
// unaccent() itself is STABLE because it reads a dictionary that could be
// changed, so it cannot back an index. public.f_unaccent pins the dictionary
// and the function by schema and declares itself IMMUTABLE. The SQL-standard
// body records its dependency on the extension and does not follow the
// caller's search_path.
const unaccentNameSearchUpSQL = `
CREATE EXTENSION IF NOT EXISTS unaccent SCHEMA public;
CREATE OR REPLACE FUNCTION public.f_unaccent(text) RETURNS text
  LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT
BEGIN ATOMIC
  SELECT public.unaccent('public.unaccent'::regdictionary, $1);
END;
`

// unaccentNameSearchDownSQL removes the wrapper and then the extension,
// unless another object has come to depend on it since.
const unaccentNameSearchDownSQL = `
DROP FUNCTION IF EXISTS public.f_unaccent(text);
DO $$
BEGIN
  DROP EXTENSION IF EXISTS unaccent;
EXCEPTION WHEN dependent_objects_still_exist THEN
  RAISE NOTICE 'unaccent stays installed: other objects depend on it';
END
$$;
`

func unaccentNameSearchUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, unaccentNameSearchUpSQL); err != nil {
		return fmt.Errorf("install unaccent for name search: %w", err)
	}
	return nil
}

func unaccentNameSearchDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, unaccentNameSearchDownSQL); err != nil {
		return fmt.Errorf("remove unaccent name search: %w", err)
	}
	return nil
}
