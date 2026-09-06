package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

func (r *Store) OfferingGradeCounts(ctx context.Context, ids []int64, from, until enrollment.Date) ([]*enrollment.OfferingGradeCount, error) {
	result := make([]*enrollment.OfferingGradeCount, 0)
	if len(ids) == 0 {
		return result, nil
	}
	if !from.Before(until) {
		return nil, fmt.Errorf("grade level range must not be empty")
	}
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		CareOfferingID int64
		GradeLevel     *int16
		ChildCount     int
	}
	err = db.NewSelect().TableExpr("enrollment.request_child_offerings AS selection").
		Join("JOIN enrollment.request_children AS child ON child.id = selection.request_child_id AND child.tenant_id = selection.tenant_id").
		ColumnExpr("selection.care_offering_id, child.target_grade_level AS grade_level, COUNT(DISTINCT selection.request_child_id) AS child_count").
		Where("selection.tenant_id = ?", tenantID).Where("selection.care_offering_id IN (?)", bun.List(ids)).
		Where("(selection.valid_from IS NULL OR selection.valid_from < ?)", until).
		Where("(selection.valid_until IS NULL OR selection.valid_until > ?)", from).
		Where("child.status NOT IN (?)", bun.List([]string{enrollment.ChildStatusRejected, enrollment.ChildStatusWithdrawn})).
		GroupExpr("1, 2").OrderExpr("1, 2").Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("failed to count grade levels for care offerings: %w", err)
	}
	for _, row := range rows {
		result = append(result, &enrollment.OfferingGradeCount{CareOfferingID: row.CareOfferingID, GradeLevel: row.GradeLevel, Count: row.ChildCount})
	}
	return result, nil
}

func (r *Store) MaterializableOfferingCount(ctx context.Context, id int64, today enrollment.Date) (int, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return 0, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return 0, err
	}
	count, err := db.NewSelect().TableExpr("enrollment.request_child_offerings AS selection").
		Join("JOIN enrollment.request_children AS child ON child.id = selection.request_child_id AND child.tenant_id = selection.tenant_id").
		Where("selection.tenant_id = ? AND selection.care_offering_id = ?", tenantID, id).
		Where("(selection.valid_until IS NULL OR selection.valid_until > ?)", today).
		Where("child.status NOT IN (?)", bun.List([]string{enrollment.ChildStatusRejected, enrollment.ChildStatusWithdrawn})).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to count materializable children for care offering %d: %w", id, err)
	}
	return count, nil
}

func (r *Store) OfferingCapacityPeaks(ctx context.Context, offeringIDs []int64, from, until enrollment.Date) (map[int64]int, error) {
	result := make(map[int64]int)
	if len(offeringIDs) == 0 {
		return result, nil
	}
	if !from.Before(until) {
		return nil, fmt.Errorf("capacity range must not be empty")
	}
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		CareOfferingID int64
		Peak           int
	}
	err = db.NewRaw(`
 WITH intervals AS (
  SELECT selection.care_offering_id, selection.request_child_id,
   GREATEST(COALESCE(selection.valid_from, ?), ?) AS starts_at,
   LEAST(COALESCE(selection.valid_until, ?), ?) AS ends_at
  FROM enrollment.request_child_offerings AS selection
  JOIN enrollment.request_children AS child
   ON child.id = selection.request_child_id AND child.tenant_id = selection.tenant_id
  WHERE selection.tenant_id = ? AND selection.care_offering_id IN (?)
   AND (selection.valid_from IS NULL OR selection.valid_from < ?)
   AND (selection.valid_until IS NULL OR selection.valid_until > ?)
   AND child.status NOT IN (?)
 ), boundaries AS (
  SELECT care_offering_id, starts_at AS boundary FROM intervals
  UNION
  SELECT care_offering_id, ends_at AS boundary FROM intervals
 )
 SELECT boundaries.care_offering_id, COALESCE(MAX((
  SELECT COUNT(DISTINCT interval_row.request_child_id)
  FROM intervals AS interval_row
  WHERE interval_row.care_offering_id = boundaries.care_offering_id
   AND interval_row.starts_at <= boundaries.boundary
   AND interval_row.ends_at > boundaries.boundary
 )), 0) AS peak
 FROM boundaries GROUP BY boundaries.care_offering_id
 `, from, from, until, until, tenantID, bun.List(offeringIDs), until, from,
		bun.List([]string{enrollment.ChildStatusRejected, enrollment.ChildStatusWithdrawn})).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("failed to count peak active children for care offerings: %w", err)
	}
	for _, row := range rows {
		result[row.CareOfferingID] = row.Peak
	}
	return result, nil
}

func (r *Store) OfferingCapacityPeak(ctx context.Context, offeringID int64, excludedChildren []int64, from, until enrollment.Date) (int, error) {
	if !from.Before(until) {
		return 0, fmt.Errorf("capacity range must not be empty")
	}
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return 0, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return 0, err
	}
	var count int
	err = db.NewRaw(`
 WITH intervals AS (
  SELECT selection.request_child_id,
   GREATEST(COALESCE(selection.valid_from, ?), ?) AS starts_at,
   LEAST(COALESCE(selection.valid_until, ?), ?) AS ends_at
  FROM enrollment.request_child_offerings AS selection
  JOIN enrollment.request_children AS child
   ON child.id = selection.request_child_id AND child.tenant_id = selection.tenant_id
  WHERE selection.tenant_id = ? AND selection.care_offering_id = ?
   AND (selection.valid_from IS NULL OR selection.valid_from < ?)
   AND (selection.valid_until IS NULL OR selection.valid_until > ?)
   AND (? = 0 OR selection.request_child_id NOT IN (?))
   AND child.status NOT IN (?)
 ), boundaries AS (
  SELECT starts_at AS boundary FROM intervals
  UNION
  SELECT ends_at AS boundary FROM intervals
 )
 SELECT COALESCE(MAX((
  SELECT COUNT(DISTINCT interval_row.request_child_id)
  FROM intervals AS interval_row
  WHERE interval_row.starts_at <= boundaries.boundary
   AND interval_row.ends_at > boundaries.boundary
 )), 0)
 FROM boundaries
 `, from, from, until, until, tenantID, offeringID, until, from,
		len(excludedChildren), bun.List(excludedChildren),
		bun.List([]string{enrollment.ChildStatusRejected, enrollment.ChildStatusWithdrawn})).Scan(ctx, &count)
	if err != nil {
		return 0, fmt.Errorf("failed to count peak active children for care offering %d: %w", offeringID, err)
	}
	return count, nil
}
