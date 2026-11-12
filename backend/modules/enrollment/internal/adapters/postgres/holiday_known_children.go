package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
)

// The children of the registrations for the hint on a known child (#6683,
// ADR 0214). The statement runs in the administrative transaction of the
// Träger projection, which reads across Träger; it filters on the Träger it
// is given, bound as the first argument, like the access trail.

var _ enrollment.HolidayRegisteredChildren = (*HolidayRegistrationStore)(nil)

// holidayRegisteredChildrenQuery takes the Träger, the period and the
// registration, 0 for every registration of the period.
const holidayRegisteredChildrenQuery = `SELECT child.holiday_registration_id, child.id, child.first_name, child.last_name, child.birth_date
	FROM enrollment.holiday_registration_children AS child
	JOIN enrollment.holiday_registrations AS registration
		ON registration.organization_id = child.organization_id AND registration.id = child.holiday_registration_id
	WHERE child.organization_id = ?0 AND child.holiday_period_id = ?1 AND (?2::bigint = 0 OR child.holiday_registration_id = ?2)
		AND registration.status <> 'cancelled'
	ORDER BY child.holiday_registration_id, child.position`

// HolidayRegisteredChildren reads the children of the registrations of the
// period that are not cancelled, in one statement.
func (s *HolidayRegistrationStore) HolidayRegisteredChildren(ctx context.Context, organizationID, periodID, registrationID int64) ([]enrollment.HolidayRegisteredChild, error) {
	if organizationID <= 0 || !tenant.IsAdminTx(ctx) {
		return nil, errors.New("enrollment postgres: the registered holiday children require a Träger and the administrative transaction")
	}
	db, err := NewDatabase()(ctx)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		RegistrationID int64         `bun:"holiday_registration_id"`
		ID             int64         `bun:"id"`
		FirstName      string        `bun:"first_name"`
		LastName       string        `bun:"last_name"`
		BirthDate      calendar.Date `bun:"birth_date,type:date"`
	}
	if err := db.NewRaw(holidayRegisteredChildrenQuery, organizationID, periodID, registrationID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: read registered holiday children: %w", err)
	}
	children := make([]enrollment.HolidayRegisteredChild, 0, len(rows))
	for _, row := range rows {
		children = append(children, enrollment.HolidayRegisteredChild{
			RegistrationID: row.RegistrationID, ChildID: row.ID, FirstName: row.FirstName, LastName: row.LastName, BirthDate: row.BirthDate,
		})
	}
	return children, nil
}
