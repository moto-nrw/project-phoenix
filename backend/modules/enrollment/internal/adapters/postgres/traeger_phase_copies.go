package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// The copy of a Träger phase in one school (ADR 0195 section 4). These
// statements run in the school's tenant transaction and filter on its tenant.

type traegerCopyStateRow struct {
	ID      int64  `bun:"id"`
	Version int    `bun:"traeger_source_version"`
	Stamp   string `bun:"stamp"`
}

const (
	traegerPhaseCopyStateQuery = `SELECT id, traeger_source_version,
		concat_ws('|', id, traeger_source_version, updated_at, is_active, array_to_string(traeger_deviations, ',')) AS stamp
		FROM enrollment.phases WHERE tenant_id = ? AND traeger_source_id = ?`
	traegerPhaseCopyStateLockQuery = traegerPhaseCopyStateQuery + ` FOR UPDATE`
)

// TraegerPhaseCopyState reads the copy of sourceID; lock takes its row lock.
func (r *Store) TraegerPhaseCopyState(ctx context.Context, sourceID int64, lock bool) (enrollment.TraegerCopyState, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return enrollment.TraegerCopyState{}, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return enrollment.TraegerCopyState{}, err
	}
	var rows []traegerCopyStateRow
	if lock {
		err = db.NewRaw(traegerPhaseCopyStateLockQuery, tenantID, sourceID).Scan(ctx, &rows)
	} else {
		err = db.NewRaw(traegerPhaseCopyStateQuery, tenantID, sourceID).Scan(ctx, &rows)
	}
	if err != nil {
		return enrollment.TraegerCopyState{}, fmt.Errorf("enrollment postgres: read träger phase copy: %w", err)
	}
	if len(rows) == 0 {
		return enrollment.TraegerCopyState{}, nil
	}
	return enrollment.TraegerCopyState{Exists: true, CopyID: rows[0].ID, Version: rows[0].Version, Stamp: rows[0].Stamp}, nil
}

// LinkTraegerPhaseCopy records the applied version and its values on the
// copy; the trigger recomputes the deviations from the row.
func (r *Store) LinkTraegerPhaseCopy(ctx context.Context, copyID, sourceID int64, version int, values map[string]any) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return fmt.Errorf("enrollment postgres: encode träger phase values: %w", err)
	}
	if _, err := db.NewRaw(`UPDATE enrollment.phases SET traeger_source_id = ?, traeger_source_version = ?, traeger_values = ?::jsonb,
		updated_at = NOW() WHERE tenant_id = ? AND id = ?`, sourceID, version, string(encoded), tenantID, copyID).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: link träger phase copy: %w", err)
	}
	return nil
}

// UnlinkTraegerPhaseCopy drops the Träger link of the copy: it becomes an
// ordinary phase of the school.
func (r *Store) UnlinkTraegerPhaseCopy(ctx context.Context, copyID int64) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw(`UPDATE enrollment.phases SET traeger_source_id = NULL, updated_at = NOW()
		WHERE tenant_id = ? AND id = ?`, tenantID, copyID).Exec(ctx); err != nil {
		return fmt.Errorf("enrollment postgres: unlink träger phase copy: %w", err)
	}
	return nil
}
