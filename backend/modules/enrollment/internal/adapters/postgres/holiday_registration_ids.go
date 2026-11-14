package postgres

import (
	"context"
	"fmt"
)

// readHolidayRegistrationMailKeysSQL reads the registrations of a period, or
// one of them, with the keys of the closing day notices that name each
// (#6976). A family's notice relates to one of its registrations only; the
// sends name all of them. An erased mail has no key left to read.
const readHolidayRegistrationMailKeysSQL = `SELECT registration.id,
		COALESCE(array_agg(send.idempotency_key ORDER BY send.idempotency_key) FILTER (WHERE send.idempotency_key IS NOT NULL), '{}') AS closure_keys
	FROM enrollment.holiday_registrations AS registration
	LEFT JOIN enrollment.holiday_closure_sends AS send
		ON send.organization_id = registration.organization_id AND send.holiday_registration_id = registration.id AND NOT send.mail_erased
	WHERE registration.organization_id = ? AND registration.holiday_period_id = ? AND (? = 0 OR registration.id = ?)
	GROUP BY registration.id
	ORDER BY registration.id`

// ReadHolidayRegistrationMailKeys reads the IDs of the registrations of a
// period of the Träger, or of the one registration when registrationID is
// not 0, with the keys of their closing day notices, in the caller's Träger
// transaction and in one statement (#6737, #6976).
func (s *HolidayRegistrationStore) ReadHolidayRegistrationMailKeys(ctx context.Context, organizationID, periodID, registrationID int64) ([]int64, map[int64][]string, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, nil, err
	}
	var rows []struct {
		ID          int64    `bun:"id"`
		ClosureKeys []string `bun:"closure_keys,array"`
	}
	if err := db.NewRaw(readHolidayRegistrationMailKeysSQL, organizationID, periodID, registrationID, registrationID).Scan(ctx, &rows); err != nil {
		return nil, nil, fmt.Errorf("enrollment postgres: read holiday registration mail keys: %w", err)
	}
	ids := make([]int64, 0, len(rows))
	keys := make(map[int64][]string, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
		if len(row.ClosureKeys) > 0 {
			keys[row.ID] = row.ClosureKeys
		}
	}
	return ids, keys, nil
}
