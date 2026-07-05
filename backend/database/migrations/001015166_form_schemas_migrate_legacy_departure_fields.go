package migrations

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	formSchemasMigrateLegacyDepartureVersion     = "1.15.166"
	formSchemasMigrateLegacyDepartureDescription = "Publish a new form-schema version replacing legacy departure fields (departure/bus/bus_days/pickup_status) with student.allowed_departure_modes and repoint phases"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     formSchemasMigrateLegacyDepartureVersion,
		Description: formSchemasMigrateLegacyDepartureDescription,
		DependsOn: []string{
			formSchemaLegalBlocksVersion,  // legal_blocks column copied onto the new version
			createEnrollmentPhasesVersion, // phases.form_schema_id repoint target
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return formSchemasMigrateLegacyDepartureUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			// Down is intentionally a no-op: the migration only ADDS new
			// schema versions (mirroring what an admin edit does) and
			// advances phase bindings. Old versions stay untouched, and
			// already-submitted requests keep their own pinned schema_id,
			// so nothing needs restoring on rollback.
			return nil
		},
	)
}

// Legacy departure targets superseded by student.allowed_departure_modes.
// They map onto single-purpose columns (Buskind, Abholregelung, single-mode
// departure) while the modern field stores every allowed way home per care
// day — which is also what the care-offering required-per-care-day logic on
// the public form keys off.
var legacyDepartureTargets = map[string]bool{
	"student.departure":     true,
	"student.bus":           true,
	"student.bus_days":      true,
	"student.pickup_status": true,
}

const (
	modernDepartureTarget   = "student.allowed_departure_modes"
	modernDepartureKey      = "student_allowed_departure_modes"
	modernDepartureLabel    = "Erlaubte Heimwege"
	modernDepartureHelpText = "Wähle für jeden Betreuungstag alle erlaubten Heimwege aus."
)

