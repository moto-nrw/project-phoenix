package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerOwnNameVersion = "1.15.890"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerOwnNameVersion,
		Description: "Träger-Transaktion: Name des eigenen Trägers für Elternmails nach Büroaktionen (#6437)",
		DependsOn: []string{
			parentBillingDatevBookedDocumentsVersion, // 1.15.888, the newest on development
		},
	})
	Migrations.MustRegister(traegerOwnNameUp, traegerOwnNameDown)
}

// traegerOwnNameUpSQL lets the Träger transaction read the name of its own
// Träger (#6437). The request role reads no row of platform.organizations;
// a mail the Träger-Büro triggers, such as a place that became free after a
// cancellation, a rebooking or a higher capacity, must still name the
// Träger. platform.current_traeger_name returns the name of the undeleted
// organization in app.current_org_id and nothing else: no other Träger, no
// other column, and NULL outside a Träger transaction.
const traegerOwnNameUpSQL = `
CREATE FUNCTION platform.current_traeger_name()
	RETURNS TEXT
	LANGUAGE sql
	STABLE
	SECURITY DEFINER
	SET search_path = pg_catalog, pg_temp
	AS $$
	SELECT organization.name
	FROM platform.organizations AS organization
	WHERE organization.id = NULLIF(current_setting('app.current_org_id', true), '')::bigint
		AND organization.deleted_at IS NULL
	$$;
COMMENT ON FUNCTION platform.current_traeger_name() IS
	'Name of the undeleted Träger in app.current_org_id, NULL outside its Träger transaction (#6437).';

REVOKE ALL ON FUNCTION platform.current_traeger_name() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.current_traeger_name() TO phoenix_tenant;
`

const traegerOwnNameDownSQL = `
DROP FUNCTION IF EXISTS platform.current_traeger_name();
`

func traegerOwnNameUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerOwnNameUpSQL); err != nil {
		return fmt.Errorf("let the Träger transaction read its Träger's name: %w", err)
	}
	return nil
}

func traegerOwnNameDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerOwnNameDownSQL); err != nil {
		return fmt.Errorf("drop the Träger's own name: %w", err)
	}
	return nil
}
