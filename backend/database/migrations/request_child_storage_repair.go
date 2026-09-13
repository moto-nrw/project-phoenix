package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/uptrace/bun"
)

// CompatibilityRepairOptions never authorizes writes to the two owner targets.
type CompatibilityRepairOptions struct {
	BatchSize  int
	TenantIDs  []int64
	VerifyOnly bool
}

type CompatibilityTenantEvidence struct {
	TenantID                int64
	TargetRows              int64
	CompatibilityRows       int64
	TargetChecksum          string
	CompatibilityChecksum   string
	EffectiveChecksumsEqual bool
}

type CompatibilityRepairReport struct {
	Batches      int
	RepairedRows int64
	Tenants      []CompatibilityTenantEvidence
}

// RepairRequestChildStorageCompatibility repairs only rollback metadata and
// schema definitions. Completed batches are their own durable checkpoint:
// corrected rows no longer satisfy the pending-drift predicate on retry.
func RepairRequestChildStorageCompatibility(ctx context.Context, db *bun.DB, options CompatibilityRepairOptions) (CompatibilityRepairReport, error) {
	return repairRequestChildStorageCompatibility(ctx, db, options, nil)
}

func repairRequestChildStorageCompatibility(ctx context.Context, db *bun.DB, options CompatibilityRepairOptions, afterBatch func() error) (report CompatibilityRepairReport, err error) {
	if options.BatchSize == 0 {
		options.BatchSize = 500
	}
	if options.BatchSize < 1 || options.BatchSize > 10000 {
		return report, fmt.Errorf("compatibility repair batch size must be between 1 and 10000")
	}
	for _, id := range options.TenantIDs {
		if id <= 0 {
			return report, fmt.Errorf("compatibility repair requires positive school IDs")
		}
	}
	release, err := lockRequestChildStorageBackfill(ctx, db)
	if err != nil {
		return report, err
	}
	defer release()
	var kind string
	if err := db.NewRaw("SELECT relkind::text FROM pg_class WHERE oid = 'enrollment.request_child_offerings'::regclass").Scan(ctx, &kind); err != nil {
		return report, err
	}
	if kind != "v" {
		return report, fmt.Errorf("compatibility repair requires the committed cutover view")
	}
	if !options.VerifyOnly {
		err = compatibilityRepairTransaction(ctx, db, func(ctx context.Context, tx bun.Tx) error {
			return refreshRequestChildStorageCompatibility(ctx, tx)
		})
		if err != nil {
			return report, err
		}
		for {
			if err := ctx.Err(); err != nil {
				return report, err
			}
			var repaired int64
			err = compatibilityRepairTransaction(ctx, db, func(ctx context.Context, tx bun.Tx) error {
				result, err := tx.NewRaw(compatibilityRepairBatchSQL, sqlBigintArray(options.TenantIDs), options.BatchSize).Exec(ctx)
				if err != nil {
					return err
				}
				repaired, err = result.RowsAffected()
				return err
			})
			if err != nil {
				return report, err
			}
			if repaired == 0 {
				break
			}
			report.Batches++
			report.RepairedRows += repaired
			if afterBatch != nil {
				if err := afterBatch(); err != nil {
					return report, err
				}
			}
		}
	}
	err = db.RunInTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, "SET LOCAL statement_timeout = '30s'; SET LOCAL TIME ZONE 'UTC'"); err != nil {
			return err
		}
		query := "SELECT * FROM (" + compatibilityVerificationSQL + ") evidence"
		if len(options.TenantIDs) > 0 {
			return tx.NewRaw(query+" WHERE tenant_id IN (?) ORDER BY tenant_id", bun.List(options.TenantIDs)).Scan(ctx, &report.Tenants)
		}
		return tx.NewRaw(query+" ORDER BY tenant_id").Scan(ctx, &report.Tenants)
	})
	if err != nil {
		return report, err
	}
	for _, school := range report.Tenants {
		if !school.EffectiveChecksumsEqual {
			return report, fmt.Errorf("compatibility checksum drift remains for school %d", school.TenantID)
		}
	}
	return report, nil
}

