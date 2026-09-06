package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

func (r *Store) RequestChildOfferingHistory(ctx context.Context, childID int64) ([]*enrollment.RequestChildOffering, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []*requestChildOfferingRow
	if err := db.NewSelect().Model(&rows).
		Where("request_child_offering.request_child_id = ? AND request_child_offering.tenant_id = ?", childID, tenantID).
		OrderExpr("request_child_offering.id").Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to list request child offerings: %w", err)
	}
	var values []*enrollment.RequestChildOffering
	for _, row := range rows {
		values = append(values, row.value())
	}
	return values, nil
}

func (r *Store) InsertRequestChildOffering(ctx context.Context, selection *enrollment.RequestChildOffering) error {
	if selection == nil || selection.RequestChildID <= 0 {
		return fmt.Errorf("request_child_id is required")
	}
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	if selection.TenantID != 0 && selection.TenantID != tenantID {
		return fmt.Errorf("request child offering tenant mismatch")
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	start, exclusive, err := requestChildOfferingWindow(ctx, db, tenantID, selection.RequestChildID)
	if err != nil {
		return err
	}
	row := requestChildOfferingStorage(selection)
	row.TenantID = tenantID
	if row.ValidFrom == nil {
		row.ValidFrom = &start
	}
	if row.ValidUntil == nil {
		row.ValidUntil = &exclusive
	}
	now := time.Now()
	if row.CreatedAt.IsZero() {
		row.CreatedAt = now
	}
	row.UpdatedAt = now
	if _, err := db.NewInsert().Model(row).Returning("*").Exec(ctx); err != nil {
		return fmt.Errorf("failed to create request child offering: %w", err)
	}
	*selection = *row.value()
	return nil
}

func requestChildOfferingWindow(ctx context.Context, db bun.IDB, tenantID, childID int64) (enrollment.Date, enrollment.Date, error) {
	var window struct {
		Start enrollment.Date `bun:"service_start_date"`
		End   enrollment.Date `bun:"service_end_date"`
	}
	err := db.NewSelect().
		TableExpr("enrollment.request_children AS child").
		Join("JOIN enrollment.requests AS request ON request.id = child.request_id AND request.tenant_id = child.tenant_id").
		Join("JOIN enrollment.phases AS phase ON phase.id = request.phase_id AND phase.tenant_id = request.tenant_id").
		ColumnExpr("phase.service_start_date, phase.service_end_date").
		Where("child.id = ? AND child.tenant_id = ?", childID, tenantID).
		Scan(ctx, &window)
	if err != nil {
		return "", "", fmt.Errorf("find care period for request child %d: %w", childID, err)
	}
	end, err := time.Parse(time.DateOnly, string(window.End))
	if err != nil {
		return "", "", fmt.Errorf("parse request child care period end: %w", err)
	}
	exclusive := enrollment.Date(end.AddDate(0, 0, 1).Format(time.DateOnly))
	return window.Start, exclusive, nil
}

func (r *Store) ReplaceRequestChildOfferings(ctx context.Context, childID int64, selections []*enrollment.RequestChildOffering) error {
	if childID <= 0 {
		return fmt.Errorf("request_child_id is required")
	}
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	if err := lockRequestChildOfferingRows(ctx, db, tenantID, childID); err != nil {
		return err
	}
	start, exclusive, err := requestChildOfferingWindow(ctx, db, tenantID, childID)
	if err != nil {
		return err
	}
	rows := make([]*requestChildOfferingRow, 0, len(selections))
	now := time.Now()
	for _, selection := range selections {
		if selection == nil {
			return fmt.Errorf("request child offering row cannot be nil")
		}
		if selection.TenantID != 0 && selection.TenantID != tenantID {
			return fmt.Errorf("request child offering tenant mismatch")
		}
		row := requestChildOfferingStorage(selection)
		row.RequestChildID, row.TenantID = childID, tenantID
		if row.ValidFrom == nil {
			row.ValidFrom = &start
		}
		if row.ValidUntil == nil {
			row.ValidUntil = &exclusive
		}
		if row.CreatedAt.IsZero() {
			row.CreatedAt = now
		}
		row.UpdatedAt = now
		rows = append(rows, row)
	}
	if _, err := db.NewDelete().Model((*requestChildOfferingRow)(nil)).Where("request_child_id = ? AND tenant_id = ?", childID, tenantID).Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete request child offerings: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}
	if _, err := db.NewInsert().Model(&rows).Returning("*").Exec(ctx); err != nil {
		return fmt.Errorf("failed to insert replacement request child offerings: %w", err)
	}
	for index, row := range rows {
		*selections[index] = *row.value()
	}
	return nil
}

