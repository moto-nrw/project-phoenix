package postgres

import (
	"context"
	"fmt"
)

// ReadHolidayRegistrationIDs reads the IDs of the registrations of a period
// of the Träger in the caller's Träger transaction, in one statement (#6737).
func (s *HolidayRegistrationStore) ReadHolidayRegistrationIDs(ctx context.Context, organizationID, periodID int64) ([]int64, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	ids := []int64{}
	if err := db.NewRaw(`SELECT id FROM enrollment.holiday_registrations
		WHERE organization_id = ? AND holiday_period_id = ? ORDER BY id`, organizationID, periodID).Scan(ctx, &ids); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read holiday registration IDs: %w", err)
	}
	return ids, nil
}