func compatibilityRepairTransaction(ctx context.Context, db *bun.DB, fn func(context.Context, bun.Tx) error) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout = '5s'; SET LOCAL statement_timeout = '30s';
   LOCK TABLE enrollment.care_offering_bookings, enrollment.request_child_offerings_legacy IN SHARE ROW EXCLUSIVE MODE`); err != nil {
			return err
		}
		return fn(ctx, tx)
	})
}

// Only an archived selected-days payload that the compatibility view actually
// exposes can drift. Preserve every healthy historical JSON shape and all notes.
const compatibilityRepairBatchSQL = `
 WITH pending AS (
  SELECT archive.id
  FROM enrollment.request_child_offerings_legacy archive
  JOIN enrollment.care_offering_bookings booking ON booking.id = archive.id AND booking.tenant_id = archive.tenant_id
  WHERE (cardinality(?0::bigint[]) = 0 OR booking.tenant_id = ANY(?0::bigint[]))
   AND booking.manual_selected_days IS NOT DISTINCT FROM enrollment.request_child_legacy_manual(archive.selected_days, archive.manual_selected_days, archive.automatic_selected_days)
   AND booking.automatic_selected_days IS NOT DISTINCT FROM archive.automatic_selected_days
   AND enrollment.request_child_effective_days(archive.selected_days, NULL)
    IS DISTINCT FROM enrollment.request_child_effective_days(booking.manual_selected_days, booking.automatic_selected_days)
  ORDER BY archive.tenant_id, archive.id LIMIT ?1
 )
 UPDATE enrollment.request_child_offerings_legacy archive
 SET selected_days = enrollment.request_child_effective_days(booking.manual_selected_days, booking.automatic_selected_days)
 FROM enrollment.care_offering_bookings booking, pending
 WHERE archive.id = pending.id AND booking.id = archive.id AND booking.tenant_id = archive.tenant_id
`

const compatibilityVerificationSQL = `
WITH rows AS (
    SELECT 'target' AS provider, tenant_id, id, request_child_id, care_offering_id,
           manual_selected_days, automatic_selected_days, valid_from, valid_until,
           created_at, updated_at,
           CASE WHEN manual_selected_days IS NULL AND automatic_selected_days IS NULL THEN NULL ELSE COALESCE((
               SELECT jsonb_agg(day ORDER BY array_position(ARRAY['mon','tue','wed','thu','fri','sat','sun'], day), day)
               FROM (SELECT DISTINCT jsonb_array_elements_text(
                   CASE WHEN jsonb_typeof(manual_selected_days) = 'array' THEN manual_selected_days ELSE '[]'::jsonb END ||
                   CASE WHEN jsonb_typeof(automatic_selected_days) = 'array' THEN automatic_selected_days ELSE '[]'::jsonb END
               ) AS day) normalized_days
           ), '[]'::jsonb) END AS effective_days
    FROM enrollment.care_offering_bookings
    UNION ALL
    SELECT 'compatibility', tenant_id, id, request_child_id, care_offering_id,
           CASE WHEN COALESCE(manual_selected_days, '[]'::jsonb) IN ('[]'::jsonb, 'null'::jsonb)
                AND COALESCE(automatic_selected_days, '[]'::jsonb) IN ('[]'::jsonb, 'null'::jsonb)
                THEN selected_days ELSE manual_selected_days END,
           automatic_selected_days, valid_from, valid_until, created_at, updated_at,
           CASE WHEN selected_days IS NULL AND NULL::jsonb IS NULL THEN NULL ELSE COALESCE((
               SELECT jsonb_agg(day ORDER BY array_position(ARRAY['mon','tue','wed','thu','fri','sat','sun'], day), day)
               FROM (SELECT DISTINCT jsonb_array_elements_text(
                   CASE WHEN jsonb_typeof(selected_days) = 'array' THEN selected_days ELSE '[]'::jsonb END ||
                   CASE WHEN jsonb_typeof(NULL::jsonb) = 'array' THEN NULL::jsonb ELSE '[]'::jsonb END
               ) AS day) normalized_days
           ), '[]'::jsonb) END
    FROM enrollment.request_child_offerings
), checksums AS (
    SELECT provider, tenant_id, count(*) AS row_count,
           encode(sha256(convert_to(string_agg(jsonb_build_array(
               id, request_child_id, care_offering_id, manual_selected_days,
               automatic_selected_days, valid_from, valid_until, created_at,
               updated_at, effective_days)::text, E'\n' ORDER BY id), 'UTF8')), 'hex') AS checksum
    FROM rows GROUP BY provider, tenant_id
)
SELECT COALESCE(target.tenant_id, compatibility.tenant_id) AS tenant_id,
       target.row_count AS target_rows, compatibility.row_count AS compatibility_rows,
       target.checksum AS target_checksum, compatibility.checksum AS compatibility_checksum,
       target.row_count IS NOT DISTINCT FROM compatibility.row_count
       AND target.checksum IS NOT DISTINCT FROM compatibility.checksum AS effective_checksums_equal
FROM (SELECT * FROM checksums WHERE provider = 'target') target
FULL JOIN (SELECT * FROM checksums WHERE provider = 'compatibility') compatibility USING (tenant_id)
ORDER BY tenant_id
`