func lockRequestChildOfferingRows(ctx context.Context, db bun.IDB, tenantID, childID int64) error {
	var lockedID int64
	if err := db.NewSelect().TableExpr("enrollment.request_children AS child").ColumnExpr("child.id").Where("child.id = ? AND child.tenant_id = ?", childID, tenantID).For("UPDATE").Scan(ctx, &lockedID); err != nil {
		return fmt.Errorf("failed to lock request child offerings: %w", err)
	}
	return nil
}

func (r *Store) ScheduleRequestChildOfferings(ctx context.Context, childID int64, effectiveFrom enrollment.Date, selections []*enrollment.RequestChildOffering) error {
	if childID <= 0 {
		return fmt.Errorf("request_child_id is required")
	}
	if effectiveFrom.IsZero() {
		return fmt.Errorf("effective_from is required")
	}
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	if err := lockRequestChildOfferingRows(ctx, db, tenantID, childID); err != nil {
		return err
	}
	_, exclusive, err := requestChildOfferingWindow(ctx, db, tenantID, childID)
	if err != nil {
		return err
	}
	rows := make([]*requestChildOfferingRow, 0, len(selections))
	now := time.Now()
	for _, selection := range selections {
		if selection == nil {
			return fmt.Errorf("request child offering row cannot be nil")
		}
		if selection.TenantID != 0 && selection.TenantID != tenantID {
			return fmt.Errorf("request child offering tenant mismatch")
		}
		row := requestChildOfferingStorage(selection)
		row.RequestChildID, row.TenantID = childID, tenantID
		row.ValidFrom = &effectiveFrom
		if row.ValidUntil == nil || exclusive.Before(*row.ValidUntil) {
			row.ValidUntil = &exclusive
		}
		if row.CreatedAt.IsZero() {
			row.CreatedAt = now
		}
		row.UpdatedAt = now
		rows = append(rows, row)
	}
	if _, err := db.NewDelete().Model((*requestChildOfferingRow)(nil)).Where("request_child_id = ? AND tenant_id = ?", childID, tenantID).Where("valid_from >= ?", effectiveFrom).Exec(ctx); err != nil {
		return fmt.Errorf("failed to delete superseded scheduled request child offerings: %w", err)
	}
	if _, err := db.NewUpdate().Model((*requestChildOfferingRow)(nil)).Set("valid_until = ?", effectiveFrom).Where("request_child_id = ? AND tenant_id = ?", childID, tenantID).Where("(valid_from IS NULL OR valid_from < ?)", effectiveFrom).Where("(valid_until IS NULL OR valid_until > ?)", effectiveFrom).Exec(ctx); err != nil {
		return fmt.Errorf("failed to close current request child offerings: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}
	if _, err := db.NewInsert().Model(&rows).Returning("*").Exec(ctx); err != nil {
		return fmt.Errorf("failed to insert scheduled request child offerings: %w", err)
	}
	for index, row := range rows {
		*selections[index] = *row.value()
	}
	return nil
}

func (r *Store) RequestChildOfferingsAtDate(ctx context.Context, childID int64, onDate enrollment.Date) ([]*enrollment.RequestChildOffering, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []*requestChildOfferingRow
	query := func() *bun.SelectQuery {
		return db.NewSelect().Model(&rows).
			Where("request_child_offering.request_child_id = ? AND request_child_offering.tenant_id = ?", childID, tenantID).
			OrderExpr("request_child_offering.id")
	}
	err = query().Where("(valid_from IS NULL OR valid_from <= ?)", onDate).
		Where("(valid_until IS NULL OR valid_until > ?)", onDate).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list request child offerings: %w", err)
	}
	if len(rows) == 0 {
		var phaseStart enrollment.Date
		err = db.NewSelect().TableExpr("enrollment.request_children AS child").
			Join("JOIN enrollment.requests AS request ON request.id = child.request_id AND request.tenant_id = child.tenant_id").
			Join("JOIN enrollment.phases AS phase ON phase.id = request.phase_id AND phase.tenant_id = request.tenant_id").
			ColumnExpr("phase.service_start_date").Where("child.id = ? AND child.tenant_id = ?", childID, tenantID).Scan(ctx, &phaseStart)
		if err != nil {
			return nil, fmt.Errorf("failed to find request child care period: %w", err)
		}
		if !onDate.Before(phaseStart) {
			return nil, nil
		}
		next := db.NewSelect().TableExpr("enrollment.request_child_offerings AS next_selection").
			ColumnExpr("MIN(next_selection.valid_from)").
			Where("next_selection.request_child_id = ? AND next_selection.tenant_id = ?", childID, tenantID).
			Where("next_selection.valid_from > ?", onDate)
		if err := query().Where("valid_from = (?)", next).Scan(ctx); err != nil {
			return nil, fmt.Errorf("failed to list upcoming request child offerings: %w", err)
		}
	}
	var values []*enrollment.RequestChildOffering
	for _, row := range rows {
		values = append(values, row.value())
	}
	return values, nil
}

func (r *Store) RequestChildOfferingsForChildrenAtDate(ctx context.Context, childIDs []int64, onDate enrollment.Date) ([]*enrollment.RequestChildOffering, error) {
	if len(childIDs) == 0 {
		return nil, nil
	}
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []*requestChildOfferingRow
	if err := db.NewSelect().Model(&rows).
		Where("request_child_offering.tenant_id = ?", tenantID).
		Where("request_child_offering.request_child_id IN (?)", bun.List(childIDs)).
		Where("(valid_from IS NULL OR valid_from <= ?)", onDate).
		Where("(valid_until IS NULL OR valid_until > ?)", onDate).
		OrderExpr("request_child_offering.request_child_id, request_child_offering.id").Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to list request child offerings by child ids at date: %w", err)
	}
	var values []*enrollment.RequestChildOffering
	for _, row := range rows {
		values = append(values, row.value())
	}
	return values, nil
}

