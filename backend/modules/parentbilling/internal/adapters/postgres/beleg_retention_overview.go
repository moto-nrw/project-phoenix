package postgres

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// belegDeletionListLimit bounds the deletion log the overview shows.
const belegDeletionListLimit = 50

// BelegRetentionOverview reads the counts per type and state, the
// clarification list grouped by type and reason, the latest deletions and
// the day of the last evaluation of one Träger. It reads no content of a
// Beleg.
func (s *Store) BelegRetentionOverview(ctx context.Context, organizationID int64, today calendar.Date) (parentbilling.BelegRetentionOverview, error) {
	db, err := adminDB(ctx)
	if err != nil {
		return parentbilling.BelegRetentionOverview{}, err
	}
	overview := parentbilling.BelegRetentionOverview{}
	if overview.Summary, err = belegSummary(ctx, db, organizationID); err != nil {
		return overview, err
	}
	if overview.Clarifications, err = belegClarifications(ctx, db, organizationID, today); err != nil {
		return overview, err
	}
	if overview.Deletions, err = belegDeletions(ctx, db, organizationID); err != nil {
		return overview, err
	}
	var evaluated []struct {
		Day calendar.Date `bun:"day,type:date"`
	}
	if err := db.NewRaw(`SELECT max(evaluated_on) AS day FROM parent_billing.beleg_retentions WHERE organization_id = ? HAVING count(*) > 0`,
		organizationID).Scan(ctx, &evaluated); err != nil {
		return overview, fmt.Errorf("parent billing postgres: beleg evaluation day: %w", err)
	}
	if len(evaluated) == 1 {
		overview.EvaluatedOn = &evaluated[0].Day
	}
	return overview, nil
}

func belegSummary(ctx context.Context, db bun.IDB, organizationID int64) ([]parentbilling.BelegRetentionSummary, error) {
	var rows []struct {
		BelegType     string         `bun:"beleg_type"`
		Scheduled     int            `bun:"scheduled"`
		Running       int            `bun:"running"`
		Clarification int            `bun:"clarification"`
		Suspended     int            `bun:"suspended"`
		NextEnd       *calendar.Date `bun:"next_end,type:date"`
	}
	if err := db.NewRaw(`SELECT beleg_type,
			count(*) FILTER (WHERE state = 'scheduled')::int AS scheduled,
			count(*) FILTER (WHERE state = 'running')::int AS running,
			count(*) FILTER (WHERE state = 'clarification')::int AS clarification,
			count(*) FILTER (WHERE state = 'suspended')::int AS suspended,
			min(deletable_on) FILTER (WHERE state = 'scheduled') AS next_end
		FROM parent_billing.beleg_retentions WHERE organization_id = ? GROUP BY beleg_type ORDER BY beleg_type`, organizationID).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent billing postgres: beleg retention summary: %w", err)
	}
	summary := make([]parentbilling.BelegRetentionSummary, len(rows))
	for i, row := range rows {
		summary[i] = parentbilling.BelegRetentionSummary(row)
	}
	return summary, nil
}

func belegClarifications(ctx context.Context, db bun.IDB, organizationID int64, today calendar.Date) ([]parentbilling.BelegClarification, error) {
	var rows []struct {
		BelegType   string        `bun:"beleg_type"`
		Reason      string        `bun:"reason"`
		Count       int           `bun:"count"`
		Overdue     int           `bun:"overdue"`
		ListedOn    calendar.Date `bun:"listed_on,type:date"`
		ReviewOn    calendar.Date `bun:"review_on,type:date"`
		Responsible string        `bun:"responsible"`
	}
	if err := db.NewRaw(`SELECT beleg_type, reason, count(*)::int AS count,
			count(*) FILTER (WHERE review_on < ?0::date)::int AS overdue,
			min(listed_on) AS listed_on, min(review_on) AS review_on, min(responsible) AS responsible
		FROM parent_billing.beleg_retentions WHERE organization_id = ?1 AND state = 'clarification'
		GROUP BY beleg_type, reason ORDER BY min(review_on), beleg_type, reason`, today, organizationID).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent billing postgres: beleg clarification list: %w", err)
	}
	clarifications := make([]parentbilling.BelegClarification, len(rows))
	for i, row := range rows {
		clarifications[i] = parentbilling.BelegClarification(row)
	}
	return clarifications, nil
}

func belegDeletions(ctx context.Context, db bun.IDB, organizationID int64) ([]parentbilling.BelegDeletion, error) {
	var rows []struct {
		BelegType string        `bun:"beleg_type"`
		DeletedOn calendar.Date `bun:"deleted_on,type:date"`
		Count     int           `bun:"deleted_count"`
	}
	if err := db.NewRaw(`SELECT beleg_type, deleted_on, sum(deleted_count)::int AS deleted_count
		FROM parent_billing.beleg_deletions WHERE organization_id = ? GROUP BY beleg_type, deleted_on
		ORDER BY deleted_on DESC, beleg_type LIMIT ?`, organizationID, belegDeletionListLimit).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("parent billing postgres: beleg deletion log: %w", err)
	}
	deletions := make([]parentbilling.BelegDeletion, len(rows))
	for i, row := range rows {
		deletions[i] = parentbilling.BelegDeletion(row)
	}
	return deletions, nil
}