// formSchemasMigrateLegacyDepartureUp converts every form-schema lineage
// whose LATEST version still declares legacy departure fields. It mirrors
// exactly what an admin edit through the form editor does: publish a new
// version with the converted fields and advance phase bindings to it.
// Older versions are never mutated — submitted requests pin their
// schema_id, and the decision service still dispatches the legacy targets
// for those pinned schemas, so pending legacy submissions keep working.
//
// Conversion rules per lineage (same semantics the editor's manual
// conversion used):
//   - the modern field already exists → drop the legacy fields
//   - otherwise → replace the first legacy field with one
//     student.allowed_departure_modes field (required when any dropped
//     legacy field was required) and drop the rest
//   - sort_order is renumbered 0..n-1
//
// Runs on the superuser CLI connection, so RLS is bypassed and all tenants
// are covered in one pass. Idempotent: a migrated lineage's latest version
// has no legacy fields, so a re-run skips it.
func formSchemasMigrateLegacyDepartureUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.166: Replacing legacy departure fields in enrollment.form_schemas with student.allowed_departure_modes...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	type schemaRow struct {
		ID       int64
		TenantID int64
		Name     string
		Version  int
		Fields   string
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT id, tenant_id, name, version, fields::text
		FROM enrollment.form_schemas
		ORDER BY tenant_id, name, version, id
	`)
	if err != nil {
		return fmt.Errorf("failed to list form schemas: %w", err)
	}

	// Group version rows into lineages keyed by (tenant, name). The last
	// row per key wins as "latest" thanks to the ORDER BY above.
	type lineage struct {
		latest     schemaRow
		versionIDs []int64
	}
	lineages := map[string]*lineage{}
	var keys []string
	for rows.Next() {
		var r schemaRow
		if err := rows.Scan(&r.ID, &r.TenantID, &r.Name, &r.Version, &r.Fields); err != nil {
			_ = rows.Close()
			return fmt.Errorf("failed to scan form schema row: %w", err)
		}
		key := fmt.Sprintf("%d|%s", r.TenantID, r.Name)
		l, ok := lineages[key]
		if !ok {
			l = &lineage{}
			lineages[key] = l
			keys = append(keys, key)
		}
		l.latest = r
		l.versionIDs = append(l.versionIDs, r.ID)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return fmt.Errorf("failed iterating form schemas: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("failed closing form schema rows: %w", err)
	}

	migrated := 0
	for _, key := range keys {
		l := lineages[key]
		converted, changed, err := convertLegacyDepartureFields(l.latest.Fields)
		if err != nil {
			return fmt.Errorf("schema %d (%s v%d): %w", l.latest.ID, l.latest.Name, l.latest.Version, err)
		}
		if !changed {
			continue
		}

		// Publish the converted fields as a new version, copying every
		// other template attribute (core requirements, legal blocks,
		// active flag, author) from the latest version.
		var newID int64
		err = tx.QueryRowContext(ctx, `
			INSERT INTO enrollment.form_schemas
				(tenant_id, name, version, fields, core_requirements, legal_blocks, is_active, created_by, created_at, updated_at)
			SELECT tenant_id, name, version + 1, ?::jsonb, core_requirements, legal_blocks, is_active, created_by, NOW(), NOW()
			FROM enrollment.form_schemas
			WHERE id = ?
			RETURNING id
		`, converted, l.latest.ID).Scan(&newID)
		if err != nil {
			return fmt.Errorf("failed publishing converted version of schema %d: %w", l.latest.ID, err)
		}

		// Advance every phase bound to ANY prior version of this lineage,
		// mirroring formSchemaService.repointPhasesToVersion.
		if _, err := tx.ExecContext(ctx, `
			UPDATE enrollment.phases
			SET form_schema_id = ?, updated_at = NOW()
			WHERE tenant_id = ? AND form_schema_id IN (?)
		`, newID, l.latest.TenantID, bun.List(l.versionIDs)); err != nil {
			return fmt.Errorf("failed repointing phases of schema %q (tenant %d): %w", l.latest.Name, l.latest.TenantID, err)
		}
		migrated++
	}

	fmt.Printf("Migration 1.15.166: Converted %d form-schema lineage(s).\n", migrated)
	return tx.Commit()
}

// convertLegacyDepartureFields rewrites a fields JSONB payload, replacing
// legacy departure fields with the modern allowed-departure-modes field.
// Returns the converted JSON and whether anything changed. Fields are kept
// as generic maps so unknown/extra attributes survive the round trip.
func convertLegacyDepartureFields(fieldsJSON string) (string, bool, error) {
	var fields []map[string]any
	if err := json.Unmarshal([]byte(fieldsJSON), &fields); err != nil {
		return "", false, fmt.Errorf("failed decoding fields: %w", err)
	}

	firstLegacyIndex := -1
	hasModern := false
	anyLegacyRequired := false
	usedKeys := map[string]bool{}
	for i, f := range fields {
		key, _ := f["key"].(string)
		usedKeys[key] = true
		target, _ := f["target"].(string)
		if target == modernDepartureTarget {
			hasModern = true
		}
		if legacyDepartureTargets[target] {
			if firstLegacyIndex < 0 {
				firstLegacyIndex = i
			}
			if required, _ := f["required"].(bool); required {
				anyLegacyRequired = true
			}
		}
	}
	if firstLegacyIndex < 0 {
		return "", false, nil
	}

	next := make([]map[string]any, 0, len(fields))
	for i, f := range fields {
		target, _ := f["target"].(string)
		if legacyDepartureTargets[target] {
			if i == firstLegacyIndex && !hasModern {
				key := modernDepartureKey
				for usedKeys[key] {
					key += "_1"
				}
				next = append(next, map[string]any{
					"key":              key,
					"label":            modernDepartureLabel,
					"type":             "weekday_multi_mode",
					"required":         anyLegacyRequired,
					"help_text":        modernDepartureHelpText,
					"applies_to_child": true,
					"target":           modernDepartureTarget,
				})
			}
			continue
		}
		next = append(next, f)
	}
	for i := range next {
		next[i]["sort_order"] = i
	}

	out, err := json.Marshal(next)
	if err != nil {
		return "", false, fmt.Errorf("failed encoding converted fields: %w", err)
	}
	return string(out), true, nil
}
