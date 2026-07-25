package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

const (
	auditAppendOnlyGrantsVersion     = "1.15.225"
	auditAppendOnlyGrantsDescription = "Make the audit schema append-only for phoenix_tenant: revoke inherited UPDATE/DELETE and close the schema-wide default ACL (#1924)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     auditAppendOnlyGrantsVersion,
		Description: auditAppendOnlyGrantsDescription,
		DependsOn: []string{
			createTenantRolesVersion,
			enrollmentDeletionsAuditVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return auditAppendOnlyGrantsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return auditAppendOnlyGrantsDown(ctx, db)
		},
	)
}

// auditAppendOnlyGrantsUp closes the gap described in issue #1924.
//
// Migration 1.14.1 set a schema-wide default ACL granting phoenix_tenant
// SELECT, INSERT, UPDATE, DELETE on every table created later in the audit
// schema. Because migrations run as the postgres superuser, that default
// applied to all audit tables, and the deliberately narrow
// "GRANT SELECT, INSERT" statements in the later table-creating migrations
// were purely additive: the inherited UPDATE/DELETE stayed. None of the
// audit tables were append-only at the database level.
//
// The RLS policies on these tables are FOR ALL, so a request running under
// phoenix_tenant could UPDATE/DELETE its own tenant's audit rows. No
// application code issues such statements, which makes this defense in
// depth rather than a reachable hole — but the append-only guarantee these
// tables claim (GDPR deletion log, guardian change audit, data access log)
// has to hold in the database, not only by convention.
//
// DELETE stays for two tables whose GDPR retention jobs run per tenant
// inside the server's phoenix_tenant transactions (see the table map
// below). Rows removed through ON DELETE CASCADE and columns nulled through
// ON DELETE SET NULL need no privilege at all: referential actions execute
// with the privileges of the table owner and are not subject to the child
// table's RLS, so revoking here does not break the cascade paths
// (audit.work_session_edits via active.work_sessions,
// audit.deviation_events.instance_id, audit.data_access_log.student_id, …).
func auditAppendOnlyGrantsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.225: Making the audit schema append-only for phoenix_tenant...")

	// Append-only: insert and read, never change or remove.
	//
	//   auth_events          — written by the login/MFA flows, read by the
	//                          security views. No update or delete path.
	//   data_access_log      — GDPR read-access trail, insert-only repository.
	//   data_deletions       — GDPR deletion log written by the cleanup services.
	//   data_imports         — import audit, insert-only repository.
	//   guardian_changes     — guardian change audit, insert + list only.
	//   work_session_edits   — time-tracking edit trail; rows disappear only
	//                          through the ON DELETE CASCADE from
	//                          active.work_sessions during retention cleanup.
	//
	// TRUNCATE is revoked alongside them. The 1.14.1 default ACL never handed
	// it out, so this is a no-op today — but a table nobody may DELETE from is
	// not append-only while it can still be emptied in one statement, and
	// stating it here makes the guarantee explicit rather than accidental.
	if _, err := db.NewRaw(`
		REVOKE UPDATE, DELETE, TRUNCATE ON
			audit.auth_events,
			audit.data_access_log,
			audit.data_deletions,
			audit.data_imports,
			audit.guardian_changes,
			audit.work_session_edits
		FROM phoenix_tenant;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed revoking tenant UPDATE/DELETE/TRUNCATE on append-only audit tables: %w", err)
	}

	// Append-only plus tenant-scoped retention: DELETE must stay.
	//
	//   deviation_events       — TimetableCleanupService.CleanupExpiredTimetableData
	//                            deletes by occurrence_date; the scheduler runs it
	//                            per tenant inside WithTenantTx (already documented
	//                            in migration 1.15.193).
	//   unregistered_tag_scans — UnregisteredTagScanService.DeleteOlderThan (90-day
	//                            retention) runs the same way. Its only UPDATE,
	//                            Resolve(), is an operator-portal action and runs
	//                            under phoenix_admin (operator requests carry no
	//                            tenant, so TenantTxMiddleware opens no tenant tx).
	//
	// The retention jobs delete row by row (DeleteOlderThan with a date
	// predicate), so TRUNCATE goes here too — wiping the whole table is never
	// a legitimate tenant operation.
	if _, err := db.NewRaw(`
		REVOKE UPDATE, TRUNCATE ON
			audit.deviation_events,
			audit.unregistered_tag_scans
		FROM phoenix_tenant;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed revoking tenant UPDATE/TRUNCATE on retention-cleaned audit tables: %w", err)
	}

	// One-off migration backups (1.15.45 room colours, 1.15.48 WC aliases).
	// Both are the documented manual-restore source for those migrations'
	// rollbacks and must NOT be dropped. They are also the only audit tables
	// without RLS, so any phoenix_tenant SELECT would cross tenant borders.
	// No application code touches them; restores run as phoenix_admin or the
	// superuser. Revoke everything.
	if _, err := db.NewRaw(`
		REVOKE ALL ON
			audit.room_color_migration_backup,
			audit.wc_alias_migration_backup
		FROM phoenix_tenant;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed revoking tenant privileges on audit migration backups: %w", err)
	}

	// The table REVOKE above does not reach their BIGSERIAL sequences: those
	// are separate objects, and 1.14.1's "ALTER DEFAULT PRIVILEGES IN SCHEMA
	// audit GRANT USAGE ON SEQUENCES TO phoenix_tenant" handed them out when
	// 1.15.45/1.15.48 created the tables. USAGE permits nextval, i.e. mutating
	// the counter of a cross-tenant backup object the role must not touch at
	// all. Revoke ALL (USAGE, SELECT, UPDATE) so the isolation is complete.
	//
	// Only these two sequences: every other audit table is INSERT-able by
	// design and needs its sequence's USAGE to make that INSERT work, so the
	// schema-wide sequence default stays as it is.
	if _, err := db.NewRaw(`
		REVOKE ALL ON SEQUENCE
			audit.room_color_migration_backup_id_seq,
			audit.wc_alias_migration_backup_id_seq
		FROM phoenix_tenant;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed revoking tenant privileges on audit migration backup sequences: %w", err)
	}

	// Root cause: stop the schema-wide default ACL from handing UPDATE/DELETE
	// to every future audit table. New tables now start append-only and a
	// migration that genuinely needs more has to grant it explicitly — which
	// is exactly the decision a reviewer should see. TRUNCATE is included so
	// the default cannot start handing it out either.
	if _, err := db.NewRaw(`
		ALTER DEFAULT PRIVILEGES IN SCHEMA audit
			REVOKE UPDATE, DELETE, TRUNCATE ON TABLES FROM phoenix_tenant;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed narrowing default privileges on schema audit: %w", err)
	}

	// Verify the ALTER actually landed. ALTER DEFAULT PRIVILEGES without
	// FOR ROLE is implicitly FOR ROLE current_user: it narrows only the
	// pg_default_acl row whose grantor is the migrating superuser. If 1.14.1
	// was applied under a different role than this migration runs as, the
	// REVOKE above creates a *second* row and the original arwd default
	// survives — every future audit table would silently inherit
	// UPDATE/DELETE again while this migration reports success. The guard
	// tests only ever see the test database, so fail loudly here instead.
	var defaultACL []string
	if err := db.NewRaw(`
		SELECT acl::text
		FROM pg_default_acl d
		JOIN pg_namespace n ON n.oid = d.defaclnamespace
		CROSS JOIN LATERAL unnest(d.defaclacl) AS acl
		WHERE n.nspname = 'audit'
		  AND d.defaclobjtype = 'r'
		  AND acl::text LIKE 'phoenix_tenant=%';
	`).Scan(ctx, &defaultACL); err != nil {
		return fmt.Errorf("failed reading back default privileges on schema audit: %w", err)
	}
	for _, item := range defaultACL {
		// aclitem text form: "grantee=privileges/grantor".
		privileges, _, _ := strings.Cut(strings.TrimPrefix(item, "phoenix_tenant="), "/")
		// Privilege letters (case-sensitive): w=UPDATE, d=DELETE, D=TRUNCATE.
		if strings.ContainsAny(privileges, "wdD") {
			return fmt.Errorf(
				"default privileges on schema audit still grant phoenix_tenant UPDATE/DELETE/TRUNCATE (%s); "+
					"the ALTER DEFAULT PRIVILEGES above is scoped to the grantor role, so a row created by a "+
					"different superuser survives — re-run the REVOKE as that role "+
					"(ALTER DEFAULT PRIVILEGES FOR ROLE <grantor> IN SCHEMA audit ...)",
				item)
		}
	}

	fmt.Println("  ✓ audit schema is append-only for phoenix_tenant (DELETE kept on deviation_events + unregistered_tag_scans for retention)")
	return nil
}

func auditAppendOnlyGrantsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.225: Restoring tenant UPDATE/DELETE on the audit schema...")

	// TRUNCATE is deliberately NOT restored anywhere below: the 1.14.1 default
	// ACL never granted it, so re-granting would leave the database in a state
	// it was never in before this migration.

	if _, err := db.NewRaw(`
		ALTER DEFAULT PRIVILEGES IN SCHEMA audit
			GRANT UPDATE, DELETE ON TABLES TO phoenix_tenant;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed restoring default privileges on schema audit: %w", err)
	}

	if _, err := db.NewRaw(`
		GRANT UPDATE, DELETE ON
			audit.auth_events,
			audit.data_access_log,
			audit.data_deletions,
			audit.data_imports,
			audit.guardian_changes,
			audit.work_session_edits,
			audit.deviation_events,
			audit.unregistered_tag_scans
		TO phoenix_tenant;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed restoring tenant UPDATE/DELETE on audit tables: %w", err)
	}

	if _, err := db.NewRaw(`
		GRANT SELECT, INSERT, UPDATE, DELETE ON
			audit.room_color_migration_backup,
			audit.wc_alias_migration_backup
		TO phoenix_tenant;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed restoring tenant privileges on audit migration backups: %w", err)
	}

	// Only USAGE goes back: that is what 1.14.1's sequence default granted.
	// SELECT and UPDATE on these sequences were never held, so re-granting
	// them would leave a state the database was never in.
	if _, err := db.NewRaw(`
		GRANT USAGE ON SEQUENCE
			audit.room_color_migration_backup_id_seq,
			audit.wc_alias_migration_backup_id_seq
		TO phoenix_tenant;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed restoring tenant privileges on audit migration backup sequences: %w", err)
	}

	return nil
}
