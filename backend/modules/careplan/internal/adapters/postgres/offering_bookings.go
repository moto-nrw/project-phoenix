package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/uptrace/bun"
)

type careOfferingBookingRow struct {
	bun.BaseModel         `bun:"table:enrollment.care_offering_bookings,alias:care_booking"`
	ID                    int64         `bun:"id,pk,autoincrement"`
	TenantID              int64         `bun:"tenant_id"`
	RequestChildID        int64         `bun:"request_child_id"`
	CareOfferingID        int64         `bun:"care_offering_id"`
	ManualSelectedDays    []string      `bun:"manual_selected_days,type:jsonb,nullzero"`
	AutomaticSelectedDays []string      `bun:"automatic_selected_days,type:jsonb,nullzero"`
	ValidFrom             *calendarDate `bun:"valid_from,type:date"`
	ValidUntil            *calendarDate `bun:"valid_until,type:date"`
	CreatedAt             time.Time     `bun:"created_at,nullzero,default:current_timestamp"`
	UpdatedAt             time.Time     `bun:"updated_at,nullzero,default:current_timestamp"`
}

func bookingDateToStorage(date *careplan.Date) *calendarDate {
	if date == nil {
		return nil
	}
	return new(calendarDate(*date))
}

func bookingDateToPublic(date *calendarDate) *careplan.Date {
	if date == nil {
		return nil
	}
	return new(careplan.Date(*date))
}

