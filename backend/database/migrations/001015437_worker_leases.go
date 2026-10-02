package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     "1.15.437",
		Description: "Fence the Worker behind a renewable database lease (#2726)",
		DependsOn:   []string{"1.13.1", "1.14.1"},
	})
	Migrations.MustRegister(workerLeasesUp, workerLeasesDown)
}

// workerLeasesUp creates the runtime leadership table of the Worker.
//
// One row per lease name. holder_id names the process that holds it,
// fencing_token grows by one with every new holder, and lease_until is
// database time: no process clock decides who leads. The table carries no
// tenant data, so it stays outside RLS.
//
// The administrative role acquires, renews and releases the lease. Job
// transactions run under the tenant or the administrative role and confirm
// their term through platform.assert_worker_lease as their last statement
// before commit. The function share-locks the row until that commit, so a
// takeover waits for every commit its predecessor already confirmed, and
// every later commit of the predecessor sees the new token. It reads the
// row as its owner, so the tenant role needs no grant on the table.
func workerLeasesUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		CREATE TABLE platform.worker_leases (
			lease_name TEXT PRIMARY KEY CHECK (lease_name <> ''),
			holder_id TEXT NOT NULL CHECK (holder_id <> ''),
			fencing_token BIGINT NOT NULL CHECK (fencing_token > 0),
			lease_until TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		COMMENT ON TABLE platform.worker_leases IS
			'Worker runtime leadership (#2726): one fenced holder per lease name. fencing_token grows with every new holder; lease_until is database time.';

		CREATE FUNCTION platform.assert_worker_lease(
			p_lease_name TEXT,
			p_holder_id TEXT,
			p_fencing_token BIGINT,
			p_margin INTERVAL
		)
		RETURNS VOID
		LANGUAGE plpgsql
		VOLATILE
		SECURITY DEFINER
		SET search_path = pg_catalog, platform
		AS $$
		BEGIN
			PERFORM 1 FROM platform.worker_leases
			WHERE lease_name = p_lease_name
			  AND holder_id = p_holder_id
			  AND fencing_token = p_fencing_token
			  AND lease_until > clock_timestamp() + p_margin
			FOR SHARE;
			IF NOT FOUND THEN
				RAISE EXCEPTION 'worker lease % is not held by fencing token %', p_lease_name, p_fencing_token
					USING ERRCODE = 'PWL01';
			END IF;
		END;
		$$;

		REVOKE ALL ON platform.worker_leases FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin;
		GRANT SELECT, INSERT, UPDATE ON platform.worker_leases TO phoenix_admin;
		REVOKE ALL ON FUNCTION platform.assert_worker_lease(TEXT, TEXT, BIGINT, INTERVAL) FROM PUBLIC;
		GRANT EXECUTE ON FUNCTION platform.assert_worker_lease(TEXT, TEXT, BIGINT, INTERVAL) TO phoenix_tenant, phoenix_admin;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("create worker leases: %w", err)
	}
	return nil
}

func workerLeasesDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		DROP FUNCTION IF EXISTS platform.assert_worker_lease(TEXT, TEXT, BIGINT, INTERVAL);
		DROP TABLE IF EXISTS platform.worker_leases;
	`).Exec(ctx)
	return err
}
