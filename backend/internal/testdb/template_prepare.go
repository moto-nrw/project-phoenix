package testdb

import (
	"context"
	"encoding/hex"
	"fmt"
	"hash/fnv"
)

// CloneSearchPath is the search_path every clone gets from the maintenance
// session when it is created. BUN's Relation() JOIN generation sometimes uses
// unqualified table names, which fail when the target schema is not in
// search_path. A database setting is not copied by CREATE DATABASE ...
// TEMPLATE, so it is set per clone, not in the template.
const CloneSearchPath = `public, platform, auth, users, education, facilities, activities, active, schedule, iot, feedback, config, meta, audit`

// templateBootstrapStatements seed the template once after its migrations:
// the default tenant (school 1), the default room and the system staff
// fixture, and the sequence positions the suite depends on. Every clone of the
// template starts with them, so the roughly 1,900 clones of a suite no longer
// write them one by one (#6962). Each statement is idempotent and reconciles
// its fixed ID, so an adopted legacy template cannot retain a bootstrap row
// from another tenant, and a preparation that died halfway can run again.
//
// The template name hashes these statements (TemplateHash): editing one
// builds a new template instead of reusing one prepared with the old text.
var templateBootstrapStatements = []string{
	// Legacy fixtures use tenant_id=1, which requires a FK target row.
	`INSERT INTO platform.organizations (id, name, slug, active)
		VALUES (1, 'Test Org 1', 'test-org-1', true)
		ON CONFLICT (id) DO UPDATE
		SET name = EXCLUDED.name, slug = EXCLUDED.slug, active = EXCLUDED.active`,
	`INSERT INTO platform.schools (id, organization_id, name, slug, subdomain, active)
		VALUES (1, 1, 'Test School 1', 'test-school-1', 't1', true)
		ON CONFLICT (id) DO UPDATE
		SET organization_id = EXCLUDED.organization_id, name = EXCLUDED.name,
			slug = EXCLUDED.slug, subdomain = EXCLUDED.subdomain, active = EXCLUDED.active`,
	// Lift both tenant sequences clear of the test-tenant band before the
	// first test runs. Two things depend on it: a service-path school
	// (platform.CreateSchool) can never collide with an ID CreateTestTenant
	// hands out explicitly, and — because every tenant a test creates is then
	// at or above TenantIDBase — the leftover gate can tell a test's own
	// tenant from shared state by looking at the ID alone.
	fmt.Sprintf(`SELECT setval(pg_get_serial_sequence('platform.organizations', 'id'),
			GREATEST((SELECT last_value FROM platform.organizations_id_seq), %[1]d)),
		setval(pg_get_serial_sequence('platform.schools', 'id'),
			GREATEST((SELECT last_value FROM platform.schools_id_seq), %[1]d))`, TenantIDCeiling),
	// Older fixtures hardcode room_id=1 and need a FK target row.
	`INSERT INTO facilities.rooms (id, tenant_id, name, building)
		VALUES (1, 1, 'Default Room', 'Default')
		ON CONFLICT (id) DO UPDATE
		SET tenant_id = EXCLUDED.tenant_id, name = EXCLUDED.name, building = EXCLUDED.building`,
	// Default system staff (person ID 1, staff ID 1) for legacy tests that
	// hardcode CreatedBy: 1 to satisfy created_by FK constraints. Staff lives
	// in the School Membership row and its Workforce employment profile since
	// the cutover (#2753).
	`INSERT INTO users.persons (id, tenant_id, first_name, last_name)
		VALUES (1, 1, 'System', 'Test')
		ON CONFLICT (id) DO UPDATE
		SET tenant_id = EXCLUDED.tenant_id, first_name = EXCLUDED.first_name, last_name = EXCLUDED.last_name`,
	`INSERT INTO users.staff_school_memberships (id, tenant_id, person_id)
		VALUES (1, 1, 1)
		ON CONFLICT (id) DO UPDATE
		SET tenant_id = EXCLUDED.tenant_id, person_id = EXCLUDED.person_id`,
	`INSERT INTO users.staff_employment_profiles (membership_id, tenant_id)
		VALUES (1, 1)
		ON CONFLICT (membership_id) DO UPDATE SET tenant_id = EXCLUDED.tenant_id`,
	// Advance the BIGSERIAL sequences past the explicitly inserted IDs.
	`SELECT setval('facilities.rooms_id_seq', GREATEST((SELECT last_value FROM facilities.rooms_id_seq), (SELECT COALESCE(MAX(id), 1) FROM facilities.rooms))),
		setval('users.persons_id_seq', GREATEST((SELECT last_value FROM users.persons_id_seq), (SELECT COALESCE(MAX(id), 1) FROM users.persons))),
		setval('users.staff_school_memberships_id_seq', GREATEST((SELECT last_value FROM users.staff_school_memberships_id_seq), (SELECT COALESCE(MAX(id), 1) FROM users.staff_school_memberships)))`,
}

// prepareTemplate is the default preparation of a migrated template: it
// writes the bootstrap rows in one transaction. It runs on every template the
// lifecycle builds or adopts, before the template is stamped.
func prepareTemplate(ctx context.Context, templateDSN string) error {
	db := openSQL(templateDSN)
	defer func() { _ = db.Close() }()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin template bootstrap: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for _, statement := range templateBootstrapStatements {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("template bootstrap: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit template bootstrap: %w", err)
	}
	return nil
}

// TemplateHash is the hash that names the template: the migration sources and
// the preparation that runs after them. A change to either builds a new
// template.
func TemplateHash() (string, error) {
	migrations, err := MigrationsHash()
	if err != nil {
		return "", err
	}
	return templateHash(migrations, templateBootstrapStatements), nil
}

func templateHash(migrations string, bootstrap []string) string {
	h := fnv.New128a() // identifier derivation, see identifierHash
	_, _ = fmt.Fprintf(h, "migrations\x00%s\x00", migrations)
	for _, statement := range bootstrap {
		_, _ = fmt.Fprintf(h, "bootstrap\x00%d\x00%s\x00", len(statement), statement)
	}
	return hex.EncodeToString(h.Sum(nil))
}
