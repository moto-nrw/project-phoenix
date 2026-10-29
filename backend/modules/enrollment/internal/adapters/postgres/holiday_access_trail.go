package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The access trail of the holiday registrations (#5326) for the access log
// of the Träger-Portal. Both statements run in the administrative
// transaction of the Träger projection, which reads across Träger; each
// filters on the Träger it is given, bound as the first argument.

var _ enrollment.HolidayAccessTrail = (*HolidayRegistrationStore)(nil)

// holidayAccessEntries is one row per entry of the audit trail with its
// kind: the actions list, read, export and proof read personal data, the
// others change it. The data is the action with a prefix, so the access log
// tells it apart from the resource types of a school.
const holidayAccessEntries = `SELECT entry.id, entry.occurred_at, entry.actor_account_id,
		CASE WHEN entry.action IN ('list', 'read', 'export', 'proof') THEN 'read' ELSE 'change' END AS kind,
		'holiday_registration_' || entry.action AS data
	FROM enrollment.holiday_registration_audit AS entry
	WHERE entry.organization_id = ?0 AND entry.occurred_at >= ?1 AND entry.occurred_at < ?2`

// holidayAccessQuery lists the entries of a filter, newest first, in the
// order of the Audit Platform's access trail; the optional filters are off
// at 0 or the empty kind.
const holidayAccessQuery = `SELECT trail.id, trail.occurred_at, trail.actor_account_id, trail.kind, trail.data
	FROM (` + holidayAccessEntries + `) AS trail
	WHERE (?3 = 0 OR trail.actor_account_id = ?3) AND (?4 = '' OR trail.kind = ?4)
	ORDER BY trail.occurred_at DESC, trail.kind, trail.data, trail.id DESC
	LIMIT ?5`

const holidayAccessActorsQuery = `SELECT DISTINCT entry.actor_account_id
	FROM enrollment.holiday_registration_audit AS entry
	WHERE entry.organization_id = ?0 AND entry.occurred_at >= ?1 AND entry.occurred_at < ?2
		AND entry.actor_account_id IS NOT NULL
	ORDER BY entry.actor_account_id`

// holidayAccessDB returns the administrative transaction for a read of the
// trail of one Träger.
func holidayAccessDB(ctx context.Context, organizationID int64) (bun.IDB, error) {
	if organizationID <= 0 || !tenant.IsAdminTx(ctx) {
		return nil, errors.New("enrollment postgres: the holiday access trail requires a Träger and the administrative transaction")
	}
	return NewDatabase()(ctx)
}

// ListHolidayAccess lists the entries of the filter, newest first.
func (s *HolidayRegistrationStore) ListHolidayAccess(ctx context.Context, organizationID int64, filter enrollment.HolidayAccessFilter) ([]enrollment.HolidayAccessEntry, error) {
	result := []enrollment.HolidayAccessEntry{}
	if filter.Limit <= 0 {
		return result, nil
	}
	db, err := holidayAccessDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID             int64     `bun:"id"`
		OccurredAt     time.Time `bun:"occurred_at"`
		ActorAccountID *int64    `bun:"actor_account_id"`
		Kind           string    `bun:"kind"`
		Data           string    `bun:"data"`
	}
	if err := db.NewRaw(holidayAccessQuery, organizationID, filter.From, filter.To, filter.AccountID, filter.Kind, filter.Limit).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("enrollment postgres: list holiday access trail: %w", err)
	}
	for _, row := range rows {
		result = append(result, enrollment.HolidayAccessEntry{
			ID: row.ID, OccurredAt: row.OccurredAt, AccountID: row.ActorAccountID, Kind: row.Kind, Data: row.Data,
		})
	}
	return result, nil
}

// ListHolidayAccessActors lists the accounts with an entry in the window.
func (s *HolidayRegistrationStore) ListHolidayAccessActors(ctx context.Context, organizationID int64, from, to time.Time) ([]int64, error) {
	db, err := holidayAccessDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	accounts := []int64{}
	if err := db.NewRaw(holidayAccessActorsQuery, organizationID, from, to).Scan(ctx, &accounts); err != nil {
		return nil, fmt.Errorf("enrollment postgres: list holiday access actors: %w", err)
	}
	return accounts, nil
}
