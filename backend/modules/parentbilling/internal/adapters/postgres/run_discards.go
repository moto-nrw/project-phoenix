package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
)

// MarkCareRunDiscarded discards a calculated monthly run of the Träger and
// closes its schools, so a new run of the month may bill them (#6626). A
// run that is not calculated, or one of another Träger, is not touched:
// ErrBillingRunReleased, which the caller has already told apart from a
// discarded run under the run lock.
func (s *Store) MarkCareRunDiscarded(ctx context.Context, organizationID, runID int64) (parentbilling.CareRun, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.CareRun{}, err
	}
	row := &careRunRow{}
	if err := db.NewUpdate().Model(row).
		Set("status = ?", parentbilling.RunDiscarded).Set("discarded_at = CURRENT_TIMESTAMP").
		Where("organization_id = ?", organizationID).Where("id = ?", runID).Where("status = ?", parentbilling.RunCalculated).
		Returning("*").Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return parentbilling.CareRun{}, parentbilling.ErrBillingRunReleased
		}
		return parentbilling.CareRun{}, fmt.Errorf("discard care run: %w", err)
	}
	schools, err := closeCareRunSchools(ctx, db, organizationID, runID)
	if err != nil {
		return parentbilling.CareRun{}, err
	}
	row.SchoolIDs = schools
	return row.run(), nil
}

// MarkCareNoticeRunDiscarded discards a calculated notice run of the
// Träger; the year is free for a new run (#6626). Like
// MarkCareRunDiscarded it touches no other run.
func (s *Store) MarkCareNoticeRunDiscarded(ctx context.Context, organizationID, runID int64) (parentbilling.CareNoticeRun, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.CareNoticeRun{}, err
	}
	row := &careNoticeRunRow{}
	if err := db.NewRaw(`UPDATE parent_billing.care_notice_runs
		SET status = ?, discarded_at = CURRENT_TIMESTAMP
		WHERE organization_id = ? AND id = ? AND status = ?
		RETURNING `+careNoticeRunSelectColumnsQ, parentbilling.RunDiscarded, organizationID, runID, parentbilling.RunCalculated).
		Scan(ctx, row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return parentbilling.CareNoticeRun{}, parentbilling.ErrBillingRunReleased
		}
		return parentbilling.CareNoticeRun{}, fmt.Errorf("discard notice run: %w", err)
	}
	row.OrganizationID = organizationID
	return row.run(), nil
}

// MarkRunDiscarded discards a calculated holiday run of the Träger; its
// period leaves uq_runs_calculated_period and is free for a new run
// (#6628). Like MarkCareRunDiscarded it touches no other run.
func (s *Store) MarkRunDiscarded(ctx context.Context, organizationID, runID int64) (parentbilling.BillingRun, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return parentbilling.BillingRun{}, err
	}
	row := &runRow{}
	if err := db.NewUpdate().Model(row).
		Set("status = ?", parentbilling.RunDiscarded).Set("discarded_at = CURRENT_TIMESTAMP").
		Where("organization_id = ?", organizationID).Where("id = ?", runID).Where("status = ?", parentbilling.RunCalculated).
		Returning("*").Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return parentbilling.BillingRun{}, parentbilling.ErrBillingRunReleased
		}
		return parentbilling.BillingRun{}, fmt.Errorf("discard billing run: %w", err)
	}
	return row.run(), nil
}
