package postgres

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

type requestChildOfferingRow struct {
	bun.BaseModel         `bun:"table:enrollment.request_child_offerings,alias:request_child_offering"`
	ID                    int64     `bun:"id,pk,autoincrement"`
	TenantID              int64     `bun:"tenant_id"`
	CreatedAt             time.Time `bun:"created_at"`
	UpdatedAt             time.Time `bun:"updated_at"`
	RequestChildID        int64     `bun:"request_child_id"`
	CareOfferingID        int64     `bun:"care_offering_id"`
	SelectedDays          []string  `bun:"selected_days,type:jsonb,nullzero"`
	ManualSelectedDays    []string  `bun:"manual_selected_days,type:jsonb,nullzero"`
	AutomaticSelectedDays []string  `bun:"automatic_selected_days,type:jsonb,nullzero"`
	Notes                 *string   `bun:"notes"`
	// ValidFrom / ValidUntil make an approved offering switch effective on its
	// requested date. ValidUntil is exclusive, matching student enrollments.
	ValidFrom  *enrollment.Date `bun:"valid_from,type:date"`
	ValidUntil *enrollment.Date `bun:"valid_until,type:date"`
}

func (r requestChildOfferingRow) value() *enrollment.RequestChildOffering {
	return &enrollment.RequestChildOffering{ID: r.ID, TenantID: r.TenantID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, RequestChildID: r.RequestChildID, CareOfferingID: r.CareOfferingID, SelectedDays: r.SelectedDays, ManualSelectedDays: r.ManualSelectedDays, AutomaticSelectedDays: r.AutomaticSelectedDays, Notes: r.Notes, ValidFrom: r.ValidFrom, ValidUntil: r.ValidUntil}
}
func requestChildOfferingStorage(r *enrollment.RequestChildOffering) *requestChildOfferingRow {
	return &requestChildOfferingRow{ID: r.ID, TenantID: r.TenantID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, RequestChildID: r.RequestChildID, CareOfferingID: r.CareOfferingID, SelectedDays: r.SelectedDays, ManualSelectedDays: r.ManualSelectedDays, AutomaticSelectedDays: r.AutomaticSelectedDays, Notes: r.Notes, ValidFrom: r.ValidFrom, ValidUntil: r.ValidUntil}
}