func (r *Store) RequestChildOfferingHistoryForChildren(ctx context.Context, childIDs []int64) ([]*enrollment.RequestChildOffering, error) {
	if len(childIDs) == 0 {
		return nil, nil
	}
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var rows []*requestChildOfferingRow
	if err := db.NewSelect().Model(&rows).
		Where("request_child_offering.tenant_id = ?", tenantID).
		Where("request_child_offering.request_child_id IN (?)", bun.List(childIDs)).
		OrderExpr("request_child_offering.request_child_id, request_child_offering.id").Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to list request child offerings by child ids: %w", err)
	}
	var values []*enrollment.RequestChildOffering
	for _, row := range rows {
		values = append(values, row.value())
	}
	return values, nil
}

func (r *Store) RequestChildOfferingsAtDates(ctx context.Context, dates map[int64]enrollment.Date) ([]*enrollment.RequestChildOffering, error) {
	if len(dates) == 0 {
		return nil, nil
	}
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(dates))
	for id := range dates {
		ids = append(ids, id)
	}
	var history []*requestChildOfferingRow
	if err := db.NewSelect().Model(&history).
		Where("request_child_offering.tenant_id = ?", tenantID).
		Where("request_child_offering.request_child_id IN (?)", bun.List(ids)).
		OrderExpr("request_child_offering.request_child_id, request_child_offering.id").Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to list request child offerings by child ids at dates: %w", err)
	}
	rows := make([]*enrollment.RequestChildOffering, 0, len(history))
	for _, row := range history {
		date, ok := dates[row.RequestChildID]
		if !ok || (row.ValidFrom != nil && date.Before(*row.ValidFrom)) || (row.ValidUntil != nil && !date.Before(*row.ValidUntil)) {
			continue
		}
		rows = append(rows, row.value())
	}
	return rows, nil
}

