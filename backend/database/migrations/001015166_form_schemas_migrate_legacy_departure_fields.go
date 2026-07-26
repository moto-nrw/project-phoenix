package migrations

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
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

type departureSchemaRow struct {
	ID               int64
	TenantID         int64
	Name             string
	Version          int
	Fields           string
	CreatedBy        int64
	CoreRequirements string
	LegalBlocks      string
}

// formSchemasMigrateLegacyDepartureUp converts every form-schema lineage
// whose LATEST version still declares legacy departure fields. It mirrors
// exactly what an admin edit through the form editor does: publish a new
// version with the converted fields and advance phase bindings to it.
// Older versions are never mutated — submitted requests pin their
// schema_id, and the decision service still dispatches the legacy targets
// for those pinned schemas, so pending legacy submissions keep working.
// If the latest version is already clean, live phases pinned to older
// legacy versions are still advanced onto that clean latest version.
//
// Conversion rules per lineage:
//   - all legacy departure fields and any duplicate modern departure fields
//     collapse into exactly one student.allowed_departure_modes field
//   - the first existing modern field is reused when present; otherwise the
//     first legacy field is replaced with a default modern field
//   - requiredness is OR-ed across every collapsed departure field
//   - visibility is kept only when every collapsed departure field had the
//     exact same condition; mixed or broader scopes become unconditional
//   - surviving fields that depended on removed legacy field keys become
//     unconditional, because the modern weekday_multi_mode field is not a
//     valid scalar visibility controller
//   - sort_order is renumbered 0..n-1
//
// Runs on the superuser CLI connection, so RLS is bypassed and all tenants
// are covered in one pass. Idempotent: a migrated lineage's latest version
// has no legacy fields, so a re-run only repairs any remaining legacy phase
// bindings and publishes nothing new.
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

	rows, err := tx.QueryContext(ctx, `
		SELECT id, tenant_id, name, version, fields::text, created_by, core_requirements::text, legal_blocks::text
		FROM enrollment.form_schemas
		ORDER BY tenant_id, name, version, id
	`)
	if err != nil {
		return fmt.Errorf("failed to list form schemas: %w", err)
	}

	// Group version rows into lineages keyed by (tenant, name). The last
	// row per key wins as "latest" thanks to the ORDER BY above.
	type lineage struct {
		latest     departureSchemaRow
		versionIDs []int64
		versions   []departureSchemaRow
	}
	lineages := map[string]*lineage{}
	var keys []string
	for rows.Next() {
		var r departureSchemaRow
		if err := rows.Scan(&r.ID, &r.TenantID, &r.Name, &r.Version, &r.Fields, &r.CreatedBy, &r.CoreRequirements, &r.LegalBlocks); err != nil {
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
		l.versions = append(l.versions, r)
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
			legacyVersionIDs, err := legacyDepartureVersionIDs(l.versions)
			if err != nil {
				return fmt.Errorf("schema lineage %s (tenant %d): %w", l.latest.Name, l.latest.TenantID, err)
			}
			if len(legacyVersionIDs) > 0 {
				if _, err := tx.ExecContext(ctx, `
					UPDATE enrollment.phases
					SET form_schema_id = ?, updated_at = NOW()
					WHERE tenant_id = ? AND form_schema_id IN (?)
				`, l.latest.ID, l.latest.TenantID, bun.List(legacyVersionIDs)); err != nil {
					return fmt.Errorf("failed repointing legacy phases of clean schema %q (tenant %d): %w", l.latest.Name, l.latest.TenantID, err)
				}
			}
			continue
		}
		if err := validateConvertedDepartureSchema(l.latest, l.latest.Version+1, converted); err != nil {
			return fmt.Errorf("schema %d (%s v%d): converted schema is invalid: %w", l.latest.ID, l.latest.Name, l.latest.Version, err)
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

func validateConvertedDepartureSchema(source departureSchemaRow, version int, fieldsJSON string) error {
	var fields []enrollmentModels.FormField
	if err := json.Unmarshal([]byte(fieldsJSON), &fields); err != nil {
		return fmt.Errorf("failed decoding converted fields: %w", err)
	}
	var coreRequirements enrollmentModels.CoreRequirements
	if err := json.Unmarshal([]byte(source.CoreRequirements), &coreRequirements); err != nil {
		return fmt.Errorf("failed decoding core requirements: %w", err)
	}
	var legalBlocks []enrollmentModels.FormLegalBlock
	if err := json.Unmarshal([]byte(source.LegalBlocks), &legalBlocks); err != nil {
		return fmt.Errorf("failed decoding legal blocks: %w", err)
	}
	schema := &enrollmentModels.FormSchema{
		Name:             source.Name,
		Version:          version,
		Fields:           fields,
		CoreRequirements: coreRequirements,
		LegalBlocks:      legalBlocks,
		CreatedBy:        source.CreatedBy,
	}
	return schema.Validate()
}

func legacyDepartureVersionIDs(versions []departureSchemaRow) ([]int64, error) {
	ids := make([]int64, 0, len(versions))
	for _, version := range versions {
		hasLegacy, err := hasLegacyDepartureFields(version.Fields)
		if err != nil {
			return nil, fmt.Errorf("schema %d (%s v%d): %w", version.ID, version.Name, version.Version, err)
		}
		if hasLegacy {
			ids = append(ids, version.ID)
		}
	}
	return ids, nil
}

func hasLegacyDepartureFields(fieldsJSON string) (bool, error) {
	var fields []map[string]any
	if err := json.Unmarshal([]byte(fieldsJSON), &fields); err != nil {
		return false, fmt.Errorf("failed decoding fields: %w", err)
	}
	for _, f := range fields {
		target, _ := f["target"].(string)
		if legacyDepartureTargets[target] {
			return true, nil
		}
	}
	return false, nil
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
	firstDepartureIndex := -1
	var firstModernField map[string]any
	anyDepartureRequired := false
	var firstDepartureVisibleWhen any
	departureVisibleWhenShared := true
	removedLegacyKeys := map[string]bool{}
	usedKeys := map[string]bool{}
	for i, f := range fields {
		key, _ := f["key"].(string)
		usedKeys[key] = true
		target, _ := f["target"].(string)
		isLegacy := legacyDepartureTargets[target]
		isModern := target == modernDepartureTarget
		if isLegacy || isModern {
			if firstDepartureIndex < 0 {
				firstDepartureIndex = i
				firstDepartureVisibleWhen = f["visible_when"]
			} else if !jsonValuesEqual(firstDepartureVisibleWhen, f["visible_when"]) {
				departureVisibleWhenShared = false
			}
			if required, _ := f["required"].(bool); required {
				anyDepartureRequired = true
			}
		}
		if target == modernDepartureTarget {
			if firstModernField == nil {
				firstModernField = f
			} else {
				removedLegacyKeys[key] = true
			}
		}
		if legacyDepartureTargets[target] {
			if firstLegacyIndex < 0 {
				firstLegacyIndex = i
			}
			removedLegacyKeys[key] = true
		}
	}
	if firstLegacyIndex < 0 {
		return "", false, nil
	}

	next := make([]map[string]any, 0, len(fields))
	for i, f := range fields {
		target, _ := f["target"].(string)
		isLegacy := legacyDepartureTargets[target]
		isModern := target == modernDepartureTarget
		if isLegacy || isModern {
			if i == firstDepartureIndex {
				var modern map[string]any
				if firstModernField != nil {
					modern = firstModernField
				} else {
					key := modernDepartureKey
					for usedKeys[key] {
						key += "_1"
					}
					modern = map[string]any{
						"key":              key,
						"label":            modernDepartureLabel,
						"type":             "weekday_multi_mode",
						"help_text":        modernDepartureHelpText,
						"applies_to_child": true,
						"target":           modernDepartureTarget,
					}
				}
				modern["required"] = anyDepartureRequired
				if departureVisibleWhenShared && firstDepartureVisibleWhen != nil {
					modern["visible_when"] = firstDepartureVisibleWhen
				} else {
					delete(modern, "visible_when")
				}
				clearLegacyFieldVisibilityDependency(modern, removedLegacyKeys)
				next = append(next, modern)
			}
			continue
		}
		clearLegacyFieldVisibilityDependency(f, removedLegacyKeys)
		next = append(next, f)
	}
	for _, f := range next {
		clearLegacyFieldVisibilityDependency(f, removedLegacyKeys)
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

func jsonValuesEqual(a, b any) bool {
	aJSON, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bJSON, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(aJSON) == string(bJSON)
}

func clearLegacyFieldVisibilityDependency(field map[string]any, removedLegacyKeys map[string]bool) {
	visibleWhen, ok := field["visible_when"].(map[string]any)
	if !ok {
		return
	}
	source, _ := visibleWhen["source"].(string)
	if source != enrollmentModels.ConditionSourceField {
		return
	}
	fieldKey, _ := visibleWhen["field"].(string)
	if removedLegacyKeys[fieldKey] {
		delete(field, "visible_when")
	}
}
