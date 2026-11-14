package postgres

import (
	"context"
	"errors"

	"github.com/uptrace/bun/dialect/pgdialect"
)

// EraseHolidayRegistrationMails removes every mail that hangs at a named
// registration, whatever its kind, so a new holiday mail cannot outlive the
// registration (#7000). A mail bundled for several registrations lists them
// in registration_ids and goes when any of them goes.
func (s *Store) EraseHolidayRegistrationMails(ctx context.Context, organizationIDs, registrationIDs []int64) (int, error) {
	db, err := RequireAdminDatabase(ctx)
	if err != nil {
		return 0, err
	}
	if len(organizationIDs) != len(registrationIDs) {
		return 0, errors.New("delivery postgres: registration references must have matching lengths")
	}
	if len(registrationIDs) == 0 {
		return 0, nil
	}
	return affected("erase holiday registration mail")(db.ExecContext(ctx, `DELETE FROM platform.email_outbox AS mail
  USING unnest(?::bigint[], ?::bigint[]) AS erased(organization_id, registration_id)
  WHERE mail.tenant_id IS NULL AND mail.organization_id = erased.organization_id
   AND mail.related_entity_type = 'holiday_registration'
   AND (mail.related_entity_id = erased.registration_id
    OR mail.payload->'registration_ids' @> to_jsonb(erased.registration_id))`, pgdialect.Array(organizationIDs), pgdialect.Array(registrationIDs)))
}
