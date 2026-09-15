package postgres

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

type submittedOfferingRow struct {
	bun.BaseModel  `bun:"table:enrollment.request_child_offering_selections,alias:submitted_offering"`
	ID             int64     `bun:"id,pk,autoincrement"`
	TenantID       int64     `bun:"tenant_id"`
	RequestChildID int64     `bun:"request_child_id"`
	CareOfferingID int64     `bun:"care_offering_id"`
	SelectedDays   []string  `bun:"selected_days,type:jsonb,nullzero"`
	Notes          *string   `bun:"notes"`
	CreatedAt      time.Time `bun:"created_at,nullzero,default:current_timestamp"`
}

func (r *Store) SubmittedOfferingChoices(ctx context.Context, childIDs []int64) ([]enrollment.SubmittedOfferingChoice, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	rows := []submittedOfferingRow{}
	if err := db.NewSelect().Model(&rows).
		Where("submitted_offering.tenant_id = ? AND submitted_offering.request_child_id IN (?)", tenantID, bun.List(childIDs)).
		OrderExpr("submitted_offering.request_child_id, submitted_offering.id").Scan(ctx); err != nil {
		return nil, fmt.Errorf("read submitted offering choices: %w", err)
	}
	choices := make([]enrollment.SubmittedOfferingChoice, 0, len(rows))
	for _, row := range rows {
		choices = append(choices, enrollment.SubmittedOfferingChoice{
			ID: row.ID, TenantID: row.TenantID, RequestChildID: row.RequestChildID,
			CareOfferingID: row.CareOfferingID, SelectedDays: row.SelectedDays, Notes: row.Notes, CreatedAt: row.CreatedAt,
		})
	}
	return choices, nil
}

func (r *Store) RecordSubmittedOfferingChoices(ctx context.Context, childID int64, choices []enrollment.SubmittedOfferingChoice) error {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return err
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return err
	}
	for _, choice := range choices {
		if choice.TenantID != 0 && choice.TenantID != tenantID {
			return fmt.Errorf("submitted offering choice tenant mismatch")
		}
		row := &submittedOfferingRow{TenantID: tenantID, RequestChildID: childID, CareOfferingID: choice.CareOfferingID, SelectedDays: choice.SelectedDays, Notes: choice.Notes}
		if _, err := db.NewInsert().Model(row).
			On("CONFLICT (tenant_id, request_child_id, care_offering_id) DO NOTHING").Exec(ctx); err != nil {
			return fmt.Errorf("record submitted offering choice: %w", err)
		}
		var stored submittedOfferingRow
		if err := db.NewSelect().Model(&stored).
			Where("tenant_id = ? AND request_child_id = ? AND care_offering_id = ?", tenantID, childID, choice.CareOfferingID).Scan(ctx); err != nil {
			return fmt.Errorf("verify submitted offering retry: %w", err)
		}
		if !slices.Equal(stored.SelectedDays, choice.SelectedDays) || !sameSubmissionNotes(stored.Notes, choice.Notes) {
			return enrollment.ErrSubmittedOfferingChoiceConflict
		}
	}
	return nil
}

func sameSubmissionNotes(first, second *string) bool {
	if first == nil || second == nil {
		return first == nil && second == nil
	}
	return *first == *second
}
