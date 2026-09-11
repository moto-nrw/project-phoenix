package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/uptrace/bun"
)

func (r *Store) PendingAnnouncementApplicants(ctx context.Context) ([]enrollment.PendingAnnouncementApplicant, error) {
	tenantID, err := r.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	return r.PendingAnnouncementApplicantsForSchools(ctx, []int64{tenantID})
}

func (r *Store) PendingAnnouncementApplicantsForSchools(ctx context.Context, schoolIDs []int64) ([]enrollment.PendingAnnouncementApplicant, error) {
	if len(schoolIDs) == 0 {
		return []enrollment.PendingAnnouncementApplicant{}, nil
	}
	db, err := r.resolve(ctx)
	if err != nil {
		return nil, err
	}
	rows := []enrollment.PendingAnnouncementApplicant{}
	err = db.NewRaw(`SELECT DISTINCT request.tenant_id, request.guardian_account_id, request.guardian_email, request.guardian_first_name, request.guardian_last_name
	 FROM enrollment.requests AS request
	 WHERE request.tenant_id IN (?) AND request.withdrawn_at IS NULL
	 AND EXISTS (SELECT 1 FROM enrollment.request_children AS child
	 WHERE child.request_id = request.id AND child.tenant_id = request.tenant_id
	 AND child.status IN ('submitted','under_review','pending_renewal','auto_renewed','pending_admin_review'))`, bun.List(schoolIDs)).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("list pending announcement applicants: %w", err)
	}
	return rows, nil
}