func bookingValues(rows []careOfferingBookingRow) []careplan.CareOfferingBooking {
	values := make([]careplan.CareOfferingBooking, 0, len(rows))
	for _, row := range rows {
		values = append(values, careplan.CareOfferingBooking{
			ID: row.ID, TenantID: row.TenantID, RequestChildID: row.RequestChildID, CareOfferingID: row.CareOfferingID,
			ManualSelectedDays: row.ManualSelectedDays, AutomaticSelectedDays: row.AutomaticSelectedDays,
			ValidFrom: bookingDateToPublic(row.ValidFrom), ValidUntil: bookingDateToPublic(row.ValidUntil), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	return values
}

func (s *Store) CareOfferingBookingsForPhase(ctx context.Context, phaseID int64) ([]careplan.CareOfferingBooking, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []careOfferingBookingRow
	err = db.NewSelect().Model(&rows).
		Join("JOIN enrollment.care_offerings AS offering ON offering.id = care_booking.care_offering_id AND offering.tenant_id = care_booking.tenant_id").
		Where("care_booking.tenant_id = ? AND offering.phase_id = ?", tenantID, phaseID).
		OrderExpr("care_booking.id").Scan(ctx)
	return bookingValues(rows), err
}

func (s *Store) CareOfferingBookingsForOfferings(ctx context.Context, offeringIDs []int64) ([]careplan.CareOfferingBooking, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []careOfferingBookingRow
	err = db.NewSelect().Model(&rows).
		Where("care_booking.tenant_id = ? AND care_booking.care_offering_id IN (?)", tenantID, bun.List(offeringIDs)).
		OrderExpr("care_booking.id").Scan(ctx)
	return bookingValues(rows), err
}

func (s *Store) CountCareOfferingBookings(ctx context.Context, childIDs []int64) (int, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, err
	}
	return db.NewSelect().Model((*careOfferingBookingRow)(nil)).
		Where("care_booking.tenant_id = ? AND care_booking.request_child_id IN (?)", tenantID, bun.List(childIDs)).Count(ctx)
}

func (s *Store) CareOfferingBookingHistory(ctx context.Context, childIDs []int64) ([]careplan.CareOfferingBooking, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []careOfferingBookingRow
	err = db.NewSelect().Model(&rows).
		Where("care_booking.tenant_id = ? AND care_booking.request_child_id IN (?)", tenantID, bun.List(childIDs)).
		OrderExpr("care_booking.request_child_id, care_booking.id").Scan(ctx)
	return bookingValues(rows), err
}

func (s *Store) CareOfferingBookingsAtDates(ctx context.Context, dates map[int64]careplan.Date) ([]careplan.CareOfferingBooking, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	childIDs := make([]int64, 0, len(dates))
	for childID := range dates {
		childIDs = append(childIDs, childID)
	}
	slices.Sort(childIDs)
	var rows []careOfferingBookingRow
	query := db.NewSelect().Model(&rows).Where("care_booking.tenant_id = ?", tenantID)
	query = query.WhereGroup(" AND ", func(query *bun.SelectQuery) *bun.SelectQuery {
		for _, childID := range childIDs {
			query = query.WhereOr("(care_booking.request_child_id = ? AND (care_booking.valid_from IS NULL OR care_booking.valid_from <= ?) AND (care_booking.valid_until IS NULL OR care_booking.valid_until > ?))", childID, calendarDate(dates[childID]), calendarDate(dates[childID]))
		}
		return query
	})
	err = query.OrderExpr("care_booking.request_child_id, care_booking.id").Scan(ctx)
	return bookingValues(rows), err
}

func (s *Store) RecordCareOfferingBookings(ctx context.Context, childID int64, bookings []careplan.CareOfferingBooking) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("care-offering-bookings:%d:%d", tenantID, childID)
	if _, err := db.NewRaw(`SELECT pg_advisory_xact_lock(hashtextextended(?, 0))`, key).Exec(ctx); err != nil {
		return err
	}
	for _, booking := range bookings {
		if booking.TenantID != 0 && booking.TenantID != tenantID {
			return fmt.Errorf("care offering booking tenant mismatch")
		}
		row := &careOfferingBookingRow{TenantID: tenantID, RequestChildID: childID, CareOfferingID: booking.CareOfferingID,
			ManualSelectedDays: booking.ManualSelectedDays, AutomaticSelectedDays: booking.AutomaticSelectedDays,
			ValidFrom: bookingDateToStorage(booking.ValidFrom), ValidUntil: bookingDateToStorage(booking.ValidUntil)}
		var existing careOfferingBookingRow
		err := db.NewSelect().Model(&existing).
			Where("tenant_id = ? AND request_child_id = ? AND care_offering_id = ?", tenantID, childID, row.CareOfferingID).
			Where("(valid_from = ?::date OR (valid_from IS NULL AND ?::date IS NULL))", row.ValidFrom, row.ValidFrom).
			Where("(valid_until = ?::date OR (valid_until IS NULL AND ?::date IS NULL))", row.ValidUntil, row.ValidUntil).Scan(ctx)
		if err == nil {
			if !slices.Equal(existing.ManualSelectedDays, row.ManualSelectedDays) || !slices.Equal(existing.AutomaticSelectedDays, row.AutomaticSelectedDays) {
				return careplan.ErrCareOfferingBookingConflict
			}
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err := db.NewInsert().Model(row).Exec(ctx); err != nil {
			return fmt.Errorf("record effective care booking: %w", err)
		}
	}
	return nil
}

func (s *Store) ScheduleCareOfferingBookings(ctx context.Context, childID int64, effectiveFrom careplan.Date, bookings []careplan.CareOfferingBooking) error {
	return s.replaceCareOfferingBookings(ctx, childID, &effectiveFrom, bookings)
}

func (s *Store) ReplaceCareOfferingBookings(ctx context.Context, childID int64, bookings []careplan.CareOfferingBooking) error {
	return s.replaceCareOfferingBookings(ctx, childID, nil, bookings)
}

func (s *Store) replaceCareOfferingBookings(ctx context.Context, childID int64, effectiveFrom *careplan.Date, bookings []careplan.CareOfferingBooking) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("care-offering-bookings:%d:%d", tenantID, childID)
	if _, err := db.NewRaw(`SELECT pg_advisory_xact_lock(hashtextextended(?, 0))`, key).Exec(ctx); err != nil {
		return err
	}
	var future []careOfferingBookingRow
	futureQuery := db.NewSelect().Model(&future).Where("tenant_id = ? AND request_child_id = ?", tenantID, childID)
	if effectiveFrom != nil {
		futureQuery = futureQuery.Where("valid_from >= ?", calendarDate(*effectiveFrom))
	}
	if err := futureQuery.Scan(ctx); err != nil {
		return err
	}
	var retained []int64
	for _, row := range future {
		for _, booking := range bookings {
			if row.CareOfferingID == booking.CareOfferingID && sameBookingDate(row.ValidFrom, booking.ValidFrom) &&
				sameBookingDate(row.ValidUntil, booking.ValidUntil) && slices.Equal(row.ManualSelectedDays, booking.ManualSelectedDays) && slices.Equal(row.AutomaticSelectedDays, booking.AutomaticSelectedDays) {
				retained = append(retained, row.ID)
			}
		}
	}
	query := db.NewDelete().Model((*careOfferingBookingRow)(nil)).
		Where("tenant_id = ? AND request_child_id = ?", tenantID, childID)
	if effectiveFrom != nil {
		query = query.Where("valid_from >= ?", calendarDate(*effectiveFrom))
	}
	if len(retained) > 0 {
		query = query.Where("id NOT IN (?)", bun.List(retained))
	}
	if _, err := query.Exec(ctx); err != nil {
		return fmt.Errorf("delete superseded care bookings: %w", err)
	}
	if effectiveFrom != nil {
		if _, err := db.NewUpdate().Model((*careOfferingBookingRow)(nil)).
			Set("valid_until = ?", calendarDate(*effectiveFrom)).Set("updated_at = CURRENT_TIMESTAMP").
			Where("tenant_id = ? AND request_child_id = ?", tenantID, childID).
			Where("valid_from IS NULL OR valid_from < ?", calendarDate(*effectiveFrom)).
			Where("valid_until IS NULL OR valid_until > ?", calendarDate(*effectiveFrom)).Exec(ctx); err != nil {
			return fmt.Errorf("close current care bookings: %w", err)
		}
	}
	return s.RecordCareOfferingBookings(ctx, childID, bookings)
}

func sameBookingDate(stored *calendarDate, requested *careplan.Date) bool {
	if stored == nil || requested == nil {
		return stored == nil && requested == nil
	}
	return careplan.Date(*stored) == *requested
}
