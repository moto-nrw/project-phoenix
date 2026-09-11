package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

func careOfferingLinks(rows []requestChildOfferingRow) []enrollment.CareOfferingLink {
	links := make([]enrollment.CareOfferingLink, 0, len(rows))
	for _, row := range rows {
		links = append(links, enrollment.CareOfferingLink{
			ID: row.ID, TenantID: row.TenantID, RequestChildID: row.RequestChildID, CareOfferingID: row.CareOfferingID,
			SelectedDays: row.SelectedDays, ValidFrom: row.ValidFrom, ValidUntil: row.ValidUntil,
		})
	}
	return links
}

func (r *Store) ApprovedBookingOfferingLinks(ctx context.Context) ([]enrollment.CareOfferingLink, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []requestChildOfferingRow
	err = db.NewSelect().Model(&rows).
		Join(`JOIN enrollment.request_children AS child ON child.id = request_child_offering.request_child_id AND child.tenant_id = request_child_offering.tenant_id`).
		Where("request_child_offering.tenant_id = ?", tenantID).
		Where("child.status = ?", enrollment.ChildStatusApproved).
		OrderExpr("request_child_offering.id ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("list approved booking offering links: %w", err)
	}
	return careOfferingLinks(rows), nil
}

func (r *Store) CareExitOfferingLinks(ctx context.Context, studentIDs []int64) ([]enrollment.CareOfferingLink, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []requestChildOfferingRow
	query := db.NewSelect().Model(&rows).
		Join(`JOIN enrollment.request_children AS child ON child.id = request_child_offering.request_child_id AND child.tenant_id = request_child_offering.tenant_id`).
		Where("request_child_offering.tenant_id = ?", tenantID)
	if len(studentIDs) > 0 {
		query = query.Where("(child.created_student_id IN (?) OR child.matched_student_id IN (?))", bun.List(studentIDs), bun.List(studentIDs))
	}
	if err := query.OrderExpr("request_child_offering.id ASC").Scan(ctx); err != nil {
		return nil, fmt.Errorf("list care-exit offering links: %w", err)
	}
	return careOfferingLinks(rows), nil
}

func (r *Store) LockCareExitOfferingLinks(ctx context.Context, requestChildIDs []int64, validUntil enrollment.Date) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	_, err = db.NewRaw(`SELECT link.id
		FROM enrollment.request_child_offerings AS link
		WHERE link.tenant_id = ? AND link.request_child_id IN (?)
		  AND (link.valid_until IS NULL OR link.valid_until > ?)
		FOR UPDATE OF link`, tenantID, bun.List(requestChildIDs), validUntil).Exec(ctx)
	if err != nil {
		return fmt.Errorf("lock care-exit offering links: %w", err)
	}
	return nil
}

func (r *Store) CareExitOfferingSnapshots(ctx context.Context, studentIDs []int64, validUntil enrollment.Date, sourceRequestChildID *int64) ([]enrollment.CareExitOfferingSnapshot, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	snapshots := []enrollment.CareExitOfferingSnapshot{}
	err = db.NewRaw(`SELECT link.tenant_id, child.created_student_id AS student_id,
		  link.request_child_id, link.id AS source_row_id,
		  COALESCE(link.valid_from, '-infinity'::date) >= ? AS was_deleted,
		  to_jsonb(link) AS snapshot
		FROM enrollment.request_child_offerings AS link
		JOIN enrollment.request_children AS child
		  ON child.id = link.request_child_id AND child.tenant_id = link.tenant_id
		WHERE link.tenant_id = ? AND child.created_student_id IN (?)
		  AND (? IS NULL OR link.request_child_id = ?)
		  AND (link.valid_until IS NULL OR link.valid_until > ?)
		ORDER BY link.id`, validUntil, tenantID, bun.List(studentIDs), sourceRequestChildID, sourceRequestChildID, validUntil).Scan(ctx, &snapshots)
	if err != nil {
		return nil, fmt.Errorf("snapshot care-exit offering links: %w", err)
	}
	return snapshots, nil
}

func (r *Store) EndCareExitOfferingLinks(ctx context.Context, requestChildIDs []int64, sourceRequestChildID *int64, validUntil enrollment.Date) (int64, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return 0, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return 0, err
	}
	deleted, err := db.NewRaw(`DELETE FROM enrollment.request_child_offerings AS link
		WHERE link.tenant_id = ? AND link.request_child_id IN (?)
		  AND (? IS NULL OR link.request_child_id = ?)
		  AND COALESCE(link.valid_from, '-infinity'::date) >= ?
		  AND (link.valid_until IS NULL OR link.valid_until > ?)`,
		tenantID, bun.List(requestChildIDs), sourceRequestChildID, sourceRequestChildID, validUntil, validUntil).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete future care-exit offering links: %w", err)
	}
	deletedRows, err := deleted.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count deleted care-exit offering links: %w", err)
	}
	capped, err := db.NewRaw(`UPDATE enrollment.request_child_offerings AS link
		SET valid_until = ?, updated_at = NOW()
		WHERE link.tenant_id = ? AND link.request_child_id IN (?)
		  AND (? IS NULL OR link.request_child_id = ?)
		  AND COALESCE(link.valid_from, '-infinity'::date) < ?
		  AND (link.valid_until IS NULL OR link.valid_until > ?)`,
		validUntil, tenantID, bun.List(requestChildIDs), sourceRequestChildID, sourceRequestChildID, validUntil, validUntil).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("cap care-exit offering links: %w", err)
	}
	cappedRows, err := capped.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count capped care-exit offering links: %w", err)
	}
	return deletedRows + cappedRows, nil
}

const careExitOfferingSnapshotRecordset = `jsonb_to_recordset(?::jsonb) AS rm(
	source_row_id bigint, was_deleted boolean, snapshot jsonb
)`

func (r *Store) RestoreCareExitOfferingLinks(ctx context.Context, snapshots []enrollment.CareExitOfferingSnapshotRestore) (int64, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return 0, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return 0, err
	}
	encoded, err := json.Marshal(snapshots)
	if err != nil {
		return 0, fmt.Errorf("encode care-exit offering snapshots: %w", err)
	}
	ledger := string(encoded)
	_, err = db.NewRaw(`UPDATE enrollment.request_child_offerings AS link
		SET valid_until = NULLIF(rm.snapshot->>'valid_until', '')::date,
		    updated_at = NOW()
		FROM `+careExitOfferingSnapshotRecordset+`
		WHERE rm.was_deleted = FALSE
		  AND link.tenant_id = ? AND link.id = rm.source_row_id`, ledger, tenantID).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("restore capped care-exit offering links: %w", err)
	}
	inserted, err := db.NewRaw(`INSERT INTO enrollment.request_child_offerings
		SELECT (jsonb_populate_record(NULL::enrollment.request_child_offerings, rm.snapshot)).*
		FROM `+careExitOfferingSnapshotRecordset+`
		WHERE rm.was_deleted = TRUE
		  AND (rm.snapshot->>'tenant_id')::bigint = ?
		ON CONFLICT DO NOTHING`, ledger, tenantID).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("restore deleted care-exit offering links: %w", err)
	}
	rows, err := inserted.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("count restored care-exit offering links: %w", err)
	}
	return rows, nil
}
