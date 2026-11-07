package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewTraegerCareBookings reads on the caller's administrative transaction.
func NewTraegerCareBookings(transaction func(context.Context) (any, bool), admin func(context.Context) bool) careplan.TraegerCareBookings {
	return traegerCareBookings{transaction: transaction, admin: admin}
}

type traegerCareBookings struct {
	transaction func(context.Context) (any, bool)
	admin       func(context.Context) bool
}

// careOfferingBookingsSQL takes the day, the school set and the running
// phases. A booking is valid from valid_from (inclusive) to valid_until
// (exclusive), as CareOfferingBookingsAtDates reads it. An offering is listed
// when it holds such a booking, or when it is active and belongs to a running
// phase (#5977); then it carries no request child.
const careOfferingBookingsSQL = `SELECT offering.tenant_id, offering.id, offering.name, offering.capacity,
  COALESCE(array_agg(DISTINCT booking.request_child_id ORDER BY booking.request_child_id)
    FILTER (WHERE booking.request_child_id IS NOT NULL), '{}') AS request_child_ids
FROM enrollment.care_offerings AS offering
LEFT JOIN enrollment.care_offering_bookings AS booking
  ON booking.tenant_id = offering.tenant_id AND booking.care_offering_id = offering.id
  AND (booking.valid_from IS NULL OR booking.valid_from <= ?0::date)
  AND (booking.valid_until IS NULL OR booking.valid_until > ?0::date)
WHERE offering.tenant_id = ANY(?1) AND offering.counts_as_care
GROUP BY offering.tenant_id, offering.id, offering.name, offering.capacity
HAVING count(booking.request_child_id) > 0
  OR (offering.is_active AND offering.phase_id = ANY(?2))
ORDER BY offering.tenant_id, offering.name, offering.id`

// CareOfferingBookingsByTenant lists the care offerings of each school that
// hold a booking on the day or run in one of the given phases.
func (s traegerCareBookings) CareOfferingBookingsByTenant(ctx context.Context, schools []int64, day calendar.Date, runningPhases []int64) (map[int64][]careplan.BookedCareOffering, error) {
	result := make(map[int64][]careplan.BookedCareOffering, len(schools))
	if len(schools) == 0 {
		return result, nil
	}
	if !s.admin(ctx) {
		return nil, errors.New("care plan: care offering bookings need the administrative transaction")
	}
	db, err := sickStudents(s).database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		TenantID        int64   `bun:"tenant_id"`
		ID              int64   `bun:"id"`
		Name            string  `bun:"name"`
		Capacity        *int    `bun:"capacity"`
		RequestChildIDs []int64 `bun:"request_child_ids,array"`
	}
	if err := db.NewRaw(careOfferingBookingsSQL, day, pgdialect.Array(schools), pgdialect.Array(runningPhases)).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("care plan postgres: care offering bookings: %w", err)
	}
	for _, row := range rows {
		result[row.TenantID] = append(result[row.TenantID], careplan.BookedCareOffering{
			ID: row.ID, Name: row.Name, Capacity: row.Capacity, RequestChildIDs: row.RequestChildIDs,
		})
	}
	return result, nil
}