func (r *Store) ApprovedSelectionsForOfferings(ctx context.Context, offeringIDs []int64, onOrAfter enrollment.Date) ([]*enrollment.ApprovedOfferingSelection, error) {
	result := make([]*enrollment.ApprovedOfferingSelection, 0)
	if len(offeringIDs) == 0 {
		return result, nil
	}
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var links []*requestChildOfferingRow
	if err := db.NewSelect().Model(&links).
		Join("JOIN enrollment.request_children AS child ON child.id = request_child_offering.request_child_id AND child.tenant_id = request_child_offering.tenant_id").
		Where("request_child_offering.tenant_id = ?", tenantID).
		Where("request_child_offering.care_offering_id IN (?)", bun.List(offeringIDs)).
		Where("child.status = ?", enrollment.ChildStatusApproved).
		Where("(request_child_offering.valid_until IS NULL OR request_child_offering.valid_until > ?)", onOrAfter).
		OrderExpr("request_child_offering.id").Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to list approved children for care offerings: %w", err)
	}
	return resolveApprovedSelections(ctx, db, tenantID, links)
}

func resolveApprovedSelections(ctx context.Context, db bun.IDB, tenantID int64, links []*requestChildOfferingRow) ([]*enrollment.ApprovedOfferingSelection, error) {
	result := make([]*enrollment.ApprovedOfferingSelection, 0)
	if len(links) == 0 {
		return result, nil
	}
	ids := make([]int64, 0, len(links))
	seen := make(map[int64]bool)
	for _, link := range links {
		if !seen[link.RequestChildID] {
			ids = append(ids, link.RequestChildID)
			seen[link.RequestChildID] = true
		}
	}
	var children []struct {
		ChildID   int64
		StudentID int64
	}
	if err := db.NewSelect().TableExpr("enrollment.request_children AS child").
		ColumnExpr("child.id AS child_id, COALESCE(child.created_student_id, child.matched_student_id) AS student_id").
		Where("child.tenant_id = ?", tenantID).Where("child.id IN (?)", bun.List(ids)).
		Where("COALESCE(child.created_student_id, child.matched_student_id) IS NOT NULL").Scan(ctx, &children); err != nil {
		return nil, fmt.Errorf("failed to resolve students for approved offering children: %w", err)
	}
	students := make(map[int64]int64, len(children))
	for _, child := range children {
		students[child.ChildID] = child.StudentID
	}
	for _, link := range links {
		if studentID, ok := students[link.RequestChildID]; ok {
			result = append(result, &enrollment.ApprovedOfferingSelection{Selection: link.value(), StudentID: studentID})
		}
	}
	return result, nil
}

func (r *Store) ApprovedSelectionsForStudents(ctx context.Context, studentIDs []int64, from, to enrollment.Date) ([]*enrollment.ApprovedOfferingSelection, error) {
	result := make([]*enrollment.ApprovedOfferingSelection, 0)
	if len(studentIDs) == 0 || to.Before(from) {
		return result, nil
	}
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	var links []*requestChildOfferingRow
	if err := db.NewSelect().Model(&links).
		Join("JOIN enrollment.request_children AS child ON child.id = request_child_offering.request_child_id AND child.tenant_id = request_child_offering.tenant_id").
		Where("request_child_offering.tenant_id = ?", tenantID).
		Where("COALESCE(child.created_student_id, child.matched_student_id) IN (?)", bun.List(studentIDs)).
		Where("child.status = ?", enrollment.ChildStatusApproved).
		Where("(request_child_offering.valid_until IS NULL OR request_child_offering.valid_until > ?)", from).
		Where("(request_child_offering.valid_from IS NULL OR request_child_offering.valid_from <= ?)", to).
		OrderExpr("request_child_offering.id").Scan(ctx); err != nil {
		return nil, fmt.Errorf("failed to list approved offering links for students in range: %w", err)
	}
	return resolveApprovedSelections(ctx, db, tenantID, links)
}
