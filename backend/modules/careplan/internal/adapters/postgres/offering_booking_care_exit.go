package postgres

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/uptrace/bun"
)

func (s *Store) LockCareOfferingBookings(ctx context.Context, childIDs []int64) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	ids := slices.Clone(childIDs)
	slices.Sort(ids)
	for _, id := range slices.Compact(ids) {
		key := fmt.Sprintf("care-offering-bookings:%d:%d", tenantID, id)
		if _, err := db.NewRaw(`SELECT pg_advisory_xact_lock(hashtextextended(?, 0))`, key).Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) EndCareOfferingBookings(ctx context.Context, childIDs []int64, until careplan.Date) (int64, error) {
	if len(childIDs) == 0 {
		return 0, nil
	}
	if err := s.LockCareOfferingBookings(ctx, childIDs); err != nil {
		return 0, err
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, err
	}
	date := calendarDate(until)
	deleted, err := db.NewDelete().Model((*careOfferingBookingRow)(nil)).
		Where("tenant_id = ? AND request_child_id IN (?)", tenantID, bun.List(childIDs)).
		Where("valid_from >= ? AND (valid_until IS NULL OR valid_until > ?)", date, date).Exec(ctx)
	if err != nil {
		return 0, err
	}
	deletedRows, err := deleted.RowsAffected()
	if err != nil {
		return 0, err
	}
	capped, err := db.NewUpdate().Model((*careOfferingBookingRow)(nil)).
		Set("valid_until = ?", date).Set("updated_at = NOW()").
		Where("tenant_id = ? AND request_child_id IN (?)", tenantID, bun.List(childIDs)).
		Where("(valid_from IS NULL OR valid_from < ?) AND (valid_until IS NULL OR valid_until > ?)", date, date).Exec(ctx)
	if err != nil {
		return 0, err
	}
	cappedRows, err := capped.RowsAffected()
	return deletedRows + cappedRows, err
}

func (s *Store) RestoreCareOfferingBookings(ctx context.Context, restores []careplan.CareOfferingBookingRestore) (int64, error) {
	ids := make([]int64, 0, len(restores))
	for _, restore := range restores {
		ids = append(ids, restore.Booking.RequestChildID)
	}
	if err := s.LockCareOfferingBookings(ctx, ids); err != nil {
		return 0, err
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, err
	}
	var inserted int64
	for _, restore := range restores {
		booking := restore.Booking
		if booking.TenantID != tenantID {
			continue
		}
		if !restore.WasDeleted {
			_, err := db.NewUpdate().Model((*careOfferingBookingRow)(nil)).
				Set("valid_until = ?", bookingDateToStorage(booking.ValidUntil)).Set("updated_at = NOW()").
				Where("tenant_id = ? AND id = ? AND request_child_id = ? AND care_offering_id = ?", tenantID, booking.ID, booking.RequestChildID, booking.CareOfferingID).Exec(ctx)
			if err != nil {
				return 0, err
			}
			continue
		}
		row := &careOfferingBookingRow{ID: booking.ID, TenantID: tenantID, RequestChildID: booking.RequestChildID, CareOfferingID: booking.CareOfferingID,
			ManualSelectedDays: booking.ManualSelectedDays, AutomaticSelectedDays: booking.AutomaticSelectedDays,
			ValidFrom: bookingDateToStorage(booking.ValidFrom), ValidUntil: bookingDateToStorage(booking.ValidUntil), CreatedAt: booking.CreatedAt, UpdatedAt: booking.UpdatedAt}
		result, err := db.NewInsert().Model(row).On("CONFLICT DO NOTHING").Exec(ctx)
		if err != nil {
			return 0, err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return 0, err
		}
		inserted += count
	}
	return inserted, nil
}
